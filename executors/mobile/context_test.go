package mobile

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ovh/venom"
)

const contextsRoute = "GET /session/{id}/contexts"

// contextSwitchRoute is the POST that switches the context; its path is the one of contextPath.
const contextSwitchRoute = "POST /session/{id}/context"

// contextsAre makes the fake list these contexts, a JSON list text.
func contextsAre(f *fakeServer, list string) {
	f.on(contextsRoute, reply(http.StatusOK, list))
	f.on(contextSwitchRoute, reply(http.StatusOK, `null`))
}

// statefulContext makes the fake keep the active context: POST sets it, GET answers it.
func statefulContext(f *fakeServer, list string) {
	var mu sync.Mutex
	current := "NATIVE_APP"
	f.on(contextsRoute, reply(http.StatusOK, list))
	f.on(contextSwitchRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mu.Lock()
		current = bodyName(body)
		mu.Unlock()
		reply(http.StatusOK, `null`)(w, r, body)
	})
	f.on(contextRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mu.Lock()
		now := current
		mu.Unlock()
		reply(http.StatusOK, `"`+now+`"`)(w, r, body)
	})
}

// bodyName reads the name of a switch body.
func bodyName(body []byte) string {
	var in struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return ""
	}
	return in.Name
}

func TestFR10_ContextSwitchesToANamedWebView(t *testing.T) {
	ctx, f := caseOn(t, nil)
	contextsAre(f, `["NATIVE_APP","WEBVIEW_x","WEBVIEW_y"]`)

	result := runWith(ctx, t, venom.TestStep{"action": "context", "context": "WEBVIEW_y"})
	assert.Empty(t, result.Err)
	assert.Equal(t, "WEBVIEW_y", result.Context)
	assert.Equal(t, []string{"NATIVE_APP", "WEBVIEW_x", "WEBVIEW_y"}, result.Contexts)
	assert.Equal(t, map[string]interface{}{"name": "WEBVIEW_y"}, f.first(t, http.MethodPost, contextPath))
}

func TestFR10_ContextWithoutANameTakesTheFirstWebView(t *testing.T) {
	ctx, f := caseOn(t, nil)
	contextsAre(f, `["NATIVE_APP","WEBVIEW_a","WEBVIEW_b"]`)

	result := runWith(ctx, t, venom.TestStep{"action": "context"})
	assert.Empty(t, result.Err)
	assert.Equal(t, "WEBVIEW_a", result.Context)
	assert.Equal(t, map[string]interface{}{"name": "WEBVIEW_a"}, f.first(t, http.MethodPost, contextPath))
}

func TestFR10_ContextNativeInAnyCaseSwitchesWithoutListing(t *testing.T) {
	ctx, f := caseOn(t, nil)
	contextsAre(f, `["NATIVE_APP"]`)

	for _, name := range []string{"native", "NATIVE", "NATIVE_APP", "Native_App"} {
		result := runWith(ctx, t, venom.TestStep{"action": "context", "context": name})
		assert.Empty(t, result.Err, name)
		assert.Equal(t, "NATIVE_APP", result.Context, name)
		assert.Empty(t, result.Contexts, name)
	}
	assert.Zero(t, f.count(http.MethodGet, "/session/{id}/contexts"))
	assert.Equal(t, map[string]interface{}{"name": "NATIVE_APP"}, f.first(t, http.MethodPost, contextPath))
}

func TestFR10_AWebViewThatAppearsLateIsWaitedFor(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(contextSwitchRoute, reply(http.StatusOK, `null`))
	var mu sync.Mutex
	asked := 0
	f.on(contextsRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		mu.Lock()
		asked++
		list := `["NATIVE_APP"]`
		if asked >= 2 {
			list = `["NATIVE_APP","WEBVIEW_late"]`
		}
		mu.Unlock()
		reply(http.StatusOK, list)(w, r, body)
	})

	result := runWith(ctx, t, venom.TestStep{"action": "context", "syncTimeout": 5})
	assert.Empty(t, result.Err)
	assert.Equal(t, "WEBVIEW_late", result.Context)
	assert.Equal(t, 2, f.count(http.MethodGet, "/session/{id}/contexts"))
}

func TestFR10_NoWebViewInTimeNamesTheContextsSeen(t *testing.T) {
	ctx, f := caseOn(t, nil)
	contextsAre(f, `["NATIVE_APP"]`)

	result := runWith(ctx, t, venom.TestStep{"action": "context"})
	assert.Contains(t, result.Err, "no web view within 0.05s")
	assert.Contains(t, result.Err, "contexts seen: [NATIVE_APP]")
	assert.Zero(t, f.count(http.MethodPost, contextPath))
	assert.Empty(t, result.Context)
}

func TestFR10_ANamedWebViewThatIsNotListedNamesTheContextsSeen(t *testing.T) {
	ctx, f := caseOn(t, nil)
	contextsAre(f, `["NATIVE_APP","WEBVIEW_x"]`)

	result := runWith(ctx, t, venom.TestStep{"action": "context", "context": "WEBVIEW_gone"})
	assert.Contains(t, result.Err, `"WEBVIEW_gone"`)
	assert.Contains(t, result.Err, "contexts seen: [NATIVE_APP WEBVIEW_x]")
	assert.Zero(t, f.count(http.MethodPost, contextPath))
}

func TestFR10_AContextNameThatIsNeitherNativeNorAWebViewIsAUsageError(t *testing.T) {
	ctx, f := caseOn(t, nil)
	before := f.total()

	err := usageOf(ctx, t, venom.TestStep{"action": "context", "context": "CHROMIUM"})
	assert.Contains(t, err.Error(), `"CHROMIUM"`)
	assert.Contains(t, err.Error(), "WEBVIEW")
	assert.Equal(t, before, f.total())
}

func TestFR10_AFailingContextListIsAServerFailure(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(contextsRoute, fail(http.StatusInternalServerError, "unknown error", "no contexts"))

	result := runWith(ctx, t, venom.TestStep{"action": "context"})
	assert.Contains(t, result.Err, "action context")
}

// TestFR10_HybridAppSequence is the acceptance example: native, into the web view, css there, back to native.
func TestFR10_HybridAppSequence(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clickRoute, reply(http.StatusOK, `null`))
	statefulContext(f, `["NATIVE_APP","WEBVIEW_x"]`)

	native := venom.TestStep{"action": "tap", "accessibilityId": "open"}
	assert.Empty(t, runWith(ctx, t, native).Err)

	err := usageOf(ctx, t, venom.TestStep{"action": "tap", "css": "#go"})
	assert.Contains(t, err.Error(), "context action")

	assert.Equal(t, "WEBVIEW_x", runWith(ctx, t, venom.TestStep{"action": "context"}).Context)
	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "tap", "css": "#go"}).Err)
	assert.Equal(t, map[string]string{"using": "css selector", "value": "#go"}, lastQuery(t, f))

	assert.Equal(t, "NATIVE_APP", runWith(ctx, t, venom.TestStep{"action": "context", "context": "native"}).Context)
	assert.Empty(t, runWith(ctx, t, native).Err)
	assert.Equal(t, map[string]string{"using": "accessibility id", "value": "open"}, lastQuery(t, f))
}
