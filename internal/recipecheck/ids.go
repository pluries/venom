package recipecheck

import (
	"fmt"
	"regexp"
	"strings"
)

var scenarioStart = regexp.MustCompile(`^(US\d+-AS\d+)(?:\s|$)`)

// scenarioIDOf returns the scenario id a testcase name starts with, or "" when it starts with none.
func scenarioIDOf(name string) string {
	if m := scenarioStart.FindStringSubmatch(name); m != nil {
		return m[1]
	}
	return ""
}

// ruleRC01 reports a testcase whose name does not start with a scenario id.
func ruleRC01(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			if scenarioIDOf(tc.Name) == "" {
				out = append(out, s.finding(tc, "RC01", tc.Line, "the testcase name does not start with a scenario id"))
			}
		}
	}
	return out
}

type idUse struct {
	suite *Suite
	tc    *Testcase
}

// ruleRC02 reports the same scenario id in testcases of two different suite files.
func ruleRC02(f *Folder) []Finding {
	byID, order := usesByID(f)
	var out []Finding
	for _, id := range order {
		uses := byID[id]
		if !inSeveralFiles(uses) {
			continue
		}
		for _, u := range uses {
			out = append(out, u.suite.finding(u.tc, "RC02", u.tc.Line, fmt.Sprintf("scenario id %s starts testcases of several files", id)))
		}
	}
	return out
}

// usesByID groups the testcases that start with a scenario id by that id, with the ids in first-seen order.
func usesByID(f *Folder) (map[string][]idUse, []string) {
	byID := map[string][]idUse{}
	var order []string
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			id := scenarioIDOf(tc.Name)
			if id == "" {
				continue
			}
			if _, known := byID[id]; !known {
				order = append(order, id)
			}
			byID[id] = append(byID[id], idUse{s, tc})
		}
	}
	return byID, order
}

func inSeveralFiles(uses []idUse) bool {
	for _, u := range uses {
		if u.suite.Path != uses[0].suite.Path {
			return true
		}
	}
	return false
}

// ruleRC03 reports a scenario id that starts a testcase name and is not in the spec named by the suite.
func ruleRC03(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		if s.Spec == "" {
			continue
		}
		ids, found := f.Specs[s.Spec]
		for _, tc := range s.Testcases {
			id := scenarioIDOf(tc.Name)
			if id == "" || ids[id] {
				continue
			}
			msg := fmt.Sprintf("scenario id %s is not in the spec %s", id, s.Spec)
			if !found {
				msg = fmt.Sprintf("scenario id %s cannot be resolved: the spec %s was not found", id, s.Spec)
			}
			out = append(out, s.finding(tc, "RC03", tc.Line, msg))
		}
	}
	return out
}

// ruleRC04 reports a scenario id found in the text of a suite file that starts no live testcase name.
func ruleRC04(f *Folder) []Finding {
	proven := provenIDs(f)
	var out []Finding
	for _, s := range f.Suites {
		out = append(out, unprovenIDs(s, proven)...)
	}
	return out
}

// provenIDs are the ids that start the name of a testcase with no skip list, in any file of the folder.
func provenIDs(f *Folder) map[string]bool {
	proven := map[string]bool{}
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			if id := scenarioIDOf(tc.Name); id != "" && !tc.Skipped {
				proven[id] = true
			}
		}
	}
	return proven
}

// unprovenIDs reports, once per id, the ids found anywhere in the text of a file that are not proven.
func unprovenIDs(s *Suite, proven map[string]bool) []Finding {
	var out []Finding
	reported := map[string]bool{}
	for i, line := range strings.Split(s.Text, "\n") {
		for _, id := range scenarioID.FindAllString(line, -1) {
			if proven[id] || reported[id] {
				continue
			}
			reported[id] = true
			out = append(out, s.finding(nil, "RC04", i+1, fmt.Sprintf("scenario id %s is not proven: no live testcase of the folder starts with it", id)))
		}
	}
	return out
}

// ruleRC06 reports a suite that names no spec.
func ruleRC06(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		if s.Spec == "" {
			out = append(out, s.finding(nil, "RC06", s.line(), "the suite names no spec: set the variable spec"))
		}
	}
	return out
}
