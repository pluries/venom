package recipecheck

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// folderFrom builds the Folder a rule sees from suite files given as YAML text, keyed by file name, and from the
// scenario ids of the specs they name (spec folder name to ids). The files are read in sorted order, as Check does.
func folderFrom(t *testing.T, files map[string]string, specs map[string][]string) *Folder {
	t.Helper()
	folder := &Folder{Specs: map[string]map[string]bool{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		suite, finding := parseSuite(name, []byte(files[name]))
		require.Nil(t, finding, "the test YAML %s does not parse", name)
		require.NotNil(t, suite)
		suite.Text = files[name]
		folder.Suites = append(folder.Suites, suite)
	}
	for spec, ids := range specs {
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		folder.Specs[spec] = set
	}
	return folder
}

// oneFile is folderFrom for a single file a.yml.
func oneFile(t *testing.T, text string, specs map[string][]string) *Folder {
	t.Helper()
	return folderFrom(t, map[string]string{"a.yml": text}, specs)
}

// specOf is the spec used by the rule tests, with the ids of its scenarios.
var specOf = map[string][]string{"014-web-mobile-recipes": {"US1-AS1", "US1-AS2", "US1-AS3", "US2-AS1"}}

// suiteWithAssertions is a one-testcase suite whose web step carries the given assertions, one per line, from line
// firstAssertionLine on (see assertionLine).
func suiteWithAssertions(assertions ...string) string {
	lines := []string{
		"name: Suite",                    // 1
		"vars:",                          // 2
		"  spec: 014-web-mobile-recipes", // 3
		"testcases:",                     // 4
		"- name: US1-AS1 one",            // 5
		"  steps:",                       // 6
		"  - type: web",                  // 7
		"    action:",                    // 8
		"      find: h1",                 // 9
		"    assertions:",                // 10
	}
	for _, a := range assertions {
		lines = append(lines, "    - "+yamlQuote(a))
	}
	return strings.Join(lines, "\n") + "\n"
}

// assertionLine is the line of the i-th assertion (from 0) of suiteWithAssertions.
func assertionLine(i int) int { return 11 + i }

// suiteWithStep is a one-testcase suite whose web step is the given YAML (already indented by 4 spaces after the
// "- type: web" line), plus a good assertion so that RC05 stays quiet.
func suiteWithStep(stepYAML string) string {
	return "name: Suite\nvars:\n  spec: 014-web-mobile-recipes\ntestcases:\n- name: US1-AS1 one\n  steps:\n  - type: web\n" +
		stepYAML + "    assertions:\n    - result.find ShouldEqual 1\n"
}

// yamlQuote writes s as a single-quoted YAML scalar.
func yamlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// onlyRule fails the test unless every finding is of the given rule, and returns them.
func onlyRule(t *testing.T, findings []Finding, rule string) []Finding {
	t.Helper()
	for _, f := range findings {
		require.Equal(t, rule, f.Rule, "unexpected finding: %+v", f)
	}
	return findings
}
