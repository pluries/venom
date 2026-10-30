package mobile

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	screenshotRoute = "GET /session/{id}/screenshot"
	sourceRoute     = "GET /session/{id}/source"
	executeRoute    = "POST /session/{id}/execute/sync"
	diagPNG         = "\x89PNG synthetic screen"
	diagXML         = "<hierarchy><node text='synthetic'/></hierarchy>"
	boomFailure     = "action boom: unknown error: device gone wrong"
)

// diagCase opens a case in a fresh working directory, with a failing action "boom" and a passing one "fine", and a
// fake that serves both diagnostics resources.
func diagCase(t *testing.T) (context.Context, *fakeServer) {
	t.Helper()
	t.Chdir(t.TempDir())
	shorten(t, &diagnosticsTimeout, 100*time.Millisecond)
	f := newFakeServer(t)
	f.on(screenshotRoute, reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString([]byte(diagPNG))+`"`))
	f.on(sourceRoute, reply(http.StatusOK, `"`+diagXML+`"`))
	f.on(executeRoute, fail(http.StatusInternalServerError, "unknown error", "device gone wrong"))
	withAction(t, "boom", callAction(http.MethodPost, "/execute/sync"))
	withAction(t, "fine", func(context.Context, *session, Executor, *Result) error { return nil })
	return openCase(t, f.baseVars()), f
}

func listFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestFR14_FailingStepWritesTheScreenshotAndTheSource(t *testing.T) {
	ctx, _ := diagCase(t)

	result := runStep(ctx, t, "boom")
	assert.Contains(t, result.Err, "device gone wrong")
	png, err := os.ReadFile(diagnosticScreenshot)
	require.NoError(t, err)
	assert.Equal(t, diagPNG, string(png))
	xml, err := os.ReadFile(diagnosticSource)
	require.NoError(t, err)
	assert.Equal(t, diagXML, string(xml))
	assert.Equal(t, []string{"screen.dump.xml", "screen.png"}, listFiles(t), "no temporary file is left")
}

func TestFR14_DiagnosticFilesAreOwnerOnlyEvenOverAnOlderFile(t *testing.T) {
	requireUnixModes(t)
	ctx, _ := diagCase(t)
	for _, name := range []string{diagnosticScreenshot, diagnosticSource} {
		require.NoError(t, os.WriteFile(name, []byte("old"), 0o644)) //nolint:gosec // G306: a looser file is the point, the step must narrow it
	}

	runStep(ctx, t, "boom")
	for _, name := range []string{diagnosticScreenshot, diagnosticSource} {
		info, err := os.Stat(name)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}
}

func TestFR14_PassingStepWritesNoneAndDeletesNone(t *testing.T) {
	ctx, f := diagCase(t)

	runStep(ctx, t, "boom")
	before := listFiles(t)
	result := runStep(ctx, t, "fine")
	assert.Empty(t, result.Err)
	assert.Equal(t, before, listFiles(t), "the files of the earlier failure stay")
	assert.Equal(t, 1, f.count(http.MethodGet, "/session/{id}/screenshot"), "only the failure asked for a screenshot")

	t.Chdir(t.TempDir())
	runStep(ctx, t, "fine")
	assert.Empty(t, listFiles(t), "a passing step on its own writes nothing")
}

func TestFR14_NextFailingStepReplacesThePair(t *testing.T) {
	ctx, f := diagCase(t)

	runStep(ctx, t, "boom")
	f.on(screenshotRoute, reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString([]byte("second"))+`"`))
	f.on(sourceRoute, reply(http.StatusOK, `"<second/>"`))
	runStep(ctx, t, "boom")

	png, err := os.ReadFile(diagnosticScreenshot)
	require.NoError(t, err)
	assert.Equal(t, "second", string(png))
	xml, err := os.ReadFile(diagnosticSource)
	require.NoError(t, err)
	assert.Equal(t, "<second/>", string(xml))
	assert.Len(t, listFiles(t), 2)
}

func TestFR15_ScreenshotErrorKeepsTheStepFailureAndTheSource(t *testing.T) {
	ctx, f := diagCase(t)
	f.on(screenshotRoute, fail(http.StatusInternalServerError, "unknown error", "no screen"))
	logs := captureLogs(t)

	result := runStep(ctx, t, "boom")
	assert.Equal(t, boomFailure, result.Err)
	assert.Equal(t, []string{"screen.dump.xml"}, listFiles(t))
	assert.Contains(t, logs.text(), "screen.png not saved")
}

