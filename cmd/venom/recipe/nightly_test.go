package recipe

// The nightly job replays the reference web recipe (spec 014, FR-008, D-003, plan O-5). These tests read
// jenkins/nightly.Jenkinsfile as text and check the entries of nightly.suites.setup, run and teardown. The public fork
// may strip the Jenkinsfile: the tests skip when it is absent.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	nightlyClean      = "git clean -fdxq ."
	nightlyInstall    = "go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium"
	nightlyBuildSite  = "python3 ../scripts/build_site.py --out ../build/site"
	nightlyGoTest     = "cd .. && go test -tags playwright ./cmd/venom/recipe/..."
	nightlyServe      = "python3 -m http.server 8000 --directory ../build/site --bind 127.0.0.1"
	nightlyReplay     = "./venom run site/site-languages.yml --var appUrl=http://127.0.0.1:8000"
	nightlyRunTestCmd = "make run-test STACK_HOST="
)

// nightlyJenkinsfile returns the text of jenkins/nightly.Jenkinsfile, or skips the test.
func nightlyJenkinsfile(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	require.NoError(t, err)
	path := filepath.Join(strings.TrimSpace(string(out)), "jenkins", "nightly.Jenkinsfile")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("jenkins/nightly.Jenkinsfile is absent (stripped from the public fork)")
	}
	require.NoError(t, err)
	return string(data)
}

// nightlyList returns the entries of the Groovy list `key: [ ... ]`, each entry being its string literals joined.
func nightlyList(t *testing.T, src, key string) []string {
	t.Helper()
	loc := regexp.MustCompile(`(?m)^\s*` + key + `\s*:\s*\[`).FindStringIndex(src)
	require.NotNil(t, loc, "list %q not found in the Jenkinsfile", key)
	return nightlyEntries(src[loc[1]:])
}

func nightlyEntries(s string) []string {
	var entries []string
	var cur strings.Builder
	hasLit := false
	flush := func() {
		if hasLit {
			entries = append(entries, cur.String())
		}
		cur.Reset()
		hasLit = false
	}
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\'':
			lit, next := nightlyLiteral(s, i+1)
			cur.WriteString(lit)
			hasLit = true
			i = next
		case strings.HasPrefix(s[i:], "//"):
			i += strings.IndexByte(s[i:], '\n')
		case s[i] == ',':
			flush()
		case s[i] == ']':
			flush()
			return entries
		}
	}
	return entries
}

// nightlyLiteral reads a single-quoted Groovy literal from start; it returns its text and the index of the closing quote.
func nightlyLiteral(s string, start int) (string, int) {
	var b strings.Builder
	for i := start; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case s[i] == '\'':
			return b.String(), i
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String(), len(s)
}

func nightlyIndex(entries []string, pred func(string) bool) int {
	for i, e := range entries {
		if pred(e) {
			return i
		}
	}
	return -1
}

func nightlyEquals(want string) func(string) bool {
	return func(e string) bool { return strings.TrimSpace(e) == want }
}

func nightlyContains(parts ...string) func(string) bool {
	return func(e string) bool {
		for _, p := range parts {
			if !strings.Contains(e, p) {
				return false
			}
		}
		return true
	}
}

func TestNightlySetupInstallsPlaywrightBrowserAfterTheClean(t *testing.T) {
	setup := nightlyList(t, nightlyJenkinsfile(t), "setup")

	clean := nightlyIndex(setup, nightlyEquals(nightlyClean))
	install := nightlyIndex(setup, nightlyEquals(nightlyInstall))

	require.GreaterOrEqual(t, clean, 0, "the git clean entry must stay")
	require.GreaterOrEqual(t, install, 0, "setup has no entry that installs the Playwright browser")
	assert.Greater(t, install, clean, "the install must come after git clean, which would remove it")
}

func TestNightlyRunBuildsTheSiteInItsOwnEntry(t *testing.T) {
	run := nightlyList(t, nightlyJenkinsfile(t), "run")

	build := nightlyIndex(run, nightlyEquals(nightlyBuildSite))
	replay := nightlyIndex(run, nightlyContains(nightlyReplay))

	require.GreaterOrEqual(t, build, 0, "run has no separate entry that builds the site")
	require.GreaterOrEqual(t, replay, 0, "run has no replay entry")
	assert.Less(t, build, replay, "the site must be built before the replay")
}

func TestNightlyRunExecutesThePlaywrightRecipeTests(t *testing.T) {
	run := nightlyList(t, nightlyJenkinsfile(t), "run")

	assert.GreaterOrEqual(t, nightlyIndex(run, nightlyEquals(nightlyGoTest)), 0,
		"run has no separate entry for the tagged recipe tests")
}

func TestNightlyRunReplaysInOneShellThatServesAndKeepsTheExitStatus(t *testing.T) {
	run := nightlyList(t, nightlyJenkinsfile(t), "run")

	idx := nightlyIndex(run, nightlyContains(nightlyReplay))
	require.GreaterOrEqual(t, idx, 0, "run has no replay entry")
	entry := run[idx]

	assert.True(t, strings.HasPrefix(strings.TrimSpace(entry), "sh -c"), "the replay is one sh -c entry: %q", entry)
	assert.Contains(t, entry, nightlyServe, "the same entry starts the server")
	assert.Regexp(t, `http\.server[^;&|]*&`, entry, "the server starts in the background")
	assert.Contains(t, entry, "kill", "the same entry kills the server")
	assert.Regexp(t, `exit\s+"?\$`, entry, "the entry exits with the replay's status, not the kill's")
	assert.Less(t, strings.Index(entry, nightlyServe), strings.Index(entry, nightlyReplay), "serve before replay")
	assert.Less(t, strings.Index(entry, nightlyReplay), strings.LastIndex(entry, "kill"), "kill after replay")
}

func TestNightlyTeardownAlsoKillsTheServer(t *testing.T) {
	teardown := nightlyList(t, nightlyJenkinsfile(t), "teardown")

	assert.GreaterOrEqual(t, nightlyIndex(teardown, nightlyContains("kill", "http.server")), 0,
		"teardown has no entry that kills the site server")
	assert.GreaterOrEqual(t, nightlyIndex(teardown, nightlyEquals("make stop-test-stack")), 0,
		"make stop-test-stack must stay")
}

// Guard: what the job had before spec 014 must not change.
func TestNightlyKeepsItsExistingConfiguration(t *testing.T) {
	src := nightlyJenkinsfile(t)

	assert.Regexp(t, `enabled\s*:\s*true`, src)
	assert.Regexp(t, `agentLabel\s*:\s*'docker-e2e'`, src)
	assert.Regexp(t, `dir\s*:\s*'tests'`, src)

	setup := nightlyList(t, src, "setup")
	for _, want := range []string{
		nightlyClean,
		"make start-test-stack STACK_HOST=",
		"make wait-for-kafka STACK_HOST=",
		"make build-test-binary",
		"make build-venom",
		"ssh-keygen",
	} {
		assert.GreaterOrEqual(t, nightlyIndex(setup, nightlyContains(want)), 0, "setup lost %q", want)
	}
	assert.GreaterOrEqual(t, nightlyIndex(nightlyList(t, src, "run"), nightlyContains(nightlyRunTestCmd)), 0,
		"run lost make run-test")
}
