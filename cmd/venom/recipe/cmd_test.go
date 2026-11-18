package recipe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom/internal/recipecheck"
)

const (
	helperArgsEnv = "VENOM_TEST_RECIPE_ARGS"
	testSpec      = "014-web-mobile-recipes"
)

// repoRoot is the module folder, found as root_test.go does.
func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// repoSpecsDir is the repository's docs/specs; the test is skipped where it is absent (the public fork has none).
func repoSpecsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "docs", "specs")
	if _, err := os.Stat(filepath.Join(dir, testSpec, "spec.md")); err != nil {
		t.Skip("docs/specs is absent (public fork): the spec of the fixtures cannot be read")
	}
	return dir
}

// specsDirWith writes a specs folder holding one spec with the given scenario ids and returns it.
func specsDirWith(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, testSpec), 0o750))
	text := "# Spec\n\n" + strings.Join(ids, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, testSpec, "spec.md"), []byte(text), 0o600))
	return dir
}

func writeSuite(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(text), 0o600))
	return p
}

// testcaseYAML is a testcase with one app-reading assertion.
func testcaseYAML(name string) string {
	return "- name: " + name + "\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n"
}

func suiteYAML(testcases ...string) string {
	return "name: Suite\nvars:\n  spec: " + testSpec + "\ntestcases:\n" + strings.Join(testcases, "")
}

// runCheckIn runs the check in this process with the given flags and returns the exit code and what was printed on
// the standard output and on the error output.
func runCheckIn(t *testing.T, outFormat, specs string, paths ...string) (int, string, string) {
	t.Helper()
	oldFormat, oldSpecs := format, specsDir
	t.Cleanup(func() { format, specsDir = oldFormat, oldSpecs })
	format, specsDir = outFormat, specs
	var out, errOut bytes.Buffer
	code := runCheck(&out, &errOut, paths)
	return code, out.String(), errOut.String()
}

// TestRecipeHelperProcess is not a test: TestRecipeExitStatus starts the test binary again with the arguments in
// VENOM_TEST_RECIPE_ARGS and this function then runs the real `venom recipe` command. The command ends in os.Exit
// (venom.OSExit), so the exit status is read from a child process, as cmd/venom/run/exit_status_test.go does.
func TestRecipeHelperProcess(t *testing.T) {
	args := os.Getenv(helperArgsEnv)
	if args == "" {
		t.Skip("helper process of TestRecipeExitStatus")
	}
	Cmd.SetArgs(strings.Split(args, "\x1f"))
	if err := Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(99)
	}
	os.Exit(0)
}

// recipeExit runs `venom recipe args...` in a child process and returns its exit status and what it printed.
func recipeExit(t *testing.T, args ...string) (int, string) {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	// The test binary is started again as it is: nothing here comes from outside the test.
	cmd := &exec.Cmd{
		Path: self,
		Args: []string{self, "-test.run=^TestRecipeHelperProcess$"},
		Dir:  t.TempDir(),
		Env:  append(os.Environ(), helperArgsEnv+"="+strings.Join(args, "\x1f")),
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "the child did not start: %v", err)
	return exitErr.ExitCode(), out.String()
}

// US3-AS6: a clean folder exits 0, and no browser is involved (the check only reads files).
func TestRecipeExitStatus(t *testing.T) {
	specs := specsDirWith(t, "US1-AS1")
	clean := t.TempDir()
	writeSuite(t, clean, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))
	findings := t.TempDir()
	writeSuite(t, findings, "a.yml", suiteYAML(testcaseYAML("a name with no scenario id")))
	unparsable := t.TempDir()
	badFile := writeSuite(t, unparsable, "bad.yml", "name: [unclosed\n")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"a clean folder", []string{"check", "--specs-dir", specs, clean}, 0},
		{"a finding", []string{"check", "--specs-dir", specs, findings}, 1},
		{"no path", []string{"check"}, 2},
		{"a missing path", []string{"check", filepath.Join(clean, "absent.yml")}, 2},
		{"an empty folder", []string{"check", t.TempDir()}, 2},
		{"an unparsable file", []string{"check", badFile}, 2},
		{"an unknown format", []string{"check", "--format", "xml", clean}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exit, out := recipeExit(t, tt.args...)

			assert.Equal(t, tt.want, exit, out)
		})
	}
}

