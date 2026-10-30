package mobile

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ovh/venom"
)

const deepLink = "myapp://item/42?x=1"

func TestAC4_OpenURLSendsTheDeepLinkWithTheSessionsPackage(t *testing.T) {
	ctx, f := caseOn(t, map[string]interface{}{"appId": "org.example.app"})
	f.on(execRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "openURL", "url": deepLink})
	assert.Empty(t, result.Err)
	body := f.first(t, http.MethodPost, "/session/{id}/execute/sync")
	assert.Equal(t, "mobile: deepLink", body["script"])
	assert.Equal(t, []interface{}{map[string]interface{}{"url": deepLink, "package": "org.example.app"}}, body["args"])
}

func TestAC4_OpenURLStepAppIdOverridesTheSessions(t *testing.T) {
	ctx, f := caseOn(t, map[string]interface{}{"appId": "org.example.app"})
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "openURL", "url": deepLink, "appId": "org.example.other"}).Err)
	body := f.first(t, http.MethodPost, "/session/{id}/execute/sync")
	assert.Equal(t, []interface{}{map[string]interface{}{"url": deepLink, "package": "org.example.other"}}, body["args"])
}

func TestAC4_OpenURLWithNoPackageKnownOmitsIt(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "openURL", "url": "https://example.org/a"}).Err)
	body := f.first(t, http.MethodPost, "/session/{id}/execute/sync")
	assert.Equal(t, []interface{}{map[string]interface{}{"url": "https://example.org/a"}}, body["args"])
}

func TestAC4_OpenURLRefusesABadLinkBeforeAnyRequest(t *testing.T) {
	ctx, f := caseOn(t, nil)
	before := f.total()
	bad := map[string]string{
		"empty":         "",
		"no scheme":     "notaurl",
		"one slash":     "http:/x",
		"no rest":       "scheme://",
		"newline":       "myapp://a\nb",
		"space":         "myapp://a b",
		"no slashes":    "javascript:alert(1)",
		"digit first":   "1app://x",
		"too long":      "myapp://" + strings.Repeat("a", maxLinkBytes-len("myapp://")+1),
		"control chars": "myapp://a\x7fb",
	}
	for name, link := range bad {
		err := usageOf(ctx, t, venom.TestStep{"action": "openURL", "url": link})
		assert.NotContains(t, err.Error(), "alert(1)", name) // the value stays out of the message
	}
	assert.Equal(t, before, f.total())
}

func TestAC4_OpenURLAcceptsALinkAtTheLimit(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	link := "myapp://" + strings.Repeat("a", maxLinkBytes-len("myapp://"))
	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "openURL", "url": link}).Err)
}

func TestAC4_OpenURLServerFailureLandsInTheResult(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, fail(http.StatusInternalServerError, "unknown error", "no activity handles the link"))

	result := runWith(ctx, t, venom.TestStep{"action": "openURL", "url": deepLink})
	assert.Contains(t, result.Err, "no activity handles the link")
	assert.NotContains(t, result.Err, deepLink)
}
