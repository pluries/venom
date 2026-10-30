package venom

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepRunOpts are the Venom settings and the caller-side result a runTestSteps characterisation run uses.
type stepRunOpts struct {
	verbose int
	stop    bool
	strict  bool
	tsIn    *TestStepResult
}

// stepRun is what one runTestSteps call left behind: the test case, the console output and the fakes' event log.
type stepRun struct {
	tc  *TestCase
	out string
	log *stepEventLog
}

// stepRunSteps builds the test case from YAML the way readFiles does, prepares it the way runTestCase does (original
// name, non-nil vars, empty computed vars) and runs its steps with the fakes registered as builtins, all sharing one
// event log.
func stepRunSteps(t *testing.T, yaml string, vars H, o stepRunOpts, fakes ...*stepFakeExecutor) stepRun {
	t.Helper()
	log := &stepEventLog{}
	for _, f := range fakes {
		f.log = log
	}
	v, c := stepNewVenom(t, o.verbose, fakes...)
	v.StopOnFailure = o.stop
	v.StrictVariables = o.strict
	if vars == nil {
		vars = H{}
	}
	tc := testCaseFromYAML(t, yaml, vars)
	tc.originalName = tc.Name
	tc.computedVars = H{}
	v.runTestSteps(context.Background(), tc, o.tsIn)
	return stepRun{tc: tc, out: c.String(), log: log}
}

// stepSeen is the part of a TestStepResult most runTestSteps tests compare.
type stepSeen struct {
	Number      int
	RangedIndex int
	Ranged      bool
	Name        string
	Status      Status
	Errors      []string
}

func stepSummaries(rs []TestStepResult) []stepSeen {
	out := []stepSeen{}
	for _, r := range rs {
		out = append(out, stepSeen{Number: r.Number, RangedIndex: r.RangedIndex, Ranged: r.RangedEnable, Name: r.Name, Status: r.Status, Errors: stepErrorValues(r.Errors)})
	}
	return out
}

const (
	stepFakeOKLine = " \t\t• fake PASS\n"
	stepFakeSteps  = "- type: fake\n  label: s1\n- type: fake\n  label: s2\n"
)

// TestRunTestStepsTestCaseSkip pins the test-case-level skip: failed conditions skip the whole case before any step,
// a condition that cannot be evaluated becomes a single failed result.
func TestRunTestStepsTestCaseSkip(t *testing.T) {
	tests := []struct {
		name        string
		skip        string
		wantStatus  Status
		wantSkipped []Skipped
		wantResults []stepSeen
		wantRuns    int
	}{
		{
			name: "a failed condition skips the case", skip: "- mode ShouldEqual run\n", wantStatus: StatusSkip,
			wantSkipped: []Skipped{{Value: `skipping testcase "tc": expected: run  got: skip`}}, wantResults: []stepSeen{},
		},
		{
			name: "every failed condition is listed", skip: "- mode ShouldEqual run\n- mode ShouldEqual walk\n", wantStatus: StatusSkip,
			wantSkipped: []Skipped{{Value: `skipping testcase "tc": expected: run  got: skip`}, {Value: `skipping testcase "tc": expected: walk  got: skip`}},
			wantResults: []stepSeen{},
		},
		{
			name: "fulfilled conditions run the steps", skip: "- mode ShouldEqual skip\n", wantRuns: 1,
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}},
		},
		{
			name: "a condition that cannot be evaluated is one failed result", skip: "- mode ShouldBogus run\n",
			wantResults: []stepSeen{{Status: StatusFail, Errors: []string{"assertion not supported"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stepNewFake("fake")
			r := stepRunSteps(t, "name: tc\nskip:\n"+tt.skip+"steps:\n- type: fake\n", H{"mode": "skip"}, stepRunOpts{verbose: 1}, fake)
			assert.Equal(t, tt.wantStatus, r.tc.Status)
			assert.Equal(t, tt.wantSkipped, r.tc.Skipped)
			assert.Equal(t, tt.wantResults, stepSummaries(r.tc.TestStepResults))
			runs, _, _ := fake.counts()
			assert.Equal(t, tt.wantRuns, runs)
		})
	}
}