// US3-AS6: findings exit 1 and name the rule, the file and the line.
func TestCheckFindingsExitOneAndAreNamed(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one"), testcaseYAML("a name with no scenario id")))

	code, out, _ := runCheckIn(t, "text", specsDirWith(t, "US1-AS1"), path)

	assert.Equal(t, 1, code, out)
	assert.Contains(t, out, path+":12: RC01")
	assert.Contains(t, out, "1 finding(s) in 1 file(s)")
}

// US3-AS6: a clean folder prints a count and exits 0.
func TestCheckCleanFolderExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	code, out, errOut := runCheckIn(t, "text", specsDirWith(t, "US1-AS1"), dir)

	assert.Equal(t, 0, code, out+errOut)
	assert.Contains(t, out, "1 files read, no finding")
}

// US3-AS7: input that cannot be used exits 2, and the output names the path.
func TestCheckUnusableInputExitsTwoAndNamesThePath(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(empty, 0o750))
	missing := filepath.Join(dir, "absent.yml")
	bad := writeSuite(t, dir, "bad.yml", "name: [unclosed\n")
	notAList := writeSuite(t, dir, "notalist.yml", "name: x\ntestcases: nope\n")

	for name, path := range map[string]string{"empty folder": empty, "missing path": missing, "unparsable file": bad, "testcases not a list": notAList} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := runCheckIn(t, "text", "", path)

			assert.Equal(t, 2, code)
			// The error text quotes the path (backslashes doubled on Windows): the file name is what must appear.
			assert.Contains(t, out+errOut, filepath.Base(path))
		})
	}
}

// US3-AS7: an unparsable file is also reported as RC10 in the output.
func TestCheckUnparsableFileIsReportedAsRC10(t *testing.T) {
	bad := writeSuite(t, t.TempDir(), "bad.yml", "name: [unclosed\n")

	code, out, _ := runCheckIn(t, "text", "", bad)

	assert.Equal(t, 2, code)
	assert.Contains(t, out, "RC10")
}

// US3-AS6: no path at all is unusable input.
func TestCheckNoPathExitsTwo(t *testing.T) {
	code, _, errOut := runCheckIn(t, "text", "")

	assert.Equal(t, 2, code)
	assert.NotEmpty(t, errOut)
}

