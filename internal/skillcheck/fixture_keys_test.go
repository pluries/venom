package skillcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// opensslConfig is the one configuration every fixture command runs under, so the host's own
// configuration can never change the fixture.
const opensslConfig = `[req]
distinguished_name = dn
prompt = no
[dn]
CN = skillcheck
[v3_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign
[signer_ext]
keyUsage = critical,digitalSignature
extendedKeyUsage = codeSigning
[tsa_ext]
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,timeStamping
`

// keyPair is a certificate and its private key, both PEM files in the fixture folder.
type keyPair struct {
	certPath    string
	keyPath     string
	fingerprint string // lowercase hex, no colons: the form of SIGNER_SHA256
}

// fixtureKeys is a throwaway CA, two signing certificates (the pinned one and a wrong one) and a
// time-stamping certificate. Nothing of it is kept in the repository.
type fixtureKeys struct {
	dir      string
	confPath string
	caPath   string
	caKey    string
	one      keyPair
	two      keyPair
	tsa      keyPair
}

var (
	keysOnce sync.Once
	keysVal  *fixtureKeys
	keysErr  error
)

// TestMain removes the fixture folder once every test of the package ran.
func TestMain(m *testing.M) {
	code := m.Run()
	if keysVal != nil {
		_ = os.RemoveAll(keysVal.dir)
	}
	if noVenomDir != "" {
		_ = os.RemoveAll(noVenomDir)
	}
	os.Exit(code)
}

// fixtureKeysFor makes the keys on first use, behind a sync.Once, so a missing openssl never stops
// a test that does not need them.
func fixtureKeysFor(t *testing.T) *fixtureKeys {
	t.Helper()
	requireOpenSSL3(t)
	keysOnce.Do(func() { keysVal, keysErr = makeFixtureKeys() })
	if keysErr != nil {
		t.Fatalf("fixture keys: %v", keysErr)
	}
	return keysVal
}

// opensslIn runs openssl under the fixture configuration and returns its standard output.
func opensslIn(conf string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "openssl", args...)
	cmd.Env = mergeEnv("OPENSSL_CONF=" + conf)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("openssl %s: %w: %s", args[0], err, out)
	}
	return string(out), nil
}

func makeFixtureKeys() (*fixtureKeys, error) {
	dir, err := os.MkdirTemp("", "skillcheck-keys-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	k := &fixtureKeys{dir: dir, confPath: slash(filepath.Join(dir, "fixture.cnf"))}
	k.caPath, k.caKey = slash(filepath.Join(dir, "ca.pem")), slash(filepath.Join(dir, "ca.key"))
	if err := os.WriteFile(k.confPath, []byte(opensslConfig), 0o600); err != nil {
		return nil, err
	}
	if err := k.makeCA(); err != nil {
		return nil, err
	}
	for _, e := range []struct {
		pair *keyPair
		name string
		ext  string
	}{{&k.one, "signer-one", "signer_ext"}, {&k.two, "signer-two", "signer_ext"}, {&k.tsa, "tsa", "tsa_ext"}} {
		if err := k.issue(e.pair, e.name, e.ext); err != nil {
			return nil, err
		}
	}
	if err := k.prepareTSA(); err != nil {
		return nil, err
	}
	return k, nil
}

func (k *fixtureKeys) makeCA() error {
	_, err := opensslIn(k.confPath, "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1",
		"-nodes", "-keyout", k.caKey, "-out", k.caPath, "-subj", "/CN=Skillcheck Test CA", "-days", "7",
		"-extensions", "v3_ca")
	return err
}

// issue creates a key and a certificate signed by the fixture CA, then reads its SHA-256 fingerprint.
func (k *fixtureKeys) issue(p *keyPair, name, ext string) error {
	p.certPath = slash(filepath.Join(k.dir, name+".pem"))
	p.keyPath = slash(filepath.Join(k.dir, name+".key"))
	csr := slash(filepath.Join(k.dir, name+".csr"))
	if _, err := opensslIn(k.confPath, "req", "-new", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1",
		"-nodes", "-keyout", p.keyPath, "-out", csr, "-subj", "/CN=Skillcheck "+name); err != nil {
		return err
	}
	if _, err := opensslIn(k.confPath, "x509", "-req", "-in", csr, "-CA", k.caPath, "-CAkey", k.caKey,
		"-CAcreateserial", "-out", p.certPath, "-days", "7", "-extfile", k.confPath, "-extensions", ext); err != nil {
		return err
	}
	for _, f := range []string{p.certPath, p.keyPath, csr} {
		if err := os.Chmod(f, 0o600); err != nil {
			return err
		}
	}
	fp, err := opensslIn(k.confPath, "x509", "-in", p.certPath, "-noout", "-fingerprint", "-sha256")
	if err != nil {
		return err
	}
	p.fingerprint = parseFingerprint(fp)
	return nil
}

// parseFingerprint turns "sha256 Fingerprint=AB:CD:..." into lowercase hex without colons.
func parseFingerprint(out string) string {
	_, value, _ := strings.Cut(strings.TrimSpace(out), "=")
	return strings.ToLower(strings.ReplaceAll(value, ":", ""))
}