// TestRunTestStepsRange pins ranged steps: one result per item, the index, key and value variables, the item key in
// the step name, and a range that cannot be parsed stopping the case.
func TestRunTestStepsRange(t *testing.T) {
	ranged := func(n int, key string) stepSeen {
		return stepSeen{Number: 1, RangedIndex: n, Ranged: true, Name: "fake (range=" + key + ")", Status: StatusPass}
	}
	tests := []struct {
		name        string
		rangeYAML   string
		wantResults []stepSeen
		wantEvents  []string
		wantOut     string
	}{
		{
			name: "an array", rangeYAML: "[a, b]",
			wantResults: []stepSeen{ranged(0, "0"), ranged(1, "1"), {Number: 2, Name: "fake", Status: StatusPass}},
			wantEvents:  []string{"setup:fake", "run:fake:0-0-a:fake", "run:fake:1-1-b:fake", "run:fake:after:fake", "teardown:fake:fake"},
			wantOut:     " \t\t• fake (range=0) PASS\n \t\t• fake (range=1) PASS\n" + stepFakeOKLine,
		},
		{
			name: "a map", rangeYAML: "{k: v}",
			wantResults: []stepSeen{ranged(0, "k"), {Number: 2, Name: "fake", Status: StatusPass}},
			wantEvents:  []string{"setup:fake", "run:fake:0-k-v:fake", "run:fake:after:fake", "teardown:fake:fake"},
			wantOut:     " \t\t• fake (range=k) PASS\n" + stepFakeOKLine,
		},
		{
			name: "a number", rangeYAML: "2",
			wantResults: []stepSeen{ranged(0, "0"), ranged(1, "1"), {Number: 2, Name: "fake", Status: StatusPass}},
			wantEvents:  []string{"setup:fake", "run:fake:0-0-0:fake", "run:fake:1-1-1:fake", "run:fake:after:fake", "teardown:fake:fake"},
			wantOut:     " \t\t• fake (range=0) PASS\n \t\t• fake (range=1) PASS\n" + stepFakeOKLine,
		},
		// quirk: an empty range leaves no result at all for its step; the next step keeps its own number
		{
			name: "an empty array", rangeYAML: "[]",
			wantResults: []stepSeen{{Number: 2, Name: "fake", Status: StatusPass}},
			wantEvents:  []string{"setup:fake", "run:fake:after:fake", "teardown:fake:fake"},
			wantOut:     stepFakeOKLine,
		},
		{
			name: "an unsupported range stops the case", rangeYAML: "true",
			wantResults: []stepSeen{{Status: StatusFail, Errors: []string{`"range" was provided an unsupported type bool`}}},
			wantEvents:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := "name: tc\nsteps:\n- type: fake\n  range: " + tt.rangeYAML + "\n  label: \"{{.index}}-{{.key}}-{{.value}}\"\n- type: fake\n  label: after\n"
			r := stepRunSteps(t, yaml, nil, stepRunOpts{verbose: 1}, stepNewFake("fake"))
			assert.Equal(t, tt.wantResults, stepSummaries(r.tc.TestStepResults))
			assert.Equal(t, tt.wantEvents, r.log.events())
			assert.Equal(t, tt.wantOut, r.out)
		})
	}
}

