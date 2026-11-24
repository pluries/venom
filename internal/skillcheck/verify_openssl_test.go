package skillcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// brewOpenSSL are the places verify-release.sh may fall back to when the openssl on the PATH is not OpenSSL 3 or later.
var brewOpenSSL = []string{
	"/opt/homebrew/bin/openssl",
	"/opt/homebrew/opt/openssl@3/bin/openssl",
	"/usr/local/opt/openssl@3/bin/openssl",
}

// requireNoBrewOpenSSL skips on a machine that has one of the fallback binaries: there the script would rightly find it.
func requireNoBrewOpenSSL(t *testing.T) {
	t.Helper()
	for _, p := range brewOpenSSL {
		if _, err := os.Stat(p); err == nil {
			t.Skipf("%s exists, so the fallback would be found", p)
		}
	}
}

// fakeOpenSSLPath returns a PATH whose first folder holds an `openssl` that prints versionLine for `version` and is
// otherwise the real binary, so only the version it announces can change a verdict.
func fakeOpenSSLPath(t *testing.T, versionLine string) string {
	t.Helper()
	realPath, err := exec.LookPath("openssl")
	require.NoError(t, err)
	dir := t.TempDir()
	body := "#!/bin/sh\nif [ \"$1\" = version ]; then echo '" + versionLine + "'; exit 0; fi\nexec \"" + slash(realPath) + "\" \"$@\"\n"
	p := filepath.Join(dir, "openssl")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	require.NoError(t, os.Chmod(p, 0o700))
	return dir + string(os.PathListSeparator) + noVenomPath(t)
}

// US1-AS8, T015: a PATH whose openssl is LibreSSL or OpenSSL older than 3 stops verify-release.sh with exit 14, naming
// OpenSSL 3 (and, for LibreSSL, saying so), even though that binary would otherwise work.
func TestVerifyOpenSSL_US1AS8_NotOpenSSL3Exits14(t *testing.T) {
	cases := []struct {
		name, versionLine string
		mentions          []string
	}{
		{"LibreSSL 3.3.6", "LibreSSL 3.3.6", []string{"OpenSSL 3", "LibreSSL"}},
		{"OpenSSL 1.1.1w", "OpenSSL 1.1.1w  11 Sep 2023", []string{"OpenSSL 3"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireVerifyTools(t)
			requireNoBrewOpenSSL(t)
			k := fixtureKeysFor(t)
			res := execVerify(t, k, verifyRun{dir: newRelease(t, k, k.one), pinned: k.one, pathDir: fakeOpenSSLPath(t, c.versionLine)})
			require.Equal(t, 14, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
			require.NotContains(t, res.stdout, "OK signed-by")
			for _, m := range c.mentions {
				require.Contains(t, res.stderr, m)
			}
		})
	}
}

// US1-AS8, T015: the same through the install script: exit 14 and nothing reaches the destination.
func TestInstallOpenSSL_US1AS8_NotOpenSSL3InstallsNothing(t *testing.T) {
	for _, c := range []struct{ name, versionLine string }{
		{"LibreSSL 3.3.6", "LibreSSL 3.3.6"},
		{"OpenSSL 1.1.1w", "OpenSSL 1.1.1w  11 Sep 2023"},
	} {
		t.Run(c.name, func(t *testing.T) {
			requireInstallTools(t)
			requireNoBrewOpenSSL(t)
			k := fixtureKeysFor(t)
			sb := newSandbox(t)
			rel := newRelease(t, k, k.one)
			res := execInstall(t, k, installRun{sb: sb, pinned: k.one, args: []string{"--from-dir", slash(rel)},
				extraEnv: []string{"PATH=" + fakeOpenSSLPath(t, c.versionLine)}})
			require.Equal(t, 14, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
			require.Contains(t, res.stderr, "OpenSSL 3")
			require.Empty(t, listing(t, sb.dest))
			sb.requireNothingOutsideDest(t)
		})
	}
}

// US1-AS8, T015: the control, a PATH whose openssl announces OpenSSL 3 and otherwise behaves the same passes,
// so only the announced version decides.
func TestVerifyOpenSSL_US1AS8_OpenSSL3OnThePathPasses(t *testing.T) {
	requireVerifyTools(t)
	k := fixtureKeysFor(t)
	res := execVerify(t, k, verifyRun{dir: newRelease(t, k, k.one), pinned: k.one, pathDir: fakeOpenSSLPath(t, "OpenSSL 3.0.13 30 Jan 2024 (Library: OpenSSL 3.0.13 30 Jan 2024)")})
	assertVerdict(t, res, 0, "")
}
