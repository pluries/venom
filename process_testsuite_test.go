package venom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// suiteFakeExecutor is the builtin executor "fake" of the #2031 suite-level characterisation tests (the real executors
// live in package executors, which imports venom). A step's "error" makes Run fail with that message; otherwise the
// step's "code" comes back as result.code. The result is a struct named Result because go-dump prefixes a struct's
// keys with its type name: that is what makes "result.code" resolve, where a map result would dump as "code".
type suiteFakeExecutor struct{}

func (suiteFakeExecutor) Run(_ context.Context, step TestStep) (interface{}, error) {
	if msg, ok := step["error"].(string); ok {
		return nil, errors.New(msg)
	}
	type Result struct {
		Code interface{} `json:"code"`
	}
	return Result{Code: step["code"]}, nil
}

// suiteNewVenom returns a Venom with "fake" registered and its console captured. slug.Lowercase is set to false, the
// value InitLogger gives it in a real run, so test case names are slugged as venom slugs them; it is restored after.
func suiteNewVenom(t *testing.T, verbose int) (*Venom, *capture) {
	t.Helper()
	InitTestLogger(t)
	previous := slug.Lowercase
	slug.Lowercase = false
	t.Cleanup(func() { slug.Lowercase = previous })

	v := New()
	v.Verbose = verbose
	v.RegisterExecutorBuiltin("fake", suiteFakeExecutor{})
	c := &capture{}
	v.PrintFunc = c.printf
	return v, c
}

// suiteParsed writes content as a suite file, runs Parse on it and returns the parsed suite and the file path. The
// suite is exactly what Process would hand to runTestSuite.
func suiteParsed(t *testing.T, v *Venom, content string) (*TestSuite, string) {
	t.Helper()
	p := writeSuiteFile(t, t.TempDir(), "suite.yml", content)
	require.NoError(t, v.Parse(context.Background(), []string{p}))
	require.Len(t, v.Tests.TestSuites, 1)
	return &v.Tests.TestSuites[0], p
}

// suiteForRunTestCases parses content and prepares the suite the way runTestSuite does before it calls runTestCases:
// the only field runTestCases needs from it is a non-nil ComputedVars.
func suiteForRunTestCases(t *testing.T, v *Venom, content string) (*TestSuite, string) {
	t.Helper()
	ts, p := suiteParsed(t, v, content)
	ts.ComputedVars = H{}
	return ts, p
}

// TestRunTestSuiteComputesTheSuiteVariables pins the variables runTestSuite leaves on the suite: the run's variables
// win over the suite's, every value is interpolated once against the dump taken before the pass (so a variable that
// references a variable that references another keeps one level of templating), every value becomes a string, the
// dump's own __Len__ and __Type__ keys become suite variables, and the venom.* variables are added.
func TestRunTestSuiteComputesTheSuiteVariables(t *testing.T) {
	v, c := suiteNewVenom(t, 0)
	v.OutputDir = "some-output-dir"
	v.LibDir = "lib-a" + string(os.PathListSeparator) + "lib-b"
	v.AddVariables(map[string]interface{}{"a": "from-the-run", "r": "run-only"})
	ts := &TestSuite{
		Name: "Hand Built",
		Vars: H{"a": "from-the-suite", "b": "{{.a}}-y", "c": "{{.b}}", "n": 1},
	}

	require.NoError(t, v.runTestSuite(context.Background(), ts))

	exe, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, H{
		"a":                          "from-the-run",
		"b":                          "from-the-run-y",
		"c":                          "{{.a}}-y",
		"n":                          "1",
		"r":                          "run-only",
		"__Len__":                    "5",
		"__Type__":                   "Map",
		"venom.executable":           exe,
		"venom.outputdir":            "some-output-dir",
		"venom.libdir":               "lib-a" + string(os.PathListSeparator) + "lib-b",
		"venom.testsuite":            "Hand Built",
		"venom.testsuite.totalSteps": 0,
	}, ts.Vars)
	assert.Equal(t, H{}, ts.ComputedVars)
	assert.Equal(t, H{"a": "from-the-run", "r": "run-only"}, v.variables, "the run's variables are cloned, not changed")
	assert.Equal(t, " • Hand Built ()\n", c.String())
}

