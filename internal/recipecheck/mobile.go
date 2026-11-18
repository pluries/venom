package recipecheck

import (
	"strings"
)

// mobileLocatorFields are the locator candidates of executors/mobile/locator.go; in a mobile step text is a locator.
var mobileLocatorFields = []string{"accessibilityId", "text", "id", "xpath", "uiautomator", "css", "name", "predicate", "classChain"}

// locatorNeed is how many locators an action takes: the least and the most.
type locatorNeed struct {
	min, max int
	orPoint  bool
}

// mobileLocatorNeeds maps the actions of executors/mobile/actions.go that take a locator, or none, to their need.
var mobileLocatorNeeds = map[string]locatorNeed{
	"fill": {1, 1, false}, "clear": {1, 1, false}, "read": {1, 1, false}, "count": {1, 1, false}, "waitFor": {1, 1, false},
	"tap":   {1, 1, true},
	"swipe": {0, 1, false}, "scroll": {0, 1, false},
	"back": {}, "hideKeyboard": {}, "launch": {}, "terminate": {}, "screenshot": {}, "source": {}, "context": {},
	"openURL": {}, "alert": {},
}

// namedLocators returns the locator fields the step sets.
func namedLocators(st *Step) []string {
	var found []string
	for _, field := range mobileLocatorFields {
		if mapValue(st.Node, field) != nil {
			found = append(found, field)
		}
	}
	return found
}

// locatorProblem returns the message for a step whose locators do not fit its action, or "".
func locatorProblem(st *Step, action string) string {
	need, known := mobileLocatorNeeds[action]
	if !known {
		return ""
	}
	locators := namedLocators(st)
	switch {
	case need.max == 0 && len(locators) > 0:
		return "the step names " + strings.Join(locators, " and ") + ": " + action + " takes no locator"
	case len(locators) > need.max:
		return "the step names " + strings.Join(locators, " and ") + ": a mobile step takes at most " + countWord(need.max)
	case need.orPoint && len(locators) > 0 && (mapValue(st.Node, "x") != nil || mapValue(st.Node, "y") != nil):
		return "the " + action + " action names a locator and a coordinate: give one or the other"
	case len(locators) >= need.min:
		return ""
	case need.orPoint && mapValue(st.Node, "x") != nil && mapValue(st.Node, "y") != nil:
		return ""
	}
	return "the " + action + " action needs a locator"
}

func countWord(n int) string {
	if n == 1 {
		return "one locator"
	}
	return "locators"
}

// ruleRC11 reports a mobile step that names two locators, or none where its action needs one.
func ruleRC11(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			for _, st := range tc.Steps {
				if st.Type != "mobile" {
					continue
				}
				if msg := locatorProblem(st, scalarOf(mapValue(st.Node, "action"))); msg != "" {
					out = append(out, s.finding(tc, "RC11", st.Line, msg))
				}
			}
		}
	}
	return out
}