// TestRunTestStepsPreparationErrors pins the errors raised while a step is prepared, before it runs: which ones leave
// a printed failure, which ones stop the case, and how far the result was filled in.
func TestRunTestStepsPreparationErrors(t *testing.T) {
	tests := []struct {
		name         string
		step         string
		vars         H
		wantResult   stepSeen
		wantOut      string
		wantInterpol bool
	}{
		// quirk: the result is appended before the variables are read, so it stays numberless and nameless
		{
			name: "a variable that cannot be interpolated", step: "type: fake\n", vars: H{"bad": "{{ .x"},
			wantResult: stepSeen{Status: StatusFail, Errors: []string{`invalid template format "{{ .x": template: input:1: unclosed action`}},
		},
		{
			name: "a step that cannot be interpolated", step: "type: fake\n  label: \"{{ .x\"\n",
			wantResult: stepSeen{Status: StatusFail, Errors: []string{`invalid template format "{"label":"{{ .x","type":"fake"}": template: input:1: bad character U+0022 '"'`}},
		},
		// quirk: printed without the step's name line, so " FAIL" joins whatever the console printed before
		{
			name: "a step that is no longer YAML once interpolated", step: "type: fake\n  label: \"{{.q}}\"\n", vars: H{"q": `a\`},
			wantResult: stepSeen{Status: StatusFail, Errors: []string{"error converting YAML to JSON: yaml: did not find expected ',' or '}'"}},
			wantOut:    " FAIL\n \t\t  error converting YAML to JSON: yaml: did not find expected ',' or '}'\n",
		},
		{
			name: "an unknown executor", step: "type: nope\n",
			wantResult:   stepSeen{Number: 1, Status: StatusFail, Errors: []string{`user executor "nope" not found - loaded executors are: []`}},
			wantOut:      " FAIL\n \t\t  user executor \"nope\" not found - loaded executors are: []\n",
			wantInterpol: true,
		},
		{
			name: "a malformed runner attribute", step: "type: fake\n  retry: abc\n",
			wantResult:   stepSeen{Number: 1, Status: StatusFail, Errors: []string{`attribute "retry" is not an integer`}},
			wantOut:      " FAIL\n \t\t  attribute \"retry\" is not an integer\n",
			wantInterpol: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stepNewFake("fake")
			r := stepRunSteps(t, "name: tc\nsteps:\n- "+tt.step+"- type: fake\n  label: never\n", tt.vars, stepRunOpts{verbose: 1}, fake)
			require.Len(t, r.tc.TestStepResults, 1, "the case stops at the failing step")
			assert.Equal(t, tt.wantResult, stepSummaries(r.tc.TestStepResults)[0])
			assert.Equal(t, tt.wantOut, r.out)
			assert.Equal(t, tt.wantInterpol, r.tc.TestStepResults[0].Interpolated != nil, "Interpolated set")
			assert.Empty(t, r.log.events(), "nothing reaches the executor")
		})
	}
}

// TestRunTestStepsSetupFailure pins a failing executor Setup. It is a plain break of the range loop: the remaining
// items of that step are dropped without a result, nothing is printed, and the next raw step runs — trying Setup again,
// since a failed executor is not remembered as set up.
func TestRunTestStepsSetupFailure(t *testing.T) {
	bad := stepNewFake("bad")
	bad.setupErr = errors.New("no connection")
	r := stepRunSteps(t, "name: tc\nsteps:\n- type: bad\n  range: [a, b]\n- type: bad\n- type: fake\n", nil, stepRunOpts{verbose: 1}, bad, stepNewFake("fake"))

	assert.Equal(t, []stepSeen{
		{Number: 1, RangedIndex: 0, Ranged: true, Status: StatusFail, Errors: []string{"no connection"}},
		{Number: 2, Status: StatusFail, Errors: []string{"no connection"}},
		{Number: 3, Name: "fake", Status: StatusPass},
	}, stepSummaries(r.tc.TestStepResults))
	assert.Equal(t, []string{"setup:bad", "setup:bad", "setup:fake", "run:fake::fake", "teardown:fake:fake"}, r.log.events())
	assert.Equal(t, stepFakeOKLine, r.out, "a Setup failure prints nothing")
}

// TestRunTestStepsExecutorLifecycle pins when Setup and TearDown run: Setup once per executor name, before its first
// step, with the test case variables; TearDown once per executor at the very end of runTestSteps, the last set up first
// (defer order), with the context its Setup returned — also when a required failure ends the case early.
func TestRunTestStepsExecutorLifecycle(t *testing.T) {
	t.Run("two executors", func(t *testing.T) {
		a, b := stepNewFake("A"), stepNewFake("B")
		yaml := "name: tc\nsteps:\n- type: A\n  label: s1\n- type: B\n  label: s2\n- type: A\n  label: s3\n"
		r := stepRunSteps(t, yaml, H{"who": "tester"}, stepRunOpts{}, a, b)

		assert.Equal(t, []string{
			"setup:A", "run:A:s1:A", "setup:B", "run:B:s2:AB", "run:A:s3:AB", "teardown:B:AB", "teardown:A:A",
		}, r.log.events())
		require.Len(t, a.setupVars, 1)
		assert.Equal(t, H{"who": "tester"}, a.setupVars[0], "Setup gets tc.Vars, not the step variables")
	})

	t.Run("a required failure still tears down", func(t *testing.T) {
		yaml := "name: tc\nsteps:\n- type: fake\n  label: s1\n  code: 1\n  assertions: [result.code MustEqual 0]\n- type: fake\n  label: s2\n"
		r := stepRunSteps(t, yaml, nil, stepRunOpts{}, stepNewFake("fake"))
		assert.Equal(t, []string{"setup:fake", "run:fake:s1:fake", "teardown:fake:fake"}, r.log.events())
	})

	t.Run("a step without an executor runs nothing and passes", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- assertions: []\n", nil, stepRunOpts{verbose: 1})
		assert.Equal(t, []stepSeen{{Number: 1, Status: StatusPass}}, stepSummaries(r.tc.TestStepResults))
		assert.Equal(t, " \t\t•  PASS\n", r.out)
	})
}

// TestRunTestStepsTearDownError pins where a TearDown error goes: it is appended, at the end of runTestSteps, through a
// pointer to the result of the step that first used the executor — after that step was already printed PASS.
//
// quirk: when a later step grew tc.TestStepResults past its capacity, that pointer targets the old backing array and
// the error is lost: the case keeps no trace of it.
func TestRunTestStepsTearDownError(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantResults []stepSeen
		wantOut     string
	}{
		{
			name:        "a single step gets the error, printed as a pass",
			yaml:        "name: tc\nsteps:\n- type: bad\n",
			wantResults: []stepSeen{{Number: 1, Name: "bad", Status: StatusFail, Errors: []string{"teardown failed"}}},
			wantOut:     " \t\t• bad PASS\n",
		},
		{
			name:        "the error is lost when a later step reallocated the results",
			yaml:        "name: tc\nsteps:\n- type: bad\n- type: fake\n",
			wantResults: []stepSeen{{Number: 1, Name: "bad", Status: StatusPass}, {Number: 2, Name: "fake", Status: StatusPass}},
			wantOut:     " \t\t• bad PASS\n" + stepFakeOKLine,
		},
		{
			name:        "an executor first used by the last step keeps its error",
			yaml:        "name: tc\nsteps:\n- type: fake\n- type: bad\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}, {Number: 2, Name: "bad", Status: StatusFail, Errors: []string{"teardown failed"}}},
			wantOut:     stepFakeOKLine + " \t\t• bad PASS\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad := stepNewFake("bad")
			bad.teardownErr = errors.New("teardown failed")
			r := stepRunSteps(t, tt.yaml, nil, stepRunOpts{verbose: 1}, bad, stepNewFake("fake"))
			assert.Equal(t, tt.wantResults, stepSummaries(r.tc.TestStepResults))
			assert.Equal(t, tt.wantOut, r.out)
			_, _, teardowns := bad.counts()
			assert.Equal(t, 1, teardowns)
		})
	}
}

// TestRunTestStepsStepSkip pins the step-level skip: evaluated against the test case variables plus the previous
// steps' assignments under the test case name only, before the executor runs.
func TestRunTestStepsStepSkip(t *testing.T) {
	tests := []struct {
		name        string
		steps       string
		wantResults []stepSeen
		wantSkipped []Skipped
		wantOut     string
	}{
		{
			name: "a failed condition skips the step", steps: "- type: fake\n  skip: [mode ShouldEqual run]\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusSkip}},
			wantSkipped: []Skipped{{Value: `skipping testcase "tc" step #1: expected: run  got: skip`}},
			wantOut:     " \t\t• fake SKIP\n",
		},
		{
			name: "a fulfilled condition runs the step", steps: "- type: fake\n  skip: [mode ShouldEqual skip]\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}}, wantOut: stepFakeOKLine,
		},
		{
			name: "a condition that cannot be evaluated fails the step, the case goes on", steps: "- type: fake\n  skip: [mode ShouldBogus run]\n- type: fake\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{"assertion not supported"}}, {Number: 2, Name: "fake", Status: StatusPass}},
			wantOut:     " \t\t• fake FAIL\n \t\t  assertion not supported\n" + stepFakeOKLine,
		},
		{
			name: "a skip that is not a list fails the step", steps: "- type: fake\n  skip: {a: 1}\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{
				`unable to parse "skip" assertions: json: cannot unmarshal object into Go struct field .skip of type []string`,
			}}},
			wantOut: " \t\t• fake FAIL\n \t\t  unable to parse \"skip\" assertions: json: cannot unmarshal object into Go struct field .skip of type []string\n",
		},
		// quirk: the assignments of a skipped step are still processed, against no result: the step fails and the case stops
		{
			name: "a skipped step with an assignment fails and stops the case", steps: "- type: fake\n  skip: [mode ShouldEqual run]\n  vars:\n    x:\n      from: result.code\n- type: fake\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{"result.code reference not found"}}},
			wantSkipped: []Skipped{{Value: `skipping testcase "tc" step #1: expected: run  got: skip`}},
			wantOut:     " \t\t• fake FAIL\n \t\t  result.code reference not found\n",
		},
		{
			name: "a previous assignment is visible under the test case name only",
			steps: "- type: fake\n  out: yes\n  vars:\n    out:\n      from: result.systemout\n" +
				"- type: fake\n  skip: [tc.out ShouldEqual yes]\n- type: fake\n  skip: [out ShouldEqual yes]\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}, {Number: 2, Name: "fake", Status: StatusPass}, {Number: 3, Name: "fake", Status: StatusSkip}},
			wantOut:     stepFakeOKLine + stepFakeOKLine + " \t\t• fake SKIP\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := stepRunSteps(t, "name: tc\nsteps:\n"+tt.steps, H{"mode": "skip"}, stepRunOpts{verbose: 1}, stepNewFake("fake"))
			assert.Equal(t, tt.wantResults, stepSummaries(r.tc.TestStepResults))
			assert.Equal(t, tt.wantSkipped, r.tc.TestStepResults[0].Skipped)
			assert.Equal(t, tt.wantOut, r.out)
		})
	}
}

