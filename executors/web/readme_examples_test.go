package web

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/internal/yamljson"
)

// FR-012, US4-AS1 (spec 015, T024): every runnable example of the README sections of the new actions parses as steps
// and runs on the fake without an error, and reaches the seam with the call its action makes. The README text is
// checked by scripts/test_site_recipes.py.

// readmeExampleSections are the exact `### ` headings of README.md whose ```yaml blocks are run, each with the prefix
// of a seam call that its examples must produce.
var readmeExampleSections = []struct{ heading, call string }{
	{"Wait for", "WaitFor "},
	{"Hover", "Hover "},
	{"Drag and drop", "DragTo "},
	{"Press", "Press"},
}

var yamlFence = regexp.MustCompile("(?s)```yaml\n(.*?)```")

// readmeSection is the text under the `### heading` of the README, up to the next heading of level 1 to 3.
func readmeSection(t *testing.T, readme, heading string) string {
	t.Helper()
	_, after, found := strings.Cut(readme, "\n### "+heading+"\n")
	require.Truef(t, found, "README.md has no `### %s` section", heading)
	next := regexp.MustCompile(`(?m)^#{1,3} `).FindStringIndex(after)
	if next != nil {
		after = after[:next[0]]
	}
	return after
}

// readmeSteps decodes a yaml block of the README as the list of steps it holds.
func readmeSteps(t *testing.T, block string) []venom.TestStep {
	t.Helper()
	var steps []venom.TestStep
	require.NoError(t, yamljson.Unmarshal([]byte(block), &steps), block)
	return steps
}

func TestReadmeExamplesOfTheNewActionsRunOnTheFake(t *testing.T) {
	raw, err := os.ReadFile("README.md")
	require.NoError(t, err)
	readme := strings.ReplaceAll(string(raw), "\r\n", "\n")

	for _, section := range readmeExampleSections {
		t.Run(section.heading, func(t *testing.T) {
			blocks := yamlFence.FindAllStringSubmatch(readmeSection(t, readme, section.heading), -1)
			require.NotEmpty(t, blocks, "the section holds no ```yaml example")
			pw := withFake(t)
			ctx := openFake(t, pw)
			for _, block := range blocks {
				for _, step := range readmeSteps(t, block[1]) {
					delete(step, "assertions")
					mustRun(ctx, t, step)
				}
			}
			assert.NotEmpty(t, callsWithPrefix(pw.seen(), section.call), "no step of the examples sent %q", section.call)
		})
	}
}
