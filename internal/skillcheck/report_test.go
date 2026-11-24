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
}

var (
	headerLine = func(name string) *regexp.Regexp { return regexp.MustCompile(`(?m)^` + name + `:[ \t]*(.*?)[ \t]*$`) }
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
	problems = append(problems, checkRuns(tableRows(text, 4), spec.samples)...)
	return append(problems, checkScenarios(tableRows(text, 3), spec.scenarios)...)
}

func checkRuns(rows [][]string, samples []string) []string {
	runs := map[string]map[string]bool{}
	var problems []string
	for _, r := range rows {
		if runs[r[0]] == nil {
			runs[r[0]] = map[string]bool{}
		}
		runs[r[0]][r[1]] = true
		if unfilled(r[2]) {
			problems = append(problems, fmt.Sprintf("sample %s run %s has no transcript", r[0], r[1]))
		}
		if r[3] != "pass" && r[3] != "fail" {
			problems = append(problems, fmt.Sprintf("sample %s run %s has no result", r[0], r[1]))
		}
	}
	for _, s := range samples {
		if !runs[s]["1"] || !runs[s]["2"] || !runs[s]["3"] {
			problems = append(problems, fmt.Sprintf("sample %s does not have three runs", s))
		}
	}
	return problems
}

func checkScenarios(rows [][]string, scenarios []string) []string {
	results := map[string]string{}
	for _, r := range rows {
		results[r[0]] = r[2]
	}
	var problems []string
	for _, id := range scenarios {
		result, ok := results[id]
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
