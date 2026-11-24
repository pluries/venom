package skillcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var venomRun = regexp.MustCompile(`\bvenom run\b`)

// US3-AS6, US1-AS5: each CI body reads .venom-version, puts the destination on the PATH and calls the install
// step before any venom run, so a failed check stops the build before a suite runs.
func TestCIInstall_US3AS6_InstallStepComesBeforeTheSuite(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		install := strings.Index(body, installCommand)
		run := venomRun.FindStringIndex(body)
		require.GreaterOrEqual(t, install, 0, "the install step is in the body")
		require.NotNil(t, run, "the body runs the suite")
		require.Less(t, install, run[0], "the install step comes before the first venom run")
		require.Contains(t, body, ".venom-version")
		require.Regexp(t, regexp.MustCompile(`PATH[^\n]*\.local/bin|\.local/bin[^\n]*PATH`), body, "the destination is put on the PATH")
	})
}

// project builds a project folder holding the skill's two scripts and release.env as tools/venom/, the layout the
// CI bodies call, plus a .venom-version file, and returns it.
func project(t *testing.T, tag string) string {
	t.Helper()
	dir := t.TempDir()
	tools := filepath.Join(dir, "tools", "venom")
	require.NoError(t, os.MkdirAll(tools, 0o700))
	for _, f := range []string{"scripts/install-venom.sh", "scripts/verify-release.sh", "release.env"} {
		body, err := os.ReadFile(skillFile(t, f))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tools, filepath.Base(f)), body, 0o600)) //nolint:gosec // temporary test folder
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".venom-version"), []byte(tag+"\n"), 0o600))
	return dir
}

// runCIInstall runs the body's install step in a project, as a CI runner would, with the test seams (the loopback
// server, the fixture signer, the fixture timestamp root) added after it.
func runCIInstall(t *testing.T, k *fixtureKeys, rel string, mutate mutation) (cmdResult, sandbox) {
	t.Helper()
	applyMutation(t, mutate, rel)
	sb := newSandbox(t)
	srv := newReleaseServer(t, rel)
	home := filepath.Join(sb.home, ".local", "bin")
	require.NoError(t, os.MkdirAll(home, 0o700))
	cwd := project(t, releaseTag)
	env := installEnv(t, k, installRun{sb: sb, pinned: k.one})
	line := installCommand + ` --base-url "$1" --release-env "$2"`
	res := bashRunWith(t, bareBashPath(t), env, cwd, "-c", line, "_", srv.URL, slash(releaseEnvFor(t, k.one)))
	return res, sb
}

// US3-AS6: the install step of each CI body, run on a copy of the release files, passes on the good files and fails
// on a changed byte with the check named, installing nothing.
func TestCIInstall_US3AS6_InstallStepFailsOnAChangedByte(t *testing.T) {
	forEachCIBody(t, func(t *testing.T, _ ciRef, body string) {
		require.Contains(t, body, installCommand)
		requireInstallTools(t)
		k := fixtureKeysFor(t)

		good, sb := runCIInstall(t, k, newRelease(t, k, k.one), nil)
		require.Equal(t, 0, good.code, "stdout:\n%s\nstderr:\n%s", good.stdout, good.stderr)
		require.Equal(t, []string{"venom"}, listing(t, filepath.Join(sb.home, ".local", "bin")))

		bad, sb := runCIInstall(t, k, newRelease(t, k, k.one), flipByte(releaseAsset, -1))
		require.Equal(t, 10, bad.code, "stdout:\n%s\nstderr:\n%s", bad.stdout, bad.stderr)
		require.Contains(t, bad.stderr, "FAIL checksum:")
		require.Empty(t, listing(t, filepath.Join(sb.home, ".local", "bin")))
	})
}
