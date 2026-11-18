package recipecheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The locator fields of a mobile step are those of executors/mobile/locator.go (candidates); the actions are those of
// executors/mobile/actions.go. A mobile step is `type: mobile`, `action: <name>` (a string), and its locator and
// inputs as keys of the step itself.

// mobileLocators are the nine locator fields, each with a value that is a valid example of its kind.
var mobileLocators = map[string]string{
	"accessibilityId": "Increment",
	"text":            "Save",
	"id":              "com.example:id/save",
	"xpath":           "//android.widget.Button",
	"uiautomator":     "new UiSelector().text(\"x\")",
	"css":             "button.save",
	"name":            "Save",
	"predicate":       "label == 'Save'",
	"classChain":      "**/XCUIElementTypeButton",
}

// mobileLocatorNames lists the keys of mobileLocators in a fixed order.
var mobileLocatorNames = []string{"accessibilityId", "text", "id", "xpath", "uiautomator", "css", "name", "predicate", "classChain"}

// mobileStepLine is the line of the "- type: mobile" line of suiteWithMobileSteps.
const mobileStepLine = 7

// mobileStep is one mobile step as YAML, to put after "steps:" at two spaces of indent. Each field is a "key: value"
// pair, written below the type.
func mobileStep(action string, fields ...string) string {
	lines := []string{"  - type: mobile"}
	if action != "" {
		lines = append(lines, "    action: "+action)
	}
	for _, f := range fields {
		lines = append(lines, "    "+f)
	}
	lines = append(lines, "    assertions:", "    - result.err ShouldBeEmpty")
	return strings.Join(lines, "\n") + "\n"
}

// locatorField is the "key: value" pair of a locator of mobileLocators.
func locatorField(name string) string { return name + ": " + yamlQuote(mobileLocators[name]) }

// suiteWithMobileSteps is a one-testcase suite of the given steps (made with mobileStep); the first step starts on
// mobileStepLine.
func suiteWithMobileSteps(steps ...string) string {
	return "name: Suite\nvars:\n  spec: 014-web-mobile-recipes\ntestcases:\n- name: US1-AS1 one\n  steps:\n" + strings.Join(steps, "")
}

// rc11Of runs RC11 over a one-file folder.
func rc11Of(t *testing.T, steps ...string) []Finding {
	t.Helper()
	return ruleRC11(oneFile(t, suiteWithMobileSteps(steps...), specOf))
}

// US4-AS4: a mobile step that names two locators is reported, whatever the pair and the action.
func TestRC11ReportsTwoLocators(t *testing.T) {
	for _, action := range []string{"tap", "fill", "clear", "read", "count", "waitFor", "swipe", "scroll"} {
		t.Run(action, func(t *testing.T) {
			findings := rc11Of(t, mobileStep(action, locatorField("accessibilityId"), locatorField("xpath")))

			onlyRule(t, findings, "RC11")
			require.Len(t, findings, 1)
			assert.Equal(t, mobileStepLine, findings[0].Line)
			assert.Equal(t, "a.yml", findings[0].File)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
			assert.Equal(t, "Suite", findings[0].Suite)
		})
	}
}

// US4-AS4: two locators are reported for every pair of locator fields, text and name included.
func TestRC11ReportsEveryPairOfLocatorFields(t *testing.T) {
	for i, first := range mobileLocatorNames {
		for _, second := range mobileLocatorNames[i+1:] {
			t.Run(first+"+"+second, func(t *testing.T) {
				findings := rc11Of(t, mobileStep("tap", locatorField(first), locatorField(second)))

				require.Len(t, onlyRule(t, findings, "RC11"), 1)
			})
		}
	}
}

// US4-AS4: three locators are one finding, not one per extra.
func TestRC11ReportsThreeLocatorsOnce(t *testing.T) {
	findings := rc11Of(t, mobileStep("read", locatorField("accessibilityId"), locatorField("text"), locatorField("id")))

	require.Len(t, onlyRule(t, findings, "RC11"), 1)
}

// US4-AS4: two locators on an action that takes none are still reported.
func TestRC11ReportsTwoLocatorsOnAnActionThatTakesNone(t *testing.T) {
	for _, action := range []string{"back", "hideKeyboard", "launch", "terminate", "screenshot", "source", "context", "openURL", "alert"} {
		t.Run(action, func(t *testing.T) {
			findings := rc11Of(t, mobileStep(action, locatorField("accessibilityId"), locatorField("xpath")))

			require.Len(t, onlyRule(t, findings, "RC11"), 1)
		})
	}
}

