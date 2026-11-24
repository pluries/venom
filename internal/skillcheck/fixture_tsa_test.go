package skillcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// tsaConfigFormat is the configuration of the local timestamp authority: its own serial file and the
// certificate with the time-stamping usage made by fixtureKeys.
const tsaConfigFormat = `[tsa]
default_tsa = tsa_config
[tsa_config]
dir = %[1]s
serial = %[1]s/tsa.serial
crypto_device = builtin
signer_cert = %[2]s
certs = %[3]s
signer_key = %[4]s
signer_digest = sha256
default_policy = 1.2.3.4.1
other_policies = 1.2.3.4.5
digests = sha256, sha384, sha512
accuracy = secs:1
ordering = yes
tsa_name = yes
ess_cert_id_chain = no
ess_cert_id_alg = sha256
`

func (k *fixtureKeys) tsaConfPath() string { return slash(filepath.Join(k.dir, "tsa.cnf")) }

// prepareTSA writes the authority's configuration and serial file.
func (k *fixtureKeys) prepareTSA() error {
	conf := fmt.Sprintf(tsaConfigFormat, slash(k.dir), k.tsa.certPath, k.caPath, k.tsa.keyPath)
	if err := os.WriteFile(k.tsaConfPath(), []byte(conf), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(k.dir, "tsa.serial"), []byte("01\n"), 0o600)
}

// timestamp asks the local authority for a token over the file and writes it to out.
func (k *fixtureKeys) timestamp(data, out string) error {
	query := slash(filepath.Join(k.dir, "request.tsq"))
	if _, err := opensslIn(k.confPath, "ts", "-query", "-data", data, "-sha256", "-cert", "-out", query); err != nil {
		return err
	}
	_, err := opensslIn(k.confPath, "ts", "-reply", "-config", k.tsaConfPath(), "-queryfile", query,
		"-signer", k.tsa.certPath, "-inkey", k.tsa.keyPath, "-out", out)
	return err
}

// timestampFile fails the test when the authority cannot answer.
func timestampFile(t *testing.T, k *fixtureKeys, data, out string) {
	t.Helper()
	if err := k.timestamp(data, out); err != nil {
		t.Fatalf("local timestamp authority: %v", err)
	}
}