// TestRunTestSuiteVariableInterpolationError pins the error of a suite variable that is not a valid template: returned
// before any test case runs, with the variable's name and quoted value.
func TestRunTestSuiteVariableInterpolationError(t *testing.T) {
	v, c := suiteNewVenom(t, 0)
	ts := &TestSuite{Name: "broken", Vars: H{"z": "{{ .x"}}

	err := v.runTestSuite(context.Background(), ts)

	require.Error(t, err)
	assert.Equal(t, `error while computing variable z="{{ .x": invalid template format "{{ .x": template: input:1: unclosed action`, err.Error())
	assert.Equal(t, Status(""), ts.Status, "the suite never started")
	assert.Nil(t, ts.ComputedVars)
	assert.Empty(t, c.String(), "nothing is printed, not even the suite header")
}

// TestRunTestSuiteStatusAndTallies pins how runTestSuite derives the suite status and the per-status test case counts
// from its test cases, and the venom.testsuite.totalSteps variable, which counts the steps of every test case,
// skipped ones included.
func TestRunTestSuiteStatusAndTallies(t *testing.T) {
	const okCase = "- name: p%d\n  steps:\n  - type: fake\n    code: 0\n"
	const failedCase = "- name: f%d\n  steps:\n  - type: fake\n    error: boom\n"
	const skippedCase = "- name: s%d\n  skip:\n  - foo ShouldBeEmpty\n  steps:\n  - type: fake\n  - type: fake\n"
	const emptyCase = "- name: e%d\n  steps: []\n"
	tests := []struct {
		name                         string
		testcases                    []string
		wantStatus                   Status
		wantPass, wantFail, wantSkip int
		wantTotalSteps               int
	}{
		{name: "every test case passes", testcases: []string{okCase, okCase}, wantStatus: StatusPass, wantPass: 2, wantTotalSteps: 2},
		{name: "one test case fails", testcases: []string{okCase, failedCase, okCase}, wantStatus: StatusFail, wantPass: 2, wantFail: 1, wantTotalSteps: 3},
		{name: "a failure wins over skipped test cases", testcases: []string{skippedCase, failedCase}, wantStatus: StatusFail, wantFail: 1, wantSkip: 1, wantTotalSteps: 3},
		{name: "every test case is skipped", testcases: []string{skippedCase, skippedCase}, wantStatus: StatusSkip, wantSkip: 2, wantTotalSteps: 4},
		{name: "some test cases are skipped", testcases: []string{skippedCase, okCase}, wantStatus: StatusPass, wantPass: 1, wantSkip: 1, wantTotalSteps: 3},
		// quirk: a test case without steps is vacuously "all steps skipped", so a suite made of them is SKIP
		{name: "test cases without steps count as skipped", testcases: []string{emptyCase}, wantStatus: StatusSkip, wantSkip: 1},
		{name: "no test case at all passes", wantStatus: StatusPass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := suiteNewVenom(t, 0)
			content := "name: tallies\nvars:\n  foo: bar\ntestcases:\n"
			for i, tc := range tt.testcases {
				content += fmt.Sprintf(tc, i)
			}
			ts, _ := suiteParsed(t, v, content)

			require.NoError(t, v.runTestSuite(context.Background(), ts))

			assert.Equal(t, tt.wantStatus, ts.Status)
			assert.Equal(t, tt.wantPass, ts.NbTestcasesPass, "pass")
			assert.Equal(t, tt.wantFail, ts.NbTestcasesFail, "fail")
			assert.Equal(t, tt.wantSkip, ts.NbTestcasesSkip, "skip")
			assert.Equal(t, tt.wantTotalSteps, ts.Vars["venom.testsuite.totalSteps"])
		})
	}
}

