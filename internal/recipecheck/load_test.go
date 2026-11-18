package recipecheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Line numbers below are those of the joined lines (the first line is 1).
var sampleSuite = strings.Join([]string{
	"name: Sample suite",                  // 1
	"vars:",                               // 2
	"  spec: 014-web-mobile-recipes",      // 3
	"testcases:",                          // 4
	"- name: US1-AS1 loads",               // 5
	"  steps:",                            // 6
	"  - type: exec",                      // 7
	"    script: echo hi",                 // 8
	"    assertions:",                     // 9
	"    - result.code ShouldEqual 0",     // 10
	"  - type: web",                       // 11
	"    action:",                         // 12
	"      navigate:",                     // 13
	`        url: "{{.appUrl}}"`,          // 14
	"    assertions:",                     // 15
	"    - result.title ShouldEqual Home", // 16
	"    - result.find ShouldEqual 1",     // 17
	"- name: US1-AS2 skipped",             // 18
	"  skip:",                             // 19
	"  - appUrl ShouldEqual x",            // 20
	"  steps:",                            // 21
	"  - type: mobile",                    // 22
	"    action: tap",                     // 23
	"    accessibilityId: ok",             // 24
	"",
}, "\n")

// FR-015, US3-AS7: the loader reads suite, testcases, steps and assertions with their lines.
func TestParseSuiteReadsNodesWithLines(t *testing.T) {
	suite, finding := parseSuite("sample.yml", []byte(sampleSuite))

	require.Nil(t, finding)
	require.NotNil(t, suite)
	assert.Equal(t, "sample.yml", suite.Path)
	assert.Equal(t, "Sample suite", suite.Name)
	assert.Equal(t, "014-web-mobile-recipes", suite.Spec)
	require.Len(t, suite.Testcases, 2)

	first := suite.Testcases[0]
	assert.Equal(t, "US1-AS1 loads", first.Name)
	assert.Equal(t, 5, first.Line)
	assert.False(t, first.Skipped)
	require.Len(t, first.Steps, 1)
	web := first.Steps[0]
	assert.Equal(t, "web", web.Type)
	assert.Equal(t, 11, web.Line)
	assert.Equal(t, []Assertion{
		{Text: "result.title ShouldEqual Home", Line: 16},
		{Text: "result.find ShouldEqual 1", Line: 17},
	}, web.Assertions)
}

// FR-015: only web and mobile steps are examined; a step of another executor is dropped.
func TestParseSuiteKeepsOnlyWebAndMobileSteps(t *testing.T) {
	suite, finding := parseSuite("sample.yml", []byte(sampleSuite))
	require.Nil(t, finding)
	require.NotNil(t, suite)
	require.Len(t, suite.Testcases, 2)

	for _, tc := range suite.Testcases {
		for _, s := range tc.Steps {
			assert.Contains(t, []string{"web", "mobile"}, s.Type)
		}
	}
	second := suite.Testcases[1]
	require.Len(t, second.Steps, 1)
	assert.Equal(t, "mobile", second.Steps[0].Type)
	assert.Equal(t, 22, second.Steps[0].Line)
	assert.Empty(t, second.Steps[0].Assertions)
}

// RC04, RC05: a testcase with a skip key is marked skipped.
func TestParseSuiteMarksSkippedTestcase(t *testing.T) {
	suite, finding := parseSuite("sample.yml", []byte(sampleSuite))
	require.Nil(t, finding)
	require.NotNil(t, suite)
	require.Len(t, suite.Testcases, 2)

	assert.True(t, suite.Testcases[1].Skipped)
	assert.Equal(t, 18, suite.Testcases[1].Line)
}

// RC06: a suite that names no spec has an empty Spec.
func TestParseSuiteWithoutSpecVariable(t *testing.T) {
	suite, finding := parseSuite("s.yml", []byte("name: S\nvars:\n  appUrl: http://localhost\ntestcases: []\n"))
	require.Nil(t, finding)
	require.NotNil(t, suite)
	assert.Empty(t, suite.Spec)
}

