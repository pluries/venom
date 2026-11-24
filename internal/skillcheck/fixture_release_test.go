package skillcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	releaseTag   = "v1.5.0-pluries.1"
	releaseAsset = "venom.linux-amd64"
	sumsName     = "SHA256SUMS"
	p7sName      = "SHA256SUMS.p7s"
	tsrName      = "SHA256SUMS.p7s.tsr"
)

// stubVenom is the "executable" of a fixture release: `venom version` prints what the real one prints.
func stubVenom(tag string) string {
	return "#!/bin/sh\necho \"Version venom: " + tag + "\"\n"
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeRelease writes the four files of a release into dir, signed by the given certificate: the stub
// named asset, SHA256SUMS (with entries for assets that are not present, as the real file has), the
// detached DER CMS signature and the timestamp token over that signature.
func writeRelease(t *testing.T, k *fixtureKeys, signer keyPair, dir, asset string) {
	t.Helper()
	exe := []byte(stubVenom(releaseTag))
	sums := fmt.Sprintf("%s  %s\n%s  venom.darwin-arm64\n%s  venom.windows-amd64.exe\n",
		sha256Hex(exe), asset, sha256Hex([]byte("other one")), sha256Hex([]byte("other two")))
	write := func(name string, body []byte) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), body, 0o600))
	}
	write(asset, exe)
	write(sumsName, []byte(sums))
	sumsPath, p7s, tsr := slash(filepath.Join(dir, sumsName)), slash(filepath.Join(dir, p7sName)), slash(filepath.Join(dir, tsrName))
	_, err := opensslIn(k.confPath, "cms", "-sign", "-binary", "-in", sumsPath, "-signer", signer.certPath,
		"-inkey", signer.keyPath, "-outform", "DER", "-out", p7s, "-md", "sha256")
	require.NoError(t, err, "signing the fixture SHA256SUMS")
	timestampFile(t, k, p7s, tsr)
}

// newRelease makes a release folder signed by signer, holding the asset of the Linux amd64 build.
func newRelease(t *testing.T, k *fixtureKeys, signer keyPair) string {
	t.Helper()
	return newReleaseFor(t, k, signer, releaseAsset)
}

// newReleaseFor makes a release folder whose executable has the given asset name.
func newReleaseFor(t *testing.T, k *fixtureKeys, signer keyPair, asset string) string {
	t.Helper()
	dir := t.TempDir()
	writeRelease(t, k, signer, dir, asset)
	return dir
}

// releaseEnvFor writes a release.env that pins the given signer, in a new folder.
func releaseEnvFor(t *testing.T, pinned keyPair) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "release.env")
	body := fmt.Sprintf("SKILL_VERSION=0.0.0-test\nVENOM_VERSION=%s\nSIGNER_SHA256=%s\nREPOSITORY=example/venom\n",
		releaseTag, pinned.fingerprint)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

// mutation changes a release folder before the script runs.
type mutation func(t *testing.T, dir string)

// flipByte changes one byte of a file; a negative index counts from the end.
func flipByte(name string, index int) mutation {
	return func(t *testing.T, dir string) {
		t.Helper()
		p := filepath.Join(dir, name)
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		if index < 0 {
			index += len(b)
		}
		b[index] ^= 0x01
		require.NoError(t, os.WriteFile(p, b, 0o600)) //nolint:gosec // temporary fixture folder
	}
}

func removeFile(name string) mutation {
	return func(t *testing.T, dir string) {
		t.Helper()
		require.NoError(t, os.Remove(filepath.Join(dir, name)))
	}
}

func allOf(ms ...mutation) mutation {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, m := range ms {
			m(t, dir)
		}
	}
}

func applyMutation(t *testing.T, m mutation, dir string) {
	t.Helper()
	if m != nil {
		m(t, dir)
	}
}

// verifyRun describes one run of verify-release.sh.
type verifyRun struct {
	dir     string
	asset   string
	pinned  keyPair
	caFile  string   // VENOM_CA_FILE, the local timestamp root by default
	env     []string // extra KEY=value pairs
	pathDir string   // when set, the whole PATH, and the shell is the one that does not add to it
}

// execVerify runs verify-release.sh on the folder and returns its outcome.
func execVerify(t *testing.T, k *fixtureKeys, r verifyRun) cmdResult {
	t.Helper()
	script := skillFile(t, "scripts/verify-release.sh")
	if r.asset == "" {
		r.asset = releaseAsset
	}
	if r.caFile == "" {
		r.caFile = k.caPath
	}
	env := append([]string{"VENOM_CA_FILE=" + r.caFile}, r.env...)
	if r.pathDir != "" {
		env = append(env, "PATH="+r.pathDir)
	}
	shell := bashPath(t)
	if r.pathDir != "" {
		shell = bareBashPath(t)
	}
	return bashRunWith(t, shell, mergeEnv(env...), "", slash(script), "--dir", slash(r.dir), "--asset", r.asset,
		"--release-env", slash(releaseEnvFor(t, r.pinned)))
}

// assertVerdict checks the exit code and, when wantName is not empty, that standard error holds the
// line `FAIL <wantName>:`; a pass (code 0) must print the single success line.
func assertVerdict(t *testing.T, res cmdResult, wantCode int, wantName string) {
	t.Helper()
	require.Equal(t, wantCode, res.code, "stdout:\n%s\nstderr:\n%s", res.stdout, res.stderr)
	if wantCode == 0 {
		require.Contains(t, res.stdout, "OK signed-by")
		return
	}
	if wantName != "" {
		require.Contains(t, res.stderr, "FAIL "+wantName+":")
	}
}

// runVerify is the table helper of the verify cases: a release signed by the pinned signer, changed by
// mutate, then checked; the verdict must be wantCode and wantName.
func runVerify(t *testing.T, mutate mutation, wantCode int, wantName string) cmdResult {
	t.Helper()
	requireVerifyTools(t)
	k := fixtureKeysFor(t)
	dir := newRelease(t, k, k.one)
	applyMutation(t, mutate, dir)
	res := execVerify(t, k, verifyRun{dir: dir, pinned: k.one})
	assertVerdict(t, res, wantCode, wantName)
	return res
}

// normalizeFingerprints lowercases text and drops colons, so a fingerprint matches in any usual notation.
func normalizeFingerprints(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, ":", ""))
}