// TestRunTestStepsPassingStep pins every field a passing step fills in.
//
// quirk: the step is appended to tc.testSteps twice, once when it is parsed and once when it has run; nothing reads
// that field.
func TestRunTestStepsPassingStep(t *testing.T) {
	yaml := "name: tc\nsteps:\n- type: fake\n  name: first\n  out: \"hi {{.who}}\"\n  assertions: [result.code ShouldEqual 0]\n"
	r := stepRunSteps(t, yaml, H{"who": "tester"}, stepRunOpts{verbose: 1}, stepNewFake("fake"))

	require.Len(t, r.tc.TestStepResults, 1)
	res := r.tc.TestStepResults[0]
	// InputVars is filled when the result is reported (#2475)
	res.materializeInputVars()
	assert.Equal(t, stepSeen{Number: 1, Name: "first", Status: StatusPass}, stepSummaries(r.tc.TestStepResults)[0])
	assert.Equal(t, H{"__type__": "Result", "result.__Len__": 3, "result.code": 0, "result.systemout": "hi tester", "result.systemerr": ""}, res.ComputedVars)
	keys := make([]string, 0, len(res.InputVars))
	for k := range res.InputVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"__Len__", "__Type__", "venom.teststep.number", "who"}, keys)
	assert.Equal(t, "1", res.InputVars["venom.teststep.number"])
	assert.NotEmpty(t, res.Raw)
	assert.NotEmpty(t, res.Interpolated)
	assert.Equal(t, 0, res.Retries)
	assert.Equal(t, "hi tester\n", res.Systemout)
	assert.False(t, res.Start.IsZero())
	assert.False(t, res.End.IsZero())
	assert.Len(t, r.tc.testSteps, 2)
	assert.Equal(t, " \t\t• first PASS\n", r.out)

	t.Run("a name that is not a string falls back to the executor name", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  name: 3\n", nil, stepRunOpts{}, stepNewFake("fake"))
		assert.Equal(t, "fake", r.tc.TestStepResults[0].Name)
	})

	t.Run("info lines are printed after the status", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  info: \"code {{.result.code}}\"\n", nil, stepRunOpts{verbose: 1}, stepNewFake("fake"))
		assert.Equal(t, " \t\t• fake PASS\n \t\t  [info] code 0\n", r.out)
	})

	// quirk: "after 1 attempts" — the count is the number of retries, and the noun is always plural
	t.Run("a pass after a retry", func(t *testing.T) {
		fake := stepNewFake("fake")
		fake.results = []interface{}{stepResult(1, "", ""), stepResult(0, "", "")}
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  retry: 1\n  assertions: [result.code ShouldEqual 0]\n", nil, stepRunOpts{verbose: 1}, fake)
		assert.Equal(t, " \t\t• fake PASS (after 1 attempts)\n", r.out)
	})

	t.Run("nothing is printed below verbose 1", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n"+stepFakeSteps, nil, stepRunOpts{}, stepNewFake("fake"))
		assert.Empty(t, r.out)
		assert.Len(t, r.tc.TestStepResults, 2)
	})
}

