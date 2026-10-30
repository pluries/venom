package mobile

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	contextRoute = "GET /session/{id}/context"
	contextPath  = "/session/{id}/context"
	elementsPath = "/session/{id}/elements"
)

func TestFR10_CSSMapsToTheCSSSelectorStrategy(t *testing.T) {
	using, value := locator{locCSS, "form > button.primary"}.strategy()
	assert.Equal(t, "css selector", using)
	assert.Equal(t, "form > button.primary", value)

	loc, err := Executor{CSS: "#go"}.locator()
	require.NoError(t, err)
	assert.Equal(t, locator{locCSS, "#go"}, loc)
}

func TestFR10_CSSInTheNativeContextIsRefusedWithoutSearching(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(contextRoute, reply(http.StatusOK, `"NATIVE_APP"`))

	err := usageOf(ctx, t, venom.TestStep{"action": "tap", "css": "#go"})
	assert.Contains(t, err.Error(), "context action")
	assert.Contains(t, err.Error(), "web view")
	assert.Equal(t, 1, f.count(http.MethodGet, contextPath))
	assert.Zero(t, f.count(http.MethodPost, elementsPath))
}

func TestFR10_CSSInAWebViewIsSearched(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(contextRoute, reply(http.StatusOK, `"WEBVIEW_com.example.app"`))
	f.on(clickRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "tap", "css": "#go"})
	assert.Empty(t, result.Err)
	assert.True(t, result.Found)
	assert.Equal(t, 1, f.count(http.MethodGet, contextPath))
	assert.Equal(t, map[string]string{"using": "css selector", "value": "#go"}, lastQuery(t, f))
}

func TestFR10_CSSWorksThroughReadToo(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(contextRoute, reply(http.StatusOK, `"WEBVIEW_x"`))
	f.on(textRoute, reply(http.StatusOK, `"Hello"`))

	result := runWith(ctx, t, venom.TestStep{"action": "read", "css": "h1"})
	assert.Empty(t, result.Err)
	assert.Equal(t, "Hello", result.Text)
}

func TestFR10_AFailingContextCallIsAServerFailure(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(contextRoute, fail(http.StatusNotFound, "invalid session id", "gone"))

	result := runWith(ctx, t, venom.TestStep{"action": "tap", "css": "#go"})
	assert.NotEmpty(t, result.Err)
	assert.Zero(t, f.count(http.MethodPost, elementsPath))
}

func TestFR10_OtherLocatorsMakeNoContextCall(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clickRoute, reply(http.StatusOK, `null`))

	for _, step := range []venom.TestStep{
		{"action": "tap", "accessibilityId": "a"},
		{"action": "tap", "text": "t"},
		{"action": "tap", "id": "i"},
		{"action": "tap", "xpath": "//x"},
		{"action": "tap", "uiautomator": "u"},
	} {
		assert.Empty(t, runWith(ctx, t, step).Err)
	}
	assert.Zero(t, f.count(http.MethodGet, contextPath))
}

func TestFR10_CSSWithAnotherLocatorIsTheOneLocatorError(t *testing.T) {
	ctx, f := caseOn(t, nil)
	before := f.total()

	err := usageOf(ctx, t, venom.TestStep{"action": "tap", "css": "#go", "id": "i"})
	assert.Contains(t, err.Error(), "one locator per step")
	assert.Contains(t, err.Error(), "id and css")
	assert.Equal(t, before, f.total())

	_, err = Executor{}.locator()
	assert.Contains(t, err.Error(), "uiautomator, css")
}
