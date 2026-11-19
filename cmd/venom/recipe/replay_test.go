//go:build playwright

package recipe

// Replay tests: the venom binary runs recipes in a real browser (Playwright's Chromium). They run where a browser
// exists, behind the playwright build tag: go test -tags playwright ./cmd/venom/recipe/...

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv3 "go.yaml.in/yaml/v3"
)

// venomBin is the venom binary TestMain builds once.
var venomBin string

func TestMain(m *testing.M) {
	// The child process of recipeExit runs in a folder with no go.mod and needs no binary.
	if os.Getenv(helperArgsEnv) != "" {
		os.Exit(m.Run())
	}
	os.Exit(buildAndRun(m))
}

func buildAndRun(m *testing.M) int {
	root, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go list:", err)
		return 1
	}
	dir, err := os.MkdirTemp("", "venom-recipe-replay")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	venomBin = filepath.Join(dir, "venom")
	if runtime.GOOS == "windows" {
		venomBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", venomBin, "./cmd/venom")
	build.Dir = strings.TrimSpace(string(root))
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ./cmd/venom: %v\n%s", err, out)
		return 1
	}
	return m.Run()
}

// fileURL is the file URL of a path, built with net/url (a leading slash and forward slashes on Windows).
func fileURL(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func cleanRecipe(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixturesDir(t), "clean", "page.yml")
}

func pageURL(t *testing.T) string {
	t.Helper()
	return fileURL(t, filepath.Join(fixturesDir(t), "page", "index.html"))
}

// recipeUnderTest is the recipe and the address of T014 and T034: the clean fixture and its page by default, any
// other recipe and address through RECIPE_PATH and RECIPE_APP_URL.
func recipeUnderTest(t *testing.T) (string, string) {
	t.Helper()
	path, address := os.Getenv("RECIPE_PATH"), os.Getenv("RECIPE_APP_URL")
	if path == "" {
		path = cleanRecipe(t)
	}
	if address == "" {
		address = pageURL(t)
	}
	return path, address
}

// replayResult is what one run of the binary left.
type replayResult struct {
	Exit   int
	Output string
	// Statuses maps each testcase name to its status in the JSON report.
	Statuses map[string]string
}

// replay runs `venom run` on the recipe in a clean folder, with the address as the variable appUrl.
func replay(t *testing.T, recipe, address string) replayResult {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command(venomBin, "run", "--format=json", "--output-dir", dir, "--var", "appUrl="+address, recipe) //nolint:gosec // the binary the test built
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	result := replayResult{Statuses: map[string]string{}}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr), "the binary did not start: %v", err)
		result.Exit = exitErr.ExitCode()
	}
	result.Output = out.String()
	reports, _ := filepath.Glob(filepath.Join(dir, "test_results_*.json"))
	for _, r := range reports {
		text, err := os.ReadFile(r) //nolint:gosec // the report the run just wrote
		require.NoError(t, err)
		var report struct {
			TestSuites []struct {
				Testcases []struct {
					Name   string `json:"name"`
					Status string `json:"status"`
				} `json:"testcases"`
			} `json:"test_suites"`
		}
		require.NoError(t, json.Unmarshal(text, &report))
		for _, s := range report.TestSuites {
			for _, tc := range s.Testcases {
				result.Statuses[tc.Name] = tc.Status
			}
		}
	}
	return result
}

// scenarioOf is the scenario id a testcase name starts with, for the messages.
func scenarioOf(name string) string {
	first, _, _ := strings.Cut(name, " ")
	return first
}

// US1-AS1 (T012): the clean fixture replays and exits 0 against the static page, with the address as a file URL.
func TestCleanFixtureReplaysAndExitsZero(t *testing.T) {
	address := pageURL(t)
	require.True(t, strings.HasPrefix(address, "file:///"), address)

	result := replay(t, cleanRecipe(t), address)

	assert.Equal(t, 0, result.Exit, result.Output)
	assert.Contains(t, result.Output, "final status:")
	assert.NotEmpty(t, result.Statuses)
	for name, status := range result.Statuses {
		assert.Equal(t, "PASS", status, name)
	}
}

// US1-AS1 (T012): a copy whose expected value is changed exits 2 and names the testcase.
func TestRecipeWithAWrongExpectedValueExitsTwoAndNamesTheTestcase(t *testing.T) {
	text, err := os.ReadFile(cleanRecipe(t))
	require.NoError(t, err)
	const right = `result.title ShouldEqual "Recipe fixture page"`
	require.Contains(t, string(text), right)
	copyPath := filepath.Join(t.TempDir(), "wrong.yml")
	require.NoError(t, os.WriteFile(copyPath, []byte(strings.Replace(string(text), right, `result.title ShouldEqual "A different title"`, 1)), 0o600))

	result := replay(t, copyPath, pageURL(t))

	assert.Equal(t, 2, result.Exit, result.Output)
	assert.Contains(t, result.Output, `Testcase "US1-AS1 the page shows its title and heading"`)
}

