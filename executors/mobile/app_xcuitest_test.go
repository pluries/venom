package mobile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBundleID = "org.example.sim"

// writeBundleDir makes a synthetic simulator bundle and returns its path.
func writeBundleDir(t *testing.T, parent, name string) string {
	t.Helper()
	root := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Frameworks"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Info.plist"), []byte("plist"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Frameworks", "lib.dylib"), []byte("lib"), 0o600))
	return root
}

// requireUsage runs Setup and requires a usage error that made no request.
func requireUsage(t *testing.T, f *fakeServer, vars map[string]interface{}, text string) {
	t.Helper()
	_, err := Executor{}.Setup(testCtx(vars), nil)
	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Contains(t, err.Error(), text)
	assert.Equal(t, 0, f.total())
}

func TestAC2_IPAIsRefusedInAnyCase(t *testing.T) {
	for _, name := range []string{"build.ipa", "BUILD.IPA", "Build.Ipa"} {
		t.Run(name, func(t *testing.T) {
			f := newFakeServer(t)
			path, _ := writeApp(t, t.TempDir(), name)
			requireUsage(t, f, iosVars(f, map[string]interface{}{"app": path}), "does not install on a simulator")
		})
	}
}

func TestAC2_IOSAppMustBeAppOrZip(t *testing.T) {
	dir := t.TempDir()
	f := newFakeServer(t)
	other, _ := writeApp(t, dir, "build.apk")
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": other}), ".app folder or a .zip")

	plain := filepath.Join(dir, "plain")
	require.NoError(t, os.Mkdir(plain, 0o700))
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": plain}), ".app folder or a .zip")

	file, _ := writeApp(t, dir, "file.app")
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": file}), "mobile.app")

	zipDir := filepath.Join(dir, "dir.zip")
	require.NoError(t, os.Mkdir(zipDir, 0o700))
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": zipDir}), "directory")
}

func TestAC2_ZipOfASymlinkedAppRootIsRefused(t *testing.T) {
	parent := t.TempDir()
	bundle := writeBundleDir(t, parent, "Real.app")
	link := filepath.Join(parent, "Link.app")
	if err := os.Symlink(bundle, link); err != nil {
		t.Skipf("symbolic links are not available here: %v", err)
	}
	zipped, cleanup, err := zipBundle(link)
	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	assert.Contains(t, err.Error(), "must not be a symlink")
	assert.Empty(t, zipped)
	assert.Nil(t, cleanup)
}

func TestAC2_AppFolderIsPassedByItsAbsolutePath(t *testing.T) {
	f := newFakeServer(t)
	iosAnswer(f)
	bundle := writeBundleDir(t, t.TempDir(), "Demo.APP")
	ctx := openCase(t, iosVars(f, map[string]interface{}{"app": bundle}))

	assert.Equal(t, bundle, capsOf(t, f)["appium:app"])
	assert.Empty(t, getSession(ctx).facts.appSHA256, "a folder has no hash")
	assert.Equal(t, bundle, getSession(ctx).facts.app)
}

func TestAC2_ZipHashIsCheckedLikeOnAndroid(t *testing.T) {
	f := newFakeServer(t)
	iosAnswer(f)
	path, sum := writeApp(t, t.TempDir(), "Demo.zip")
	ctx := openCase(t, iosVars(f, map[string]interface{}{"app": path, "appSHA256": strings.ToUpper(sum)}))

	assert.Equal(t, path, capsOf(t, f)["appium:app"])
	assert.Equal(t, sum, getSession(ctx).facts.appSHA256)
}

func TestAC2_ZipWithAWrongHashMakesNoRequest(t *testing.T) {
	f := newFakeServer(t)
	path, sum := writeApp(t, t.TempDir(), "Demo.zip")
	wrong := strings.Repeat("0", len(sum))
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": path, "appSHA256": wrong}), sum)
}

func TestAC2_HashWithAnAppFolderIsRefused(t *testing.T) {
	f := newFakeServer(t)
	bundle := writeBundleDir(t, t.TempDir(), "Demo.app")
	requireUsage(t, f, iosVars(f, map[string]interface{}{"app": bundle, "appSHA256": "ab"}), "no single file to hash")
}

func TestAC3_BundleIDLaunchesWithoutAnApp(t *testing.T) {
	f := newFakeServer(t)
	iosAnswer(f)
	openCase(t, iosVars(f, map[string]interface{}{"appId": testBundleID}))

	caps := capsOf(t, f)
	assert.Equal(t, testBundleID, caps["appium:bundleId"])
	assert.NotContains(t, caps, "appium:app")
}

func TestAC3_NoAppAndNoBundleIDIsRefused(t *testing.T) {
	f := newFakeServer(t)
	vars := without(iosVars(f, nil), "appId")
	requireUsage(t, f, vars, "mobile.appId (the bundle id) is required on iOS when mobile.app is not given")
}

func TestAC3_AppAndBundleIDAreBothSent(t *testing.T) {
	f := newFakeServer(t)
	iosAnswer(f)
	bundle := writeBundleDir(t, t.TempDir(), "Demo.app")
	openCase(t, iosVars(f, map[string]interface{}{"app": bundle, "appId": testBundleID}))

	caps := capsOf(t, f)
	assert.Equal(t, bundle, caps["appium:app"])
	assert.Equal(t, testBundleID, caps["appium:bundleId"])
}

func TestAC3_AndroidWithoutAppNeedsNoPackage(t *testing.T) {
	f := newFakeServer(t)
	openCase(t, f.baseVars())
	assert.NotContains(t, capsOf(t, f), "appium:appPackage")
}