// TestRunTestStepsFailures pins what happens after a failed step: a plain failure lets the case go on, a required
// assertion or --stop-on-failure adds a failure and ends the case, counting the raw steps left (not the range items).
func TestRunTestStepsFailures(t *testing.T) {
	const failed = `Testcase "tc", step #1-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (:0)`
	const mustFailed = `Testcase "tc", step #1-0: Assertion "result.code MustEqual 0" failed. expected: 0  got: 1 (:0)`
	const stopped = `Testcase "tc", step #1-0: At least one required assertion failed, skipping remaining steps (:0)`
	tests := []struct {
		name        string
		first       string
		steps       string
		stop        bool
		wantResults []stepSeen
		wantOut     string
	}{
		{
			name: "a failure lets the case go on", first: "  assertions: [result.code ShouldEqual 0]\n", steps: stepFakeSteps,
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{failed}}, {Number: 2, Name: "fake", Status: StatusPass}, {Number: 3, Name: "fake", Status: StatusPass}},
			wantOut:     " \t\t• fake FAIL\n \t\t  " + failed + "\n" + stepFakeOKLine + stepFakeOKLine,
		},
		{
			name: "a required assertion ends the case", first: "  assertions: [result.code MustEqual 0]\n", steps: stepFakeSteps,
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{mustFailed, stopped}}},
			wantOut:     " \t\t• fake FAIL\n \t\t  " + mustFailed + "\n \t\t  " + stopped + "\n \t\t  2 other steps were skipped\n",
		},
		{
			name: "one step left is singular", first: "  assertions: [result.code MustEqual 0]\n", steps: "- type: fake\n",
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{mustFailed, stopped}}},
			wantOut:     " \t\t• fake FAIL\n \t\t  " + mustFailed + "\n \t\t  " + stopped + "\n \t\t  1 other step was skipped\n",
		},
		{
			name: "--stop-on-failure ends the case on a plain failure", first: "  assertions: [result.code ShouldEqual 0]\n", steps: "- type: fake\n", stop: true,
			wantResults: []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{failed, stopped}}},
			wantOut:     " \t\t• fake FAIL\n \t\t  " + failed + "\n \t\t  " + stopped + "\n \t\t  1 other step was skipped\n",
		},
		{
			name: "a required failure in a range drops the remaining items", first: "  range: [a, b]\n  assertions: [result.code MustEqual 0]\n", steps: "- type: fake\n",
			wantResults: []stepSeen{{Number: 1, Ranged: true, Name: "fake (range=0)", Status: StatusFail, Errors: []string{mustFailed, stopped}}},
			wantOut:     " \t\t• fake (range=0) FAIL\n \t\t  " + mustFailed + "\n \t\t  " + stopped + "\n \t\t  1 other step was skipped\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := "name: tc\nsteps:\n- type: fake\n  code: 1\n" + tt.first + tt.steps
			r := stepRunSteps(t, yaml, nil, stepRunOpts{verbose: 1, stop: tt.stop}, stepNewFake("fake"))
			assert.Equal(t, tt.wantResults, stepSummaries(r.tc.TestStepResults))
			assert.Equal(t, tt.wantOut, r.out)
		})
	}
}