// US3-AS8, FR-004: the JSON output is one object with the files read and the findings, each with exactly the
// contract's fields.
func TestCheckJSONOutputCarriesTheFields(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("a name with no scenario id")))

	code, out, _ := runCheckIn(t, "json", specsDirWith(t, "US1-AS1"), path)

	assert.Equal(t, 1, code, out)
	var raw struct {
		Files    int              `json:"files"`
		Findings []map[string]any `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &raw), out)
	assert.Equal(t, 1, raw.Files)
	require.Len(t, raw.Findings, 1, out)
	keys := make([]string, 0, 6)
	for k := range raw.Findings[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	assert.Equal(t, []string{"file", "line", "message", "rule", "suite", "testcase"}, keys)
	assert.Equal(t, "RC01", raw.Findings[0]["rule"])
	assert.Equal(t, path, raw.Findings[0]["file"])
	assert.Equal(t, "Suite", raw.Findings[0]["suite"])
	assert.Equal(t, "a name with no scenario id", raw.Findings[0]["testcase"])
	assert.InDelta(t, 5, raw.Findings[0]["line"], 0)
}

// US3-AS8: the JSON output of a clean folder has an empty list, not null.
func TestCheckJSONOutputOfACleanFolder(t *testing.T) {
	dir := t.TempDir()
	writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	code, out, _ := runCheckIn(t, "json", specsDirWith(t, "US1-AS1"), dir)

	assert.Equal(t, 0, code, out)
	assert.JSONEq(t, `{"files":1,"findings":[]}`, out)
}

// US3-AS7, US3-AS8: the JSON output of an unparsable file carries its RC10 finding with its path.
func TestCheckJSONOutputOfAnUnparsableFile(t *testing.T) {
	bad := writeSuite(t, t.TempDir(), "bad.yml", "name: [unclosed\n")

	code, out, _ := runCheckIn(t, "json", "", bad)

	assert.Equal(t, 2, code)
	var report recipecheck.Report
	require.NoError(t, json.Unmarshal([]byte(out), &report), out)
	require.Len(t, report.Findings, 1)
	assert.Equal(t, "RC10", report.Findings[0].Rule)
	assert.Equal(t, bad, report.Findings[0].File)
}

// SC-004: a generated folder of 100 testcases is checked under one second on the maker's machine (RECIPE_PERF=1), and
// under five seconds elsewhere, a bound that catches a regression without flaking in CI. The folder is clean, so a
// rule that wrongly reports on valid input fails here too.
func TestCheckOfOneHundredTestcasesIsFast(t *testing.T) {
	bound := 5 * time.Second
	if os.Getenv("RECIPE_PERF") == "1" {
		bound = time.Second
	}
	dir := t.TempDir()
	var ids []string
	for file := 0; file < 10; file++ {
		var testcases []string
		for i := 1; i <= 10; i++ {
			id := fmt.Sprintf("US%d-AS1", file*10+i)
			ids = append(ids, id)
			testcases = append(testcases, testcaseYAML(id+" generated testcase"))
		}
		writeSuite(t, dir, fmt.Sprintf("suite%02d.yml", file), suiteYAML(testcases...))
	}

	start := time.Now()
	code, out, errOut := runCheckIn(t, "text", specsDirWith(t, ids...), dir)
	elapsed := time.Since(start)

	assert.Equal(t, 0, code, out+errOut)
	assert.Contains(t, out, "10 files read")
	assert.Less(t, elapsed, bound)
}

// fixturesDir is tests/recipes/fixtures of the repository.
func fixturesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "recipes", "fixtures")
}

// rulesOf runs the check as the command does, in JSON, and returns the exit code and the distinct rules reported.
func rulesOf(t *testing.T, specs string, paths ...string) (int, []string) {
	t.Helper()
	code, out, errOut := runCheckIn(t, "json", specs, paths...)
	var report recipecheck.Report
	require.NoError(t, json.Unmarshal([]byte(out), &report), out+errOut)
	seen := map[string]bool{}
	var rules []string
	for _, f := range report.Findings {
		if !seen[f.Rule] {
			seen[f.Rule] = true
			rules = append(rules, f.Rule)
		}
	}
	sort.Strings(rules)
	return code, rules
}

// SC-002: the check, run once per defect folder, reports findings of exactly its own rule and no other. The folders
// are those of the manifest tests/recipes/fixtures/defects/README.md (T018).
func TestCheckReportsExactlyTheRuleOfEachDefectFolder(t *testing.T) {
	specs := repoSpecsDir(t)
	for _, rule := range []string{"RC01", "RC02", "RC03", "RC04", "RC05", "RC07", "RC09"} {
		t.Run(rule, func(t *testing.T) {
			dir := filepath.Join(fixturesDir(t), "defects", rule)
			require.DirExists(t, dir)

			code, rules := rulesOf(t, specs, dir)

			assert.Equal(t, 1, code)
			assert.Equal(t, []string{rule}, rules)
		})
	}
}

// SC-002: a copy of the clean fixture whose typed text is a connection string with a password yields RC08 and nothing
// else. The fixture is generated here, with the password-shaped value assembled from fragments, so that no file of the
// repository holds credential-shaped text (D-008).
func TestCheckReportsExactlyRC08OnTheCredentialFixture(t *testing.T) {
	specs := repoSpecsDir(t)
	clean, err := os.ReadFile(filepath.Join(fixturesDir(t), "clean", "page.yml"))
	require.NoError(t, err)
	const typed = "text: milk"
	require.Contains(t, string(clean), typed)
	credential := "Server=db;Pass" + "word=" + "Zq7Lm9Xk2Pw4"
	dir := t.TempDir()
	writeSuite(t, dir, "page.yml", strings.Replace(string(clean), typed, `text: "`+credential+`"`, 1))

	code, rules := rulesOf(t, specs, dir)

	assert.Equal(t, 1, code)
	assert.Equal(t, []string{"RC08"}, rules)
}

// SC-002: the clean fixture reports nothing.
func TestCheckReportsNothingOnTheCleanFixture(t *testing.T) {
	specs := repoSpecsDir(t)

	code, rules := rulesOf(t, specs, filepath.Join(fixturesDir(t), "clean"))

	assert.Equal(t, 0, code)
	assert.Empty(t, rules)
}

// T018: the manifest lists the eight planted defects and their folders, and says which rules a fixture does not cover.
func TestDefectsManifestListsEveryFixture(t *testing.T) {
	text, err := os.ReadFile(filepath.Join(fixturesDir(t), "defects", "README.md"))
	require.NoError(t, err)

	for _, rule := range []string{"RC01", "RC02", "RC03", "RC04", "RC05", "RC07", "RC08", "RC09"} {
		assert.Contains(t, string(text), rule)
	}
	for _, rule := range []string{"RC06", "RC10", "RC11"} {
		assert.Contains(t, string(text), rule, "the manifest names the rules proved by tests instead of a fixture")
	}
}

// US3-AS7, RC10: a self-referencing anchor (venom run says "anchor 'a' value contains itself") is unusable input, not
// a crash. A Go stack overflow is fatal and cannot be recovered, so the check runs in a child process: the test fails
// on an assertion whether the child ends with RC10 or dies. A fatal Go error also exits with status 2, so the status
// alone proves nothing: the output must hold the RC10 finding and no runtime trace.
func TestCheckOfASelfReferencingAnchorIsRC10AndDoesNotCrash(t *testing.T) {
	path := writeSuite(t, t.TempDir(), "loop.yml", "name: Loop\nvars:\n  v: &a {k: *a}\n"+
		"testcases:\n"+testcaseYAML("US1-AS1 one"))

	exit, out := recipeExit(t, "check", path)

	head := out[:min(len(out), 300)] // a runtime trace is long: the start names the cause
	assert.Equal(t, 2, exit, head)
	assert.True(t, strings.Contains(out, "RC10"), "the output must carry the RC10 finding; it starts with: %s", head)
	assert.False(t, strings.Contains(out, "fatal error") || strings.Contains(out, "goroutine "), "the check crashed: %s", head)
}

// reportedLine returns the line of out that holds the rule, or "" when no line does.
func reportedLine(out, rule string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ": "+rule+" ") {
			return line
		}
	}
	return ""
}

// US3-AS6: the text line of a testcase-level finding is `file:line: RCnn message (suite / testcase)`.
func TestCheckTextLineOfATestcaseFinding(t *testing.T) {
	path := writeSuite(t, t.TempDir(), "a.yml", suiteYAML(testcaseYAML("a name with no scenario id")))

	code, out, _ := runCheckIn(t, "text", specsDirWith(t, "US1-AS1"), path)

	assert.Equal(t, 1, code, out)
	assert.Regexp(t, "^"+regexp.QuoteMeta(path)+`:5: RC01 \S.* \(Suite / a name with no scenario id\)$`, reportedLine(out, "RC01"))
}

// US3-AS6: the text line of a suite-level finding is `file:line: RCnn message (suite)`, with no trailing " / ".
func TestCheckTextLineOfASuiteFinding(t *testing.T) {
	path := writeSuite(t, t.TempDir(), "a.yml", "name: Suite\ntestcases:\n"+testcaseYAML("US1-AS1 one"))

	code, out, _ := runCheckIn(t, "text", specsDirWith(t, "US1-AS1"), path)

	assert.Equal(t, 1, code, out)
	assert.Regexp(t, "^"+regexp.QuoteMeta(path)+`:1: RC06 \S.* \(Suite\)$`, reportedLine(out, "RC06"))
}

// US3-AS7, RC10: a path that matches no suite file is an RC10 finding in the JSON output, which names the path; the
// exit status is 2.
func TestCheckJSONOutputOfAPathThatMatchesNoFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(empty, 0o750))
	for name, path := range map[string]string{"missing path": filepath.Join(dir, "absent.yml"), "empty folder": empty} {
		t.Run(name, func(t *testing.T) {
			code, out, _ := runCheckIn(t, "json", "", path)

			assert.Equal(t, 2, code)
			var report recipecheck.Report
			require.NoError(t, json.Unmarshal([]byte(out), &report), "stdout must be the JSON report: %q", out)
			require.Len(t, report.Findings, 1, out)
			assert.Equal(t, "RC10", report.Findings[0].Rule)
			assert.Contains(t, report.Findings[0].Message, filepath.Base(path))
		})
	}
}

// US3-AS7, RC10: the text format prints the RC10 line for a path that matches no suite file.
func TestCheckTextOutputOfAPathThatMatchesNoFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.yml")

	code, out, _ := runCheckIn(t, "text", "", missing)

	assert.Equal(t, 2, code)
	line := reportedLine(out, "RC10")
	assert.NotEmpty(t, line, "no RC10 line on the standard output: %q", out)
	assert.Contains(t, line, "absent.yml")
}

// US3-AS6: an unknown --format value is refused with a message and exit 2, and nothing is checked.
func TestCheckUnknownFormatExitsTwoWithAMessage(t *testing.T) {
	dir := t.TempDir()
	writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	code, out, errOut := runCheckIn(t, "xml", specsDirWith(t, "US1-AS1"), dir)

	assert.Equal(t, 2, code)
	assert.Contains(t, errOut, "unknown --format")
	assert.Contains(t, errOut, "xml")
	assert.Empty(t, out)
}

// US3-AS7: a file that does not parse does not stop the rules on the other files: their findings are printed and the
// exit code is 2.
func TestCheckRunsTheRulesOnTheOtherFilesWhenOneDoesNotParse(t *testing.T) {
	dir := t.TempDir()
	writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("a name with no scenario id")))
	writeSuite(t, dir, "bad.yml", "name: [unclosed\n")

	code, out, _ := runCheckIn(t, "text", specsDirWith(t, "US1-AS1"), dir)

	assert.Equal(t, 2, code, out)
	assert.Contains(t, out, "RC10")
	assert.Contains(t, out, "RC01")
}

// runCheckWithCoverage is runCheckIn with the --coverage flag set to the given spec id.
func runCheckWithCoverage(t *testing.T, outFormat, specs, spec string, paths ...string) (int, string, string) {
	t.Helper()
	old := coverage
	t.Cleanup(func() { coverage = old })
	coverage = spec
	return runCheckIn(t, outFormat, specs, paths...)
}

// coverageIDs are the scenarios of the temp spec used by the coverage tests, written out of order on purpose.
var coverageIDs = []string{"US5-AS5", "US1-AS4", "US1-AS1", "US2-AS1", "US1-AS3", "US1-AS2"}

// specsDirOf writes a specs folder holding the given specs (folder name to scenario ids) and returns it.
func specsDirOf(t *testing.T, specs map[string][]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, ids := range specs {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o750))
		text := "# Spec\n\n" + strings.Join(ids, "\n") + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name, "spec.md"), []byte(text), 0o600))
	}
	return dir
}

// suiteYAMLOf is suiteYAML for a suite that names the given spec.
func suiteYAMLOf(spec string, testcases ...string) string {
	return strings.Replace(suiteYAML(testcases...), "spec: "+testSpec, "spec: "+spec, 1)
}

// US2-AS1: --coverage lists the scenarios of the spec in spec order, in text.
func TestCheckCoverageTextListsProvenAndNotProven(t *testing.T) {
	clean := filepath.Join(fixturesDir(t), "clean", "page.yml")

	code, out, errOut := runCheckWithCoverage(t, "text", specsDirWith(t, coverageIDs...), testSpec, clean)

	assert.Equal(t, 0, code, out+errOut)
	assert.Contains(t, out, "US1-AS1 proven: "+clean+" / US1-AS1 the page shows its title and heading")
	assert.Contains(t, out, "US1-AS3 proven: "+clean+" / US1-AS3 an item typed in the input is added to the list")
	assert.Contains(t, out, "US2-AS1 proven: "+clean+" / US2-AS1 the list is empty before anything is added")
	assert.Contains(t, out, "US1-AS4 not proven")
	assert.Contains(t, out, "US5-AS5 not proven")
	assert.Less(t, strings.Index(out, "US1-AS1 proven"), strings.Index(out, "US1-AS2 not proven"))
	assert.Less(t, strings.Index(out, "US1-AS2 not proven"), strings.Index(out, "US5-AS5 not proven"))
	assert.Equal(t, 3, strings.Count(out, " proven: "))
}

// US2-AS1: with --format json the report carries coverage.
func TestCheckCoverageJSONCarriesTheCoverage(t *testing.T) {
	clean := filepath.Join(fixturesDir(t), "clean", "page.yml")

	code, out, errOut := runCheckWithCoverage(t, "json", specsDirWith(t, coverageIDs...), testSpec, clean)

	assert.Equal(t, 0, code, out+errOut)
	var report struct {
		Coverage []recipecheck.ScenarioCoverage `json:"coverage"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &report), out)
	require.Len(t, report.Coverage, 6)
	assert.Equal(t, "US1-AS1", report.Coverage[0].Scenario)
	assert.Equal(t, "US5-AS5", report.Coverage[5].Scenario)
	proven := map[string]recipecheck.ScenarioCoverage{}
	for _, c := range report.Coverage {
		if c.Proven {
			proven[c.Scenario] = c
		}
	}
	assert.Len(t, proven, 3)
	first := proven["US1-AS1"]
	require.Len(t, first.Testcases, 1)
	assert.Equal(t, clean, first.Testcases[0].File)
	assert.Equal(t, "US1-AS1 the page shows its title and heading", first.Testcases[0].Testcase)
	assert.Equal(t, 12, first.Testcases[0].Line)
	assert.Contains(t, out, `"scenario":"US1-AS4","proven":false`)
}

