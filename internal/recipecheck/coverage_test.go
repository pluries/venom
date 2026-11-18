package recipecheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const coverageSpec = "014-web-mobile-recipes"

var coverageScenarios = []string{"US1-AS1", "US1-AS2", "US1-AS3", "US1-AS4", "US2-AS1"}

func coverageTestcase(name, extra, assertion string) string {
	return "- name: " + name + "\n" + extra + "  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - " +
		assertion + "\n"
}

func coverageSuite(testcases ...string) string {
	return coverageSuiteOf(coverageSpec, testcases...)
}

// coverageSuiteOf is a suite whose var spec names the given spec.
func coverageSuiteOf(spec string, testcases ...string) string {
	text := "name: Suite\nvars:\n  spec: " + spec + "\ntestcases:\n"
	for _, tc := range testcases {
		text += tc
	}
	return text
}

const otherSpec = "020-other-spec"

// coverageForSpecs is the coverage of coverageSpec in a folder whose specs all list US1-AS1.
func coverageForSpecs(t *testing.T, files map[string]string, specs ...string) map[string]ScenarioCoverage {
	t.Helper()
	known := map[string][]string{}
	for _, s := range specs {
		known[s] = []string{"US1-AS1"}
	}
	byID := map[string]ScenarioCoverage{}
	for _, c := range Coverage(folderFrom(t, files, known), coverageSpec) {
		byID[c.Scenario] = c
	}
	return byID
}

func coverageOf(t *testing.T, files map[string]string) map[string]ScenarioCoverage {
	t.Helper()
	got := Coverage(folderFrom(t, files, map[string][]string{coverageSpec: coverageScenarios}), coverageSpec)
	byID := map[string]ScenarioCoverage{}
	for _, c := range got {
		byID[c.Scenario] = c
	}
	return byID
}

// US2-AS1: every scenario of the spec is listed, in spec order, proven or not.
func TestCoverageListsEveryScenarioInSpecOrder(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS3 three", "", "result.find ShouldEqual 1"))

	got := Coverage(oneFile(t, text, map[string][]string{coverageSpec: coverageScenarios}), coverageSpec)

	var ids []string
	for _, c := range got {
		ids = append(ids, c.Scenario)
	}
	assert.Equal(t, coverageScenarios, ids)
}

// US2-AS1: a live testcase with an app-reading assertion proves its scenario, with file, testcase and line.
func TestCoverageProvenScenarioNamesFileTestcaseAndLine(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1"))

	byID := coverageOf(t, map[string]string{"a.yml": text})

	require.True(t, byID["US1-AS1"].Proven)
	assert.Equal(t, []CoveringTestcase{{File: "a.yml", Testcase: "US1-AS1 one", Line: 5}}, byID["US1-AS1"].Testcases)
	assert.False(t, byID["US1-AS2"].Proven)
	assert.Empty(t, byID["US1-AS2"].Testcases)
}

// US2-AS1: a testcase with a skip list does not prove.
func TestCoverageSkippedTestcaseDoesNotProve(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS1 one", "  skip:\n  - 1 ShouldEqual 1\n", "result.find ShouldEqual 1"))

	byID := coverageOf(t, map[string]string{"a.yml": text})

	assert.False(t, byID["US1-AS1"].Proven)
	assert.Empty(t, byID["US1-AS1"].Testcases)
}

// US2-AS1: a live testcase with no app-reading assertion does not prove.
func TestCoverageTestcaseWithoutAppReadingAssertionDoesNotProve(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS1 one", "", "result.timeseconds ShouldBeLessThan 5"))

	byID := coverageOf(t, map[string]string{"a.yml": text})

	assert.False(t, byID["US1-AS1"].Proven)
	assert.Empty(t, byID["US1-AS1"].Testcases)
}

// US2-AS1: every testcase that starts with the id appears, across files, in file then testcase order.
func TestCoverageListsEveryTestcaseOfTheSameId(t *testing.T) {
	first := coverageSuite(
		coverageTestcase("US1-AS2 first", "", "result.find ShouldEqual 1"),
		coverageTestcase("US1-AS2 second", "", "result.text ShouldEqual x"),
	)
	second := coverageSuite(coverageTestcase("US1-AS2 third", "", "result.find ShouldEqual 1"))

	byID := coverageOf(t, map[string]string{"a.yml": first, "b.yml": second})

	require.True(t, byID["US1-AS2"].Proven)
	var names []string
	for _, tc := range byID["US1-AS2"].Testcases {
		names = append(names, tc.File+":"+tc.Testcase)
	}
	assert.Equal(t, []string{"a.yml:US1-AS2 first", "a.yml:US1-AS2 second", "b.yml:US1-AS2 third"}, names)
}