// TestRunTestStepsAssignments pins the vars block: each assignment reads the step's computed variables, lands in
// tc.computedVars and in the next steps' variables (bare and under the test case name); an assignment that fails is
// printed and ends the case.
func TestRunTestStepsAssignments(t *testing.T) {
	t.Run("assignments flow to the next step and to the test case", func(t *testing.T) {
		yaml := "name: tc\nsteps:\n- type: fake\n  out: hello world\n  vars:\n    out:\n      from: result.systemout\n" +
			"    part:\n      from: result.systemout\n      regex: w(or)ld\n    none:\n      from: result.systemout\n      regex: xyz\n" +
			"    code:\n      from: result.code\n      regex: (.*)\n    fallback:\n      from: result.nothing\n      default: dflt\n" +
			"- type: fake\n  label: \"{{.out}}|{{.tc.out}}|{{.part}}|{{.fallback}}\"\n"
		r := stepRunSteps(t, yaml, nil, stepRunOpts{}, stepNewFake("fake"))

		assert.Equal(t, H{"out": "hello world", "part": "or", "none": "", "code": "", "fallback": "dflt"}, r.tc.computedVars,
			"a regex on a non-string value, like one that matches nothing, assigns an empty string")
		assert.Contains(t, r.log.events(), "run:fake:hello world|hello world|or|dflt:fake")
		assert.Equal(t, []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}, {Number: 2, Name: "fake", Status: StatusPass}}, stepSummaries(r.tc.TestStepResults))
	})

	tests := []struct {
		name    string
		vars    string
		wantErr string
	}{
		{name: "a missing reference", vars: "    x:\n      from: result.nothing\n", wantErr: "result.nothing reference not found"},
		{name: "an invalid regex", vars: "    x:\n      from: result.systemout\n      regex: \"(\"\n", wantErr: "error parsing regexp: missing closing ): `(`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := "name: tc\nsteps:\n- type: fake\n  vars:\n" + tt.vars + "- type: fake\n  label: never\n"
			fake := stepNewFake("fake")
			r := stepRunSteps(t, yaml, nil, stepRunOpts{verbose: 1}, fake)
			assert.Equal(t, []stepSeen{{Number: 1, Name: "fake", Status: StatusFail, Errors: []string{tt.wantErr}}}, stepSummaries(r.tc.TestStepResults))
			assert.Equal(t, " \t\t• fake FAIL\n \t\t  "+tt.wantErr+"\n", r.out)
			assert.Empty(t, r.tc.computedVars)
			runs, _, _ := fake.counts()
			assert.Equal(t, 1, runs)
		})
	}
}

