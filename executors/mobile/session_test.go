package mobile

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capsOf returns the alwaysMatch capabilities of the first session request.
func capsOf(t *testing.T, f *fakeServer) map[string]interface{} {
	t.Helper()
	body := f.first(t, http.MethodPost, sessionRoot)
	outer, ok := body["capabilities"].(map[string]interface{})
	require.True(t, ok)
	caps, ok := outer["alwaysMatch"].(map[string]interface{})
	require.True(t, ok)
	return caps
}

func TestFR05_SessionNamesTheDeviceAndTheDriver(t *testing.T) {
	f := newFakeServer(t)
	openCase(t, f.withVars(map[string]interface{}{"osVersion": "14"}))

	caps := capsOf(t, f)
	assert.Equal(t, "Android", caps["platformName"])
	assert.Equal(t, "UiAutomator2", caps["appium:automationName"])
	assert.Equal(t, testDevice, caps["appium:deviceName"])
	assert.Equal(t, testDevice, caps["appium:udid"])
	assert.Equal(t, "14", caps["appium:platformVersion"])
}

func TestAC12_IdleTimeoutIsSent(t *testing.T) {
	t.Run("default 120", func(t *testing.T) {
		f := newFakeServer(t)
		openCase(t, f.baseVars())
		assert.InDelta(t, 120, capsOf(t, f)["appium:newCommandTimeout"], 0)
	})
	t.Run("set by the suite", func(t *testing.T) {
		f := newFakeServer(t)
		openCase(t, f.withVars(map[string]interface{}{"idleTimeout": 30}))
		assert.InDelta(t, 30, capsOf(t, f)["appium:newCommandTimeout"], 0)
	})
}

func TestAC3_AppIDWithoutAnAppFile(t *testing.T) {
	f := newFakeServer(t)
	openCase(t, f.withVars(map[string]interface{}{"appId": "org.example.app"}))

	caps := capsOf(t, f)
	assert.Equal(t, "org.example.app", caps["appium:appPackage"])
	assert.NotContains(t, caps, "appium:app")
}

func TestFR06_UserCapabilitiesReachTheServer(t *testing.T) {
	f := newFakeServer(t)
	openCase(t, f.withVars(map[string]interface{}{
		"capabilities": map[interface{}]interface{}{"appium:noReset": true, "appium:extra": map[interface{}]interface{}{"k": "v"}},
	}))

	caps := capsOf(t, f)
	assert.Equal(t, true, caps["appium:noReset"])
	assert.Equal(t, map[string]interface{}{"k": "v"}, caps["appium:extra"])
	assert.Equal(t, testDevice, caps["appium:deviceName"], "the executor's own entries stay")
}

func TestAC7_ConflictingCapabilityIsRefusedBeforeAnyRequest(t *testing.T) {
	for _, name := range []string{
		"platformName", "appium:deviceName", "appium:udid", "appium:platformVersion",
		"appium:app", "appium:appPackage", "appium:automationName", "appium:newCommandTimeout",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeServer(t)
			_, err := Executor{}.Setup(testCtx(f.withVars(map[string]interface{}{"capabilities": map[string]interface{}{name: "x"}})), nil)
			require.ErrorContains(t, err, name)
			assert.Equal(t, 0, f.total())
		})
	}
	t.Run("inside appium:options", func(t *testing.T) {
		f := newFakeServer(t)
		caps := map[string]interface{}{"appium:options": map[string]interface{}{"udid": "x"}}
		_, err := Executor{}.Setup(testCtx(f.withVars(map[string]interface{}{"capabilities": caps})), nil)
		require.ErrorContains(t, err, "udid")
		assert.Equal(t, 0, f.total())
	})
}

func TestFR02_RefusedSettingsMakeNoRequest(t *testing.T) {
	cases := map[string]struct {
		vars map[string]interface{}
		want string
	}{
		"unknown platform": {map[string]interface{}{"platform": "windows"}, `mobile.platform must be android or ios, got "windows"`},
		"no platform":      {map[string]interface{}{"platform": ""}, "must be android or ios"},
		"no device":        {map[string]interface{}{"device": ""}, "never picks a device"},
		"no url":           {map[string]interface{}{"url": ""}, "mobile.url is not set"},
		"not http":         {map[string]interface{}{"url": "ftp://host"}, "http or https"},
		"credentials":      {map[string]interface{}{"url": withUserinfo("http://127.0.0.1:4723")}, "credentials"},
		"negative idle":    {map[string]interface{}{"idleTimeout": -1}, "negative"},
		"negative session": {map[string]interface{}{"sessionTimeout": -1}, "negative"},
		"app file":         {map[string]interface{}{"app": "/tmp/none.apk"}, "mobile.app"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeServer(t)
			vars := f.withVars(tc.vars)
			if tc.vars["url"] != nil {
				vars["url"] = tc.vars["url"]
			}
			_, err := Executor{}.Setup(testCtx(vars), nil)
			require.ErrorContains(t, err, tc.want)
			assert.Equal(t, 0, f.total())
		})
	}
}

func TestFR05_RefusedSessionNamesTheDevice(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST "+sessionRoot, fail(http.StatusInternalServerError, "session not created", "device is busy"))

	_, err := Executor{}.Setup(testCtx(f.baseVars()), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testDevice)
	assert.Contains(t, err.Error(), "session not created: device is busy")
	assert.NotContains(t, err.Error(), "Trace.java")
	assert.Equal(t, 0, f.count(http.MethodDelete, sessionByID))
}

func TestFR05_AnswerWithoutSessionIDIsRefused(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST "+sessionRoot, reply(http.StatusOK, `{"capabilities":{}}`))

	_, err := Executor{}.Setup(testCtx(f.baseVars()), nil)
	require.ErrorContains(t, err, "without a session id")
}

func TestFR15_UnreachableServerFailsOnlyTheCase(t *testing.T) {
	// .invalid never resolves (RFC 6761): no port is probed and nothing is listening to be racy about
	vars := map[string]interface{}{"url": "http://automation.invalid:4723", "platform": "android", "device": testModel}

	_, err := Executor{}.Setup(testCtx(vars), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testModel)
	assert.Contains(t, err.Error(), "unreachable")
	assert.NotContains(t, err.Error(), "http://", "the request URL stays out of the message")
}

func TestFR16_SessionTimeoutSettingIsRead(t *testing.T) {
	f := newFakeServer(t)
	cfg, err := readConfig(testCtx(f.withVars(map[string]interface{}{"sessionTimeout": 7, "syncTimeout": 3})))
	require.NoError(t, err)
	assert.Equal(t, "7s", cfg.sessionTimeout.String())
	assert.Equal(t, "3s", cfg.syncTimeout.String())

	cfg, err = readConfig(testCtx(f.baseVars()))
	require.NoError(t, err)
	assert.Equal(t, sessionOpenTimeout, cfg.sessionTimeout)
	assert.Equal(t, "10s", cfg.syncTimeout.String())
}

// withUserinfo adds a synthetic user and password to an address.
func withUserinfo(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		panic(err)
	}
	u.User = url.UserPassword("tester", "synthetic")
	return u.String()
}
