// Package recipecheck reads web and mobile recipe files (venom suites) as YAML nodes and reports the rules RC01 to
// RC11 of docs/specs/014-web-mobile-recipes/contracts/recipe-check.md.
package recipecheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mattn/go-zglob"
)

const defaultSpecsDir = "docs/specs"

var (
	// ErrUnusable marks input the check cannot use: no path, a path that does not exist, an empty folder, a file that
	// does not parse. The command maps it to exit code 2.
	ErrUnusable = errors.New("recipe check: input cannot be used")
)

// Finding is one rule violation, with the line of the node that holds it.
type Finding struct {
	File     string `json:"file"`
	Suite    string `json:"suite"`
	Testcase string `json:"testcase"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Line     int    `json:"line"`
}

// Report is the result of a check: the number of files read and the findings.
type Report struct {
	Files    int       `json:"files"`
	Findings []Finding `json:"findings"`
	// Coverage lists the scenarios of the spec named by Options.Coverage.
	Coverage []ScenarioCoverage `json:"coverage,omitempty"`
}

// MarshalJSON writes "coverage":[] when --coverage was given and lists nothing (a non-nil empty slice), and no
// coverage key when it was not given (nil).
func (r Report) MarshalJSON() ([]byte, error) {
	type plain Report
	if r.Coverage == nil {
		return json.Marshal(plain(r))
	}
	return json.Marshal(struct {
		plain
		Coverage []ScenarioCoverage `json:"coverage"`
	}{plain(r), r.Coverage})
}

// Options are the settings of a check.
type Options struct {
	// SpecsDir is where the spec named by a suite is looked up (default docs/specs, set by the command).
	SpecsDir string
	// Coverage is the id of the spec whose scenarios are listed with their proving testcases; empty means off.
	Coverage string
}

// Folder is what a rule sees: the suites of the checked files and the scenario ids of the specs they name.
type Folder struct {
	Suites []*Suite
	// Specs maps a spec folder name to the set of its scenario ids; a spec that was not found has no entry.
	Specs map[string]map[string]bool
}

// rule inspects a folder and returns its findings.
type rule func(*Folder) []Finding

// rules is the ordered list Check runs.
var rules = []rule{ruleRC01, ruleRC02, ruleRC03, ruleRC04, ruleRC05, ruleRC06, ruleRC07, ruleRC08, ruleRC09, ruleRC11}

// finding builds a finding of the suite, for a testcase or, when tc is nil, for the suite itself.
func (s *Suite) finding(tc *Testcase, ruleID string, line int, msg string) Finding {
	f := Finding{File: s.Path, Suite: s.Name, Rule: ruleID, Line: line, Message: msg}
	if tc != nil {
		f.Testcase = tc.Name
	}
	return f
}

// line is the line of the suite's first key, or 1 for an empty file.
func (s *Suite) line() int {
	if s.Node == nil {
		return 1
	}
	return s.Node.Line
}

// dedupe keeps the first of the findings that agree on file, testcase, rule, line and message.
func dedupe(findings []Finding) []Finding {
	seen := map[Finding]bool{}
	var out []Finding
	for _, f := range findings {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// Check reads the files named by paths (files, folders, glob patterns, as venom run takes them) and runs the rules.
// Input that cannot be used returns an error wrapping ErrUnusable; an unparsable file also adds an RC10 finding to
// the report.
func Check(paths []string, opts Options) (Report, error) {
	files, bad, err := resolveFiles(paths)
	if err != nil {
		if bad != nil {
			return Report{Findings: []Finding{*bad}}, err
		}
		return Report{}, err
	}
	report := Report{}
	folder := &Folder{Specs: map[string]map[string]bool{}}
	var unparsable []string
	for _, f := range files {
		text, rerr := os.ReadFile(f) //nolint:gosec // the files to check are the user's argument, as for venom run
		if rerr != nil {
			return report, fmt.Errorf("%w: %v", ErrUnusable, rerr)
		}
		report.Files++
		suite, finding := parseSuite(f, text)
		if finding != nil {
			report.Findings = append(report.Findings, *finding)
			unparsable = append(unparsable, f)
			continue
		}
		folder.Suites = append(folder.Suites, suite)
		loadSpec(folder, suite.Spec, opts.SpecsDir)
	}
	var found []Finding
	for _, r := range rules {
		found = append(found, r(folder)...)
	}
	report.Findings = append(report.Findings, dedupe(found)...)
	if len(unparsable) > 0 {
		return report, fmt.Errorf("%w: cannot parse %s", ErrUnusable, strings.Join(unparsable, ", "))
	}
	if opts.Coverage != "" {
		return withCoverage(report, folder, opts)
	}
	return report, nil
}

// withCoverage adds the coverage of the spec named by opts.Coverage; a spec that cannot be found is unusable input.
func withCoverage(report Report, folder *Folder, opts Options) (Report, error) {
	loadSpec(folder, opts.Coverage, opts.SpecsDir)
	if _, found := folder.Specs[opts.Coverage]; !found {
		return report, fmt.Errorf("%w: the spec %s was not found for --coverage", ErrUnusable, opts.Coverage)
	}
	report.Coverage = Coverage(folder, opts.Coverage)
	return report, nil
}

var scenarioID = regexp.MustCompile(`\bUS\d+-AS\d+\b`)

// loadSpec adds the scenario ids of <specsDir>/<spec>/spec.md to the folder, once; a missing spec adds nothing.
func loadSpec(folder *Folder, spec, specsDir string) {
	if spec == "" {
		return
	}
	if _, done := folder.Specs[spec]; done {
		return
	}
	if specsDir == "" {
		specsDir = defaultSpecsDir
	}
	text, err := os.ReadFile(filepath.Join(specsDir, spec, "spec.md")) //nolint:gosec // the specs dir is the user's flag
	if err != nil {
		return
	}
	ids := map[string]bool{}
	for _, id := range scenarioID.FindAllString(string(text), -1) {
		ids[id] = true
	}
	folder.Specs[spec] = ids
}

// resolveFiles expands the paths as venom run does (getFilesPath: a file, a folder's *.yml and *.yaml files, a
// glob pattern) and returns the sorted, de-duplicated files. A path that yields no file is unusable input.
// The finding is the RC10 of a path that matches no file.
func resolveFiles(paths []string) ([]string, *Finding, error) {
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("%w: no path given", ErrUnusable)
	}
	seen := map[string]bool{}
	var files []string
	for _, p := range paths {
		found, msg := expandPath(strings.TrimSpace(p))
		if msg != "" {
			return nil, rc10(p, 0, msg), fmt.Errorf("%w: %s", ErrUnusable, msg)
		}
		for _, f := range found {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	sort.Strings(files)
	return files, nil, nil
}

// expandPath returns the files of a path, or the reason when the path yields none.
func expandPath(p string) ([]string, string) {
	pattern := p
	// A glob pattern always fails Stat; only a real folder is rewritten.
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		pattern = p + string(os.PathSeparator) + "*.y*ml"
	}
	matches, err := zglob.Glob(pattern)
	if err != nil {
		return nil, fmt.Sprintf("reading files on path %q: %v", p, err)
	}
	var files []string
	for _, m := range matches {
		if ext := filepath.Ext(m); ext == ".yml" || ext == ".yaml" {
			files = append(files, m)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Sprintf("no YAML (*.yml or *.yaml) file found on path %q", p)
	}
	return files, ""
}