// TestRunTestStepsNilVarsPanicAfterAnAssignment pins a test case built without variables (runTestCase always sets
// some; a library caller may not): the skip variables built from nil tc.Vars panic on the first previous-step assignment
// they copy.
//
// quirk: AddAllWithPrefix guards against a nil *H, not a nil map. Since #2475 those variables are built only for a step
// that has a skip clause, so a step without one no longer panics after an assignment.
func TestRunTestStepsNilVarsPanicAfterAnAssignment(t *testing.T) {
	v, _ := stepNewVenom(t, 0, stepNewFake("fake"))
	yaml := "name: tc\nsteps:\n- type: fake\n  vars:\n    x:\n      from: result.code\n- type: fake\n  skip: [result.code ShouldEqual 9]\n"
	tc := testCaseFromYAML(t, yaml, nil)
	tc.computedVars = H{}
	assert.PanicsWithError(t, "assignment to entry in nil map", func() { v.runTestSteps(context.Background(), tc, nil) })

	t.Run("a step without a skip clause does not build them", func(t *testing.T) {
		tc := testCaseFromYAML(t, "name: tc\nsteps:\n- type: fake\n  vars:\n    x:\n      from: result.code\n- type: fake\n", nil)
		tc.computedVars = H{}
		v.runTestSteps(context.Background(), tc, nil)
		assert.Len(t, tc.TestStepResults, 2)
	})

	t.Run("without an assignment nil variables are fine", func(t *testing.T) {
		tc := testCaseFromYAML(t, "name: tc\nsteps:\n"+stepFakeSteps, nil)
		tc.computedVars = H{}
		v.runTestSteps(context.Background(), tc, nil)
		assert.Len(t, tc.TestStepResults, 2)
	})
}

// TestRunTestStepsFromUserExecutor pins the inner steps of a user executor (a non-nil tsIn): the same results, but
// nothing reaches the console, whatever the verbosity.
func TestRunTestStepsFromUserExecutor(t *testing.T) {
	yaml := "name: tc\nsteps:\n- type: fake\n  code: 1\n  assertions: [result.code ShouldEqual 0]\n- type: fake\n- type: nope\n"
	r := stepRunSteps(t, yaml, nil, stepRunOpts{verbose: 1, tsIn: &TestStepResult{}}, stepNewFake("fake"))
	assert.Empty(t, r.out)
	assert.Equal(t, []Status{StatusFail, StatusPass, StatusFail}, []Status{
		r.tc.TestStepResults[0].Status, r.tc.TestStepResults[1].Status, r.tc.TestStepResults[2].Status,
	})
}

// TestRunTestStepsStrictVariables pins StrictVariables (#2194): off (the default) an unknown variable stays as written
// and the step runs; on, the step fails before it runs and the error names the step and the variables, never a value.
func TestRunTestStepsStrictVariables(t *testing.T) {
	const secretValue = "s3cr3t-synthetic"
	vars := H{"token": secretValue, "venom.testsuite": "suite"}

	t.Run("off: the unknown variable stays as written and the step runs", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  label: \"a {{.typo}} {{.token}}\"\n", vars, stepRunOpts{}, stepNewFake("fake"))
		assert.Equal(t, []stepSeen{{Number: 1, Name: "fake", Status: StatusPass}}, stepSummaries(r.tc.TestStepResults))
		assert.Contains(t, r.log.events(), "run:fake:a {{.typo}} "+secretValue+":fake")
	})

	t.Run("on: an unknown variable fails the step before it runs", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  label: \"{{.zed}} {{.typo}} {{.zed}} {{.token}}\"\n- type: fake\n  label: never\n",
			vars, stepRunOpts{strict: true}, stepNewFake("fake"))
		require.Len(t, r.tc.TestStepResults, 1)
		assert.Equal(t, stepSeen{Status: StatusFail, Errors: []string{`step #1: unknown variable(s) "typo", "zed"`}}, stepSummaries(r.tc.TestStepResults)[0])
		assert.Empty(t, r.log.events(), "nothing reaches the executor")
		assert.NotContains(t, stepErrorValues(r.tc.TestStepResults[0].Errors)[0], secretValue)
	})

	t.Run("on: the second step is the one named", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n- type: fake\n  label: \"{{.typo}}\"\n", nil, stepRunOpts{strict: true}, stepNewFake("fake"))
		require.Len(t, r.tc.TestStepResults, 2)
		assert.Equal(t, []string{`step #2: unknown variable(s) "typo"`}, stepSummaries(r.tc.TestStepResults)[1].Errors)
	})

	t.Run("on: a ranged step names the item", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  range: [a, b]\n  label: \"{{.typo}}\"\n", nil, stepRunOpts{strict: true}, stepNewFake("fake"))
		require.NotEmpty(t, r.tc.TestStepResults)
		assert.Equal(t, []string{`step #1-0: unknown variable(s) "typo"`}, stepSummaries(r.tc.TestStepResults)[0].Errors)
	})

	t.Run("on: allowed forms do not fail", func(t *testing.T) {
		yaml := "name: tc\nsteps:\n- type: fake\n  out: hello\n  vars:\n    out:\n      from: result.systemout\n" +
			"- type: fake\n  range: [a, b]\n  label: \"{{.out}} {{.index}} {{.key}} {{.value}} {{.token}} {{.venom.teststep.number}} {{.venom.testsuite}}\"\n" +
			"- type: fake\n  label: \"{{.missing | default 1}} {{.nothere | ternary 1 2}}\"\n"
		r := stepRunSteps(t, yaml, vars, stepRunOpts{strict: true}, stepNewFake("fake"))
		for _, res := range stepSummaries(r.tc.TestStepResults) {
			assert.Equal(t, StatusPass, res.Status, "%+v", res)
		}
		assert.Len(t, r.tc.TestStepResults, 4)
	})

	t.Run("on: result variables, resolved after the step ran, do not fail", func(t *testing.T) {
		yaml := "name: tc\nsteps:\n- type: fake\n  info: \"got {{.result.systemout}} and {{.result.systemoutjson.foo}}\"\n  assertions:\n  - result.code ShouldEqual \"{{.result.code}}\"\n"
		r := stepRunSteps(t, yaml, vars, stepRunOpts{strict: true}, stepNewFake("fake"))
		require.Len(t, r.tc.TestStepResults, 1)
		for _, e := range stepSummaries(r.tc.TestStepResults)[0].Errors {
			assert.NotContains(t, e, "unknown variable")
		}
	})

	t.Run("on: a name that only starts with result is still unknown", func(t *testing.T) {
		r := stepRunSteps(t, "name: tc\nsteps:\n- type: fake\n  label: \"{{.resultx}} {{.results.a}}\"\n", nil, stepRunOpts{strict: true}, stepNewFake("fake"))
		require.Len(t, r.tc.TestStepResults, 1)
		assert.Equal(t, []string{`step #1: unknown variable(s) "results.a", "resultx"`}, stepSummaries(r.tc.TestStepResults)[0].Errors)
	})
}

