package skillcheck

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// wrapperScript fills a folder with one wrapper script per command found on the PATH bash sees, except
// the names given as arguments; running a wrapper runs the real command, so a PATH made of this folder
// holds everything the script may need but the excluded tools.
const wrapperScript = `out=$1; shift
IFS=:
for d in $PATH; do
  for f in "$d"/*; do
    [ -f "$f" ] && [ -x "$f" ] || continue
    n=${f##*/}; n=${n%.exe}
    for skip in "$@"; do [ "$n" = "$skip" ] && continue 2; done
    [ -e "$out/$n" ] || printf '#!/bin/sh\nexec "%s" "$@"\n' "$f" > "$out/$n"
  done
done
chmod +x "$out"/*
`

// pathWithout returns a folder of wrapper scripts for every command but the excluded ones.
func pathWithout(t *testing.T, excluded ...string) string {
	t.Helper()
	return buildWrappers(t, t.TempDir(), excluded...)
}

var (
	noVenomOnce sync.Once
	noVenomDir  string
)

// noVenomPath is a folder of wrappers for every command but venom, built once per test run: whatever venom the
// developer machine has on its PATH stays invisible to the install tests. TestMain removes it.
func noVenomPath(t *testing.T) string {
	t.Helper()
	noVenomOnce.Do(func() {
		d, err := os.MkdirTemp("", "skillcheck-path-")
		if err != nil {
			t.Fatalf("temporary folder: %v", err)
		}
		noVenomDir = d
		buildWrappers(t, d, "venom")
	})
	return noVenomDir
}

func buildWrappers(t *testing.T, dir string, excluded ...string) string {
	t.Helper()
	args := append([]string{"-c", wrapperScript, "_", slash(dir)}, excluded...)
	res := bashRun(t, mergeEnv(), "", args...)
	require.Equal(t, 0, res.code, "building the wrapper folder: %s", res.stderr)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "no wrapper was made")
	for _, name := range excluded {
		_, err := os.Stat(filepath.Join(dir, name))
		require.True(t, os.IsNotExist(err), "%s must be left out", name)
	}
	return dir
}

// US1-AS8, T010: a system with no tool for a check stops with exit 14 and names what to install.
func TestVerifyTools_US1AS8_MissingToolExits14(t *testing.T) {
	cases := []struct {
		name     string
		excluded []string
		install  string
	}{
		{"no openssl", []string{"openssl"}, "openssl"},
		{"no SHA-256 tool", []string{"sha256sum", "shasum"}, "sha256"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireVerifyTools(t)
			k := fixtureKeysFor(t)
			dir := newRelease(t, k, k.one)
			res := execVerify(t, k, verifyRun{dir: dir, pinned: k.one, pathDir: pathWithout(t, c.excluded...)})
			require.Equal(t, 14, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
			require.Contains(t, res.stderr, "FAIL ")
			require.Contains(t, res.stderr, c.install)
			require.NotContains(t, res.stdout, "OK signed-by")
		})
	}
}

// hostileEnv points OpenSSL at a hostile configuration and a trust file that does not exist.
func hostileEnv(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.cnf")
	// Loads only the legacy provider: any openssl that honours it cannot compute SHA-256.
	conf := "openssl_conf = oi\n[oi]\nproviders = ps\n[ps]\nlegacy = ls\n[ls]\nactivate = 1\n"
	require.NoError(t, os.WriteFile(broken, []byte(conf), 0o600))
	missing := slash(filepath.Join(dir, "missing.pem"))
	return []string{"OPENSSL_CONF=" + slash(broken), "SSL_CERT_FILE=" + missing, "SSL_CERT_DIR=" + slash(dir)}
}

// US1-AS2, T010: a hostile OPENSSL_CONF or SSL_CERT_FILE in the caller's environment changes no verdict.
func TestVerifyTools_US1AS2_HostileEnvironmentChangesNoVerdict(t *testing.T) {
	cases := []struct {
		name     string
		mutate   mutation
		wantCode int
		wantName string
	}{
		{"good release", nil, 0, ""},
		{"changed executable", flipByte(releaseAsset, -1), 10, "checksum"},
		{"changed signature", flipByte(p7sName, -1), 11, "signature"},
		{"changed timestamp token", flipByte(tsrName, -1), 13, "timestamp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireVerifyTools(t)
			k := fixtureKeysFor(t)
			dir := newRelease(t, k, k.one)
			applyMutation(t, c.mutate, dir)
			res := execVerify(t, k, verifyRun{dir: dir, pinned: k.one, env: hostileEnv(t)})
			assertVerdict(t, res, c.wantCode, c.wantName)
		})
	}
}
