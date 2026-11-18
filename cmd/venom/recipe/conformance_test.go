package recipe

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv3 "go.yaml.in/yaml/v3"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors"
	"github.com/ovh/venom/executors/web"
	"github.com/ovh/venom/internal/recipecheck"
)

// yamlKeys returns the keys of a decoded YAML mapping.
func yamlKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// yamlFieldNames returns the YAML names of the fields of a struct type, from their tags.
func yamlFieldNames(typ reflect.Type) map[string]bool {
	names := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("yaml"), ",")
		if name != "" && name != "-" {
			names[name] = true
		}
	}
	return names
}

// unknownKeys lists the keys of m that are not in allowed.
func unknownKeys(m map[string]any, allowed map[string]bool) []string {
	var unknown []string
	for _, k := range yamlKeys(m) {
		if !allowed[k] {
			unknown = append(unknown, k)
		}
	}
	return unknown
}

// stepEngineKeys are the keys of a step that the engine itself reads, whatever the executor.
var stepEngineKeys = map[string]bool{"type": true, "assertions": true, "info": true, "vars": true, "retry": true, "delay": true, "range": true}

// webStepKeyProblems returns, for one decoded web step, every key that is not a field of web.Executor or web.Action
// (or web.Mock under mock), prefixed by where it was found.
func webStepKeyProblems(step map[string]any) []string {
	allowed := map[string]bool{}
	for k := range yamlFieldNames(reflect.TypeOf(web.Executor{})) {
		allowed[k] = true
	}
	for k := range stepEngineKeys {
		allowed[k] = true
	}
	problems := prefixKeys("step key ", unknownKeys(step, allowed))
	if action, ok := step["action"].(map[string]any); ok {
		problems = append(problems, prefixKeys("action key ", unknownKeys(action, yamlFieldNames(reflect.TypeOf(web.Action{}))))...)
	}
	if mocks, ok := step["mock"].([]any); ok {
		problems = append(problems, mockKeyProblems(mocks)...)
	}
	return problems
}

func prefixKeys(prefix string, keys []string) []string {
	var out []string
	for _, k := range keys {
		out = append(out, prefix+k)
	}
	return out
}

func mockKeyProblems(mocks []any) []string {
	var problems []string
	for _, m := range mocks {
		if mock, ok := m.(map[string]any); ok {
			problems = append(problems, prefixKeys("mock key ", unknownKeys(mock, yamlFieldNames(reflect.TypeOf(web.Mock{}))))...)
		}
	}
	return problems
}

// webSteps returns the decoded web steps of a suite file.
func webSteps(t *testing.T, path string) []map[string]any {
	t.Helper()
	text, err := os.ReadFile(path) //nolint:gosec // a fixture of the repository
	require.NoError(t, err)
	var suite struct {
		Testcases []struct {
			Steps []map[string]any `yaml:"steps"`
		} `yaml:"testcases"`
	}
	require.NoError(t, yamlv3.Unmarshal(text, &suite), path)
	var steps []map[string]any
	for _, tc := range suite.Testcases {
		for _, s := range tc.Steps {
			if s["type"] == "web" {
				steps = append(steps, s)
			}
		}
	}
	return steps
}

// recipeFiles returns the suite files of the fixtures and of the reference web recipes that exist.
func recipeFiles(t *testing.T, withDefects bool) []string {
	t.Helper()
	root := repoRoot(t)
	var files []string
	for _, base := range []string{fixturesDir(t), filepath.Join(root, "tests", "recipes", "reference")} {
		files = append(files, yamlFilesUnder(base)...)
	}
	if site := filepath.Join(root, "tests", "site", "site-languages.yml"); fileExists(site) {
		files = append(files, site)
	}
	if !withDefects {
		files = withoutDefectFixtures(files)
	}
	sort.Strings(files)
	return files
}