// US4-AS4: fill, clear, read, count and waitFor with no locator are reported, on the line of the step.
func TestRC11ReportsNoLocatorWhereTheActionNeedsOne(t *testing.T) {
	for _, action := range []string{"fill", "clear", "read", "count", "waitFor"} {
		t.Run(action, func(t *testing.T) {
			findings := rc11Of(t, mobileStep(action))

			onlyRule(t, findings, "RC11")
			require.Len(t, findings, 1)
			assert.Equal(t, mobileStepLine, findings[0].Line)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
		})
	}
}

// US4-AS4: the other inputs of an action are not locators; fill with a value but no locator is reported.
func TestRC11DoesNotTakeAnInputForALocator(t *testing.T) {
	findings := rc11Of(t, mobileStep("fill", "value: hello", "clear: true", "syncTimeout: 5"))

	require.Len(t, onlyRule(t, findings, "RC11"), 1)
}

// US4-AS4: tap needs a locator or the pair x and y; with neither it is reported.
func TestRC11ReportsATapWithNoLocatorAndNoPoint(t *testing.T) {
	findings := rc11Of(t, mobileStep("tap"))

	require.Len(t, onlyRule(t, findings, "RC11"), 1)
}

// US4-AS4: exactly one locator, whichever of the nine, is not reported for the actions that need one.
func TestRC11QuietForExactlyOneLocator(t *testing.T) {
	for _, action := range []string{"tap", "fill", "clear", "read", "count", "waitFor"} {
		for _, name := range mobileLocatorNames {
			t.Run(action+"/"+name, func(t *testing.T) {
				assert.Empty(t, rc11Of(t, mobileStep(action, locatorField(name))))
			})
		}
	}
}

// US4-AS4: in a mobile step, text is a locator and value is the text typed: fill with text and value has one locator.
func TestRC11QuietForFillWithTextAsLocatorAndAValue(t *testing.T) {
	assert.Empty(t, rc11Of(t, mobileStep("fill", locatorField("text"), "value: hello", "clear: true")))
}

// US4-AS4: a locator given by a template counts as a locator.
func TestRC11CountsATemplatedLocator(t *testing.T) {
	assert.Empty(t, rc11Of(t, mobileStep("tap", "accessibilityId: '{{.button}}'")))
	require.Len(t, rc11Of(t, mobileStep("tap", "accessibilityId: '{{.button}}'", "text: '{{.label}}'")), 1)
}

// US4-AS4: tap at a point (x and y) with no locator is fine.
func TestRC11QuietForATapAtAPoint(t *testing.T) {
	assert.Empty(t, rc11Of(t, mobileStep("tap", "x: 100", "y: 200")))
	assert.Empty(t, rc11Of(t, mobileStep("tap", "x: 50", "y: 50", "unit: percent")))
}

// US4-AS4: swipe and scroll take zero or one locator, or the point forms.
func TestRC11QuietForSwipeAndScrollWithZeroOrOneLocator(t *testing.T) {
	for _, action := range []string{"swipe", "scroll"} {
		t.Run(action+"/none", func(t *testing.T) {
			assert.Empty(t, rc11Of(t, mobileStep(action, "direction: up")))
		})
		t.Run(action+"/one", func(t *testing.T) {
			assert.Empty(t, rc11Of(t, mobileStep(action, "direction: up", locatorField("accessibilityId"))))
		})
	}
	t.Run("swipe/points", func(t *testing.T) {
		assert.Empty(t, rc11Of(t, mobileStep("swipe", "x: 10", "y: 20", "toX: 10", "toY: 300")))
	})
}

// US4-AS4: back, hideKeyboard, launch, terminate, screenshot, source, context, openURL and alert take no locator and
// are not reported when they have none.
func TestRC11QuietForAnActionThatTakesNoLocator(t *testing.T) {
	for _, action := range []string{"back", "hideKeyboard", "launch", "terminate", "screenshot", "source", "context", "openURL", "alert"} {
		t.Run(action, func(t *testing.T) {
			assert.Empty(t, rc11Of(t, mobileStep(action)))
		})
	}
}