// TestRunTestSuiteProfilingCreateError pins the -vvv path when a profile file cannot be created: one error naming both
// files (the one that did open reads <nil>), returned before anything runs, and no profile file left behind.
func TestRunTestSuiteProfilingCreateError(t *testing.T) {
	const cpu, mem = "pprof_cpu_profile_f.yml.prof", "pprof_mem_profile_f.yml.prof"
	tests := []struct {
		name    string
		prepare func(t *testing.T, dir string) string // returns the output dir
		wantErr func(out string) string
	}{
		{
			name:    "the output dir does not exist",
			prepare: func(_ *testing.T, dir string) string { return filepath.Join(dir, "missing") },
			wantErr: func(out string) string {
				return fmt.Sprintf("error while create profile file CPU:open %s: no such file or directory MEM:open %s: no such file or directory",
					filepath.Join(out, cpu), filepath.Join(out, mem))
			},
		},
		{
			name: "only the memory profile cannot be created",
			prepare: func(t *testing.T, dir string) string {
				require.NoError(t, os.Mkdir(filepath.Join(dir, mem), 0o755))
				return dir
			},
			wantErr: func(out string) string {
				return fmt.Sprintf("error while create profile file CPU:<nil> MEM:open %s: is a directory", filepath.Join(out, mem))
			},
		},
		{
			name: "only the cpu profile cannot be created",
			prepare: func(t *testing.T, dir string) string {
				require.NoError(t, os.Mkdir(filepath.Join(dir, cpu), 0o755))
				return dir
			},
			wantErr: func(out string) string {
				return fmt.Sprintf("error while create profile file CPU:open %s: is a directory MEM:<nil>", filepath.Join(out, cpu))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c := suiteNewVenom(t, 3)
			v.OutputDir = tt.prepare(t, t.TempDir())
			ts := &TestSuite{Name: "profiled", Filename: "f.yml"}

			err := v.runTestSuite(context.Background(), ts)

			require.Error(t, err)
			assert.Equal(t, tt.wantErr(v.OutputDir), err.Error())
			for _, name := range []string{cpu, mem} {
				info, statErr := os.Stat(filepath.Join(v.OutputDir, name))
				if statErr == nil {
					assert.True(t, info.IsDir(), "%s was left behind as a file", name)
				}
			}
			assert.Equal(t, Status(""), ts.Status)
			assert.Empty(t, c.String())
		})
	}
}

// TestRunTestSuiteProfilingWritesBothFiles pins the -vvv success path through runTestSuite itself: one CPU and one
// memory profile per test file, both readable. The CPU profiler is process-wide, so the test skips rather than flakes
// when something else in the process holds it.
func TestRunTestSuiteProfilingWritesBothFiles(t *testing.T) {
	if err := pprof.StartCPUProfile(io.Discard); err != nil {
		t.Skipf("the process-wide CPU profiler is held elsewhere, so runTestSuite could not start it: %v", err)
	}
	pprof.StopCPUProfile()

	v, _ := suiteNewVenom(t, 3)
	v.OutputDir = t.TempDir()
	ts := &TestSuite{Name: "profiled", Filename: "ok.yml"}

	require.NoError(t, v.runTestSuite(context.Background(), ts))

	profileIsReadable(t, filepath.Join(v.OutputDir, "pprof_cpu_profile_ok.yml.prof"))
	profileIsReadable(t, filepath.Join(v.OutputDir, "pprof_mem_profile_ok.yml.prof"))
	assert.Equal(t, StatusPass, ts.Status)
}

