package recipecheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FR-011: the splitter cuts on spaces, keeps a quoted argument whole and strips its quotes, as the engine's does.
func TestSplitAssertion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"plain", "result.title ShouldEqual Home", []string{"result.title", "ShouldEqual", "Home"}},
		{"repeated spaces", "result.title   ShouldEqual    Home", []string{"result.title", "ShouldEqual", "Home"}},
		{"double quoted argument", `result.text ShouldEqual "Recipe fixture"`, []string{"result.text", "ShouldEqual", "Recipe fixture"}},
		{"single quoted argument", `result.text ShouldEqual 'a b c'`, []string{"result.text", "ShouldEqual", "a b c"}},
		{"empty quoted argument", `result.text ShouldEqual ""`, []string{"result.text", "ShouldEqual", ""}},
		{"several arguments", "result.find ShouldBeBetween 1 3", []string{"result.find", "ShouldBeBetween", "1", "3"}},
		{"template argument", "result.title ShouldEqual {{.appTitle}}", []string{"result.title", "ShouldEqual", "{{.appTitle}}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitAssertion(tt.in))
		})
	}
}

// FR-011: the app-reading classifier, one row per closed list of the contract (Terms).
func TestIsAppReading(t *testing.T) {
	tests := []struct {
		name      string
		assertion string
		want      bool
	}{
		// each field of the closed list
		{"find", "result.find ShouldEqual 1", true},
		{"title", "result.title ShouldEqual Home", true},
		{"url", "result.url ShouldContainSubstring /done", true},
		{"text", "result.text ShouldEqual milk", true},
		{"html", "result.html ShouldContainSubstring <li>", true},
		{"value", "result.value ShouldEqual Ada", true},
		{"valuejson", "result.valuejson ShouldEqual 3", true},
		{"valuejson with a path", "result.valuejson.items.0.name ShouldEqual Apples", true},
		{"count", "result.count ShouldEqual 2", true},
		{"found", "result.found ShouldEqual true", true},
		{"source", "result.source ShouldContainSubstring <hierarchy", true},
		{"context", "result.context ShouldEqual NATIVE_APP", true},
		{"device", "result.device ShouldEqual emulator-5554", true},
		{"platform", "result.platform ShouldEqual Android", true},

		// operators and expected values
		{"operator with a number", "result.find ShouldBeGreaterThan 0", true},
		{"operator with two arguments", "result.find ShouldBeBetween 1 3", true},
		{"negated operator", "result.title ShouldNotEqual Error", true},
		{"quoted expected value", `result.text ShouldContainSubstring "two words"`, true},
		{"template variable not derived from the step", "result.title ShouldEqual {{.expectedTitle}}", true},

		// the engine never returns nil for a read value: these operators prove nothing on a listed field
		{"ShouldBeNil still reads", "result.value ShouldBeNil", true},
		{"ShouldBeEmpty still reads", "result.text ShouldBeEmpty", true},
		{"ShouldNotBeEmpty still reads", "result.text ShouldNotBeEmpty", true},
		{"ShouldBeTrue still reads", "result.found ShouldBeTrue", true},
		{"ShouldBeFalse still reads", "result.found ShouldBeFalse", true},
		{"ShouldNotBeNil on a listed field", "result.title ShouldNotBeNil", false},
		{"ShouldNotExist on a listed field", "result.html ShouldNotExist", false},

		// fields outside the list
		{"err is not the app", "result.err ShouldBeEmpty", false},
		{"systemerr is not the app", "result.systemerr ShouldBeEmpty", false},
		{"success-only ShouldNotBeNil on systemerr", "result.systemerr ShouldNotBeNil", false},
		{"success-only ShouldBeNil on err", "result.err ShouldBeNil", false},
		{"success-only ShouldNotExist on an unlisted field", "result.systemout ShouldNotExist", false},
		{"success-only ShouldBeBlank on an unlisted field", "result.systemerr ShouldBeBlank", false},
		{"timeseconds is not the app", "result.timeseconds ShouldBeGreaterThan 0", false},
		{"http field is not the app", "result.statuscode ShouldEqual 200", false},
		{"a field that only starts like a listed one", "result.titlex ShouldEqual Home", false},
		{"no result prefix", "title ShouldEqual Home", false},

		// operators
		{"unregistered operator", "result.title ShouldBogus Home", false},

		// expected value derived from the same step
		{"expected value is the result itself", "result.title ShouldEqual {{.result.title}}", false},
		{"one expected value of two is derived", "result.find ShouldBeBetween 1 {{.result.find}}", false},

		// malformed
		{"field only", "result.title", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isAppReading(tt.assertion))
		})
	}
}

// US3-AS1: a testcase with no app-reading assertion is reported with its line.
func TestRC05ReportsATestcaseWithNoAppReadingAssertion(t *testing.T) {
	folder := oneFile(t, suiteText("014-web-mobile-recipes",
		liveTestcase("US1-AS1 proves something"),
		weakTestcase("US1-AS2 only checks that no error happened"),
	), specOf)

	findings := onlyRule(t, ruleRC05(folder), "RC05")

	require.Len(t, findings, 1)
	assert.Equal(t, "US1-AS2 only checks that no error happened", findings[0].Testcase)
	assert.Equal(t, "a.yml", findings[0].File)
	assert.Equal(t, 12, findings[0].Line)
}

