package skillcheck

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// git runs a git command in the repository and returns its error (an exit code 1 is a plain "no").
func git(t *testing.T, args ...string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoRoot(t)}, args...)...) //nolint:gosec // fixed git subcommands, arguments are commit names from the gate file
	return cmd.Run()
}

// dataLines reads a data file of the package, dropping blank lines and # comments.
func dataLines(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(name)
	require.NoError(t, err)
	var out []string
	for _, l := range splitLines(string(b)) {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

// gateCommits returns the commit hashes of release-gate.txt, skipping the test when one is not in this repository.
func gateCommits(t *testing.T) map[string]string {
	t.Helper()
	commits := map[string]string{}
	for _, l := range dataLines(t, "release-gate.txt") {
		name, sha, ok := strings.Cut(l, " ")
		require.True(t, ok, "release-gate.txt line %q: want <name> <hash>", l)
		if git(t, "cat-file", "-e", sha+"^{commit}") != nil {
			t.Skipf("commit %s (%s) is not in this repository: the gate runs in the private one", sha, name)
		}
		commits[name] = sha
	}
	require.NotEmpty(t, commits)
	return commits
}

// publicTexts returns the text of README.md and of the site's pages, the places the install facts are published.
func publicTexts(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	texts := map[string]string{"README.md": readFile(t, filepath.Join(root, "README.md"))}
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "site"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch filepath.Ext(p) {
		case ".md", ".html", ".json", ".txt":
			rel, relErr := filepath.Rel(root, p)
			require.NoError(t, relErr)
			texts[filepath.ToSlash(rel)] = readFile(t, p)
		}
		return nil
	}))
	return texts
}

// FR-018 (a): the venom release the skill targets contains the merge commits of specs 014 and 015.
func requireReleaseHasTheCommits(t *testing.T, commits map[string]string) {
	t.Helper()
	tag := readReleaseEnv(t)["VENOM_VERSION"]
	require.NoError(t, git(t, "rev-parse", "-q", "--verify", "refs/tags/"+tag+"^{commit}"), "the release %s is not a tag of this repository", tag)
	for name, sha := range commits {
		require.NoError(t, git(t, "merge-base", "--is-ancestor", sha, tag), "%s (%s) is not in %s", name, sha, tag)
	}
}

// FR-018 (b): the signer fingerprint of release.env is published in the README or the site.
func requireFingerprintPublished(t *testing.T) {
	t.Helper()
	pin := readReleaseEnv(t)["SIGNER_SHA256"]
	for _, text := range publicTexts(t) {
		if strings.Contains(normalizeFingerprints(text), pin) {
			return
		}
	}
	t.Fatalf("the signer fingerprint %s is in neither README.md nor the site", pin)
}

// FR-018 (c): none of the stale install strings is left in the README or the site.
func requireNoStaleStrings(t *testing.T) {
	t.Helper()
	stale := dataLines(t, "release-gate-strings.txt")
	for file, text := range publicTexts(t) {
		for _, s := range stale {
			require.NotContains(t, text, s, "%s still holds a stale install text", file)
		}
	}
}

// US6-AS1, US6-AS2, US6-AS3: the filed evaluation report of this skill version meets FR-017.
func requireEvaluationReport(t *testing.T) {
	t.Helper()
	version := readReleaseEnv(t)["SKILL_VERSION"]
	p := filepath.Join(repoRoot(t), "docs", "release", "skill-"+version, "evaluation-report.md")
	b, err := os.ReadFile(p)
	require.NoError(t, err, "no evaluation report for skill version %s", version)
	spec := standardSpec()
	spec.skillVersion = version
	spec.answered = answeredDecisions(t)
	spec.reportExists = func(path string) bool {
		_, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(path)))
		return err == nil
	}
	require.Empty(t, checkEvaluationReport(string(b), spec))
}

var (
	decisionHeading = regexp.MustCompile(`^## (D-[0-9]+)`)
	statusAnswered  = regexp.MustCompile(`^\|\s*Status\s*\|\s*answered\s*\|`)
)

// answeredDecisions lists the decisions of the spec of this skill that are answered (FR-017 as amended by D-022: an
// accepted exception may cite only these).
func answeredDecisions(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "specs", "016-venom-adoption-skill", "decisions.md"))
	require.NoError(t, err)
	ids := map[string]bool{}
	current := ""
	for _, line := range splitLines(string(b)) {
		if m := decisionHeading.FindStringSubmatch(line); m != nil {
			current = m[1]
		} else if current != "" && statusAnswered.MatchString(line) {
			ids[current] = true
		}
	}
	return ids
}

// FR-018, US6: the release gate. It runs only when SKILL_RELEASE_GATE=1 (the release procedure sets it) and only
// where the commits of release-gate.txt exist (the private repository), so the build stays green.
func TestReleaseGate_FR018(t *testing.T) {
	if os.Getenv("SKILL_RELEASE_GATE") != "1" {
		t.Skip("release gate: set SKILL_RELEASE_GATE=1 to run it")
	}
	requireOnPath(t, "git")
	commits := gateCommits(t)
	t.Run("a: the targeted release contains specs 014 and 015", func(t *testing.T) { requireReleaseHasTheCommits(t, commits) })
	t.Run("b: the signer fingerprint is published", requireFingerprintPublished)
	t.Run("c: the stale install texts are corrected", requireNoStaleStrings)
	t.Run("US6: the evaluation report meets FR-017", requireEvaluationReport)
}