// suiteOutputYAML is the suite the console-output tests run. Line numbers matter: they appear in the printed failure
// and info lines.
const suiteOutputYAML = `name: Output Suite
vars:
  foo: bar
testcases:
- name: Passing Case
  steps:
  - type: fake
    code: 0
- name: Failing Case
  steps:
  - type: fake
    code: 0
    info: "code is {{.result.code}}"
  - type: fake
    code: 1
    assertions:
    - result.code ShouldEqual 0
- name: Ranged Case
  steps:
  - type: fake
    code: 0
    range: [a, b]
- name: Skipped Case
  skip:
  - foo ShouldBeEmpty
  steps:
  - type: fake
- name: Step Skipped Case
  steps:
  - type: fake
    skip:
    - foo ShouldBeEmpty
- name: No Step Case
  steps: []
`

// suiteOutputInfo and suiteOutputFailure are the two detail lines of "Failing Case" in suiteOutputYAML. Each carries
// its own source line: the info of step 1 points at line 11, step 1; the assertion of step 2 at line 17 (#1967).
func suiteOutputInfo(p string) string { return fmt.Sprintf("code is 0 (%s:11)", p) }

func suiteOutputFailure(p string) string {
	return fmt.Sprintf(`Testcase "Failing Case", step #2-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (%s:17)`, p)
}

// suiteOutputStatuses is the status of each test case of suiteOutputYAML, by slugged name.
var suiteOutputStatuses = map[string]Status{
	"Passing-Case":      StatusPass,
	"Failing-Case":      StatusFail,
	"Ranged-Case":       StatusPass,
	"Skipped-Case":      StatusSkip,
	"Step-Skipped-Case": StatusSkip,
	"No-Step-Case":      StatusSkip,
}

// suiteAssertOutputStatuses checks every test case was evaluated and got its status from suiteOutputStatuses.
func suiteAssertOutputStatuses(t *testing.T, ts *TestSuite) {
	t.Helper()
	require.Len(t, ts.TestCases, len(suiteOutputStatuses))
	for i := range ts.TestCases {
		tc := &ts.TestCases[i]
		assert.True(t, tc.IsEvaluated, tc.Name)
		assert.Equal(t, suiteOutputStatuses[tc.Name], tc.Status, tc.Name)
	}
}

// TestRunTestCasesPrintsNonVerbose pins the console output of runTestCases without -v: the suite header, one line per
// test case with its coloured status, and under a failed test case the name, info and errors of every step that has
// any — passing steps included.
//
// quirk: a ranged test case gets no line break between its name and its status. runTestCases reads hasRanged before
// the loop that sets it (process_testsuite.go:161-169), so the newline meant for ranged cases is never printed (#2031
// D60).
func TestRunTestCasesPrintsNonVerbose(t *testing.T) {
	v, c := suiteNewVenom(t, 0)
	ts, p := suiteForRunTestCases(t, v, suiteOutputYAML)

	v.runTestCases(context.Background(), ts)

	want := " • Output Suite (" + p + ")\n" +
		" \t• Passing-Case " + Green(StatusPass) + "\n" +
		" \t• Failing-Case " + Red(StatusFail) + "\n" +
		" \t\t• fake\n" +
		" \t\t  " + Cyan(suiteOutputInfo(p)) + "\n" +
		" \t\t• fake\n" +
		" \t\t  " + Yellow(suiteOutputFailure(p)) + "\n" +
		" \t• Ranged-Case " + Green(StatusPass) + "\n" +
		" \t• Skipped-Case " + Gray(StatusSkip) + "\n" +
		" \t• Step-Skipped-Case " + Gray(StatusSkip) + "\n" +
		" \t• No-Step-Case " + Gray(StatusSkip) + "\n"
	assert.Equal(t, want, c.String())
	suiteAssertOutputStatuses(t, ts)
	assert.Equal(t, StatusRun, ts.Status, "runTestCases leaves the suite RUN; runTestSuite sets the final status")
}

