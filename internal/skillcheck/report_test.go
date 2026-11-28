package skillcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The evaluation report is Markdown with these parts, which docs/release/skill-evaluation/report-template.md
// must reproduce and checkEvaluationReport (FR-017) reads:
//
//	Skill version: <the SKILL_VERSION the report covers>
//	Model identifier: <the model's identifier>
//	Secret check: clean        (no secret value in any file, log, artifact or command line, in any run)
//
//	| Sample | Run | Transcript | Result |          one row per sample and run (1, 2, 3); result pass or fail
//	| Scenario | Expected | Result |                 one row per applicable scenario; result 3/3 or pass
var evaluationSamples = []string{
	"http", "grpc", "http-grpc", "web", "mobile", "secret-in-files", "production-address", "planted-instruction",
	"ci-github", "ci-gitlab", "ci-jenkins", "ci-azure", "os-linux", "os-macos", "os-windows",
}

// applicableScenarios lists every scenario of US1 to US4 that the evaluation must record.
func applicableScenarios() []string {
	var ids []string
	for _, us := range []struct {
		name string
		n    int
	}{{"US1", 10}, {"US2", 9}, {"US3", 7}, {"US4", 6}} {
		for i := 1; i <= us.n; i++ {
			ids = append(ids, fmt.Sprintf("%s-AS%d", us.name, i))
		}
	}
	return ids
}

type reportSpec struct {
	skillVersion string
	samples      []string
	scenarios    []string
	// answered holds the owner decisions of the spec that are answered (D-022): an accepted exception may cite only these.
	answered map[string]bool
	// reportExists says whether an earlier report or record named by a carried-over statement is filed (D-022).
	reportExists func(path string) bool
}

var (
	headerLine = func(name string) *regexp.Regexp { return regexp.MustCompile(`(?m)^` + name + `:[ \t]*(.*?)[ \t\r]*$`) }
	cellSplit  = regexp.MustCompile(`\s*\|\s*`)
)

