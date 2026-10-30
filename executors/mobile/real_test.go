//go:build mobile

package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Real-device test: what the scripted fake server cannot prove, that the executor speaks the Appium 3 and
// UiAutomator2 protocol (docs/feat/2396/validation.md V14). It needs a machine with Appium 3 (UiAutomator2 driver)
// listening on loopback and an Android emulator, and it reads everything from the environment:
//
//	VENOM_MOBILE_URL             the Appium server, e.g. http://127.0.0.1:4723
//	VENOM_MOBILE_DEVICE          the emulator or device name, e.g. emulator-5554
//	VENOM_MOBILE_APP             path of an .apk to install; or
//	VENOM_MOBILE_APPID           the package id of an app already installed (one of the two is required)
//	VENOM_MOBILE_OSVERSION       optional, e.g. 14
//	VENOM_MOBILE_LOCATOR_KIND    accessibilityId, text, id, xpath or uiautomator
//	VENOM_MOBILE_LOCATOR_VALUE   the locator of an element on the first screen of the app
//	VENOM_MOBILE_EXPECT_TEXT     optional, the text that element shows
//
//	go test -tags mobile ./executors/mobile/ -run Real -v -count=1
//
// A variable that is not set skips the test with a message. Nothing here is a default value of the repository.
//
// Two more tests read the same session variables and need no locator of their own:
//
//	TestReal_CoordinatesAndDeepLink  nothing more
//	TestReal_WebViewContext          VENOM_MOBILE_CHROMEDRIVER (path of a matching chromedriver on the Appium host)
//	                                 and VENOM_MOBILE_WEBVIEW_LOCATOR (the accessibility id that opens a web view screen)
func realConfig(t *testing.T) (vars map[string]interface{}, step venom.TestStep, expectText string) {
	t.Helper()
	get := func(name string) string { return os.Getenv("VENOM_MOBILE_" + name) }
	for _, required := range []string{"LOCATOR_KIND", "LOCATOR_VALUE"} {
		if get(required) == "" {
			t.Skipf("VENOM_MOBILE_%s is not set: the real-device test needs Appium and an emulator", required)
		}
	}
	vars = realSessionVars(t)
	step = venom.TestStep{get("LOCATOR_KIND"): get("LOCATOR_VALUE")}
	return vars, step, get("EXPECT_TEXT")
}

// realSessionVars reads the variables every real test needs to open a session, and skips when one is unset.
func realSessionVars(t *testing.T) map[string]interface{} {
	t.Helper()
	get := func(name string) string { return os.Getenv("VENOM_MOBILE_" + name) }
	for _, required := range []string{"URL", "DEVICE"} {
		if get(required) == "" {
			t.Skipf("VENOM_MOBILE_%s is not set: the real-device test needs Appium and an emulator", required)
		}
	}
	if get("APP") == "" && get("APPID") == "" {
		t.Skip("neither VENOM_MOBILE_APP nor VENOM_MOBILE_APPID is set")
	}
	vars := map[string]interface{}{"url": get("URL"), "platform": "android", "device": get("DEVICE")}
	if app := get("APP"); app != "" {
		abs, err := filepath.Abs(app) // absolute before the test changes directory
		require.NoError(t, err)
		vars["app"] = abs
	}
	if id := get("APPID"); id != "" {
		vars["appId"] = id
	}
	if v := get("OSVERSION"); v != "" {
		vars["osVersion"] = v
	}
	return vars
}