// TestRunTestCasesPrintsVerbose pins the console output with -v: the test case name on its own line, then one line per
// step printed as it runs, and for a test case with no step result a gray "(all steps were skipped)" line — a test
// case skipped by its own skip clause shows no reason.
func TestRunTestCasesPrintsVerbose(t *testing.T) {
	v, c := suiteNewVenom(t, 1)
	ts, p := suiteForRunTestCases(t, v, suiteOutputYAML)

	v.runTestCases(context.Background(), ts)

	allSkipped := "\t\t" + Gray("• (all steps were skipped)") + "\n"
	want := " • Output Suite (" + p + ")\n" +
		" \t• Passing-Case\n" +
		" \t\t• fake " + Green(StatusPass) + "\n" +
		" \t• Failing-Case\n" +
		" \t\t• fake " + Green(StatusPass) + "\n" +
		" \t\t  " + Cyan("[info]") + " " + Cyan(suiteOutputInfo(p)) + "\n" +
		" \t\t• fake " + Red(StatusFail) + "\n" +
		" \t\t  " + Yellow(suiteOutputFailure(p)) + "\n" +
		" \t• Ranged-Case\n" +
		" \t\t• fake (range=0) " + Green(StatusPass) + "\n" +
		" \t\t• fake (range=1) " + Green(StatusPass) + "\n" +
		" \t• Skipped-Case\n" + allSkipped +
		" \t• Step-Skipped-Case\n" +
		" \t\t• fake " + Gray(StatusSkip) + "\n" +
		" \t• No-Step-Case\n" + allSkipped
	assert.Equal(t, want, c.String())
	suiteAssertOutputStatuses(t, ts)
}

// TestRunTestCasesPrintsComputedVerboseTraces pins the trace lines runTestCases prints from a test case's
// computedVerbose. Only RunTestStep fills it, at -vv, with one "writing <dump file>" per step that ran; the lines come
// after the test case's steps, indented one level deeper than at -v.
func TestRunTestCasesPrintsComputedVerboseTraces(t *testing.T) {
	v, c := suiteNewVenom(t, 2)
	v.OutputDir = t.TempDir()
	ts, p := suiteForRunTestCases(t, v, "name: Trace Suite\ntestcases:\n- name: Traced\n  steps:\n  - type: fake\n    code: 0\n")

	v.runTestCases(context.Background(), ts)

	dump := filepath.Join(v.OutputDir, "suite.Traced.testcase.1.step.1.0.dump.json")
	want := " • Trace Suite (" + p + ")\n" +
		" \t• Traced\n" +
		" \t\t• fake " + Green(StatusPass) + "\n" +
		"\t  \t  " + trace("[trac]") + " " + trace("writing "+dump) + "\n"
	assert.Equal(t, want, c.String())
	assert.Equal(t, []string{"writing " + dump}, ts.TestCases[0].computedVerbose)
	assert.FileExists(t, dump)
}

// TestRunTestCasesDoesNotRunAPreSkippedTestCase pins that a test case arriving with Skipped already set is not run: no
// start time, no step result, status SKIP, its Skipped kept as is — but it is still evaluated and printed.
func TestRunTestCasesDoesNotRunAPreSkippedTestCase(t *testing.T) {
	tests := []struct {
		verbose  int
		wantLine string
	}{
		{verbose: 0, wantLine: " \t• Pre-Skipped " + Gray(StatusSkip) + "\n"},
		{verbose: 1, wantLine: " \t• Pre-Skipped" + "\t\t" + Gray("• (all steps were skipped)") + "\n"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("verbose=%d", tt.verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, tt.verbose)
			ts, p := suiteForRunTestCases(t, v, "name: Pre\ntestcases:\n- name: Pre Skipped\n  steps:\n  - type: fake\n    error: never run\n")
			ts.TestCases[0].Skipped = []Skipped{{Value: "skipped before the run"}}

			v.runTestCases(context.Background(), ts)

			tc := ts.TestCases[0]
			// quirk: at -v the "(all steps were skipped)" line follows the name on the same line, since the
			// newline before a test case's steps is only printed for a test case that runs (#2031 D62)
			assert.Equal(t, " • Pre ("+p+")\n"+tt.wantLine, c.String())
			assert.True(t, tc.IsEvaluated)
			assert.Equal(t, StatusSkip, tc.Status)
			assert.Equal(t, []Skipped{{Value: "skipped before the run"}}, tc.Skipped)
			assert.Empty(t, tc.TestStepResults)
			assert.True(t, tc.Start.IsZero())
			assert.Equal(t, Status(""), ts.Status, "no test case ran, so the suite was never set to RUN")
		})
	}
}

