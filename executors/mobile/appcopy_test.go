package mobile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Synthetic suite values that must never reach a command run on the SSH host.
const (
	suiteAppName = "suite-chosen-name.apk"
	suiteAppID   = "org.example.suite.chosen"
)

// venomCommand is the shape of every command the executor sends to the SSH host: venom's own folder and file name.
var venomCommand = regexp.MustCompile(`^(mkdir -m 700 /tmp/venom-mobile-[0-9a-f]{16}|cat > /tmp/venom-mobile-[0-9a-f]{16}/app\.apk|rm -rf /tmp/venom-mobile-[0-9a-f]{16})$`)

// copyVars are the settings of a case that copies an app through the fake SSH server.
func copyVars(t *testing.T, f *fakeServer, s *fakeSSH, extra map[string]interface{}) map[string]interface{} {
	t.Helper()
	vars := sshVars(t, f, s)
	vars["appId"] = suiteAppID
	for k, v := range extra {
		vars[k] = v
	}
	return vars
}

// onlyFolder returns the one folder the emulated host holds.
func onlyFolder(t *testing.T, s *fakeSSH) string {
	t.Helper()
	dirs := s.folders()
	require.Len(t, dirs, 1)
	return dirs[0]
}

func TestAC4_CopyLandsInAVenomNamedFolderAndIsRemovedAtTearDown(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, sum := writeApp(t, t.TempDir(), suiteAppName)
	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{"app": path}))

	dir := onlyFolder(t, s)
	assert.Regexp(t, `^/tmp/venom-mobile-[0-9a-f]{16}$`, dir)
	assert.Equal(t, map[string][]byte{dir + "/app.apk": []byte(testApp)}, s.storedFiles())
	assert.Equal(t, dir+"/app.apk", capsOf(t, f)["appium:app"], "the server installs the copy")
	assert.Equal(t, sum, getSession(ctx).facts.appSHA256)

	require.NoError(t, Executor{}.TearDown(ctx))
	assert.Empty(t, s.folders())
	assert.Empty(t, s.storedFiles())
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionRoot+"/{id}"))
	requireTunnelGone(t)
}

func TestAC4_NoCommandCarriesASuiteValue(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, _ := writeApp(t, t.TempDir(), suiteAppName)
	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{
		"app": path, "capabilities": map[string]interface{}{"appium:noReset": "suite-capability"},
	}))
	require.NoError(t, Executor{}.TearDown(ctx))

	cmds := s.commands()
	require.Len(t, cmds, 3, "mkdir, cat, rm")
	for _, c := range cmds {
		assert.Regexp(t, venomCommand, c)
		for _, v := range []string{suiteAppName, suiteAppID, "suite-capability", filepath.Dir(path)} {
			assert.NotContains(t, c, v)
		}
	}
	assert.True(t, strings.HasPrefix(cmds[2], "rm -rf "), "the folder goes at TearDown")
}

func TestAC4_LargeFileStreamsIntact(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	data := make([]byte, 2<<20)
	_, err := rand.New(rand.NewSource(7)).Read(data) //nolint:gosec // synthetic bytes, not a secret
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "big.apk")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	h := sha256.Sum256(data)
	want := hex.EncodeToString(h[:])

	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{"app": path, "appSHA256": want}))

	assert.Equal(t, data, s.storedFiles()[onlyFolder(t, s)+"/app.apk"])
	assert.Equal(t, want, getSession(ctx).facts.appSHA256)
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestAC4_WithoutSSHSettingsNothingIsCopied(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, _ := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, f.withVars(map[string]interface{}{"app": path}))

	assert.Equal(t, path, capsOf(t, f)["appium:app"], "the local path, as in M1")
	assert.Empty(t, s.commands())
	assert.Zero(t, s.accepted.Load())
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestAC4_MissingLocalFileIsRefusedBeforeConnecting(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	err := setupErr(copyVars(t, f, s, map[string]interface{}{"app": filepath.Join(t.TempDir(), "none.apk")}))

	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Zero(t, s.accepted.Load(), "no SSH connection")
	assert.Zero(t, f.total())
}

func TestAC5_WrongHashMakesNoSessionRequestAndRemovesTheFolder(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	wrong := strings.Repeat("0", len(sum))

	err := setupErr(copyVars(t, f, s, map[string]interface{}{"app": path, "appSHA256": wrong}))

	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Contains(t, err.Error(), sum, "the hash of the bytes sent")
	assert.Contains(t, err.Error(), wrong, "the hash the suite expects")
	assert.Zero(t, f.total(), "nothing reached the automation server")
	assert.Empty(t, s.folders(), "the folder is gone")
	assert.Empty(t, s.storedFiles())
	requireTunnelGone(t)
}

func TestAC5_RightHashPassesInAnyCase(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, sum := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{"app": path, "appSHA256": strings.ToUpper(sum)}))

	assert.Equal(t, sum, getSession(ctx).facts.appSHA256, "the report shows the hash of the bytes sent")
	assert.Equal(t, 1, f.count(http.MethodPost, sessionRoot))
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestAC4_FolderIsRemovedWhenTheSessionFailsToOpen(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	f.on("POST /session", fail(http.StatusInternalServerError, "session not created", "install failed"))
	path, _ := writeApp(t, t.TempDir(), "app.apk")

	err := setupErr(copyVars(t, f, s, map[string]interface{}{"app": path}))

	require.ErrorContains(t, err, "cannot open a session")
	cmds := s.commands()
	require.Len(t, cmds, 3, "the copy was made, then removed")
	assert.True(t, strings.HasPrefix(cmds[2], "rm -rf "))
	assert.Empty(t, s.folders())
	requireTunnelGone(t)
}

func TestAC4_StalledRemovalIsBoundedByTheCloseTimeout(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, _ := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{"app": path}))
	shorten(t, &warnf, func(_ context.Context, _ string, _ ...interface{}) {})
	shorten(t, &closeTimeout, 200*time.Millisecond)
	s.rmHangs.Store(true)

	start := time.Now()
	err := Executor{}.TearDown(ctx)

	require.ErrorContains(t, err, "cannot remove /tmp/venom-mobile-")
	assert.Less(t, time.Since(start), 10*time.Second)
	requireTunnelGone(t)
}

func TestAC4_RemovalFailureIsReportedButTheSessionStillCloses(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, _ := writeApp(t, t.TempDir(), "app.apk")
	ctx := openCase(t, copyVars(t, f, s, map[string]interface{}{"app": path}))
	warnings := 0
	shorten(t, &warnf, func(_ context.Context, _ string, _ ...interface{}) { warnings++ })
	s.failRemoval()

	err := Executor{}.TearDown(ctx)

	require.ErrorContains(t, err, "cannot remove /tmp/venom-mobile-")
	assert.Equal(t, 1, warnings)
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionRoot+"/{id}"), "the device session closed first")
	requireTunnelGone(t)
}
