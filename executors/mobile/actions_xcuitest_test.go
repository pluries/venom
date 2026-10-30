package mobile

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// scriptSent returns the script and the argument object of the first execute/sync the fake received.
func scriptSent(t *testing.T, f *fakeServer) (string, map[string]interface{}) {
	t.Helper()
	script, ok := f.first(t, http.MethodPost, "/session/{id}/execute/sync")["script"].(string)
	require.True(t, ok)
	return script, gestureArgsSent(t, f)
}

func iosExec(t *testing.T) (*fakeServer, func(venom.TestStep) Result) {
	t.Helper()
	ctx, f := iosCase(t)
	f.on(execRoute, reply(http.StatusOK, `null`))
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":400,"height":800}`))
	return f, func(step venom.TestStep) Result { return runWith(ctx, t, step) }
}

func TestFR09_IOSLaunchAndTerminateNameTheBundleId(t *testing.T) {
	cases := map[string]string{"launch": "mobile: activateApp", "terminate": "mobile: terminateApp"}
	for action, want := range cases {
		f, run := iosExec(t)
		assert.Empty(t, run(venom.TestStep{"action": action}).Err)
		script, args := scriptSent(t, f)
		assert.Equal(t, want, script)
		assert.Equal(t, map[string]interface{}{"bundleId": "org.example.app"}, args)

		f, run = iosExec(t)
		assert.Empty(t, run(venom.TestStep{"action": action, "appId": "org.example.other"}).Err)
		_, args = scriptSent(t, f)
		assert.Equal(t, "org.example.other", args["bundleId"])
	}
}

func TestFR09_IOSBackIsRefusedBeforeAnyRequest(t *testing.T) {
	ctx, f := iosCase(t)
	before := f.total()
	err := usageOf(ctx, t, venom.TestStep{"action": "back"})
	assert.Contains(t, err.Error(), "back is Android only: iOS has no back key")
	assert.Equal(t, before, f.total())
}

func TestFR09_IOSHideKeyboardTreatsAHiddenKeyboardAsDone(t *testing.T) {
	f, run := iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "hideKeyboard"}).Err)
	script, args := scriptSent(t, f)
	assert.Equal(t, "mobile: hideKeyboard", script)
	assert.Empty(t, args)

	f, run = iosExec(t)
	f.on(execRoute, fail(http.StatusInternalServerError, "unknown error", "The keyboard is not present"))
	assert.Empty(t, run(venom.TestStep{"action": "hideKeyboard"}).Err)
}

func TestFR09_IOSSwipeAndScrollSendTheirDirection(t *testing.T) {
	for _, action := range []string{"swipe", "scroll"} {
		f, run := iosExec(t)
		assert.Empty(t, run(venom.TestStep{"action": action, "direction": "Up"}).Err)
		script, args := scriptSent(t, f)
		assert.Equal(t, "mobile: "+action, script)
		assert.Equal(t, map[string]interface{}{"direction": "up"}, args)
		assert.Zero(t, f.count(http.MethodGet, "/session/{id}/window/rect"))

		f, run = iosExec(t)
		found(f)
		assert.Empty(t, run(venom.TestStep{"action": action, "direction": "down", "accessibilityId": "list"}).Err)
		_, args = scriptSent(t, f)
		assert.Equal(t, "el-0", args["elementId"])
		assert.Equal(t, "down", args["direction"])
	}
}

func TestFR09_IOSRefusesPercentAndABadDirectionBeforeAnyRequest(t *testing.T) {
	ctx, f := iosCase(t)
	before := f.total()
	for _, step := range []venom.TestStep{
		{"action": "swipe", "direction": "up", "percent": 0.5},
		{"action": "scroll", "direction": "up", "percent": 0.5},
		{"action": "scroll", "direction": "up", "percent": 0.5, "until": true, "accessibilityId": "x"},
		{"action": "swipe", "direction": "sideways"},
	} {
		err := usageOf(ctx, t, step)
		assert.NotEmpty(t, err.Error())
	}
	assert.Equal(t, before, f.total())
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "swipe", "direction": "up", "percent": 1}).Error(), "percent is Android only")
}

func TestFR09_IOSScrollUntilRepeatsMobileScrollUntilVisible(t *testing.T) {
	shorten(t, &maxScrollAttempts, 3)
	f, run := iosExec(t)
	shown := 0
	f.on(shownRoute, func(w http.ResponseWriter, r *http.Request, b []byte) {
		shown++
		value := `false`
		if shown > 2 {
			value = `true`
		}
		reply(http.StatusOK, value)(w, r, b)
	})
	result := run(venom.TestStep{"action": "scroll", "direction": "down", "until": true, "accessibilityId": "row"})
	assert.Empty(t, result.Err)
	assert.True(t, result.Found)
	assert.Equal(t, 2, f.count(http.MethodPost, "/session/{id}/execute/sync"))
	_, args := scriptSent(t, f)
	assert.Equal(t, map[string]interface{}{"direction": "down"}, args)
}

func TestFR09_IOSTapAndSwipeByCoordinates(t *testing.T) {
	f, run := iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "tap", "x": 10, "y": 20}).Err)
	script, args := scriptSent(t, f)
	assert.Equal(t, "mobile: tap", script)
	assert.Equal(t, map[string]interface{}{"x": 10.0, "y": 20.0}, args)

	f, run = iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "tap", "x": 50, "y": 25, "unit": "percent"}).Err)
	_, args = scriptSent(t, f)
	assert.Equal(t, map[string]interface{}{"x": 200.0, "y": 200.0}, args)

	f, run = iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "swipe", "x": 10, "y": 20, "toX": 30, "toY": 40}).Err)
	script, args = scriptSent(t, f)
	assert.Equal(t, "mobile: dragFromToForDuration", script)
	assert.Equal(t, map[string]interface{}{"duration": 0.5, "fromX": 10.0, "fromY": 20.0, "toX": 30.0, "toY": 40.0}, args)

	f, run = iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "swipe", "x": 25, "y": 50, "toX": 75, "toY": 50, "unit": "percent"}).Err)
	_, args = scriptSent(t, f)
	assert.Equal(t, map[string]interface{}{"duration": 0.5, "fromX": 100.0, "fromY": 400.0, "toX": 300.0, "toY": 400.0}, args)
}

func TestFR09_IOSOpenURLNamesTheBundleIdWhenKnown(t *testing.T) {
	f, run := iosExec(t)
	assert.Empty(t, run(venom.TestStep{"action": "openURL", "url": "myapp://home"}).Err)
	script, args := scriptSent(t, f)
	assert.Equal(t, "mobile: deepLink", script)
	assert.Equal(t, map[string]interface{}{"url": "myapp://home", "bundleId": "org.example.app"}, args)

	ctx, f := iosCase(t)
	f.on(execRoute, reply(http.StatusOK, `null`))
	s := getSession(ctx)
	s.facts.appID = ""
	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "openURL", "url": "myapp://home"}).Err)
	_, args = scriptSent(t, f)
	assert.Equal(t, map[string]interface{}{"url": "myapp://home"}, args)
}