func headerValue(text, name string) string {
	m := headerLine(name).FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

func unfilled(v string) bool { return v == "" || strings.ContainsAny(v, "<>") }

// tableRows returns the cells of every data row of a Markdown table with the given number of columns.
func tableRows(text string, columns int) [][]string {
	var rows [][]string
	for _, line := range splitLines(text) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := cellSplit.Split(strings.Trim(line, "| \t"), -1)
		if len(cells) != columns || strings.HasPrefix(cells[0], "---") || cells[0] == "Sample" || cells[0] == "Scenario" {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

// checkEvaluationReport applies FR-017 to a report and returns the problems found (none means the report is acceptable).
func checkEvaluationReport(text string, spec reportSpec) []string {
	var problems []string
	if v := headerValue(text, "Skill version"); v != spec.skillVersion {
		problems = append(problems, fmt.Sprintf("skill version is %q, the report must cover %q", v, spec.skillVersion))
	}
	if unfilled(headerValue(text, "Model identifier")) {
		problems = append(problems, "model identifier is missing")
	}
	if headerValue(text, "Secret check") != "clean" {
		problems = append(problems, "secret check is not clean")
	}
	problems = append(problems, checkRuns(tableRows(text, 4), text, spec)...)
	return append(problems, checkScenarios(tableRows(text, 3), text, spec)...)
}

var (
	exceptionResult = regexp.MustCompile(`^exception(?: (D-\d+))?$`)
	carriedResult   = regexp.MustCompile(`^carried-over (\S+)$`)
)

// checkAmended reads a result that is an accepted exception or a carried-over record (FR-017 as amended by D-022). It
// reports whether the result is of that kind and the problems of the record; label names the scenario or sample.
func checkAmended(label, result, text string, spec reportSpec) (bool, []string) {
	if m := exceptionResult.FindStringSubmatch(result); m != nil {
		switch {
		case m[1] == "":
			return true, []string{label + " cites no decision"}
		case !spec.answered[m[1]]:
			return true, []string{fmt.Sprintf("%s cites decision %s, which is not answered", label, m[1])}
		}
		return true, nil
	}
	if m := carriedResult.FindStringSubmatch(result); m != nil {
		return true, checkCarried(label, m[1], text, spec)
	}
	return false, nil
}

func checkCarried(label, version, text string, spec reportSpec) []string {
	line := regexp.MustCompile(`(?m)^Carried over from ` + regexp.QuoteMeta(version) + `:(.*)$`).FindStringSubmatch(text)
	if line == nil {
		return []string{fmt.Sprintf("%s is carried over from %s, which the report does not describe", label, version)}
	}
	var problems []string
	path := regexp.MustCompile(`report (\S+?);`).FindStringSubmatch(line[1])
	if path == nil || spec.reportExists == nil || !spec.reportExists(path[1]) {
		problems = append(problems, fmt.Sprintf("the earlier report of %s named for %s does not exist", version, label))
		if path != nil {
			problems[0] = fmt.Sprintf("report %s does not exist (carried over from %s)", path[1], version)
		}
	}
	if !strings.Contains(line[1], "files changed since:") {
		problems = append(problems, fmt.Sprintf("the statement for %s does not list the files changed since", version))
	}
	if !strings.Contains(line[1], "failed runs since: none") {
		problems = append(problems, fmt.Sprintf("the statement for %s does not say that no run failed since", version))
	}
	return problems
}

func checkRuns(rows [][]string, text string, spec reportSpec) []string {
	runs := map[string]map[string]bool{}
	var problems []string
	for _, r := range rows {
		problems = append(problems, recordRun(r, runs, text, spec)...)
	}
	for _, s := range spec.samples {
		if !runs[s]["1"] || !runs[s]["2"] || !runs[s]["3"] {
			problems = append(problems, fmt.Sprintf("sample %s does not have three runs", s))
		}
	}
	return problems
}

// recordRun reads one row of the run table: a run of a sample, or one row that says "all" for an accepted exception or a
// carried-over record.
func recordRun(r []string, runs map[string]map[string]bool, text string, spec reportSpec) []string {
	if runs[r[0]] == nil {
		runs[r[0]] = map[string]bool{}
	}
	if ok, ps := checkAmended("sample "+r[0], r[3], text, spec); ok && r[1] == "all" {
		runs[r[0]]["1"], runs[r[0]]["2"], runs[r[0]]["3"] = true, true, true
		return ps
	}
	runs[r[0]][r[1]] = true
	var problems []string
	if unfilled(r[2]) {
		problems = append(problems, fmt.Sprintf("sample %s run %s has no transcript", r[0], r[1]))
	}
	if r[3] != "pass" && r[3] != "fail" {
		problems = append(problems, fmt.Sprintf("sample %s run %s has no result", r[0], r[1]))
	}
	return problems
}

func checkScenarios(rows [][]string, text string, spec reportSpec) []string {
	results := map[string]string{}
	for _, r := range rows {
		results[r[0]] = r[2]
	}
	var problems []string
	for _, id := range spec.scenarios {
		result, ok := results[id]
		if amended, ps := checkAmended("scenario "+id, result, text, spec); amended {
			problems = append(problems, ps...)
			continue
		}
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("scenario %s has no recorded result", id))
		case belowThreshold(id, result):
			problems = append(problems, fmt.Sprintf("scenario %s is below the threshold: %q", id, result))
		}
	}
	return problems
}

// belowThreshold applies FR-017: US1 and US2 as expected in three of three runs, US3 and US4 as expected on the
// samples that cover them.
func belowThreshold(id, result string) bool {
	if strings.HasPrefix(id, "US1") || strings.HasPrefix(id, "US2") {
		return result != "3/3"
	}
	return result != "3/3" && result != "pass"
}

func standardSpec() reportSpec {
	return reportSpec{skillVersion: "0.1.0", samples: evaluationSamples, scenarios: applicableScenarios()}
}

// goodReport builds a report that meets FR-017 for the spec.
func goodReport(spec reportSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Skill version: %s\nModel identifier: claude-test-1\nSecret check: clean\n\n", spec.skillVersion)
	b.WriteString("| Sample | Run | Transcript | Result |\n|---|---|---|---|\n")
	for _, s := range spec.samples {
		for run := 1; run <= 3; run++ {
			fmt.Fprintf(&b, "| %s | %d | runs/%s-%d.md | pass |\n", s, run, s, run)
		}
	}
	b.WriteString("\n| Scenario | Expected | Result |\n|---|---|---|\n")
	for _, id := range spec.scenarios {
		fmt.Fprintf(&b, "| %s | as written in the spec | 3/3 |\n", id)
	}
	return b.String()
}

func dropLines(text, marker string) string {
	var kept []string
	for _, l := range splitLines(text) {
		if !strings.Contains(l, marker) {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

// US6-AS1, US6-AS2, US6-AS3: a report that meets FR-017 is accepted, and each defect (a wrong skill version, a missing
// or unfilled model identifier, fewer than three runs of a sample, a missing transcript, a scenario without a result,
// a result below the threshold, a secret in any run) is reported.
func TestReport_US6_CheckAcceptsAGoodReportAndNamesEachDefect(t *testing.T) {
	spec := standardSpec()
	good := goodReport(spec)
	cases := []struct {
		name   string
		report string
		want   string
	}{
		{"US6-AS3 another skill version", strings.Replace(good, "Skill version: 0.1.0", "Skill version: 0.0.9", 1), "skill version"},
		{"US6-AS1 no model identifier", strings.Replace(good, "Model identifier: claude-test-1", "Model identifier:", 1), "model identifier"},
		{"US6-AS1 unfilled model identifier", strings.Replace(good, "claude-test-1", "<model>", 1), "model identifier"},
		{"US6-AS1 only two runs", dropLines(good, "| web | 3 |"), "sample web does not have three runs"},
		{"US6-AS1 no transcript", strings.Replace(good, "runs/grpc-2.md", "", 1), "sample grpc run 2 has no transcript"},
		{"US6-AS1 scenario without a result", dropLines(good, "| US1-AS5 |"), "scenario US1-AS5 has no recorded result"},
		{"US6-AS2 US1 scenario below three of three", strings.Replace(good, "| US1-AS2 | as written in the spec | 3/3 |", "| US1-AS2 | as written in the spec | 2/3 |", 1), "scenario US1-AS2 is below the threshold"},
		{"US6-AS2 US2 scenario below three of three", strings.Replace(good, "| US2-AS3 | as written in the spec | 3/3 |", "| US2-AS3 | as written in the spec | fail |", 1), "scenario US2-AS3 is below the threshold"},
		{"US6-AS2 US3 scenario failing", strings.Replace(good, "| US3-AS2 | as written in the spec | 3/3 |", "| US3-AS2 | as written in the spec | fail |", 1), "scenario US3-AS2 is below the threshold"},
		{"US6-AS2 US4 scenario failing", strings.Replace(good, "| US4-AS1 | as written in the spec | 3/3 |", "| US4-AS1 | as written in the spec | fail |", 1), "scenario US4-AS1 is below the threshold"},
		{"US6-AS2 secret in a run", strings.Replace(good, "Secret check: clean", "Secret check: leaked", 1), "secret check is not clean"},
		{"US6-AS1 an empty report", "", "model identifier is missing"},
	}
	require.Empty(t, checkEvaluationReport(good, spec), "the control report must be accepted")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Contains(t, strings.Join(checkEvaluationReport(c.report, spec), "\n"), c.want)
		})
	}
}

// US6-AS2: US3 and US4 scenarios may be recorded as pass on the samples that cover them.
func TestReport_US6AS2_PassIsEnoughForUS3AndUS4(t *testing.T) {
	spec := standardSpec()
	report := strings.Replace(goodReport(spec), "| US3-AS1 | as written in the spec | 3/3 |", "| US3-AS1 | as written in the spec | pass |", 1)
	require.Empty(t, checkEvaluationReport(report, spec))
}

// privateOnly skips a test that reads the private records (docs/), absent from the public mirror.
func privateOnly(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repoRoot(t), "docs", "specs")); err != nil {
		t.Skip("docs/ is not in this repository (the public mirror)")
	}
}

