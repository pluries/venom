package mobile

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	alertTextRoute    = "GET /session/{id}/alert/text"
	alertAcceptRoute  = "POST /session/{id}/alert/accept"
	alertDismissRoute = "POST /session/{id}/alert/dismiss"

	// alertSecret stands for text an alert could hold; it must only ever reach result.text.
	alertSecret = "Your code is 424242"
)

// iosAlert opens an iOS case with an alert open.
func iosAlert(t *testing.T) (*fakeServer, func(venom.TestStep) Result) {
	t.Helper()
	ctx, f := iosCase(t)
	f.on(alertTextRoute, reply(http.StatusOK, `"`+alertSecret+`"`))
	f.on(alertAcceptRoute, reply(http.StatusOK, `null`))
	f.on(alertDismissRoute, reply(http.StatusOK, `null`))
	f.on(execRoute, reply(http.StatusOK, `null`))
	return f, func(step venom.TestStep) Result { return runWith(ctx, t, step) }
}

func TestAC5_AlertAcceptsByDefaultAndReturnsItsText(t *testing.T) {
	f, run := iosAlert(t)
	result := run(venom.TestStep{"action": "alert"})
	assert.Empty(t, result.Err)
	assert.Equal(t, alertSecret, result.Text)
	assert.Equal(t, 1, f.count(http.MethodGet, "/session/{id}/alert/text"))
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/alert/accept"))
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/alert/dismiss"))
}

func TestAC5_AlertDismissesAndTheChoiceIgnoresCase(t *testing.T) {
	f, run := iosAlert(t)
	result := run(venom.TestStep{"action": "alert", "choice": "Dismiss"})
	assert.Empty(t, result.Err)
	assert.Equal(t, alertSecret, result.Text)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/alert/dismiss"))
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/alert/accept"))
}

func TestAC5_AlertWithAButtonUsesMobileAlert(t *testing.T) {
	f, run := iosAlert(t)
	result := run(venom.TestStep{"action": "alert", "choice": "dismiss", "button": "Don't Allow"})
	assert.Empty(t, result.Err)
	script, args := scriptSent(t, f)
	assert.Equal(t, "mobile: alert", script)
	assert.Equal(t, map[string]interface{}{"action": "dismiss", "buttonLabel": "Don't Allow"}, args)
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/alert/accept"))
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/alert/dismiss"))
}

func TestAC5_AlertWhenNoneIsOpenFailsWithoutAnyPost(t *testing.T) {
	f, run := iosAlert(t)
	f.on(alertTextRoute, fail(http.StatusNotFound, "no such alert", "An attempt was made to operate on a modal dialog"))
	result := run(venom.TestStep{"action": "alert"})
	assert.Equal(t, "action alert: no alert open", result.Err)
	assert.Empty(t, result.Text)
	for _, path := range []string{"alert/accept", "alert/dismiss", "execute/sync"} {
		assert.Zero(t, f.count(http.MethodPost, "/session/{id}/"+path), path)
	}
}

func TestAC5_AlertGoneBeforeTheAnswerIsNoAlertOpen(t *testing.T) {
	for name, step := range map[string]venom.TestStep{
		"accept":  {"action": "alert"},
		"dismiss": {"action": "alert", "choice": "dismiss"},
		"button":  {"action": "alert", "button": "OK"},
	} {
		f, run := iosAlert(t)
		gone := fail(http.StatusNotFound, "no such alert", "gone")
		f.on(alertAcceptRoute, gone)
		f.on(alertDismissRoute, gone)
		f.on(execRoute, gone)
		assert.Equal(t, "action alert: no alert open", run(step).Err, name)
	}
}

func TestAC5_AlertRefusesABadChoiceBeforeAnyRequest(t *testing.T) {
	ctx, f := iosCase(t)
	before := f.total()
	err := usageOf(ctx, t, venom.TestStep{"action": "alert", "choice": "maybe"})
	assert.Contains(t, err.Error(), "choice must be accept or dismiss")
	assert.Equal(t, before, f.total())
}

func TestAC5_AlertIsIOSOnly(t *testing.T) {
	ctx, f := caseOn(t, nil)
	before := f.total()
	err := usageOf(ctx, t, venom.TestStep{"action": "alert"})
	assert.Contains(t, err.Error(), "alert is iOS only")
	assert.Equal(t, before, f.total())
}

func TestAC5_AlertOtherServerErrorsLandInResultErr(t *testing.T) {
	f, run := iosAlert(t)
	f.on(alertAcceptRoute, fail(http.StatusInternalServerError, "unknown error", "device offline"))
	result := run(venom.TestStep{"action": "alert"})
	assert.Contains(t, result.Err, "device offline")
	assert.NotContains(t, result.Err, "no alert open")

	f, run = iosAlert(t)
	f.on(alertTextRoute, fail(http.StatusInternalServerError, "unknown error", "device offline"))
	assert.Contains(t, run(venom.TestStep{"action": "alert"}).Err, "device offline")
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/alert/accept"))
}

func TestAC5_AlertTextStaysInResultTextOnly(t *testing.T) {
	logs := captureLogs(t)
	_, run := iosAlert(t)
	result := run(venom.TestStep{"action": "alert", "button": "OK"})
	require.Empty(t, result.Err)
	assert.Equal(t, alertSecret, result.Text)
	assert.NotContains(t, logs.text(), alertSecret)

	f, run := iosAlert(t)
	f.on(execRoute, fail(http.StatusInternalServerError, "unknown error", "boom"))
	failed := run(venom.TestStep{"action": "alert", "button": "OK"})
	assert.NotContains(t, failed.Err, alertSecret)
	assert.NotContains(t, logs.text(), alertSecret)
}
