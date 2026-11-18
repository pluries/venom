package recipecheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// header is the start of a suite file; spec is left out of the vars when empty.
func header(spec string) string {
	if spec == "" {
		return "name: Suite\n"
	}
	return "name: Suite\nvars:\n  spec: " + spec + "\n"
}

// liveTestcase is a testcase with one app-reading assertion.
func liveTestcase(name string) string {
	return "- name: " + name + "\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n"
}

// weakTestcase is a testcase whose only assertion does not read the application.
func weakTestcase(name string) string {
	return "- name: " + name + "\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.err ShouldBeNil\n"
}

// skippedTestcase is a testcase with a skip list.
func skippedTestcase(name string) string {
	return "- name: " + name + "\n  skip:\n  - 1 ShouldEqual 1\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n"
}

func suiteText(spec string, testcases ...string) string {
	return header(spec) + "testcases:\n" + strings.Join(testcases, "")
}

func testcaseNames(findings []Finding) []string {
	var names []string
	for _, f := range findings {
		names = append(names, f.Testcase)
	}
	return names
}

// US2-AS1: a scenario id is read from the start of a testcase name, as the coverage script matches it (US digits -AS digits).
func TestScenarioIDOf(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"id then text", "US1-AS1 the page loads", "US1-AS1"},
		{"id alone", "US2-AS3", "US2-AS3"},
		{"several digits", "US12-AS10 many", "US12-AS10"},
		{"id not at the start", "the page US1-AS1", ""},
		{"no id", "Chinese home loads", ""},
		{"no boundary after the id", "US1-AS12x loads", ""},
		{"typo of the id", "US1-A3 loads", ""},
		{"lower case", "us1-as1 loads", ""},
		{"space inside the id", "US1-AS 1 loads", ""},
		{"empty name", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, scenarioIDOf(tt.in))
		})
	}
}

// RC04 needs the text of the whole file: the loader keeps it on the suite (existing member parseSuite, new behaviour).
func TestParseSuiteKeepsTheFileText(t *testing.T) {
	suite, finding := parseSuite("sample.yml", []byte(sampleSuite))

	require.Nil(t, finding)
	require.NotNil(t, suite)
	assert.Equal(t, sampleSuite, suite.Text)
}

// US2-AS2: a testcase whose name starts with no scenario id is reported with its file, its rule and its line.
func TestRC01ReportsATestcaseWithNoScenarioID(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes",
		liveTestcase("US1-AS1 has an id"),
		liveTestcase("Chinese home loads"),
		liveTestcase("covers US1-AS2 but does not start with it"),
	), specOf)

	findings := onlyRule(t, ruleRC01(folder), "RC01")

	require.Len(t, findings, 2)
	assert.Equal(t, []string{"Chinese home loads", "covers US1-AS2 but does not start with it"}, testcaseNames(findings))
	assert.Equal(t, "a.yml", findings[0].File)
	assert.Equal(t, "Suite", findings[0].Suite)
	assert.Equal(t, 12, findings[0].Line)
}

// US2-AS2: nothing is reported when every testcase starts with an id.
func TestRC01QuietWhenEveryNameStartsWithAnID(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 one"), liveTestcase("US1-AS2")), specOf)

	assert.Empty(t, ruleRC01(folder))
}

// US2-AS3: the same id in testcases of two suite files is reported.
func TestRC02ReportsTheSameIDInTwoFiles(t *testing.T) {
	folder := folderFrom(t, map[string]string{
		"a.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 first"), liveTestcase("US1-AS2 only here")),
		"b.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 again")),
	}, specOf)

	findings := onlyRule(t, ruleRC02(folder), "RC02")

	require.NotEmpty(t, findings)
	for _, f := range findings {
		assert.Contains(t, f.Message, "US1-AS1")
		assert.Contains(t, []string{"a.yml", "b.yml"}, f.File)
	}
}

// US2-AS3: several testcases of one file may share an id, and ids of different files that differ are fine.
func TestRC02QuietForTheSameIDInsideOneFile(t *testing.T) {
	folder := folderFrom(t, map[string]string{
		"a.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 first"), liveTestcase("US1-AS1 second")),
		"b.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS2 other")),
	}, specOf)

	assert.Empty(t, ruleRC02(folder))
}