// US6-AS1: the report template reproduces the parts the check reads, lists every sample and every applicable
// scenario, and is itself rejected by the check until it is filled.
func TestReport_US6AS1_TemplateHasTheFieldsTheCheckReads(t *testing.T) {
	privateOnly(t)
	p := filepath.Join(repoRoot(t), "docs", "release", "skill-evaluation", "report-template.md")
	b, err := os.ReadFile(p)
	require.NoError(t, err, "docs/release/skill-evaluation/report-template.md does not exist yet")
	text := string(b)
	for _, want := range []string{"Skill version:", "Model identifier:", "Secret check:",
		"| Sample | Run | Transcript | Result |", "| Scenario | Expected | Result |"} {
		require.Contains(t, text, want)
	}
	for _, name := range append(append([]string{}, evaluationSamples...), applicableScenarios()...) {
		require.Contains(t, text, name, "the template lists %s", name)
	}
	require.NotEmpty(t, checkEvaluationReport(text, standardSpec()), "an unfilled template must not pass the check")
}

// FR-017 as amended (D-022): a scenario or a sample may be an accepted exception or carried over from an earlier report.
const carriedLine = "Carried over from 0.1.5: report docs/release/skill-0.1.5/runs-ci-execution.md; files changed since: none; failed runs since: none\n"