// US2-AS1: a spec that cannot be found with --coverage is exit 2 with a message.
func TestCheckCoverageOfAMissingSpecExitsTwo(t *testing.T) {
	clean := filepath.Join(fixturesDir(t), "clean", "page.yml")

	code, out, errOut := runCheckWithCoverage(t, "text", repoSpecsDir(t), "999-absent", clean)

	assert.Equal(t, 2, code, out+errOut)
	assert.Contains(t, out+errOut, "999-absent")
}

// US2-AS1: --coverage does not change the exit status: a finding is still exit 1.
func TestCheckCoverageKeepsTheFindingsExitStatus(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one"), testcaseYAML("a name with no scenario id")))

	code, out, _ := runCheckWithCoverage(t, "text", specsDirWith(t, "US1-AS1"), testSpec, path)

	assert.Equal(t, 1, code, out)
	assert.Contains(t, out, "RC01")
}

// US2-AS1: the flag exists on the command and reaches the check (child process, as the command is run).
func TestRecipeCheckAcceptsTheCoverageFlag(t *testing.T) {
	clean := filepath.Join(fixturesDir(t), "clean", "page.yml")

	exit, out := recipeExit(t, "check", "--specs-dir", specsDirWith(t, coverageIDs...), "--coverage", testSpec, clean)

	assert.Equal(t, 0, exit, out)
	assert.Contains(t, out, "US1-AS4 not proven")
}

