package mobile

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// requestLine is one request as the golden list writes it: method, path and the JSON body.
func (f *fakeServer) requestLines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	lines := make([]string, 0, len(f.reqs))
	for _, r := range f.reqs {
		lines = append(lines, r.Method+" "+r.Path+" "+r.Body)
	}
	return lines
}

// goldenSteps is the Android scenario of AC-7: every step is one the suites of M1 and M2 use.
var goldenSteps = []venom.TestStep{
	{"action": "tap", "accessibilityId": "login"},
	{"action": "tap", "text": "Sign in"},
	{"action": "fill", "id": "name", "value": "Ada", "clear": true},
	{"action": "swipe", "direction": "left"},
	{"action": "back"},
	{"action": "terminate"},
}

func goldenCase(t *testing.T) *fakeServer {
	t.Helper()
	f := newFakeServer(t)
	found(f)
	f.on(clickRoute, reply(http.StatusOK, `null`))
	f.on(clearRoute, reply(http.StatusOK, `null`))
	f.on(valueRoute, reply(http.StatusOK, `null`))
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":1080,"height":2400}`))
	f.on(execRoute, reply(http.StatusOK, `null`))
	f.on("POST /session/{id}/back", reply(http.StatusOK, `null`))
	return f
}

// TestAC7_AndroidRequestsAreUnchanged pins every request an Android run sends, byte for byte, against the list the
// code produced before iOS support was added.
func TestAC7_AndroidRequestsAreUnchanged(t *testing.T) {
	f := goldenCase(t)
	path, _ := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path, "appId": "org.example.app", "osVersion": "14"}))
	for _, step := range goldenSteps {
		assert.Empty(t, runWith(ctx, t, step).Err)
	}
	require.NoError(t, Executor{}.TearDown(ctx))

	quoted, err := json.Marshal(path)
	require.NoError(t, err)
	got := strings.Join(f.requestLines(), "\n")
	got = strings.ReplaceAll(got, strings.Trim(string(quoted), `"`), "{app}")
	assert.Equal(t, strings.Join(goldenAndroid, "\n"), got)
}

// goldenAndroid was recorded from the code as it stood before iOS support; {app} stands for the temporary app path.
var goldenAndroid = []string{
	`POST /session {"capabilities":{"alwaysMatch":{"appium:app":"{app}","appium:appPackage":"org.example.app","appium:automationName":"UiAutomator2","appium:deviceName":"emulator-5554","appium:newCommandTimeout":120,"appium:platformVersion":"14","appium:udid":"emulator-5554","platformName":"Android"}}}`,
	`POST /session/{id}/elements {"using":"accessibility id","value":"login"}`,
	`POST /session/{id}/element/el-0/click {}`,
	`POST /session/{id}/elements {"using":"-android uiautomator","value":"new UiSelector().text(\"Sign in\")"}`,
	`POST /session/{id}/element/el-0/click {}`,
	`POST /session/{id}/elements {"using":"id","value":"name"}`,
	`POST /session/{id}/element/el-0/clear {}`,
	`POST /session/{id}/element/el-0/value {"text":"Ada","value":["A","d","a"]}`,
	`GET /session/{id}/window/rect `,
	`POST /session/{id}/execute/sync {"args":[{"direction":"left","height":1920,"left":108,"percent":0.75,"top":240,"width":864}],"script":"mobile: swipeGesture"}`,
	`POST /session/{id}/back {}`,
	`POST /session/{id}/execute/sync {"args":[{"appId":"org.example.app"}],"script":"mobile: terminateApp"}`,
	`DELETE /session/{id} `,
}