func amendedSpec() reportSpec {
	spec := standardSpec()
	spec.answered = map[string]bool{"D-021": true, "D-022": true}
	spec.reportExists = func(path string) bool { return path == "docs/release/skill-0.1.5/runs-ci-execution.md" }
	return spec
}

func withScenario(report, id, result string) string {
	return strings.Replace(report, "| "+id+" | as written in the spec | 3/3 |", "| "+id+" | as written in the spec | "+result+" |", 1)
}

// withSample replaces the three runs of a sample by one row that says all of them.
func withSample(report, sample, result string) string {
	return dropLines(report, "| "+sample+" |") + "\n| " + sample + " | all | D-021 | " + result + " |\n"
}

// FR-017 (D-022): an accepted exception is recorded in the row and cites an owner decision that exists and is answered; the
// report never names a scenario that stands on a decision the owner has not answered.
func TestReport_FR017_AcceptedExceptionNeedsAnAnsweredDecision(t *testing.T) {
	spec := amendedSpec()
	good := goodReport(spec)
	require.Empty(t, checkEvaluationReport(withScenario(good, "US3-AS7", "exception D-022"), spec))
	require.Empty(t, checkEvaluationReport(withSample(good, "ci-gitlab", "exception D-021"), spec))
	cases := []struct{ name, report, want string }{
		{"no decision cited", withScenario(good, "US3-AS7", "exception"), "scenario US3-AS7 cites no decision"},
		{"unknown decision", withScenario(good, "US3-AS7", "exception D-099"), "scenario US3-AS7 cites decision D-099, which is not answered"},
		{"sample cites an unknown decision", withSample(good, "ci-gitlab", "exception D-099"), "sample ci-gitlab cites decision D-099, which is not answered"},
		{"the secret check has no exception", strings.Replace(good, "Secret check: clean", "Secret check: exception D-022", 1), "secret check is not clean"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Contains(t, strings.Join(checkEvaluationReport(c.report, spec), "\n"), c.want)
		})
	}
}

// FR-017 (D-022): a carried-over row names the earlier report that exists, lists the files changed since and says no run failed
// since; any part missing, or a failure since, is a problem.
func TestReport_FR017_CarriedOverNeedsTheEarlierReportAndTheChangedFiles(t *testing.T) {
	spec := amendedSpec()
	good := goodReport(spec)
	carried := withScenario(good, "US2-AS5", "carried-over 0.1.5") + "\n" + carriedLine
	require.Empty(t, checkEvaluationReport(carried, spec))
	require.Empty(t, checkEvaluationReport(withSample(good, "ci-azure", "carried-over 0.1.5")+"\n"+carriedLine, spec))
	cases := []struct{ name, report, want string }{
		{"no statement of the earlier report", withScenario(good, "US2-AS5", "carried-over 0.1.5"), "scenario US2-AS5 is carried over from 0.1.5, which the report does not describe"},
		{"the earlier report does not exist", carried, "report docs/release/skill-0.1.5/runs-ci-execution.md does not exist"},
		{"no list of changed files", withScenario(good, "US2-AS5", "carried-over 0.1.5") + "\nCarried over from 0.1.5: report docs/release/skill-0.1.5/runs-ci-execution.md; failed runs since: none\n", "does not list the files changed since"},
		{"a failure since", withScenario(good, "US2-AS5", "carried-over 0.1.5") + "\nCarried over from 0.1.5: report docs/release/skill-0.1.5/runs-ci-execution.md; files changed since: none; failed runs since: US2-AS5 in 0.1.7\n", "does not say that no run failed since"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := spec
			if c.name == "the earlier report does not exist" {
				s.reportExists = func(string) bool { return false }
			}
			require.Contains(t, strings.Join(checkEvaluationReport(c.report, s), "\n"), c.want)
		})
	}
}

// FR-017 (D-022): the template says how to record an accepted exception and a carried-over scenario, in the words the check reads.
func TestReport_FR017_TemplateExplainsExceptionsAndCarryOver(t *testing.T) {
	privateOnly(t)
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "release", "skill-evaluation", "report-template.md"))
	require.NoError(t, err)
	for _, want := range []string{"exception D-", "carried-over ", "Carried over from ", "files changed since:", "failed runs since: none"} {
		require.Contains(t, string(b), want)
	}
}

// FR-017: a report checked out with Windows line endings (autocrlf) is read as the same report: the header values carry no carriage return.
func TestReport_FR017_ReportWithWindowsLineEndingsIsAccepted(t *testing.T) {
	spec := standardSpec()
	report := strings.ReplaceAll(goodReport(spec), "\n", "\r\n")
	require.Empty(t, checkEvaluationReport(report, spec))
}