// US2-AS1: a testcase of a file whose suite names another spec does not prove the scenario of the spec asked for.
func TestCheckCoverageDoesNotCountTheFilesOfAnotherSpec(t *testing.T) {
	const other = "020-other-spec"
	dir := t.TempDir()
	path := writeSuite(t, dir, "b.yml", suiteYAMLOf(other, testcaseYAML("US1-AS1 one")))
	specs := specsDirOf(t, map[string][]string{testSpec: {"US1-AS1"}, other: {"US1-AS1"}})

	code, out, errOut := runCheckWithCoverage(t, "text", specs, testSpec, path)

	assert.Equal(t, 0, code, out+errOut)
	assert.Contains(t, out, "US1-AS1 not proven")
	assert.NotContains(t, out, " proven: ")
}

// US2-AS1: a scenario that is not proven is written with an empty list of testcases in JSON, never null.
func TestCheckCoverageJSONWritesAnEmptyListForANotProvenScenario(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	code, out, errOut := runCheckWithCoverage(t, "json", specsDirWith(t, "US1-AS1", "US1-AS2"), testSpec, path)

	assert.Equal(t, 0, code, out+errOut)
	assert.Contains(t, out, `"scenario":"US1-AS2","proven":false,"testcases":[]`)
	assert.NotContains(t, out, `"testcases":null`)
}