// yamlFilesUnder lists the .yml and .yaml files under base; a folder that does not exist yet has none.
func yamlFilesUnder(base string) []string {
	var files []string
	_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // a folder that does not exist yet has no recipe
		}
		if ext := filepath.Ext(p); ext == ".yml" || ext == ".yaml" {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func withoutDefectFixtures(files []string) []string {
	var kept []string
	for _, f := range files {
		if !strings.Contains(filepath.ToSlash(f), "/fixtures/defects/") {
			kept = append(kept, f)
		}
	}
	return kept
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// US1-AS2: the check of keys used below finds a key that is not a field of the executor (a misspelt action key).
func TestWebStepKeyProblemsFindsAMisspeltKey(t *testing.T) {
	step := map[string]any{
		"type":       "web",
		"action":     map[string]any{"clik": map[string]any{"find": "a"}, "navigate": map[string]any{"url": "x"}},
		"screenshot": "a.png",
		"mock":       []any{map[string]any{"url": "u", "bodie": "b"}},
		"assertions": []any{"result.find ShouldEqual 1"},
		"nonsense":   true,
	}

	assert.Equal(t, []string{"step key nonsense", "action key clik", "mock key bodie"}, webStepKeyProblems(step))
}

// US1-AS2 (T013): every key of a web step used by the fixtures and by the reference recipes (action, screenshot, mock,
// and the keys under action and mock) is a field of web.Executor, web.Action or web.Mock.
func TestWebStepKeysOfTheRecipesAreFieldsOfTheExecutor(t *testing.T) {
	files := recipeFiles(t, true)
	require.NotEmpty(t, files)
	checked := 0
	for _, f := range files {
		for _, step := range webSteps(t, f) {
			checked++
			assert.Empty(t, webStepKeyProblems(step), f)
		}
	}
	assert.Positive(t, checked, "no web step was examined")
}

// builtinVenom returns a venom with the builtin executors registered, as PreRun of `venom run` does.
func builtinVenom() *venom.Venom {
	v := venom.New()
	for name, executorFunc := range executors.Registry {
		v.RegisterExecutorBuiltin(name, executorFunc())
	}
	return v
}

var templateVar = regexp.MustCompile(`\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)`)

// variablesUsedBy returns every top-level variable the files use, each with a dummy value, so that the loading of
// venom run never stops on a missing variable.
func variablesUsedBy(t *testing.T, files []string) map[string]any {
	t.Helper()
	vars := map[string]any{"password": "supplied-by-the-test"}
	for _, f := range files {
		text, err := os.ReadFile(f) //nolint:gosec // a fixture of the repository
		require.NoError(t, err)
		for _, m := range templateVar.FindAllStringSubmatch(string(text), -1) {
			if m[1] != "result" && m[1] != "venom" {
				vars[m[1]] = "supplied-by-the-test"
			}
		}
	}
	return vars
}

// acceptedByParse reports whether the loading of venom run (Venom.Parse) accepts the files.
func acceptedByParse(t *testing.T, files []string, vars map[string]any) (bool, error) {
	t.Helper()
	v := builtinVenom()
	v.OutputDir = t.TempDir()
	require.NoError(t, v.InitLogger())
	t.Cleanup(func() {
		if c, ok := v.LogOutput.(io.Closer); ok {
			_ = c.Close()
		}
	})
	v.AddVariables(vars)
	err := v.Parse(context.Background(), files)
	return err == nil, err
}

// acceptedByCheck reports whether the recipe check can read the files: no unusable input, no RC10.
func acceptedByCheck(files []string) bool {
	report, err := recipecheck.Check(files, recipecheck.Options{SpecsDir: "docs/specs"})
	if err != nil && errors.Is(err, recipecheck.ErrUnusable) {
		return false
	}
	for _, f := range report.Findings {
		if f.Rule == "RC10" {
			return false
		}
	}
	return err == nil
}

// FR-013 (T032): the check and venom run's own loading (Venom.Parse, builtin executors registered, every variable
// supplied) accept and reject the same files: the fixtures, and files that venom run rejects or accepts by its
// reading of the YAML.
func TestCheckAndParseAcceptAndRejectTheSameFiles(t *testing.T) {
	dir := t.TempDir()
	synthetic := map[string]string{
		"unclosed.yml":    "name: [unclosed\n",
		"tabs.yml":        "name: x\ntestcases:\n\t- name: y\n",
		"scalar.yml":      "name: x\ntestcases: nope\n",
		"mapping.yml":     "name: x\ntestcases:\n  name: y\n",
		"notestcases.yml": "name: Nothing here\nvars:\n  spec: " + testSpec + "\n",
		"emptylist.yml":   "name: Empty list\ntestcases: []\n",
		// venom run rejects both of these, so the check must too (FR-013, RC10).
		"duplicatekey.yml": "name: x\nname: y\ntestcases: []\n",
		"stepnotamap.yml":  "name: x\ntestcases:\n- name: US1-AS1 y\n  steps:\n  - just a string\n",
		"good.yml":         suiteYAML(testcaseYAML("US1-AS1 one")),
		"othertype.yml":    "name: Other\ntestcases:\n- name: US1-AS1 one\n  steps:\n  - type: exec\n    script: echo 1\n    assertions:\n    - result.code ShouldEqual 0\n",
		// FR-013: venom run reads a null testcases and a null step as nothing, and a scalar testcase as an error.
		"testcasesnull.yml":  "name: x\ntestcases:\n",
		"testcasestilde.yml": "name: x\ntestcases: ~\n",
		"nullstep.yml":       "name: x\ntestcases:\n- name: US1-AS1 y\n  steps:\n  - \n",
		"scalartestcase.yml": "name: x\ntestcases:\n- just a string\n",
		"templated.yml":      suiteYAML("- name: US1-AS1 one\n  steps:\n  - type: web\n    action:\n      fill:\n      - find: input\n        text: '{{.password}}'\n"),
	}
	var files []string
	for name, text := range synthetic {
		files = append(files, writeSuite(t, dir, name, text))
	}
	files = append(files, recipeFiles(t, true)...)
	sort.Strings(files)
	vars := variablesUsedBy(t, files)

	for _, f := range files {
		t.Run(filepath.Base(filepath.Dir(f))+"/"+filepath.Base(f), func(t *testing.T) {
			parsed, parseErr := acceptedByParse(t, []string{f}, vars)

			assert.Equal(t, parsed, acceptedByCheck([]string{f}), "venom run's loading said: %v", parseErr)
		})
	}
}

// FR-013: a file with no testcases is accepted by both.
func TestAFileWithNoTestcasesIsAcceptedByBoth(t *testing.T) {
	path := writeSuite(t, t.TempDir(), "none.yml", "name: Nothing here\nvars:\n  a: b\n")

	parsed, parseErr := acceptedByParse(t, []string{path}, nil)

	assert.True(t, parsed, parseErr)
	assert.True(t, acceptedByCheck([]string{path}))
}

// referenceWeb and referenceMobile are the two reference recipes of spec 014 (US5-AS4).
func referenceWeb(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "site", "site-languages.yml")
}

func referenceMobile(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "tests", "recipes", "reference", "mobile")
}

