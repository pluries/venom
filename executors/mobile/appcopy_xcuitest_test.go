package mobile

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// iosVenomCommand is the shape of every command sent to the SSH host for an iOS app: venom's folder and app.zip.
var iosVenomCommand = regexp.MustCompile(`^(mkdir -m 700 /tmp/venom-mobile-[0-9a-f]{16}|cat > /tmp/venom-mobile-[0-9a-f]{16}/app\.zip|rm -rf /tmp/venom-mobile-[0-9a-f]{16})$`)

// tempZips lists the archives venom made in the temporary folder.
func tempZips(t *testing.T) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(os.TempDir(), "venom-mobile-*.zip"))
	require.NoError(t, err)
	return found
}

// zipNames reads a zip held in memory and returns its entry names.
func zipNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	names := []string{}
	for _, e := range zr.File {
		names = append(names, e.Name)
	}
	return names
}

func TestAC2_AppFolderIsZippedAndCopiedThroughSSH(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	iosAnswer(f)
	bundle := writeBundleDir(t, t.TempDir(), "Demo.app")
	before := tempZips(t)
	vars := iosVars(f, map[string]interface{}{"app": bundle, "appId": suiteAppID})
	ctx := openCase(t, copyVars(t, f, s, vars))

	dir := onlyFolder(t, s)
	stored := s.storedFiles()
	require.Len(t, stored, 1)
	assert.ElementsMatch(t,
		[]string{"Demo.app/Frameworks/", "Demo.app/Frameworks/lib.dylib", "Demo.app/Info.plist"},
		zipNames(t, stored[dir+"/app.zip"]))
	assert.Equal(t, dir+"/app.zip", capsOf(t, f)["appium:app"])
	assert.Empty(t, getSession(ctx).facts.appSHA256)
	assert.Equal(t, before, tempZips(t), "the temporary zip is gone once staging ends")

	require.NoError(t, Executor{}.TearDown(ctx))
	assert.Empty(t, s.folders())
	for _, c := range s.commands() {
		assert.Regexp(t, iosVenomCommand, c)
		for _, v := range []string{"Demo.app", suiteAppID, filepath.Dir(bundle)} {
			assert.NotContains(t, c, v)
		}
	}
}

func TestAC2_ZipIsCopiedAsAppZipWithTheHashOfTheBytesSent(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	iosAnswer(f)
	path, sum := writeApp(t, t.TempDir(), "Demo.zip")
	ctx := openCase(t, copyVars(t, f, s, iosVars(f, map[string]interface{}{"app": path, "appSHA256": sum})))

	assert.Equal(t, map[string][]byte{onlyFolder(t, s) + "/app.zip": []byte(testApp)}, s.storedFiles())
	assert.Equal(t, sum, getSession(ctx).facts.appSHA256)
	for _, c := range s.commands() {
		assert.Regexp(t, iosVenomCommand, c)
	}
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestAC2_ZipWithAWrongHashRemovesTheFolder(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	path, sum := writeApp(t, t.TempDir(), "Demo.zip")
	wrong := strings.Repeat("0", len(sum))

	err := setupErr(copyVars(t, f, s, iosVars(f, map[string]interface{}{"app": path, "appSHA256": wrong})))

	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Zero(t, f.total())
	assert.Empty(t, s.folders())
	requireTunnelGone(t)
}

func TestAC2_SymlinkInTheBundleIsRefused(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	bundle := writeBundleDir(t, t.TempDir(), "Demo.app")
	if err := os.Symlink(filepath.Join(bundle, "Info.plist"), filepath.Join(bundle, "Frameworks", "link")); err != nil {
		t.Skipf("symbolic links cannot be created here: %v", err)
	}
	before := tempZips(t)

	err := setupErr(copyVars(t, f, s, iosVars(f, map[string]interface{}{"app": bundle})))

	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Contains(t, err.Error(), filepath.Join("Frameworks", "link"))
	assert.Zero(t, f.total())
	assert.Empty(t, s.folders())
	assert.Equal(t, before, tempZips(t))
	requireTunnelGone(t)
}

func TestAC2_AndroidKeepsItsRemoteName(t *testing.T) {
	assert.Equal(t, "app.apk", remoteNameOf(platformAndroid))
	assert.Equal(t, "app.zip", remoteNameOf(platformIOS))
}