// suiteStopYAML is the suite of the stop-on-failure test: "second" assigns a variable in its first step, then fails.
const suiteStopYAML = `name: Stop Suite
vars:
  foo: bar
testcases:
- name: first
  steps:
  - type: fake
    code: 0
    vars:
      got:
        from: result.code
- name: second
  steps:
  - type: fake
    code: 7
    vars:
      mine:
        from: result.code
  - type: fake
    error: boom
- name: third
  steps:
  - type: fake
- name: fourth
  skip:
  - foo ShouldBeEmpty
  steps:
  - type: fake
`

// TestRunTestCasesStopOnFailure pins --stop-on-failure inside one file: after the first test case with a step error,
// every test case without a status is marked SKIP, evaluated and given the stop-on-failure Skipped value — its own
// skip clause is never read — and runTestCases returns at once: nothing more is printed, and the failed test case's
// computed variables never reach the suite.
func TestRunTestCasesStopOnFailure(t *testing.T) {
	v, c := suiteNewVenom(t, 0)
	v.StopOnFailure = true
	ts, p := suiteForRunTestCases(t, v, suiteStopYAML)

	v.runTestCases(context.Background(), ts)

	// Line 19 of suiteStopYAML is step 2 of "second", the step that fails.
	failure := func(msg string) string { return fmt.Sprintf(`Testcase "second", step #2-0: %s (%s:19)`, msg, p) }
	want := " • Stop Suite (" + p + ")\n" +
		" \t• first " + Green(StatusPass) + "\n" +
		" \t• second " + Red(StatusFail) + "\n" +
		" \t\t• fake\n" +
		" \t\t  " + Yellow(failure("boom")) + "\n" +
		" \t\t  " + Yellow(failure("At least one required assertion failed, skipping remaining steps")) + "\n"
	assert.Equal(t, want, c.String())

	stopped := []Skipped{{Value: "===== stop-on-failure: enabled ====="}}
	require.Len(t, ts.TestCases, 4)
	assert.Equal(t, StatusPass, ts.TestCases[0].Status)
	assert.Equal(t, StatusFail, ts.TestCases[1].Status)
	for _, tc := range ts.TestCases[2:] {
		assert.Equal(t, StatusSkip, tc.Status, tc.Name)
		assert.True(t, tc.IsEvaluated, tc.Name)
		assert.Equal(t, stopped, tc.Skipped, tc.Name)
		assert.Empty(t, tc.TestStepResults, tc.Name)
	}
	assert.Nil(t, ts.TestCases[0].Skipped, "a test case that already has a status is left alone")
	assert.Equal(t, H{"mine": float64(7)}, ts.TestCases[1].computedVars, "computed, but never handed to the suite")
	assert.Equal(t, H{"first.got": float64(0)}, ts.ComputedVars)
}

// TestRunTestCasesPrefixesComputedVarsWithTheTestCaseName pins that each test case's computed variables reach the
// suite under "<slugged test case name>.<variable>", so two test cases may assign the same name. JSON numbers arrive as
// float64.
func TestRunTestCasesPrefixesComputedVarsWithTheTestCaseName(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	assign := "  steps:\n  - type: fake\n    code: %d\n    vars:\n      got:\n        from: result.code\n"
	ts, _ := suiteForRunTestCases(t, v, "name: Vars\ntestcases:\n"+
		"- name: first\n"+fmt.Sprintf(assign, 0)+
		"- name: Second Case\n"+fmt.Sprintf(assign, 1))

	v.runTestCases(context.Background(), ts)

	assert.Equal(t, H{"first.got": float64(0), "Second-Case.got": float64(1)}, ts.ComputedVars)
}