// US4-AS4: only mobile steps are examined; a web step is ignored even when it looks like a mobile one.
func TestRC11IgnoresWebSteps(t *testing.T) {
	web := suiteWithStep("    action:\n      fill:\n        find: input\n        value: hello\n")
	lookAlike := "name: Suite\nvars:\n  spec: 014-web-mobile-recipes\ntestcases:\n- name: US1-AS1 one\n  steps:\n" +
		"  - type: web\n    action: fill\n    id: a\n    xpath: //b\n    assertions:\n    - result.find ShouldEqual 1\n"

	assert.Empty(t, ruleRC11(oneFile(t, web, specOf)))
	assert.Empty(t, ruleRC11(oneFile(t, lookAlike, specOf)))
}

// US4-AS4: steps of other executors are ignored.
func TestRC11IgnoresStepsOfOtherExecutors(t *testing.T) {
	text := suiteWithMobileSteps("  - type: exec\n    script: echo hi\n    id: a\n    xpath: b\n")

	assert.Empty(t, ruleRC11(oneFile(t, text, specOf)))
}

// US4-AS4: a good step before a bad one: one finding, on the line of the bad step; the good step is not reported.
func TestRC11ReportsOnlyTheBadStepWithItsLine(t *testing.T) {
	good := mobileStep("tap", locatorField("accessibilityId")) // five lines: 7 to 11
	bad := mobileStep("read")                                  // starts on line 12

	findings := rc11Of(t, good, bad)

	onlyRule(t, findings, "RC11")
	require.Len(t, findings, 1)
	assert.Equal(t, mobileStepLine+5, findings[0].Line)
}

// US4-AS4: one finding per bad step, in several testcases and files.
func TestRC11ReportsEachBadStepOfEachFile(t *testing.T) {
	two := "name: Two\nvars:\n  spec: 014-web-mobile-recipes\ntestcases:\n" +
		"- name: US1-AS1 a\n  steps:\n" + mobileStep("fill") +
		"- name: US1-AS2 b\n  steps:\n" + mobileStep("tap", locatorField("id"), locatorField("css"))
	one := suiteWithMobileSteps(mobileStep("count"))

	folder := folderFrom(t, map[string]string{"one.yml": one, "two.yml": two}, specOf)
	findings := onlyRule(t, ruleRC11(folder), "RC11")

	require.Len(t, findings, 3)
	got := map[string]int{}
	for _, f := range findings {
		got[f.File+"|"+f.Testcase]++
	}
	assert.Equal(t, map[string]int{"one.yml|US1-AS1 one": 1, "two.yml|US1-AS1 a": 1, "two.yml|US1-AS2 b": 1}, got)
}

// US4-AS4: a mobile step in a suite with no bad step gives no finding at all.
func TestRC11QuietForAGoodSuite(t *testing.T) {
	steps := []string{
		mobileStep("waitFor", locatorField("accessibilityId"), "syncTimeout: 240"),
		mobileStep("tap", locatorField("accessibilityId")),
		mobileStep("read", locatorField("accessibilityId"), "attribute: text"),
		mobileStep("back"),
	}

	assert.Empty(t, rc11Of(t, steps...))
}

// US4-AS4: README, x and y are not given with a locator: a tap with a locator and x or y is reported.
func TestRC11ReportsATapWithALocatorAndACoordinate(t *testing.T) {
	for name, fields := range map[string][]string{
		"locator and x":    {locatorField("accessibilityId"), "x: 100"},
		"locator and y":    {locatorField("accessibilityId"), "y: 200"},
		"locator, x and y": {locatorField("accessibilityId"), "x: 100", "y: 200"},
	} {
		t.Run(name, func(t *testing.T) {
			findings := rc11Of(t, mobileStep("tap", fields...))

			require.Len(t, onlyRule(t, findings, "RC11"), 1)
		})
	}
}

// US4-AS4: a tap names the pair x and y, not one of them.
func TestRC11ReportsATapWithOnlyOneCoordinate(t *testing.T) {
	for _, field := range []string{"x: 100", "y: 200"} {
		t.Run(field, func(t *testing.T) {
			findings := rc11Of(t, mobileStep("tap", field))

			require.Len(t, onlyRule(t, findings, "RC11"), 1)
		})
	}
}

// US4-AS4: the message for an action that takes no locator reads as a sentence, not "at most no locator".
func TestRC11MessageForAnActionThatTakesNoLocator(t *testing.T) {
	findings := rc11Of(t, mobileStep("launch", locatorField("accessibilityId")))

	require.Len(t, onlyRule(t, findings, "RC11"), 1)
	assert.Contains(t, findings[0].Message, "launch takes no locator")
	assert.NotContains(t, findings[0].Message, "at most no locator")
}