func TestFR15_SourceErrorKeepsTheStepFailureAndTheScreenshot(t *testing.T) {
	ctx, f := diagCase(t)
	f.on(sourceRoute, fail(http.StatusInternalServerError, "unknown error", "no source"))

	result := runStep(ctx, t, "boom")
	assert.Equal(t, boomFailure, result.Err)
	assert.Equal(t, []string{"screen.png"}, listFiles(t))
}

func TestFR15_BothDiagnosticsFailingLeaveNoFileAndTheFailureIntact(t *testing.T) {
	ctx, f := diagCase(t)
	f.on(screenshotRoute, reply(http.StatusOK, `"!!not base64!!"`))
	f.on(sourceRoute, reply(http.StatusOK, `""`))

	result := runStep(ctx, t, "boom")
	assert.Equal(t, boomFailure, result.Err)
	assert.Empty(t, listFiles(t))
}

func TestFR16_DiagnosticsAreBoundedWhenTheServerHangs(t *testing.T) {
	ctx, f := diagCase(t)
	f.on(screenshotRoute, f.hang())
	f.on(sourceRoute, f.hang())

	within(t, 5*time.Second, func() {
		result := runStep(ctx, t, "boom")
		assert.Contains(t, result.Err, "device gone wrong")
	})
	assert.Empty(t, listFiles(t))
}

func TestFR16_DiagnosticsSurviveAStepWhoseContextIsGone(t *testing.T) {
	ctx, _ := diagCase(t)
	withAction(t, "slow", func(ctx context.Context, _ *session, _ Executor, _ *Result) error {
		<-ctx.Done()
		return ctx.Err()
	})
	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()

	out, err := Executor{}.Run(stepCtx, venom.TestStep{"action": "slow"})
	require.NoError(t, err)
	result, ok := out.(Result)
	require.True(t, ok)
	assert.NotEmpty(t, result.Err)
	assert.Equal(t, []string{"screen.dump.xml", "screen.png"}, listFiles(t))
}

func TestFR15_UsageErrorWritesNoDiagnosticsAndSendsNoRequest(t *testing.T) {
	ctx, f := diagCase(t)
	withAction(t, "bad", func(context.Context, *session, Executor, *Result) error { return usagef("two locators") })
	before := f.total()

	_, err := Executor{}.Run(ctx, venom.TestStep{"action": "bad"})
	require.Error(t, err)
	assert.Equal(t, before, f.total())
	assert.Empty(t, listFiles(t))
}

func TestFR15_DeadSessionWritesNoDiagnostics(t *testing.T) {
	ctx, f := diagCase(t)
	f.on(executeRoute, fail(http.StatusNotFound, "invalid session id", "gone"))

	runStep(ctx, t, "boom")
	assert.Equal(t, 0, f.count(http.MethodGet, "/session/{id}/screenshot"))
	assert.Empty(t, listFiles(t))

	require.NoError(t, Executor{}.TearDown(ctx))
	out, err := Executor{}.Run(ctx, venom.TestStep{"action": "boom"})
	require.Error(t, err, "a closed session is refused")
	assert.Nil(t, out)
	assert.Empty(t, listFiles(t))
}

func TestFR19_AC11_EveryResultCarriesTheRunFacts(t *testing.T) {
	t.Chdir(t.TempDir())
	f := newFakeServer(t)
	found(f)
	withAction(t, "boom", callAction(http.MethodPost, "/execute/sync"))
	withAction(t, "fine", func(context.Context, *session, Executor, *Result) error { return nil })
	appPath, appSum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"appId": "org.example.app", "app": appPath}))
	t.Cleanup(func() { assert.NoError(t, Executor{}.TearDown(ctx)) })
	names := []string{"boom", "fine"}
	for name := range actions {
		if name != "alert" { // iOS only: on this Android session it is a usage error, tested in actions_alert_test.go
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			result := runWith(ctx, t, venom.TestStep{"action": name, "id": "x", "appId": "org.example.app", "direction": "up", "value": "v", "url": "myapp://x"})
			assert.Equal(t, "android", result.Platform)
			assert.Equal(t, testDevice, result.Device)
			assert.Equal(t, "14", result.OSVersion)
			assert.Equal(t, "org.example.app", result.AppID)
			assert.Equal(t, appPath, result.App)
			assert.Equal(t, appSum, result.AppSHA256)
		})
	}
}