// TestRunTestCasesPanicsWithoutComputedVars pins that runTestCases relies on runTestSuite having set ComputedVars: on
// a nil map, the first test case with a computed variable panics. H.AddAllWithPrefix's nil guard tests the pointer,
// which is never nil, rather than the map (types.go:53-56, #2031 D61).
func TestRunTestCasesPanicsWithoutComputedVars(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	ts, _ := suiteParsed(t, v, "name: Nil\ntestcases:\n- name: first\n  steps:\n  - type: fake\n    code: 0\n    vars:\n      got:\n        from: result.code\n")
	require.Nil(t, ts.ComputedVars)

	assert.PanicsWithError(t, "assignment to entry in nil map", func() { v.runTestCases(context.Background(), ts) })
}

// TestRunTestCasesWithoutComputedVarsIsFineWhenNothingIsComputed is the other half of the panic above: with no
// computed variable, a nil ComputedVars stays nil and nothing breaks.
func TestRunTestCasesWithoutComputedVarsIsFineWhenNothingIsComputed(t *testing.T) {
	v, c := suiteNewVenom(t, 0)
	ts, _ := suiteParsed(t, v, "name: Nil\ntestcases:\n- name: first\n  steps:\n  - type: fake\n    code: 0\n")

	assert.NotPanics(t, func() { v.runTestCases(context.Background(), ts) })
	assert.Nil(t, ts.ComputedVars)
	assert.True(t, strings.HasSuffix(c.String(), " \t• first "+Green(StatusPass)+"\n"))
}

// suiteSecretNamesYAML is a suite whose name, first test case name and first step name hold the declared secret
// outputSecret; its step fails so that the non-verbose output prints the step name too. The second test case holds no
// secret.
var suiteSecretNamesYAML = "name: suite-" + outputSecret + "\n" +
	"vars:\n  token: " + outputSecret + "\n" +
	"secrets: [token]\n" +
	"testcases:\n" +
	"- name: tc-" + outputSecret + "\n" +
	"  steps:\n  - type: fake\n    name: step-{{.token}}\n    code: 1\n    assertions:\n    - result.code ShouldEqual 0\n" +
	"- name: plain\n" +
	"  steps:\n  - type: fake\n    name: plain-step\n    code: 0\n"

// TestRunTestCasesHidesSecretsInNames proves that the console lines of the suite, the test cases and the steps print a
// name that holds a declared secret hidden, with and without -v, and leave a name without a secret alone (#2132). The
// failure detail lines are not name lines: their text is out of #2132's scope and not checked here.
func TestRunTestCasesHidesSecretsInNames(t *testing.T) {
	for _, verbose := range []int{0, 1} {
		t.Run(fmt.Sprintf("verbose %d", verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, verbose)
			ts, _ := suiteForRunTestCases(t, v, suiteSecretNamesYAML)

			// runTestSuite hands runTestCases a context holding the suite's secrets.
			v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

			out := c.String()
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, " \t\t  ") {
					continue
				}
				assert.NotContains(t, line, outputSecret)
			}
			assert.Contains(t, out, " • suite-__hidden__ (")
			assert.Contains(t, out, " \t• tc-__hidden__")
			assert.Contains(t, out, " \t\t• step-__hidden__")
			assert.Contains(t, out, " \t• plain")
			if verbose > 0 {
				assert.Contains(t, out, " \t\t• plain-step")
			}
		})
	}
}

// suiteFailureSecret is a synthetic secret that slug.Make changes (the "@", the space and the "+"), so the slugged test
// case name on the console is a form of its own; the failure text quotes the test case's original, unslugged name.
// Built by concatenation for the secret scan.
var suiteFailureSecret = "P@ss w0rd" + "+x"