// US5-AS4 (T049): both reference recipes pass the recipe check with no finding.
func TestReferenceRecipesPassTheRecipeCheck(t *testing.T) {
	specs := repoSpecsDir(t)
	for name, path := range map[string]string{"web": referenceWeb(t), "mobile": referenceMobile(t)} {
		t.Run(name, func(t *testing.T) {
			require.True(t, fileExists(path), path)
			report, err := recipecheck.Check([]string{path}, recipecheck.Options{SpecsDir: specs})

			require.NoError(t, err)
			assert.Empty(t, report.Findings)
		})
	}
}

// referenceSuite is the decoded shape of the reference web recipe that the contract talks about.
type referenceSuite struct {
	Vars      map[string]any  `yaml:"vars"`
	Testcases []referenceCase `yaml:"testcases"`
}

type referenceCase struct {
	Name  string          `yaml:"name"`
	Steps []referenceStep `yaml:"steps"`
}

type referenceStep struct {
	Type   string `yaml:"type"`
	Action struct {
		Navigate *struct {
			URL string `yaml:"url"`
		} `yaml:"navigate"`
		Execute *struct {
			Command string `yaml:"command"`
		} `yaml:"execute"`
		Fill []struct {
			Text string `yaml:"text"`
		} `yaml:"fill"`
	} `yaml:"action"`
	Assertions []string `yaml:"assertions"`
}

func loadReferenceWeb(t *testing.T) (referenceSuite, string) {
	t.Helper()
	path := referenceWeb(t)
	text, err := os.ReadFile(path) //nolint:gosec // the reference recipe of the repository
	require.NoError(t, err)
	var suite referenceSuite
	require.NoError(t, yamlv3.Unmarshal(text, &suite))
	return suite, string(text)
}

// scenarioIDs returns the USn-ASm ids of a spec's spec.md.
func scenarioIDs(t *testing.T, spec string) map[string]bool {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(repoSpecsDir(t), spec, "spec.md")) //nolint:gosec // a spec of the repository
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, id := range regexp.MustCompile(`\bUS\d+-AS\d+\b`).FindAllString(string(text), -1) {
		ids[id] = true
	}
	return ids
}

var idPrefix = regexp.MustCompile(`^(US\d+-AS\d+)\b`)

// US5-AS4: the web recipe names its spec and takes its address from the variable appUrl.
func TestReferenceWebRecipeNamesItsSpecAndTakesItsAddressFromAppUrl(t *testing.T) {
	suite, text := loadReferenceWeb(t)

	assert.Equal(t, "013-site-languages", suite.Vars["spec"])
	assert.Contains(t, suite.Vars, "appUrl")
	assert.NotContains(t, text, "siteUrl", "the address variable of a recipe is appUrl")
	assert.Contains(t, text, "{{.appUrl}}")
}

