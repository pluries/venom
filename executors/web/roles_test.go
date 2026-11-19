package web

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// The ROLE locator accepts the ARIA role names of the pinned Playwright binding (spec 015, FR-004, D-010): the table of
// roles.go is checked against the binding's own constants, so an upgrade that changes the list fails here.

const playwrightModule = "github.com/mxschmitt/playwright-go"

var ariaRoleConstant = regexp.MustCompile(`getAriaRole\("([^"]+)"\)`)

// bindingRoles are the role names of the generated-enums.go of the playwright-go module in use; the test is skipped
// when the module cache is absent.
func bindingRoles(t *testing.T) []string {
	t.Helper()
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", playwrightModule).Output()
	if err != nil {
		t.Skipf("the module directory of %s is not available: %v", playwrightModule, err)
	}
	src, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), "generated-enums.go"))
	if err != nil {
		t.Skipf("the module cache does not hold generated-enums.go: %v", err)
	}
	var roles []string
	for _, m := range ariaRoleConstant.FindAllStringSubmatch(string(src), -1) {
		roles = append(roles, m[1])
	}
	require.NotEmpty(t, roles, "no AriaRole constant found in generated-enums.go: the test's reading is wrong")
	return roles
}

func TestAriaRoleNames_equalTheBinding(t *testing.T) {
	// FR-004, D-010: the table holds every role of the binding, none more, none twice.
	assert.ElementsMatch(t, bindingRoles(t), ariaRoleNames)
}

func TestParseFind_roleNames(t *testing.T) {
	// US1-AS11: button is accepted; the role is matched case-sensitively and a word that is not a role is refused with
	// the list of the binding's roles.
	venom.InitTestLogger(t)
	roles := bindingRoles(t)

	got, err := parseFind(context.Background(), loc("ROLE", "button"))
	require.NoError(t, err)
	assert.Equal(t, locatorSpec{kind: roleLocator, selector: "button"}, got)

	for _, bad := range []string{"Button", "BUTTON", "banana", " button"} {
		t.Run(bad, func(t *testing.T) {
			_, err := parseFind(context.Background(), loc("ROLE", bad))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "selector")
			for _, role := range roles {
				assert.Contains(t, err.Error(), role)
			}
		})
	}
}

func TestRun_roleOutsideTheListMakesNoBrowserCall(t *testing.T) {
	// US1-AS11: the step fails before any browser action and the message lists the accepted role names.
	roles := bindingRoles(t)
	pw := withFake(t)
	ctx := openFake(t, pw)
	_, err := runResult(ctx, t, findStep(loc("ROLE", "Button", "name", "Add")))
	require.Error(t, err)
	for _, role := range roles {
		assert.Contains(t, err.Error(), role)
	}
	assert.Equal(t, []string{"Content page1"}, pw.seen())
}