// US2-AS1: with a finding, the exit is 1 and the coverage is still printed, after the findings.
func TestCheckCoverageIsPrintedAfterTheFindings(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one"), testcaseYAML("a name with no scenario id")))

	code, out, _ := runCheckWithCoverage(t, "text", specsDirWith(t, "US1-AS1", "US1-AS2"), testSpec, path)

	assert.Equal(t, 1, code, out)
	require.Contains(t, out, "RC01")
	require.Contains(t, out, "US1-AS1 proven: "+path)
	assert.Less(t, strings.Index(out, "RC01"), strings.Index(out, "US1-AS1 proven"))
	assert.Contains(t, out, "US1-AS2 not proven")
}

// US2-AS1: a spec that holds no scenario id is said so in text, so that it does not look like a run without --coverage.
func TestCheckCoverageTextSaysTheSpecListsNoScenario(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	_, out, errOut := runCheckWithCoverage(t, "text", specsDirWith(t), testSpec, path)

	assert.Contains(t, out, "lists no scenario", out+errOut)
}

// US2-AS1: a spec that holds no scenario id keeps "coverage":[] in JSON when --coverage is given.
func TestCheckCoverageJSONKeepsAnEmptyCoverageListWhenAsked(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	_, out, errOut := runCheckWithCoverage(t, "json", specsDirWith(t), testSpec, path)

	assert.Contains(t, out, `"coverage":[]`, out+errOut)
}

// US2-AS1: without --coverage the JSON report has no coverage key.
func TestCheckJSONHasNoCoverageKeyWithoutTheFlag(t *testing.T) {
	dir := t.TempDir()
	path := writeSuite(t, dir, "a.yml", suiteYAML(testcaseYAML("US1-AS1 one")))

	code, out, errOut := runCheckIn(t, "json", specsDirWith(t, "US1-AS1"), path)

	assert.Equal(t, 0, code, out+errOut)
	assert.NotContains(t, out, `"coverage"`)
}