// suiteFailureSecretYAML is a suite whose test case is named with the secret and whose failing step carries the secret
// in its result: the info line and the assertion failure text both hold it, and the failure text starts with the test
// case name. With secrets false the variable is not declared secret.
func suiteFailureSecretYAML(secrets bool) string {
	declared := ""
	if secrets {
		declared = "secrets: [token]\n"
	}
	return "name: failures\n" +
		"vars:\n  token: '" + suiteFailureSecret + "'\n" +
		declared +
		"testcases:\n" +
		"- name: 'tc-" + suiteFailureSecret + "'\n" +
		"  steps:\n  - type: fake\n    name: check\n    code: '{{.token}}'\n    info: 'got {{.result.code}}'\n" +
		"    assertions:\n    - result.code ShouldEqual 0\n"
}

// TestRunTestCasesHidesSecretsInFailureLines proves that the console's info and failure lines of a failing step print
// a declared secret hidden, in its plain and slugged forms, with and without -v, and that the same lines print the
// value unchanged when it is not declared secret (#2176).
func TestRunTestCasesHidesSecretsInFailureLines(t *testing.T) {
	for _, verbose := range []int{0, 1} {
		t.Run(fmt.Sprintf("verbose %d", verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, verbose)
			ts, _ := suiteForRunTestCases(t, v, suiteFailureSecretYAML(true))

			v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

			out := c.String()
			assert.NotContains(t, out, suiteFailureSecret)
			assert.NotContains(t, out, slug.Make(suiteFailureSecret))
			assert.Contains(t, out, "got __hidden__")
			assert.Contains(t, out, `Testcase "tc-__hidden__", step #1-0: Assertion "result.code ShouldEqual 0" failed.`)
		})
		t.Run(fmt.Sprintf("verbose %d without secrets", verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, verbose)
			ts, _ := suiteForRunTestCases(t, v, suiteFailureSecretYAML(false))

			v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

			out := c.String()
			assert.Contains(t, out, "got "+suiteFailureSecret)
			assert.Contains(t, out, `Testcase "tc-`+suiteFailureSecret+`", step #1-0:`)
			assert.Contains(t, out, "tc-"+slug.Make(suiteFailureSecret))
			assert.NotContains(t, out, "__hidden__")
		})
	}
}

// TestRunTestCasesHidesSecretsInPassingStepInfo proves that the [info] line of a step that passes, printed with -v, hides
// a declared secret too, and prints it unchanged when it is not declared secret (#2176, code review).
func TestRunTestCasesHidesSecretsInPassingStepInfo(t *testing.T) {
	yaml := func(secrets bool) string {
		declared := ""
		if secrets {
			declared = "secrets: [token]\n"
		}
		return "name: passing\n" +
			"vars:\n  token: '" + suiteFailureSecret + "'\n" +
			declared +
			"testcases:\n- name: tc\n  steps:\n  - type: fake\n    name: check\n    code: 0\n    info: 'got {{.token}}'\n" +
			"    assertions:\n    - result.code ShouldEqual 0\n"
	}

	t.Run("declared secret", func(t *testing.T) {
		v, c := suiteNewVenom(t, 1)
		ts, _ := suiteForRunTestCases(t, v, yaml(true))
		v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

		out := c.String()
		assert.NotContains(t, out, suiteFailureSecret)
		assert.Contains(t, out, "[info]")
		assert.Contains(t, out, "got __hidden__")
	})
	t.Run("not declared secret", func(t *testing.T) {
		v, c := suiteNewVenom(t, 1)
		ts, _ := suiteForRunTestCases(t, v, yaml(false))
		v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

		assert.Contains(t, c.String(), "got "+suiteFailureSecret)
		assert.NotContains(t, c.String(), "__hidden__")
	})
}
