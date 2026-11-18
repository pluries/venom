package recipecheck

import (
	"sort"
	"strconv"
	"strings"
)

// ScenarioCoverage is one scenario of a spec with the live testcases that prove it.
type ScenarioCoverage struct {
	Scenario  string             `json:"scenario"`
	Proven    bool               `json:"proven"`
	Testcases []CoveringTestcase `json:"testcases"`
}

// CoveringTestcase is a testcase that proves a scenario.
type CoveringTestcase struct {
	File     string `json:"file"`
	Testcase string `json:"testcase"`
	Line     int    `json:"line"`
}

// Coverage lists the scenarios of the spec in spec order (story number, then scenario number), each with the
// testcases of the files naming that spec that prove it: live, starting with the whole id, and holding an app-reading assertion.
func Coverage(f *Folder, specID string) []ScenarioCoverage {
	ids := sortedScenarioIDs(f.Specs[specID])
	proving := provingTestcases(f, specID)
	out := make([]ScenarioCoverage, 0, len(ids))
	for _, id := range ids {
		tcs := proving[id]
		if tcs == nil {
			tcs = []CoveringTestcase{}
		}
		out = append(out, ScenarioCoverage{Scenario: id, Proven: len(tcs) > 0, Testcases: tcs})
	}
	return out
}

// provingTestcases groups the proving testcases by scenario id, in file then testcase order; only the files whose
// suite names the spec count.
func provingTestcases(f *Folder, specID string) map[string][]CoveringTestcase {
	byID := map[string][]CoveringTestcase{}
	for _, s := range f.Suites {
		if s.Spec != specID {
			continue
		}
		for _, tc := range s.Testcases {
			id := scenarioIDOf(tc.Name)
			if id == "" || tc.Skipped || !readsApp(tc) {
				continue
			}
			byID[id] = append(byID[id], CoveringTestcase{File: s.Path, Testcase: tc.Name, Line: tc.Line})
		}
	}
	return byID
}

func sortedScenarioIDs(set map[string]bool) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		si, ai := idNumbers(ids[i])
		sj, aj := idNumbers(ids[j])
		if si != sj {
			return si < sj
		}
		return ai < aj
	})
	return ids
}

// idNumbers returns the story and scenario numbers of an id such as US12-AS3.
func idNumbers(id string) (int, int) {
	story, scenario, _ := strings.Cut(strings.TrimPrefix(id, "US"), "-AS")
	s, _ := strconv.Atoi(story)
	a, _ := strconv.Atoi(scenario)
	return s, a
}
