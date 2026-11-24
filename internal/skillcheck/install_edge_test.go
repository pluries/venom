package skillcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// existingVenom puts a stub `venom` that prints the given version line in a new folder and returns it.
func existingVenom(t *testing.T, versionLine string) string {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\necho \"" + versionLine + "\"\n"
	p := filepath.Join(dir, "venom")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	require.NoError(t, os.Chmod(p, 0o700))
	return dir
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// pathTail is the last two folders of a stub's folder plus the file name, the form the script prints a path in on
// every system (a Git Bash path differs from the Windows one only in its head).
func pathTail(dir string) string {
	parts := strings.Split(filepath.ToSlash(dir), "/")
	return strings.Join(parts[len(parts)-2:], "/") + "/venom"
}

// requireOtherVenomReported checks that the script told, on standard output, where the other venom is, which version it
// is, what that means for the pinned tag, and that this install does not change it.
func requireOtherVenomReported(t *testing.T, res cmdResult, stubDir, found, verdict string) {
	t.Helper()
	require.Contains(t, res.stdout, pathTail(stubDir), "the path of the other venom")
	require.Contains(t, res.stdout, found, "its version")
	require.Contains(t, res.stdout, verdict)
	require.Regexp(t, regexp.MustCompile(`(?i)(does not change|will not change|not changed|left alone|unchanged)`), res.stdout)
}

// US1-AS4 and US1-AS10, T012 (changed by the maker's decision for skill 0.1.2: a venom on the PATH no longer stops the
// install): another venom on the PATH, of another version, the upstream project or the pinned version, is reported on
// standard output and left alone, and the install goes on into the clean destination with exit 0.
func TestInstall_US1AS4_AS10_OtherVenomOnPathIsReportedAndLeftAlone(t *testing.T) {
	cases := []struct{ name, versionLine, found, verdict string }{
		{"US1-AS4 another version of Venom Pluries", "Version venom: v1.4.0-pluries.2", "v1.4.0-pluries.2", "not Venom Pluries " + releaseTag},
		{"US1-AS10 the upstream project", "Version venom: v1.2.0", "v1.2.0", "not Venom Pluries " + releaseTag},
		{"US1-AS4 the pinned version", "Version venom: " + releaseTag, releaseTag, "is the pinned version"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			requireInstallTools(t)
			k := fixtureKeysFor(t)
			stubDir := existingVenom(t, c.versionLine)
			before := readFile(t, filepath.Join(stubDir, "venom"))
			sb := newSandbox(t)
			res := execInstall(t, k, installRun{sb: sb, pinned: k.one, pathHead: stubDir, args: []string{"--from-dir", slash(newRelease(t, k, k.one))}})
			require.Equal(t, 0, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
			requireOtherVenomReported(t, res, stubDir, c.found, c.verdict)
			require.Equal(t, []string{"venom"}, listing(t, sb.dest), "the install goes on into the destination")
			require.Equal(t, stubVenom(releaseTag), readFile(t, filepath.Join(sb.dest, "venom")))
			require.Equal(t, before, readFile(t, filepath.Join(stubDir, "venom")), "the other venom is left alone")
			sb.requireNothingOutsideDest(t)
		})
	}
}

// US1-AS4, T012 (changed, same decision): --replace matters only for a venom in the destination; given with a venom on
// the PATH and a clean destination it changes nothing about the outcome, and the PATH venom is still not touched.
func TestInstall_US1AS4_ReplaceLeavesTheVenomOnPathAlone(t *testing.T) {
	requireInstallTools(t)
	k := fixtureKeysFor(t)
	stubDir := existingVenom(t, "Version venom: v1.2.0")
	before := readFile(t, filepath.Join(stubDir, "venom"))
	sb := newSandbox(t)
	res := execInstall(t, k, installRun{sb: sb, pinned: k.one, pathHead: stubDir, args: []string{"--from-dir", slash(newRelease(t, k, k.one)), "--replace"}})
	require.Equal(t, 0, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	require.Equal(t, []string{"venom"}, listing(t, sb.dest))
	require.Equal(t, before, readFile(t, filepath.Join(stubDir, "venom")), "the PATH venom is not replaced")
	sb.requireNothingOutsideDest(t)
}

// US1-AS5, T012: the script writes nothing outside the destination, on a pass and on a failure
// (the pin file .venom-version is written by the skill, never by the script).
func TestInstall_US1AS5_WritesNothingOutsideDest(t *testing.T) {
	for _, c := range []installCase{
		{name: "pass"},
		{name: "failed check", mutate: flipByte(releaseAsset, -1), wantCode: 10, wantName: "checksum"},
	} {
		t.Run(c.name, func(t *testing.T) { runInstallCase(t, c) })
	}
}

// US1-AS6, T012: an unknown system exits 20, names the supported systems and downloads nothing.
func TestInstall_US1AS6_UnknownSystemExits20(t *testing.T) {
	requireInstallTools(t)
	k := fixtureKeysFor(t)
	sb := newSandbox(t)
	srv := newReleaseServer(t, newRelease(t, k, k.one))
	res := execInstall(t, k, installRun{sb: sb, pinned: k.one, unameS: "Plan9", unameM: "mips", args: []string{"--base-url", srv.URL}})
	require.Equal(t, 20, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	out := strings.ToLower(res.combined())
	for _, supported := range []string{"linux", "darwin", "windows"} {
		require.Contains(t, out, supported, "the supported systems are named")
	}
	require.Empty(t, srv.requested(), "nothing is downloaded for an unsupported system")
	require.Empty(t, listing(t, sb.dest))
	sb.requireNothingOutsideDest(t)
}

// US1-AS6, T012: the name table maps each system and architecture of the release record to its asset.
func TestInstall_US1AS6_NameTable(t *testing.T) {
	cases := []struct{ s, m, asset string }{
		{"Linux", "x86_64", "venom.linux-amd64"},
		{"Linux", "aarch64", "venom.linux-arm64"},
		{"Linux", "armv7l", "venom.linux-arm"},
		{"Linux", "i686", "venom.linux-386"},
		{"Linux", "ppc64le", "venom.linux-ppc64le"},
		{"Darwin", "x86_64", "venom.darwin-amd64"},
		{"Darwin", "arm64", "venom.darwin-arm64"},
		{"MINGW64_NT-10.0-26200", "x86_64", "venom.windows-amd64.exe"},
		{"MSYS_NT-10.0-26200", "x86_64", "venom.windows-amd64.exe"},
		{"OpenBSD", "amd64", "venom.openbsd-amd64"},
		{"FreeBSD", "amd64", "venom.freebsd-amd64"},
	}
	for _, c := range cases {
		t.Run(c.s+"_"+c.m, func(t *testing.T) {
			requireInstallTools(t)
			k := fixtureKeysFor(t)
			srv := newReleaseServer(t, newReleaseFor(t, k, k.one, c.asset))
			execInstall(t, k, installRun{sb: newSandbox(t), pinned: k.one, unameS: c.s, unameM: c.m, args: []string{"--base-url", srv.URL}})
			require.Contains(t, srv.requested(), "/releases/download/"+releaseTag+"/"+c.asset)
		})
	}
}

// US1-AS1, T012: a download that fails exits 21 and writes nothing.
func TestInstall_US1AS1_DownloadFailureExits21(t *testing.T) {
	requireInstallTools(t)
	k := fixtureKeysFor(t)
	sb := newSandbox(t)
	srv := newReleaseServer(t, t.TempDir())
	res := execInstall(t, k, installRun{sb: sb, pinned: k.one, args: []string{"--base-url", srv.URL}})
	require.Equal(t, 21, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	require.NotEmpty(t, srv.requested(), "the script tried to download")
	require.Empty(t, listing(t, sb.dest))
	sb.requireNothingOutsideDest(t)
}
