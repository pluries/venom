package mobile

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSimulator = "iPhone Test"
	testUDID      = "0A1B2C3D-4E5F-6789-ABCD-0123456789ef"
)

// iosAnswer is what an iOS server answers to POST /session.
func iosAnswer(f *fakeServer) {
	f.on("POST "+sessionRoot, reply(http.StatusOK, `{"sessionId":"`+testSessionID+`","capabilities":{"platformName":"iOS","appium:platformVersion":"17.5"}}`))
}

func iosVars(f *fakeServer, extra map[string]interface{}) map[string]interface{} {
	vars := f.withVars(extra)
	vars["platform"] = "ios"
	if _, set := extra["device"]; !set {
		vars["device"] = testSimulator
	}
	if _, set := extra["appId"]; !set {
		vars["appId"] = "org.example.app" // without mobile.app, iOS launches by bundle id, which is then required
	}
	return vars
}

func TestFR05_CapabilitiesPerPlatform(t *testing.T) {
	cases := map[string]struct {
		extra map[string]interface{}
		want  map[string]interface{}
		never []string
	}{
		"ios with a name": {
			map[string]interface{}{"osVersion": "17.5", "appId": "org.example.app"},
			map[string]interface{}{
				"platformName": "iOS", "appium:automationName": "XCUITest", "appium:deviceName": testSimulator,
				"appium:platformVersion": "17.5", "appium:bundleId": "org.example.app",
			},
			[]string{"appium:udid", "appium:appPackage", "appium:appActivity"},
		},
		"ios with a UUID": {
			map[string]interface{}{"device": testUDID, "osVersion": "17.5"},
			map[string]interface{}{"appium:deviceName": testUDID, "appium:udid": testUDID, "appium:platformVersion": "17.5"},
			[]string{"appium:appPackage"},
		},
		"ios without an os version": {
			map[string]interface{}{},
			map[string]interface{}{"appium:deviceName": testSimulator},
			[]string{"appium:platformVersion", "appium:udid"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeServer(t)
			iosAnswer(f)
			openCase(t, iosVars(f, tc.extra))

			caps := capsOf(t, f)
			for k, v := range tc.want {
				assert.Equal(t, v, caps[k], k)
			}
			for _, k := range tc.never {
				assert.NotContains(t, caps, k)
			}
			assert.InDelta(t, 120, caps["appium:newCommandTimeout"], 0)
		})
	}
	t.Run("android is unchanged", func(t *testing.T) {
		f := newFakeServer(t)
		openCase(t, f.withVars(map[string]interface{}{"osVersion": "14", "appId": "org.example.app"}))

		caps := capsOf(t, f)
		assert.Equal(t, "Android", caps["platformName"])
		assert.Equal(t, "UiAutomator2", caps["appium:automationName"])
		assert.Equal(t, testDevice, caps["appium:udid"])
		assert.Equal(t, "org.example.app", caps["appium:appPackage"])
		assert.NotContains(t, caps, "appium:bundleId")
	})
}

func TestFR05_UUIDPatternMatchesOnlyUUIDs(t *testing.T) {
	for _, ok := range []string{testUDID, "0a1b2c3d-4e5f-6789-abcd-0123456789EF"} {
		assert.True(t, uuidPattern.MatchString(ok), ok)
	}
	for _, no := range []string{testSimulator, "emulator-5554", testUDID + "0", "0A1B2C3D4E5F6789ABCD0123456789EF", ""} {
		assert.False(t, uuidPattern.MatchString(no), no)
	}
}

func TestFR06_PlatformOwnedCapabilitiesAreRefused(t *testing.T) {
	names := []string{"bundleId", "appium:bundleId", "appPackage", "automationName"}
	for _, alert := range []string{"autoAcceptAlerts", "autoDismissAlerts"} {
		names = append(names, alert, "appium:"+alert)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			err := checkCapabilities(map[string]interface{}{name: true})
			require.ErrorContains(t, err, name)
		})
	}
	for _, alert := range []string{"autoAcceptAlerts", "autoDismissAlerts"} {
		t.Run("nested "+alert, func(t *testing.T) {
			err := checkCapabilities(map[string]interface{}{"appium:options": map[string]interface{}{alert: true}})
			require.ErrorContains(t, err, "the alert action")
		})
	}
	err := checkCapabilities(map[string]interface{}{"automationName": "x"})
	require.ErrorContains(t, err, "the executor")
	assert.NotContains(t, err.Error(), "UiAutomator2")
}

func TestFR02_IOSIsAccepted(t *testing.T) {
	f := newFakeServer(t)
	iosAnswer(f)
	ctx := openCase(t, iosVars(f, nil))

	s := getSession(ctx)
	require.NotNil(t, s)
	assert.True(t, s.ios())
	assert.Equal(t, "17.5", s.facts.osVersion, "the version the server reports wins")
	assert.Equal(t, platformIOS, s.facts.platform)
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestFR02_AndroidSessionIsNotIOS(t *testing.T) {
	s := getSession(openCase(t, newFakeServer(t).baseVars()))
	require.NotNil(t, s)
	assert.False(t, s.ios())
}

func TestFR02_SessionAnswerMustMatchThePlatform(t *testing.T) {
	t.Run("android server on an ios case", func(t *testing.T) {
		f := newFakeServer(t) // answers Android
		_, err := Executor{}.Setup(testCtx(iosVars(f, nil)), nil)
		require.ErrorContains(t, err, "opened a Android session, expected ios")
		assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID))
	})
	t.Run("case does not matter", func(t *testing.T) {
		f := newFakeServer(t)
		f.on("POST "+sessionRoot, reply(http.StatusOK, `{"sessionId":"s1","capabilities":{"platformName":"IOS"}}`))
		_, err := Executor{}.Setup(testCtx(iosVars(f, nil)), nil)
		require.NoError(t, err)
	})
}

func TestFR16_IOSSessionDefaultsToFiveMinutes(t *testing.T) {
	f := newFakeServer(t)
	cfg, err := readConfig(testCtx(iosVars(f, nil)))
	require.NoError(t, err)
	assert.Equal(t, "5m0s", cfg.sessionTimeout.String())
	assert.Equal(t, iosSessionOpenTimeout, cfg.sessionTimeout)

	cfg, err = readConfig(testCtx(iosVars(f, map[string]interface{}{"sessionTimeout": 7})))
	require.NoError(t, err)
	assert.Equal(t, "7s", cfg.sessionTimeout.String(), "an explicit value still wins")

	cfg, err = readConfig(testCtx(f.baseVars()))
	require.NoError(t, err)
	assert.Equal(t, "3m0s", cfg.sessionTimeout.String(), "Android keeps its default")
}

func TestFR02_ProfilesNameBothPlatforms(t *testing.T) {
	assert.Equal(t, platformProfile{"Android", "UiAutomator2", "appPackage"}, profileOf(platformAndroid))
	assert.Equal(t, platformProfile{"iOS", "XCUITest", "bundleId"}, profileOf(platformIOS))
	assert.Equal(t, profileOf(platformAndroid), profileOf("other"))
}
