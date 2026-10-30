package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const testApp = "synthetic app bytes"

// writeApp writes a synthetic app file and returns its path and SHA-256.
func writeApp(t *testing.T, dir, name string) (path, sum string) {
	t.Helper()
	path = filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(testApp), 0o600))
	h := sha256.Sum256([]byte(testApp))
	return path, hex.EncodeToString(h[:])
}

func TestFR04_MatchingHashOpensTheSessionWithTheAbsolutePath(t *testing.T) {
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path, "appSHA256": sum}))

	assert.Equal(t, path, capsOf(t, f)["appium:app"])
	s := getSession(ctx)
	require.NotNil(t, s)
	assert.Equal(t, sum, s.facts.appSHA256)
	assert.Equal(t, path, s.facts.app)
}

func TestFR04_UppercaseHashIsAccepted(t *testing.T) {
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path, "appSHA256": strings.ToUpper(sum)}))

	assert.Equal(t, sum, getSession(ctx).facts.appSHA256, "the report shows the computed lower case hash")
}

func TestFR04_NoExpectedHashStillReportsTheComputedOne(t *testing.T) {
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path}))

	assert.Equal(t, sum, getSession(ctx).facts.appSHA256)
}

func TestAC2_MismatchingHashMakesNoRequest(t *testing.T) {
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	wrong := strings.Repeat("0", len(sum))
	_, err := Executor{}.Setup(testCtx(f.withVars(map[string]interface{}{"app": path, "appSHA256": wrong})), nil)

	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Contains(t, err.Error(), sum, "the message shows the computed hash")
	assert.Contains(t, err.Error(), wrong)
	assert.Equal(t, 0, f.total())
}

func TestAC2_UnreadableAppMakesNoRequest(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"missing":   filepath.Join(dir, "none.apk"),
		"directory": dir,
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeServer(t)
			_, err := Executor{}.Setup(testCtx(f.withVars(map[string]interface{}{"app": path})), nil)
			var misuse *usageError
			require.ErrorAs(t, err, &misuse)
			assert.Contains(t, err.Error(), "mobile.app")
			assert.Equal(t, 0, f.total())
		})
	}
}

func TestFR04_RelativePathIsResolvedAgainstTheSuiteWorkdir(t *testing.T) {
	f := newFakeServer(t)
	dir := t.TempDir()
	path, _ := writeApp(t, dir, "app.apk")
	vars := f.withVars(map[string]interface{}{"app": "app.apk"})
	ctx, err := Executor{}.Setup(context.WithValue(testCtx(vars), venom.ContextKey("var.venom.testsuite.workdir"), dir), nil)
	require.NoError(t, err)
	require.NotNil(t, getSession(ctx))

	assert.Equal(t, path, capsOf(t, f)["appium:app"])
}

func TestFR04_ExpectedHashWithoutAnAppFileIsRefused(t *testing.T) {
	f := newFakeServer(t)
	_, err := Executor{}.Setup(testCtx(f.withVars(map[string]interface{}{"appSHA256": "ab"})), nil)
	require.ErrorContains(t, err, "mobile.appSHA256")
	assert.Equal(t, 0, f.total())
}

func TestAC3_NoAppAtAllNamesNoApp(t *testing.T) {
	f := newFakeServer(t)
	openCase(t, f.baseVars())

	caps := capsOf(t, f)
	assert.NotContains(t, caps, "appium:app")
	assert.NotContains(t, caps, "appium:appPackage")
	assert.Equal(t, 1, f.count(http.MethodPost, sessionRoot))
}

func TestFR04_AppFileIsReadOnceBeforeTheSessionOpens(t *testing.T) {
	// the file changing after the hash is not detected in M1 (M2): the executor only never reads it a second time,
	// so removing it once Setup has returned changes nothing the report claims
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path, "appSHA256": sum}))
	require.NoError(t, os.Remove(path))

	assert.Equal(t, sum, runStepFacts(ctx, t).AppSHA256)
}

// runStepFacts runs a no-op action to read the run facts of a result.
func runStepFacts(ctx context.Context, t *testing.T) Result {
	t.Helper()
	withAction(t, "noop", func(context.Context, *session, Executor, *Result) error { return nil })
	return runStep(ctx, t, "noop")
}
