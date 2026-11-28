package skillcheck

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func recipesText(t *testing.T) string {
	t.Helper()
	return skillText(t, "references/recipes.md")
}

// US4-AS1, FR-010: recipes.md states the Go check, names what to install without Go (Go itself, or a Chromium path
// through web.binaryPath) and stops, installing nothing.
func TestRecipes_US4AS1_GoCheckAndStopWithoutIt(t *testing.T) {
	text := recipesText(t)
	require.Contains(t, text, "go version", "the Go check is a command")
	require.Contains(t, text, "web.binaryPath")
	requireParagraph(t, text, `\bgo\b`, `\bstop`, `(install(s|ing)? nothing|nothing (is )?install|does not install|never install)`)
}

// US4-AS3, FR-011: recipes.md states what a mobile project provides and that the skill installs and starts no device host.
func TestRecipes_US4AS3_NoDeviceHostInstalled(t *testing.T) {
	text := recipesText(t)
	requireParagraph(t, text, `appium`, `(emulator|device)`, `ssh`)
	requireParagraph(t, text, `appium`, `(never|not|no)\b`, `install`, `start`)
}

// US4-AS5: recipes.md says which version has a feature the pinned one lacks, and writes no recipe that cannot run on it.
func TestRecipes_US4AS5_VersionRule(t *testing.T) {
	text := recipesText(t)
	requireParagraph(t, text, `pinned`, `(lacks|does not (have|exist)|older|too old|missing)`, `which version`)
	requireParagraph(t, text, `(never|not|no)\b`, `(write|writes|writing)`, `recipe`, `(cannot run|can't run|pinned)`)
}

// US4-AS2 (0.1.5, evaluation of 0.1.4 runs web-f2, web-g1, web-bare-g1, web-bare-g2): the recipe check ties every testcase
// to a scenario of a spec. With no spec for the page the skill asks the developer for the scenarios; it writes no spec and
// invents no scenario from what the page does today.
func TestRecipes_US4AS2_NoSpecWrittenUnasked(t *testing.T) {
	text := recipesText(t)
	requireParagraph(t, text, `\bspec\b`, `\bask`, `scenario`)
	requireParagraph(t, text, `\bspec\b`, `(never|not|no)\b`, `(write|writes|writing|invent)`)
}

// US4-AS2, FR-010: recipes.md runs `venom recipe check` with the project's spec folder, and does not call a recipe
// proven until the check is clean and its negative control fails as expected.
func TestRecipes_US4AS2_RecipeCheck(t *testing.T) {
	text := recipesText(t)
	require.Contains(t, text, "venom recipe check")
	require.Contains(t, text, "--specs-dir")
	requireParagraph(t, text, `negative control`, `(proven|prove)`, `(clean|no finding)`)
	require.Regexp(t, regexp.MustCompile(`(?i)findings?`), text)
}

// US4-AS1 (0.1.9, evaluation of 0.1.8: web run q1 stopped for the scenarios and the documentation and wrote no recipe, q2 and q3 wrote one):
// with no spec for the page the skill still writes the first recipe from what the page offers, replays it, runs the check and reports its
// findings as such, and asks for the scenarios after; it does not stop before a recipe exists. The documentation page is read with an approved
// command when the fetch tool is not allowed.
func TestRecipes_US4AS1_FirstRecipeBeforeTheScenarios(t *testing.T) {
	text := recipesText(t)
	requireParagraph(t, text, `provisional`, `(no|without|has no) spec`, `do not stop`, `replay`, `ask for the scenarios`)
	requireParagraph(t, text, `recipes[.]html`, `fetch tool`, `approv`)
	requireParagraph(t, text, `curl -fsSL https://venom[.]pluries[.]dev/recipes[.]html`)
}