// Contract, Reading the files: a file with no testcases is read, as venom run reads it.
func TestParseSuiteWithoutTestcasesIsRead(t *testing.T) {
	suite, finding := parseSuite("empty.yml", []byte("name: Nothing here\nvars:\n  a: b\n"))

	require.Nil(t, finding)
	require.NotNil(t, suite)
	assert.Equal(t, "Nothing here", suite.Name)
	assert.Empty(t, suite.Testcases)
}

// US3-AS7: a file that does not parse is RC10, with its path.
func TestParseSuiteUnparsableIsRC10(t *testing.T) {
	for name, text := range map[string]string{
		"unclosed flow":    "name: [unclosed\n",
		"tab indentation":  "name: x\ntestcases:\n\t- name: y\n",
		"duplicate colons": "a: b: c\n",
	} {
		t.Run(name, func(t *testing.T) {
			suite, finding := parseSuite("bad.yml", []byte(text))

			assert.Nil(t, suite)
			require.NotNil(t, finding)
			assert.Equal(t, "RC10", finding.Rule)
			assert.Equal(t, "bad.yml", finding.File)
			assert.NotEmpty(t, finding.Message)
		})
	}
}

// US3-AS7: "testcases" present but not a list is RC10, which venom run rejects too.
func TestParseSuiteTestcasesNotAListIsRC10(t *testing.T) {
	for name, text := range map[string]string{
		"scalar":  "name: x\ntestcases: nope\n",
		"mapping": "name: x\ntestcases:\n  name: y\n",
	} {
		t.Run(name, func(t *testing.T) {
			suite, finding := parseSuite("bad.yml", []byte(text))

			assert.Nil(t, suite)
			require.NotNil(t, finding)
			assert.Equal(t, "RC10", finding.Rule)
			assert.Equal(t, "bad.yml", finding.File)
		})
	}
}

// US3-AS7: the finding of a scalar "testcases" carries the line of that key.
func TestParseSuiteTestcasesNotAListReportsItsLine(t *testing.T) {
	_, finding := parseSuite("bad.yml", []byte("name: x\ntestcases: nope\n"))

	require.NotNil(t, finding)
	assert.Equal(t, 2, finding.Line)
}

// FR-013, US3-AS7: venom run rejects a file with a duplicate YAML key, so the loader reports it as RC10.
func TestParseSuiteDuplicateKeyIsRC10(t *testing.T) {
	suite, finding := parseSuite("dup.yml", []byte("name: x\nname: y\ntestcases: []\n"))

	assert.Nil(t, suite)
	require.NotNil(t, finding)
	assert.Equal(t, "RC10", finding.Rule)
	assert.Equal(t, "dup.yml", finding.File)
}

// FR-013, US3-AS7: venom run rejects a step that is not a mapping, so the loader reports it as RC10, with the line of the step.
func TestParseSuiteStepNotAMappingIsRC10(t *testing.T) {
	suite, finding := parseSuite("step.yml", []byte("name: x\ntestcases:\n- name: US1-AS1 y\n  steps:\n  - just a string\n"))

	assert.Nil(t, suite)
	require.NotNil(t, finding)
	assert.Equal(t, "RC10", finding.Rule)
	assert.Equal(t, "step.yml", finding.File)
	assert.Equal(t, 5, finding.Line)
}

// FR-013: Check treats both files as unusable input, as it does an unparsable file, and reports RC10 with the path.
func TestCheckDuplicateKeyAndStepNotAMappingAreRC10AndUnusable(t *testing.T) {
	for name, text := range map[string]string{
		"dup.yml":  "name: x\nname: y\ntestcases: []\n",
		"step.yml": "name: x\ntestcases:\n- name: US1-AS1 y\n  steps:\n  - just a string\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), name, text)

			report, err := Check([]string{path}, Options{})

			assert.ErrorIs(t, err, ErrUnusable)
			require.Len(t, report.Findings, 1)
			assert.Equal(t, "RC10", report.Findings[0].Rule)
			assert.Equal(t, path, report.Findings[0].File)
		})
	}
}