// stepCtxVarsExecutor records, at every Run, the value its context holds for each of names and how many names
// ContextKey("vars") lists. stepFakeExecutor does not look at its context's variables; this one exists only to.
type stepCtxVarsExecutor struct {
	names []string
	seen  []map[string]interface{}
}

func (e *stepCtxVarsExecutor) Run(ctx context.Context, step TestStep) (interface{}, error) {
	got := map[string]interface{}{"#vars": len(cast.ToStringSlice(ctx.Value(ContextKey("vars"))))}
	for _, n := range e.names {
		got[n] = VarFromCtx(ctx, n)
	}
	e.seen = append(e.seen, got)
	return stepResult(0, cast.ToString(step["out"]), ""), nil
}

// TestRunTestStepsStepContext pins the context each step's executor runs with, through the step loop, a safety net
// for a future change to how the step scope is flattened (#2405): test case variables, nested and shadowed ones included, a variable the
// previous step captured, bare and under the test case's name, and the step number, which changes at every step.
//
// quirk: the step loop keeps the context of the previous step and stacks the next step's values on top of it, so
// ContextKey("vars") is the step's own list but a "var.<name>" of an earlier step stays readable: here the value of the
// first step's range.
func TestRunTestStepsStepContext(t *testing.T) {
	ctxFake := &stepCtxVarsExecutor{names: []string{
		"scalar", "resp.resp1.name", "body.code", "body.status", "Upper", "venom.teststep.number", "first", "tc.first", "value",
	}}
	v, _ := stepNewVenom(t, 0)
	v.RegisterExecutorBuiltin("ctxvars", ctxFake)
	tc := testCaseFromYAML(t, "name: tc\nsteps:\n- type: ctxvars\n  range: [ranged]\n  out: captured\n  vars:\n    first:\n      from: result.systemout\n"+
		"- type: ctxvars\n", stepContextVars())
	tc.originalName = tc.Name
	tc.computedVars = H{}
	v.runTestSteps(context.Background(), tc, nil)

	require.Equal(t, []stepSeen{{Number: 1, Ranged: true, Name: "ctxvars (range=0)", Status: StatusPass}, {Number: 2, Name: "ctxvars", Status: StatusPass}},
		stepSummaries(tc.TestStepResults))
	// "#vars" is the number of flat keys of the step's scope: 59 for the test case variables (49 of them for resp alone),
	// venom.teststep.number and the two aggregates, plus index, key and value on the ranged first step; the second step
	// has "first" and "tc.first" instead of the range's three.
	assert.Equal(t, []map[string]interface{}{
		{
			"#vars": 62, "scalar": "s", "resp.resp1.name": "item-1", "body.code": "200", "body.status": "ok", "Upper": "U",
			"venom.teststep.number": "1", "first": nil, "tc.first": nil, "value": "ranged",
		},
		{
			"#vars": 61, "scalar": "s", "resp.resp1.name": "item-1", "body.code": "200", "body.status": "ok", "Upper": "U",
			"venom.teststep.number": "2", "first": "captured", "tc.first": "captured",
			// quirk: the range's value, absent from this step's variables, is still in its context.
			"value": "ranged",
		},
	}, ctxFake.seen)
}
