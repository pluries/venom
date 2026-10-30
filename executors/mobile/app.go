package mobile

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// resolveApp reads the app file named by mobile.app once, hashes those bytes and checks them against
// mobile.appSHA256 when it is set (FR-04, T-28). Every refusal is a usage error, so it happens before any request.
// On success cfg.appPath is the absolute path and cfg.appSHA256 the computed hash in lower case hex, which is the
// value the report shows. Without mobile.app nothing is read and cfg.appSHA256 is left as the suite gave it (it
// has nothing to check, so a set value is refused).
//
// Without an SSH tunnel (M1) the hash covers the bytes read here, not the bytes Appium installs later from the path.
// With one, the file is sent to the SSH host (appcopy.go) and the hash is computed over the bytes sent, so here the
// file is only located and cfg.appSHA256 keeps the value the suite gave until the copy has been checked (FR-04).
//
// On iOS mobile.app is a simulator bundle (a .app folder) or a .zip file. A .app folder has no single file to hash, so
// mobile.appSHA256 is refused with one; without mobile.app the app is launched by its bundle id, which is then required.
func resolveApp(workdir string, cfg *setupConfig) error {
	if cfg.app == "" {
		return checkNoApp(*cfg)
	}
	path, bundle, err := locateApp(workdir, cfg.app, cfg.platform)
	if err != nil {
		return err
	}
	cfg.appPath, cfg.appBundle = path, bundle
	if bundle && strings.TrimSpace(cfg.appSHA256) != "" {
		return usagef("mobile.appSHA256 is set but mobile.app %q is a .app folder, which has no single file to hash: give a .zip or leave mobile.appSHA256 out", cfg.app)
	}
	if bundle || cfg.ssh != nil {
		return nil
	}
	return hashApp(cfg)
}

// checkNoApp covers a suite without mobile.app: nothing to check, and on iOS the bundle id names the app.
func checkNoApp(cfg setupConfig) error {
	if cfg.appSHA256 != "" {
		return usagef("mobile.appSHA256 is set but mobile.app is not: there is no file to check")
	}
	if cfg.platform == platformIOS && cfg.appID == "" {
		return usagef("mobile.appId (the bundle id) is required on iOS when mobile.app is not given")
	}
	return nil
}

// hashApp reads the app file once and checks its hash; cfg.appSHA256 then holds the computed one.
func hashApp(cfg *setupConfig) error {
	content, err := os.ReadFile(cfg.appPath)
	if err != nil {
		return usagef("mobile.app %q cannot be read: %v", cfg.app, err)
	}
	sum := sha256.Sum256(content)
	got := hex.EncodeToString(sum[:])
	if err := checkAppHash(cfg.app, got, cfg.appSHA256); err != nil {
		return err
	}
	cfg.appSHA256 = got
	return nil
}

// locateApp makes the path of mobile.app absolute and checks that it names a readable app: a file on Android, a .app
// folder (bundle is true) or a .zip file on iOS.
func locateApp(workdir, app, platform string) (path string, bundle bool, err error) {
	path = app
	if !filepath.IsAbs(path) {
		path = filepath.Join(workdir, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", false, usagef("mobile.app %q: %v", app, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, usagef("mobile.app %q cannot be read: %v", app, err)
	}
	if platform == platformIOS {
		bundle, err = checkIOSApp(app, info)
		return path, bundle, err
	}
	if info.IsDir() {
		return "", false, usagef("mobile.app %q is a directory, not an app file", app)
	}
	return path, false, nil
}

// checkIOSApp accepts a .app folder or a .zip file, by extension in any case, and says which one it is.
func checkIOSApp(app string, info os.FileInfo) (bundle bool, err error) {
	switch strings.ToLower(filepath.Ext(info.Name())) {
	case ".app":
		if !info.IsDir() {
			return false, usagef("mobile.app %q is a file: a .app is a simulator bundle, a folder (or give a .zip)", app)
		}
		return true, nil
	case ".zip":
		if info.IsDir() {
			return false, usagef("mobile.app %q is a directory, not an app file", app)
		}
		return false, nil
	case ".ipa":
		return false, usagef("mobile.app %q is an .ipa, a device build that does not install on a simulator: build a simulator .app (or zip it)", app)
	default:
		return false, usagef("mobile.app %q must be a .app folder or a .zip file on iOS", app)
	}
}

// checkAppHash compares the computed hash with mobile.appSHA256, when the suite set one.
func checkAppHash(app, got, expected string) error {
	if want := strings.TrimSpace(expected); want != "" && !strings.EqualFold(want, got) {
		return usagef("mobile.app %q has SHA-256 %s, mobile.appSHA256 expects %s", app, got, strings.ToLower(want))
	}
	return nil
}

// appCapabilities returns the capabilities that name the app under test: the local absolute path without a tunnel,
// the path of the copy on the SSH host once appcopy.go has replaced cfg.appPath. With no app file the
// session launches by package id, and with neither it names no app: the executor invents no default package.
func appCapabilities(cfg setupConfig) map[string]interface{} {
	caps := map[string]interface{}{}
	if cfg.appPath != "" {
		caps[capabilityPrefix+"app"] = cfg.appPath
	}
	if cfg.appID != "" {
		caps[capabilityPrefix+profileOf(cfg.platform).appIDKey] = cfg.appID
	}
	return caps
}