// US2-AS1: the id is matched as a whole, US1-AS1 is not proven by a testcase of US1-AS10.
func TestCoverageIdIsMatchedAsAWholeId(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS10 ten", "", "result.find ShouldEqual 1"))

	byID := coverageOf(t, map[string]string{"a.yml": text})

	assert.False(t, byID["US1-AS1"].Proven)
}

// US2-AS1: a testcase of a file whose suite names another spec does not prove a scenario of the spec asked for.
func TestCoverageListsOnlyTheFilesOfTheSpecAskedFor(t *testing.T) {
	tc := coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")
	files := map[string]string{"a.yml": coverageSuiteOf(coverageSpec, tc), "b.yml": coverageSuiteOf(otherSpec, tc)}

	byID := coverageForSpecs(t, files, coverageSpec, otherSpec)

	require.True(t, byID["US1-AS1"].Proven)
	assert.Equal(t, []CoveringTestcase{{File: "a.yml", Testcase: "US1-AS1 one", Line: 5}}, byID["US1-AS1"].Testcases)
}

// US2-AS1: the testcase of another spec alone leaves the scenario not proven.
func TestCoverageTestcaseOfAnotherSpecAloneDoesNotProve(t *testing.T) {
	tc := coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")

	byID := coverageForSpecs(t, map[string]string{"b.yml": coverageSuiteOf(otherSpec, tc)}, coverageSpec, otherSpec)

	assert.False(t, byID["US1-AS1"].Proven)
	assert.Empty(t, byID["US1-AS1"].Testcases)
}

// US2-AS1: a file whose spec cannot be found does not prove a scenario of the spec asked for.
func TestCoverageTestcaseOfASpecThatCannotBeFoundDoesNotProve(t *testing.T) {
	tc := coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")

	byID := coverageForSpecs(t, map[string]string{"c.yml": coverageSuiteOf("999-absent", tc)}, coverageSpec)

	assert.False(t, byID["US1-AS1"].Proven)
	assert.Empty(t, byID["US1-AS1"].Testcases)
}

// US2-AS1: a scenario that is not proven has an empty, not nil, list of testcases (JSON writes [] and not null).
func TestCoverageUnprovenScenarioHasAnEmptyNonNilTestcaseList(t *testing.T) {
	byID := coverageOf(t, map[string]string{"a.yml": coverageSuite()})

	require.False(t, byID["US1-AS2"].Proven)
	assert.NotNil(t, byID["US1-AS2"].Testcases)
	assert.Empty(t, byID["US1-AS2"].Testcases)
}

// US2-AS1: a spec the folder does not know lists nothing.
func TestCoverageOfAnUnknownSpecIsEmpty(t *testing.T) {
	text := coverageSuite(coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1"))

	got := Coverage(oneFile(t, text, map[string][]string{coverageSpec: coverageScenarios}), "999-absent")

	assert.Empty(t, got)
}

// US2-AS1: Check fills the report's coverage from the same folder and the specs dir, and leaves the findings alone.
func TestCheckFillsTheCoverageWhenAsked(t *testing.T) {
	dir := t.TempDir()
	specs := filepath.Join(dir, "specs")
	require.NoError(t, os.MkdirAll(filepath.Join(specs, coverageSpec), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(specs, coverageSpec, "spec.md"), []byte("US1-AS1\nUS1-AS2\n"), 0o600))
	path := writeFile(t, dir, "a.yml", coverageSuite(coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")))

	report, err := Check([]string{path}, Options{SpecsDir: specs, Coverage: coverageSpec})

	require.NoError(t, err)
	assert.Empty(t, report.Findings)
	require.Len(t, report.Coverage, 2)
	assert.Equal(t, "US1-AS1", report.Coverage[0].Scenario)
	assert.True(t, report.Coverage[0].Proven)
	assert.Equal(t, path, report.Coverage[0].Testcases[0].File)
	assert.False(t, report.Coverage[1].Proven)
}

// US2-AS1: without the option the report has no coverage.
func TestCheckHasNoCoverageByDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.yml", coverageSuite(coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")))

	report, err := Check([]string{path}, Options{SpecsDir: dir})

	require.NoError(t, err)
	assert.Nil(t, report.Coverage)
}

// US2-AS1: a spec that cannot be found is unusable input.
func TestCheckCoverageOfAMissingSpecIsUnusable(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.yml", coverageSuite(coverageTestcase("US1-AS1 one", "", "result.find ShouldEqual 1")))

	_, err := Check([]string{path}, Options{SpecsDir: dir, Coverage: "999-absent"})

	require.ErrorIs(t, err, ErrUnusable)
	assert.Contains(t, err.Error(), "999-absent")
}