// US3-AS1: assertions of a step that is not web or mobile do not count, and neither does an expected value taken from
// the step's own result.
func TestRC05CountsOnlyAppReadingAssertionsOfWebAndMobileSteps(t *testing.T) {
	text := header("014-web-mobile-recipes") + "testcases:\n" +
		"- name: US1-AS1 asserts on an exec step only\n  steps:\n  - type: exec\n    script: echo 1\n    assertions:\n    - result.systemout ShouldEqual 1\n" +
		"- name: US1-AS2 compares the result with itself\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual {{.result.find}}\n" +
		"- name: US1-AS3 has no assertion at all\n  steps:\n  - type: web\n    action:\n      find: h1\n"
	folder := oneFile(t, text, specOf)

	findings := onlyRule(t, ruleRC05(folder), "RC05")

	assert.Equal(t, []string{
		"US1-AS1 asserts on an exec step only",
		"US1-AS2 compares the result with itself",
		"US1-AS3 has no assertion at all",
	}, testcaseNames(findings))
}

// US3-AS1: one app-reading assertion in any web step of the testcase is enough.
func TestRC05QuietWhenAnyStepReadsTheApp(t *testing.T) {
	text := header("014-web-mobile-recipes") + "testcases:\n" +
		"- name: US1-AS1 reads in its second step\n  steps:\n  - type: web\n    action:\n      navigate:\n        url: '{{.appUrl}}'\n" +
		"  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.err ShouldBeNil\n    - result.title ShouldEqual Home\n"

	assert.Empty(t, ruleRC05(oneFile(t, text, specOf)))
}

// US3-AS1: a skipped testcase runs nothing and is ignored by RC05.
func TestRC05IgnoresASkippedTestcase(t *testing.T) {
	text := header("014-web-mobile-recipes") + "testcases:\n- name: US1-AS1 skipped\n  skip:\n  - 1 ShouldEqual 1\n  steps:\n  - type: web\n    action:\n      find: h1\n"

	assert.Empty(t, ruleRC05(oneFile(t, text, specOf)))
}

// US3-AS1 (contract, Reading the files): a suite with no testcase proves nothing and is reported.
func TestRC05ReportsASuiteWithNoTestcase(t *testing.T) {
	folder := folderFrom(t, map[string]string{
		"empty.yml": "name: Nothing here\nvars:\n  spec: 014-web-mobile-recipes\n",
		"good.yml":  suiteText("014-web-mobile-recipes", liveTestcase("US1-AS1 one")),
	}, specOf)

	findings := onlyRule(t, ruleRC05(folder), "RC05")

	require.Len(t, findings, 1)
	assert.Equal(t, "empty.yml", findings[0].File)
	assert.Equal(t, "Nothing here", findings[0].Suite)
	assert.Empty(t, findings[0].Testcase)
}

// US3-AS2: each tautology form of the contract is reported, on the line of the assertion.
func TestRC07ReportsEachTautologyForm(t *testing.T) {
	tests := []struct {
		name      string
		assertion string
	}{
		{"same expression on both sides", "result.title ShouldEqual result.title"},
		{"same literal on both sides", "1 ShouldEqual 1"},
		{"contains the empty string", `result.text ShouldContainSubstring ""`},
		{"starts with the empty string", `result.text ShouldStartWith ""`},
		{"ends with the empty string", `result.text ShouldEndWith ""`},
		{"matches everything", "result.text ShouldMatchRegex .*"},
		{"count greater than a negative number", "result.count ShouldBeGreaterThan -1"},
		{"count greater or equal to a negative number", "result.count ShouldBeGreaterThanOrEqualTo -3"},
		{"count greater or equal to zero", "result.count ShouldBeGreaterThanOrEqualTo 0"},
		{"find greater or equal to zero", "result.find ShouldBeGreaterThanOrEqualTo 0"},
		{"find greater than a negative number", "result.find ShouldBeGreaterThan -1"},
		{"between covers the whole type", "result.count ShouldBeBetween -9223372036854775808 9223372036854775807"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder := oneFile(t, suiteWithAssertions("result.find ShouldEqual 1", tt.assertion), specOf)

			findings := onlyRule(t, ruleRC07(folder), "RC07")

			require.Len(t, findings, 1)
			assert.Equal(t, assertionLine(1), findings[0].Line)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
			assert.Equal(t, "a.yml", findings[0].File)
		})
	}
}

// US3-AS2: assertions that can fail are not tautologies.
func TestRC07QuietForAssertionsThatCanFail(t *testing.T) {
	folder := oneFile(t, suiteWithAssertions(
		"result.title ShouldEqual Home",
		"result.text ShouldContainSubstring milk",
		"result.text ShouldStartWith mi",
		`result.text ShouldMatchRegex ^milk$`,
		"result.find ShouldBeGreaterThan 0",
		"result.count ShouldBeGreaterThanOrEqualTo 1",
		"result.find ShouldBeGreaterThan 0",
		"result.title ShouldBeGreaterThanOrEqualTo 0",
		"result.find ShouldBeBetween 1 3",
		"result.title ShouldEqual {{.expectedTitle}}",
	), specOf)

	assert.Empty(t, ruleRC07(folder))
}

// US3-AS1, FR-011: a testcase whose only assertion on a listed field is ShouldNotBeNil or ShouldNotExist proves nothing
// about the application, so it is reported by RC05.
func TestRC05ReportsATestcaseWhoseOnlyAssertionIsNotBeNil(t *testing.T) {
	for _, assertion := range []string{"result.title ShouldNotBeNil", "result.html ShouldNotExist"} {
		t.Run(assertion, func(t *testing.T) {
			folder := oneFile(t, suiteWithAssertions(assertion), specOf)

			findings := onlyRule(t, ruleRC05(folder), "RC05")

			require.Len(t, findings, 1)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
		})
	}
}
