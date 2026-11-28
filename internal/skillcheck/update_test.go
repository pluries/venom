package skillcheck

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// #3944 (skill 1.0.1): update.md carries the install command proven against the public fork at a tag (docs/release/skill-1.0.0/npx-skills-add-proof.md),
// shown for approval, with the tool's telemetry off, and never without a tag; the old sentence that sent the skill to the tool's documentation is gone.
func TestUpdate_InstallCommandIsTheProvenOne(t *testing.T) {
	text := skillText(t, "references/update.md")
	requireParagraph(t, text, `npx skills add pluries/venom@<tag> --skill venom-pluries`, `DISABLE_TELEMETRY=1`)
	requireParagraph(t, text, `this command`, `approval`, `anonymous usage data`)
	requireParagraph(t, text, `never run it without a tag`)
	require.NotRegexp(t, regexp.MustCompile(`(?i)check its documentation`), text)
}
