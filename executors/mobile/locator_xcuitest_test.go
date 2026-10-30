package mobile

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ovh/venom"
)

// iosCase opens a test case on a fresh fake server that answers as an iOS server.
func iosCase(t *testing.T) (context.Context, *fakeServer) {
	t.Helper()
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	iosAnswer(f)
	found(f)
	f.on(clickRoute, reply(http.StatusOK, `null`))
	return openCase(t, iosVars(f, nil)), f
}

func TestAC4_EachIOSLocatorMapsToItsStrategy(t *testing.T) {
	cases := []struct {
		field, value string
		using, want  string
	}{
		{"accessibilityId", "login", "accessibility id", "login"},
		{"text", "Sign in", "-ios predicate string", `label == "Sign in"`},
		{"name", "Done", "name", "Done"},
		{"predicate", `type == "XCUIElementTypeButton"`, "-ios predicate string", `type == "XCUIElementTypeButton"`},
		{"classChain", "**/XCUIElementTypeCell[1]", "-ios class chain", "**/XCUIElementTypeCell[1]"},
		{"xpath", "//XCUIElementTypeButton", "xpath", "//XCUIElementTypeButton"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			ctx, f := iosCase(t)
			result := runWith(ctx, t, venom.TestStep{"action": "tap", tc.field: tc.value})
			assert.Empty(t, result.Err)
			assert.Equal(t, map[string]string{"using": tc.using, "value": tc.want}, lastQuery(t, f))
		})
	}
}

func TestAC4_IOSCSSKeepsItsWebViewCheck(t *testing.T) {
	ctx, f := iosCase(t)
	f.on(contextRoute, reply(http.StatusOK, `"WEBVIEW_1"`))
	result := runWith(ctx, t, venom.TestStep{"action": "tap", "css": "#go"})
	assert.Empty(t, result.Err)
	assert.Equal(t, map[string]string{"using": "css selector", "value": "#go"}, lastQuery(t, f))
}

func TestAC4_IOSTextCannotExtendThePredicate(t *testing.T) {
	cases := map[string]string{
		`x" OR 1==1 OR label == "y`: `label == "x\" OR 1==1 OR label == \"y"`,
		`back\slash`:                `label == "back\\slash"`,
		"two\nlines\r":              `label == "two\nlines\r"`,
		`ends with \`:               `label == "ends with \\"`,
	}
	for text, want := range cases {
		_, value := locator{locText, text}.strategyOn(true)
		assert.Equal(t, want, value)
	}
}

func TestAC4_IOSRefusesAndroidOnlyLocatorsBeforeAnyRequest(t *testing.T) {
	ctx, f := iosCase(t)
	before := f.total()

	err := usageOf(ctx, t, venom.TestStep{"action": "tap", "id": "com.example:id/ok"})
	assert.Contains(t, err.Error(), "id is the Android resource id: on iOS use accessibilityId or name")
	err = usageOf(ctx, t, venom.TestStep{"action": "tap", "uiautomator": "new UiSelector()"})
	assert.Contains(t, err.Error(), "uiautomator is Android only")
	assert.Equal(t, before, f.total())
}

func TestAC4_AndroidRefusesIOSOnlyLocatorsBeforeAnyRequest(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	before := f.total()

	for _, field := range []string{"predicate", "classChain", "name"} {
		err := usageOf(ctx, t, venom.TestStep{"action": "tap", field: "x"})
		assert.Contains(t, err.Error(), field+" is iOS only")
	}
	assert.Equal(t, before, f.total())
}

func TestAC4_IOSKeepsTheOneLocatorRule(t *testing.T) {
	ctx, f := iosCase(t)
	before := f.total()

	err := usageOf(ctx, t, venom.TestStep{"action": "tap", "name": "a", "predicate": "b"})
	assert.Contains(t, err.Error(), "one locator per step: got name and predicate")
	assert.Contains(t, err.Error(), "give exactly one of accessibilityId, text, name, predicate, classChain, xpath, css")
	assert.Equal(t, before, f.total())
}

func TestAC4_TheNoLocatorMessageListsThePlatformsKinds(t *testing.T) {
	ctx, _ := iosCase(t)
	err := usageOf(ctx, t, venom.TestStep{"action": "tap"})
	assert.Contains(t, err.Error(), "no locator: give exactly one of accessibilityId, text, name, predicate, classChain, xpath, css")
	assert.NotContains(t, err.Error(), "uiautomator")

	_, err = Executor{}.locator()
	assert.Contains(t, err.Error(), "no locator: give exactly one of accessibilityId, text, id, xpath, uiautomator, css")
}