// stepWith returns a copy of the locator step with the action added.
func stepWith(base venom.TestStep, action string, extra venom.TestStep) venom.TestStep {
	out := venom.TestStep{"action": action}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func TestReal_SessionFindReadFailAndClose(t *testing.T) {
	vars, locatorStep, expect := realConfig(t)
	t.Chdir(t.TempDir()) // the diagnostics land in the working directory

	ctx, err := Executor{}.Setup(testCtx(vars), nil)
	require.NoError(t, err, "the session must open on the real server")
	closed := false
	defer func() {
		if !closed {
			assert.NoError(t, Executor{}.TearDown(ctx))
		}
	}()

	run := func(step venom.TestStep) Result {
		out, runErr := Executor{}.Run(ctx, step)
		require.NoError(t, runErr)
		result, ok := out.(Result)
		require.True(t, ok, "Run returned %T", out)
		return result
	}

	// the run facts come from the server's answer
	found := run(stepWith(locatorStep, "waitFor", nil))
	require.Empty(t, found.Err)
	assert.True(t, found.Found)
	assert.Equal(t, "android", found.Platform)
	assert.NotEmpty(t, found.Device)
	assert.NotEmpty(t, found.OSVersion, "the server reports the OS version")

	read := run(stepWith(locatorStep, "read", nil))
	require.Empty(t, read.Err)
	if expect != "" {
		assert.Equal(t, expect, read.Text)
	}

	// a passing step leaves no diagnostics
	for _, name := range []string{diagnosticScreenshot, diagnosticSource} {
		_, statErr := os.Stat(name)
		assert.True(t, os.IsNotExist(statErr), "%s exists after a passing step", name)
	}

	// a failing step reports in result.err and leaves the two files
	missing := run(venom.TestStep{"action": "tap", "accessibilityId": "venom-no-such-element", "syncTimeout": 1.0})
	assert.NotEmpty(t, missing.Err)
	for _, name := range []string{diagnosticScreenshot, diagnosticSource} {
		info, statErr := os.Stat(name)
		require.NoError(t, statErr, "%s is missing after a failing step", name)
		assert.Positive(t, info.Size())
	}

	closed = true
	require.NoError(t, Executor{}.TearDown(ctx))
}

// realRunner opens a session on the real server and returns a function that runs one step on it. The session is closed
// when the test ends.
func realRunner(t *testing.T, vars map[string]interface{}) func(venom.TestStep) Result {
	t.Helper()
	t.Chdir(t.TempDir()) // the diagnostics land in the working directory
	ctx, err := Executor{}.Setup(testCtx(vars), nil)
	require.NoError(t, err, "the session must open on the real server")
	t.Cleanup(func() { assert.NoError(t, Executor{}.TearDown(ctx)) })
	return func(step venom.TestStep) Result {
		out, runErr := Executor{}.Run(ctx, step)
		require.NoError(t, runErr)
		result, ok := out.(Result)
		require.True(t, ok, "Run returned %T", out)
		return result
	}
}

// TestReal_CoordinatesAndDeepLink proves the point gestures and the deep link reach the server (#2399 V8). It asserts
// nothing about the content of the screen: the gestures must succeed, and a link nothing handles must come back as a
// server answer in result.err, not as a misuse.
func TestReal_CoordinatesAndDeepLink(t *testing.T) {
	run := realRunner(t, realSessionVars(t))

	swipe := run(venom.TestStep{"action": "swipe", "x": 50.0, "y": 80.0, "toX": 50.0, "toY": 20.0, "unit": "percent"})
	assert.Empty(t, swipe.Err, "swipe by percent")
	swipe = run(venom.TestStep{"action": "swipe", "x": 300.0, "y": 900.0, "toX": 300.0, "toY": 400.0})
	assert.Empty(t, swipe.Err, "swipe by pixels")
	tap := run(venom.TestStep{"action": "tap", "x": 50.0, "y": 50.0, "unit": "percent"})
	assert.Empty(t, tap.Err, "tap at 50 %% / 50 %%")

	link := run(venom.TestStep{"action": "openURL", "url": "venomtest://nothing/handles/this"})
	assert.NotEmpty(t, link.Err, "the server answers that nothing handles the link")
}

// TestReal_IOSSession drives an iOS simulator (#2398 V8). It needs Appium 3 with the XCUITest driver, reached the same
// way as the Android tests, and VENOM_MOBILE_PLATFORM=ios. VENOM_MOBILE_DEVICE is the simulator name; the app is
// VENOM_MOBILE_APPID (a bundle id, for example the built-in Settings app) or VENOM_MOBILE_APP (a .app folder or .zip).
// VENOM_MOBILE_IOS_LOCATOR is the accessibility id of an element on the first screen (default "General").
func TestReal_IOSSession(t *testing.T) {
	if os.Getenv("VENOM_MOBILE_PLATFORM") != "ios" {
		t.Skip("VENOM_MOBILE_PLATFORM is not ios: the iOS simulator test is not requested")
	}
	vars := realSessionVars(t)
	vars["platform"] = "ios"
	locator := os.Getenv("VENOM_MOBILE_IOS_LOCATOR")
	if locator == "" {
		locator = "General"
	}
	run := realRunner(t, vars)

	found := run(venom.TestStep{"action": "waitFor", "accessibilityId": locator, "syncTimeout": 300.0})
	require.Empty(t, found.Err)
	assert.True(t, found.Found)
	assert.Equal(t, "ios", found.Platform)

	swipe := run(venom.TestStep{"action": "swipe", "direction": "up"})
	assert.Empty(t, swipe.Err, "swipe by direction")
	assert.Empty(t, run(venom.TestStep{"action": "terminate"}).Err, "terminate")
	assert.Empty(t, run(venom.TestStep{"action": "launch"}).Err, "launch")

	alert := run(venom.TestStep{"action": "alert"})
	assert.Contains(t, alert.Err, "no alert open")
}

// TestReal_WebViewContext switches into a web view, finds by css and switches back (#2399 V8). It needs a chromedriver
// that matches the device's WebView, on the Appium host, and the accessibility id of an element that opens a screen
// with a web view; without both it skips.
func TestReal_WebViewContext(t *testing.T) {
	driver, locator := os.Getenv("VENOM_MOBILE_CHROMEDRIVER"), os.Getenv("VENOM_MOBILE_WEBVIEW_LOCATOR")
	if driver == "" || locator == "" {
		t.Skip("VENOM_MOBILE_CHROMEDRIVER and VENOM_MOBILE_WEBVIEW_LOCATOR are not both set: the web view test needs a matching chromedriver on the Appium host")
	}
	vars := realSessionVars(t)
	vars["capabilities"] = map[string]interface{}{"appium:chromedriverExecutable": driver}
	run := realRunner(t, vars)

	tap := run(venom.TestStep{"action": "tap", "accessibilityId": locator, "syncTimeout": 30.0})
	require.Empty(t, tap.Err)

	web := run(venom.TestStep{"action": "context", "syncTimeout": 20.0})
	require.Empty(t, web.Err)
	assert.True(t, strings.HasPrefix(web.Context, "WEBVIEW"), "context is %q", web.Context)

	body := run(venom.TestStep{"action": "count", "css": "body"})
	require.Empty(t, body.Err)
	assert.Positive(t, body.Count)

	native := run(venom.TestStep{"action": "context", "context": "native"})
	require.Empty(t, native.Err)
	assert.Equal(t, "NATIVE_APP", native.Context)
}