// US1-AS3, SC-003 (T014): twenty replays give the same per-testcase results and the same exit status.
func TestTwentyReplaysAreIdentical(t *testing.T) {
	recipePath, address := recipeUnderTest(t)

	first := replay(t, recipePath, address)
	require.NotEmpty(t, first.Statuses, first.Output)
	for i := 2; i <= 20; i++ {
		next := replay(t, recipePath, address)

		assert.Equal(t, first.Exit, next.Exit, "replay %d: %s", i, next.Output)
		assert.Equal(t, first.Statuses, next.Statuses, "replay %d", i)
	}
	assert.Equal(t, 0, first.Exit, first.Output)
}

func actionsRecipe(t *testing.T) string {
	t.Helper()
	return filepath.Join(fixturesDir(t), "actions", "page.yml")
}

// US3-AS2 (T023): the actions fixture replays against the fixture page and exits 0, every testcase PASS.
func TestActionsFixtureReplaysAndExitsZero(t *testing.T) {
	result := replay(t, actionsRecipe(t), pageURL(t))

	assert.Equal(t, 0, result.Exit, result.Output)
	assert.NotEmpty(t, result.Statuses, result.Output)
	for name, status := range result.Statuses {
		assert.Equal(t, "PASS", status, name)
	}
}

// T023: a copy of the actions fixture whose first testcase expects another value exits 2 and names that testcase.
func TestActionsFixtureWithAWrongExpectedValueExitsTwoAndNamesTheTestcase(t *testing.T) {
	original, err := os.ReadFile(actionsRecipe(t)) //nolint:gosec // the fixture under test
	require.NoError(t, err)
	names, nodes := firstMutableAssertion(t, original)
	require.NotEmpty(t, names)
	require.NotNil(t, nodes[0], "the first testcase has no app-reading assertion that a changed expected value can fail")
	copyPath := filepath.Join(t.TempDir(), "control.yml")
	require.NoError(t, os.WriteFile(copyPath, mutated(original, nodes[0]), 0o600))

	result := replay(t, copyPath, pageURL(t))

	assert.Equal(t, 2, result.Exit, result.Output)
	assert.Contains(t, result.Output, fmt.Sprintf("Testcase %q", names[0]))
}

// appFieldPrefix matches an assertion that reads the application.
var appFieldPrefix = regexp.MustCompile(`^result\.(find|title|url|text|html|value|valuejson|count|found|source|context|device|platform)\b`)

// firstMutableAssertion returns, for each testcase of the recipe text, the node of the first app-reading assertion
// whose operator mutateAssertion can change so that it fails.
func firstMutableAssertion(t *testing.T, text []byte) ([]string, []*yamlv3.Node) {
	t.Helper()
	var doc yamlv3.Node
	require.NoError(t, yamlv3.Unmarshal(text, &doc))
	var suite struct {
		Testcases []struct {
			Name  string `yaml:"name"`
			Steps []struct {
				Assertions []yamlv3.Node `yaml:"assertions"`
			} `yaml:"steps"`
		} `yaml:"testcases"`
	}
	require.NoError(t, doc.Decode(&suite))
	var names []string
	var nodes []*yamlv3.Node
	for _, tc := range suite.Testcases {
		var found *yamlv3.Node
		for _, step := range tc.Steps {
			for i := range step.Assertions {
				a := &step.Assertions[i]
				if _, ok := mutateAssertion(a.Value); found == nil && ok && appFieldPrefix.MatchString(a.Value) {
					found = a
				}
			}
		}
		names = append(names, tc.Name)
		nodes = append(nodes, found)
	}
	return names, nodes
}

// mutated returns the recipe text with the assertion at node replaced by its mutation, as a YAML string.
func mutated(text []byte, node *yamlv3.Node) []byte {
	lines := strings.Split(string(text), "\n")
	changed, _ := mutateAssertion(node.Value)
	prefix := lines[node.Line-1][:node.Column-1]
	lines[node.Line-1] = prefix + strconv.Quote(changed)
	return []byte(strings.Join(lines, "\n"))
}

// US3-AS5, FR-016, SC-001 (T034): for each testcase, a copy whose first app-reading expected value is changed fails
// with exit 2 naming that testcase; the original is unchanged and still passes.
func TestNegativeControlOfEveryTestcase(t *testing.T) {
	recipePath, address := recipeUnderTest(t)
	original, err := os.ReadFile(recipePath) //nolint:gosec // the recipe under test
	require.NoError(t, err)
	names, nodes := firstMutableAssertion(t, original)
	require.NotEmpty(t, names)

	for i, name := range names {
		t.Run(scenarioOf(name), func(t *testing.T) {
			require.NotNil(t, nodes[i], "testcase %q has no app-reading assertion that a changed expected value can fail", name)
			copyPath := filepath.Join(t.TempDir(), "control.yml")
			require.NoError(t, os.WriteFile(copyPath, mutated(original, nodes[i]), 0o600))

			result := replay(t, copyPath, address)

			assert.Equal(t, 2, result.Exit, result.Output)
			assert.Contains(t, result.Output, fmt.Sprintf("Testcase %q", name))
		})
	}

	after, err := os.ReadFile(recipePath) //nolint:gosec // the recipe under test
	require.NoError(t, err)
	assert.Equal(t, string(original), string(after), "the original recipe must be unchanged")
	assert.Equal(t, 0, replay(t, recipePath, address).Exit, "the original recipe still passes")
}