// US5-AS4: every testcase of the web recipe is named after a scenario of spec 013.
func TestReferenceWebTestcasesAreNamedAfterScenariosOfSpec013(t *testing.T) {
	suite, _ := loadReferenceWeb(t)
	ids := scenarioIDs(t, "013-site-languages")
	require.NotEmpty(t, suite.Testcases)

	for _, tc := range suite.Testcases {
		m := idPrefix.FindStringSubmatch(tc.Name)
		if assert.NotNil(t, m, "testcase %q does not start with a scenario id", tc.Name) {
			assert.True(t, ids[m[1]], "testcase %q: %s is not a scenario of spec 013", tc.Name, m[1])
		}
	}
}

// US1-AS14 of spec 013: one width case per language reads scrollWidth against clientWidth through execute.
func TestReferenceWebHasAWidthCasePerLanguageThatReadsScrollWidth(t *testing.T) {
	suite, _ := loadReferenceWeb(t)
	for _, suffix := range []string{"/", "/fr/", "/de/", "/it/", "/es/", "/zh/"} {
		found := false
		for _, tc := range suite.Testcases {
			found = found || isWidthCase(tc, suffix)
		}
		assert.True(t, found, "no US1-AS14 testcase opens {{.appUrl}}%s and compares scrollWidth with clientWidth through execute", suffix)
	}
}

// isWidthCase reports whether a US1-AS14 testcase opens the page at suffix and measures it through execute.
func isWidthCase(tc referenceCase, suffix string) bool {
	if !strings.HasPrefix(tc.Name, "US1-AS14") {
		return false
	}
	navigates, measures := false, false
	for _, s := range tc.Steps {
		navigates = navigates || stepNavigatesTo(s, "{{.appUrl}}"+suffix)
		measures = measures || stepMeasuresWidth(s)
	}
	return navigates && measures
}

func stepNavigatesTo(s referenceStep, url string) bool {
	return s.Action.Navigate != nil && s.Action.Navigate.URL == url
}

func stepMeasuresWidth(s referenceStep) bool {
	return s.Action.Execute != nil && strings.Contains(s.Action.Execute.Command, "scrollWidth") &&
		strings.Contains(s.Action.Execute.Command, "clientWidth") && len(s.Assertions) > 0
}

// US1-AS8 of spec 013: the Chinese search cases: one character, a run, and two terms joined by U+3000.
func TestReferenceWebHasTheChineseSearchCases(t *testing.T) {
	suite, _ := loadReferenceWeb(t)
	cases, fullWidth := 0, false
	for _, tc := range suite.Testcases {
		if !strings.HasPrefix(tc.Name, "US1-AS8") {
			continue
		}
		cases++
		for _, s := range tc.Steps {
			for _, f := range s.Action.Fill {
				fullWidth = fullWidth || strings.ContainsRune(f.Text, '　')
			}
		}
	}

	assert.GreaterOrEqual(t, cases, 3, "US1-AS8 needs a one-character, a run and a full-width-space testcase")
	assert.True(t, fullWidth, "no US1-AS8 testcase types a term with U+3000")
}

// FR-013: the null and scalar entries are pinned one by one: both sides accept a null testcases and a null step, and
// both reject a testcase that is a scalar, which the check reports as RC10.
func TestNullAndScalarTestcaseEntriesAreReadAsVenomRunReadsThem(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		accepted bool
	}{
		{"testcases null", "name: x\ntestcases:\n", true},
		{"testcases tilde", "name: x\ntestcases: ~\n", true},
		{"a null step entry", "name: x\ntestcases:\n- name: US1-AS1 y\n  steps:\n  - \n", true},
		{"a scalar testcase", "name: x\ntestcases:\n- just a string\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeSuite(t, t.TempDir(), "a.yml", tt.text)

			parsed, parseErr := acceptedByParse(t, []string{path}, nil)
			report, _ := recipecheck.Check([]string{path}, recipecheck.Options{SpecsDir: "docs/specs"})

			assert.Equal(t, tt.accepted, parsed, "venom run's loading said: %v", parseErr)
			assert.Equal(t, tt.accepted, acceptedByCheck([]string{path}))
			assert.Equal(t, !tt.accepted, hasRule(report.Findings, "RC10"))
		})
	}
}

func hasRule(findings []recipecheck.Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
