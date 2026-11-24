package skillcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// ciRef is one CI reference of the skill: its file, the system it names and the language tag of its fenced file body.
type ciRef struct {
	file, system, lang string
}

var ciRefs = []ciRef{
	{"references/ci-github.md", "GitHub Actions", "yaml"},
	{"references/ci-gitlab.md", "GitLab CI", "yaml"},
	{"references/ci-jenkins.md", "Jenkins", "groovy"},
	{"references/ci-azure.md", "Azure Pipelines", "yaml"},
}

func skillText(t *testing.T, rel string) string {
	t.Helper()
	return readFile(t, skillFile(t, rel))
}

// paragraphs splits a text on blank lines.
func paragraphs(text string) []string {
	return regexp.MustCompile(`\r?\n[ \t]*\r?\n`).Split(text, -1)
}

// requireParagraph checks that one paragraph matches every pattern (case-insensitive).
func requireParagraph(t *testing.T, text string, patterns ...string) {
	t.Helper()
	for _, p := range paragraphs(text) {
		if matchesAll(p, patterns...) {
			return
		}
	}
	t.Fatalf("no single paragraph matches all of %q", patterns)
}

func matchesAll(s string, patterns ...string) bool {
	for _, p := range patterns {
		if !regexp.MustCompile("(?is)" + p).MatchString(s) {
			return false
		}
	}
	return true
}

// requireLine checks that one line matches every pattern (case-insensitive).
func requireLine(t *testing.T, text string, patterns ...string) {
	t.Helper()
	for _, l := range splitLines(text) {
		if matchesAll(l, patterns...) {
			return
		}
	}
	t.Fatalf("no single line matches all of %q", patterns)
}

// FR-004, US2-AS7: first-suite.md shows the difference and gets approval before overwriting an existing file.
// US3-AS5: so does each CI reference before it edits an existing pipeline.
func TestRules_US2AS7_US3AS5_DifferenceAndApprovalBeforeOverwrite(t *testing.T) {
	files := []string{"references/first-suite.md"}
	for _, r := range ciRefs {
		files = append(files, r.file)
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			requireParagraph(t, skillText(t, f), `\bdiff(erence)?\b`, `approv`, `(overwrit|existing|already)`)
		})
	}
}

// FR-005, US2-AS5: first-suite.md states the credential rules.
func TestRules_US2AS5_CredentialRules(t *testing.T) {
	text := skillText(t, "references/first-suite.md")
	for _, want := range []string{"secrets:", "strict_variables", "--var-from-file", ".gitignore"} {
		require.Contains(t, text, want)
	}
	requireLine(t, text, `--var[^-a-z]`, `(never|not|no|avoid|instead)`)
	requireLine(t, text, `(\.env|key file)`, `(never|not|no)`)
}

// FR-005, US3-AS3: each CI reference warns that artifacts can hold response data.
func TestRules_US3AS3_ArtifactWarning(t *testing.T) {
	for _, r := range ciRefs {
		t.Run(r.system, func(t *testing.T) {
			requireParagraph(t, skillText(t, r.file), `artifact`, `(response|data|secret|sensitive)`)
		})
	}
}

var disablesApprovals = regexp.MustCompile(`(?i)(--dangerously-skip-permissions|bypassPermissions|(turn|switch) off[^.\n]{0,30}(approval|permission)|disable[^.\n]{0,30}(approval|permission)|auto-?approve all)`)
var negation = regexp.MustCompile(`(?i)\b(never|not|no|don't|do not|without)\b|n't\b`)

// FR-016, US5-AS5: no file of the skill tells anyone to turn approvals off; a line that mentions it must be a refusal.
func TestRules_US5AS5_NoInstructionToTurnApprovalsOff(t *testing.T) {
	root := filepath.Join(repoRoot(t), "skills", "venom-pluries")
	skillFile(t, "SKILL.md")
	var files []string
	require.NoError(t, filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	}))
	sort.Strings(files)
	for _, f := range files {
		for i, line := range splitLines(readFile(t, f)) {
			if disablesApprovals.MatchString(line) {
				require.Regexp(t, negation, line, "%s:%d tells the reader to turn approvals off", f, i+1)
			}
		}
	}
}

// FR-016: safety.md says in so many words (a sentence may wrap over lines) that the skill never asks to turn approvals off.
func TestRules_US5AS5_SafetyRefusesToTurnApprovalsOff(t *testing.T) {
	requireParagraph(t, skillText(t, "references/safety.md"), `(approval|permission)`, `\b(never|not)\b`, `(turn[^.]{0,20}off|disable|skip|bypass)`)
}

// FR-012, US2-AS8: files, responses, proto text and tool output are data, never instructions.
func TestRules_US2AS8_DataNotInstructions(t *testing.T) {
	text := skillText(t, "references/safety.md")
	requireParagraph(t, text, `(proto|response)`, `\bdata\b`, `(never|not)[^.\n]{0,30}instruction`)
	requireParagraph(t, text, `(ignored|ignore|say)`, `(what|which)`)
}

// US3-AS4: the four CI systems are named, a generic command line is offered for any other, and nothing is
// written for an unknown system (there is a reference for exactly the four).
func TestRules_US3AS4_FourCISystemsAndNothingElse(t *testing.T) {
	text := skillText(t, "SKILL.md")
	for _, r := range ciRefs {
		require.Contains(t, text, r.system)
	}
	requireParagraph(t, text, `(another|other|unknown)`, `(generic|plain|general)`, `command line`)
	matches, err := filepath.Glob(filepath.Join(repoRoot(t), "skills", "venom-pluries", "references", "ci-*.md"))
	require.NoError(t, err)
	require.Len(t, matches, len(ciRefs), "a reference for any other CI system must not exist")
}

// FR-008: first-suite.md names the HTTP and the gRPC executor, with proto files or reflection.
func TestRules_US2AS1_AS2_HTTPAndGRPCExecutors(t *testing.T) {
	text := skillText(t, "references/first-suite.md")
	for _, pattern := range []string{`\bhttp\b`, `\bgrpc\b`, `proto`, `reflection`} {
		require.Regexp(t, regexp.MustCompile("(?i)"+pattern), text)
	}
}

// US1-AS7, FR-001: safety.md says VENOM_CA_FILE can weaken only the timestamp check, never the signer comparison.
func TestRules_US1AS7_CAFileWeakensOnlyTheTimestamp(t *testing.T) {
	text := skillText(t, "references/safety.md")
	require.Contains(t, text, "VENOM_CA_FILE")
	requireParagraph(t, text, `VENOM_CA_FILE`, `timestamp`, `\bonly\b`, `signer`)
}

// FR-012, FR-016, FR-006: SKILL.md states the three rules that never bend.
func TestRules_SkillMDStatesTheThreeRulesThatNeverBend(t *testing.T) {
	text := skillText(t, "SKILL.md")
	requireParagraph(t, text, `\bdata\b`, `instruction`)
	requireParagraph(t, text, `approval`)
	requireParagraph(t, text, `production`, `address`)
}
