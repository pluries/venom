package skillcheck

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var headingStart = regexp.MustCompile(`(?m)^#{1,6} `)

// sections splits a Markdown text at its headings; a section holds the heading, its paragraphs and its code blocks.
func sections(text string) []string {
	locs := headingStart.FindAllStringIndex(text, -1)
	var out []string
	for i, l := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, text[l[0]:end])
	}
	return out
}

// requireSection checks that one section matches every pattern (case-insensitive, dot matches a line break).
func requireSection(t *testing.T, text string, patterns ...string) {
	t.Helper()
	for _, s := range sections(text) {
		if matchesAll(s, patterns...) {
			return
		}
	}
	t.Fatalf("no single section matches all of %q", patterns)
}

func firstSuiteText(t *testing.T) string {
	t.Helper()
	return skillText(t, "references/first-suite.md")
}

// US2-AS5, FR-005 (F1): one section says the skill never creates a file that holds a credential value, variable
// file included (it may write variables.example.yml with empty or placeholder values), that the developer sets the
// variable in the environment (PowerShell and bash forms, with a placeholder) or creates a variable file
// themselves, and that the skill stops and asks when the variable is not set.
func TestRules_US2AS5_NeverCreatesAFileHoldingACredential(t *testing.T) {
	requireSection(t, firstSuiteText(t),
		`\bnever\b[^.]{0,60}\b(creat|writ)\w*[^.]{0,80}\b(credential|token|secret)`,
		`variable file`,
		`variables\.example\.yml`,
		`(empty|placeholder)`,
		`\bdeveloper\b[^.]{0,80}\b(sets?|creates?)\b`,
		`\bstop\w*\b[^.]{0,60}\bask|\bask\w*\b[^.]{0,60}\bstop`,
		`(not set|unset|is missing)`,
	)
}

// US2-AS5 (F1): the environment forms are shown, PowerShell and bash, each with a placeholder and never a value.
func TestRules_US2AS5_EnvironmentFormsShowAPlaceholder(t *testing.T) {
	text := firstSuiteText(t)
	require.Regexp(t, regexp.MustCompile(`\$env:VENOM_VAR_\w+\s*=\s*['"]?<[^>\n]+>`), text, "the PowerShell form with a placeholder")
	require.Regexp(t, regexp.MustCompile(`export VENOM_VAR_\w+=['"]?<[^>\n]+>`), text, "the bash form with a placeholder")
}

// US2-AS5 (F1): --var-from-file is only ever "a file the developer creates"; every paragraph that names it says so.
func TestRules_US2AS5_VarFromFileIsADeveloperFile(t *testing.T) {
	found := false
	for _, p := range paragraphs(firstSuiteText(t)) {
		if !regexp.MustCompile(`--var-from-file`).MatchString(p) {
			continue
		}
		found = true
		require.True(t, matchesAll(p, `\bdeveloper\b`, `(creates|created|themselves|their own)`),
			"a paragraph names --var-from-file without saying the developer creates the file:\n%s", p)
	}
	require.True(t, found, "first-suite.md names --var-from-file")
}

var writesTokenFile = regexp.MustCompile(`(?i)\b(write|create|save|put)\b[^.\n]{0,60}(variables?\.ya?ml|variable file)[^.\n]{0,60}(token|credential|secret|real value)`)

// US2-AS5 (F1): no line tells the skill to write a variable file with a token, unless the line is a refusal.
func TestRules_US2AS5_NoInstructionToWriteATokenFile(t *testing.T) {
	for i, line := range splitLines(firstSuiteText(t)) {
		if writesTokenFile.MatchString(line) {
			require.Regexp(t, negation, line, "line %d tells the skill to write a variable file with a credential", i+1)
		}
	}
}

// US2-AS3 (F2): before it starts the service, the skill checks whether the port is already in use (the health
// address with curl, or the listening ports); if something answers it says so and asks whether it is the developer's.
func TestRules_US2AS3_CheckThePortBeforeStartingTheService(t *testing.T) {
	requireParagraph(t, firstSuiteText(t),
		`\bbefore\b[^.]{0,60}\bstart`,
		`\bport\b`,
		`(already in use|already answers|in use)`,
		`(curl|listening ports)`,
		`\bask`,
		`(his|theirs|the developer's|their own)`,
	)
}

// US2-AS3 (F2): it never starts a second copy on another port without saying so.
func TestRules_US2AS3_NoSecondCopyOnAnotherPortWithoutSayingSo(t *testing.T) {
	requireParagraph(t, firstSuiteText(t),
		`\bnever\b`,
		`(second copy|another port)`,
		`(without saying|say so|tell the developer)`,
	)
}