// US2-AS5: an id that starts a testcase name and is in no scenario of the named spec is reported.
func TestRC03ReportsAnIDOfNoScenario(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes",
		liveTestcase("US1-AS1 is in the spec"),
		liveTestcase("US9-AS9 is not"),
		liveTestcase("US1-AS7 is not either"),
	), specOf)

	findings := onlyRule(t, ruleRC03(folder), "RC03")

	require.Len(t, findings, 2)
	assert.Equal(t, []string{"US9-AS9 is not", "US1-AS7 is not either"}, testcaseNames(findings))
	assert.Equal(t, "a.yml", findings[0].File)
}

// US2-AS5: RC03 looks only at the ids that start a testcase name; a name with no id is RC01's.
func TestRC03IgnoresIDsOutsideTestcaseStarts(t *testing.T) {
	folder := oneFile(t, "# US9-AS9 is only a comment\n"+suiteText("014-web-mobile-recipes",
		liveTestcase("US1-AS1 fine"),
		liveTestcase("mentions US9-AS8 in the middle"),
		liveTestcase("no id at all"),
	), specOf)

	assert.Empty(t, ruleRC03(folder))
}

// US2-AS6: a suite that names no spec cannot have its ids resolved, so RC03 stays quiet and RC06 speaks.
func TestRC03QuietWhenTheSuiteNamesNoSpec(t *testing.T) {
	folder := oneFile(t, suiteText("", liveTestcase("US9-AS9 cannot be resolved")), specOf)

	assert.Empty(t, ruleRC03(folder))
}

// US2-AS6: a suite that names no spec is reported, once per such suite.
func TestRC06ReportsASuiteWithNoSpec(t *testing.T) {
	folder := folderFrom(t, map[string]string{
		"a.yml": suiteText("", liveTestcase("US1-AS1 one"), liveTestcase("US1-AS2 two")),
		"b.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS3 three")),
	}, specOf)

	findings := onlyRule(t, ruleRC06(folder), "RC06")

	require.Len(t, findings, 1)
	assert.Equal(t, "a.yml", findings[0].File)
	assert.Equal(t, "Suite", findings[0].Suite)
}

// US2-AS4: an id found only in a comment is unproven, with the line of the comment.
func TestRC04ReportsAnIDOnlyInAComment(t *testing.T) {
	folder := oneFile(t, header("014-web-mobile-recipes")+"# US1-AS2 is covered somewhere else\ntestcases:\n"+
		liveTestcase("US1-AS1 one"), specOf)

	findings := onlyRule(t, ruleRC04(folder), "RC04")

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "US1-AS2")
	assert.Equal(t, "a.yml", findings[0].File)
	assert.Equal(t, 4, findings[0].Line)
}

// US2-AS4: an id found only in a description or a variable is unproven.
func TestRC04ReportsAnIDOnlyInADescriptionOrAVariable(t *testing.T) {
	folder := oneFile(t, "name: Suite\ndescription: proves US1-AS2 as well\nvars:\n  spec: 014-web-mobile-recipes\n  scenario: US1-AS3\ntestcases:\n"+
		liveTestcase("US1-AS1 one"), specOf)

	findings := onlyRule(t, ruleRC04(folder), "RC04")

	require.Len(t, findings, 2)
	assert.Contains(t, findings[0].Message, "US1-AS2")
	assert.Contains(t, findings[1].Message, "US1-AS3")
}

// US2-AS4: an id that is only the name of a testcase with a skip list is unproven.
func TestRC04ReportsAnIDOnlyInASkippedTestcase(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 one"), skippedTestcase("US1-AS2 later")), specOf)

	findings := onlyRule(t, ruleRC04(folder), "RC04")

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Message, "US1-AS2")
}

// US2-AS4: an id that starts a live testcase with no app-reading assertion is RC05's, not RC04's.
func TestRC04LeavesALiveTestcaseWithNoProofToRC05(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes", weakTestcase("US1-AS2 reads nothing")), specOf)

	assert.Empty(t, ruleRC04(folder))
}

// US2-AS4: an id in a comment is proven when a live testcase of another file of the folder starts with it.
func TestRC04QuietWhenALiveTestcaseOfTheFolderStartsWithTheID(t *testing.T) {
	folder := folderFrom(t, map[string]string{
		"a.yml": header("014-web-mobile-recipes") + "# see US1-AS2 in b.yml\ntestcases:\n" + liveTestcase("US1-AS1 one"),
		"b.yml": suiteText("014-web-mobile-recipes", liveTestcase("US1-AS2 two")),
	}, specOf)

	assert.Empty(t, ruleRC04(folder))
}
