package sshconn

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const testAddr = "127.0.0.1:2222"

// Fixtures: keys are generated at test time, nothing is committed.

func newEd25519Key(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

func newECDSAKey(t *testing.T) ssh.Signer {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func knownHostsFile(addr string, keys ...ssh.PublicKey) []byte {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(knownhosts.Line([]string{addr}, k) + "\n")
	}
	return []byte(b.String())
}

func certAuthorityLine(addr string, ca ssh.PublicKey) []byte {
	return []byte("@cert-authority " + knownhosts.Line([]string{addr}, ca) + "\n")
}

// presents is the error the callback of check gives when the server at testAddr presents key, nil when it accepts it.
func presents(check HostKeyCheck, key ssh.PublicKey) error {
	return check.Callback(testAddr, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}, key)
}

// warnings returns a WarnFunc that collects the formats it is called with, after re-arming the once-per-run warning.
func warnings(t *testing.T) (WarnFunc, *[]string) {
	t.Helper()
	ResetInsecureWarning()
	t.Cleanup(ResetInsecureWarning)
	var got []string
	return func(format string, _ ...any) { got = append(got, format) }, &got
}

func TestNewHostKeyCheck_NeitherSetWarnsOnce(t *testing.T) {
	warn, got := warnings(t)
	for range 3 {
		check, err := NewHostKeyCheck(HostKeySettings{Host: testAddr}, warn)
		require.NoError(t, err)
		require.NoError(t, presents(check, newEd25519Key(t).PublicKey()), "the default check accepts any key")
		assert.Nil(t, check.Algorithms)
	}
	require.Len(t, *got, 1)
	assert.Contains(t, (*got)[0], "host key")
}

func TestNewHostKeyCheck_FixedHostKey(t *testing.T) {
	warn, got := warnings(t)
	key, other := newEd25519Key(t).PublicKey(), newEd25519Key(t).PublicKey()
	hk := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))

	check, err := NewHostKeyCheck(HostKeySettings{HostKey: &hk, Host: testAddr}, warn)
	require.NoError(t, err)
	require.NoError(t, presents(check, key), "the pinned key matches")
	require.Error(t, presents(check, other), "another key is refused")
	assert.Equal(t, []string{ssh.KeyAlgoED25519}, check.Algorithms)
	assert.Empty(t, *got, "a checked key does not warn")
}

func TestNewHostKeyCheck_KnownHosts(t *testing.T) {
	warn, got := warnings(t)
	key, other := newEd25519Key(t).PublicKey(), newEd25519Key(t).PublicKey()
	dir := t.TempDir()
	writeFile(t, dir, "known_hosts", knownHostsFile(testAddr, key))

	for name, path := range map[string]string{"relative to the workdir": "known_hosts", "absolute": filepath.Join(dir, "known_hosts")} {
		t.Run(name, func(t *testing.T) {
			check, err := NewHostKeyCheck(HostKeySettings{KnownHosts: &path, Workdir: dir, Host: testAddr}, warn)
			require.NoError(t, err)
			require.NoError(t, presents(check, key))
			require.Error(t, presents(check, other), "a key the file does not list is refused")
			assert.Equal(t, []string{ssh.KeyAlgoED25519}, check.Algorithms)
		})
	}
	assert.Empty(t, *got)
}

func TestNewHostKeyCheck_Errors(t *testing.T) {
	warn, got := warnings(t)
	empty, blanks, junk := "", "  ", "not a key"
	missing := filepath.Join(t.TempDir(), "absent")
	tests := []struct {
		name string
		s    HostKeySettings
		want string
	}{
		{"both set", HostKeySettings{KnownHosts: &junk, HostKey: &junk}, "exclusive"},
		{"empty knownhosts", HostKeySettings{KnownHosts: &empty}, "knownhosts is set but empty"},
		{"blank hostkey", HostKeySettings{HostKey: &blanks}, "hostkey is set but empty"},
		{"hostkey not a key", HostKeySettings{HostKey: &junk}, "hostkey is not a public key"},
		{"missing file", HostKeySettings{KnownHosts: &missing}, "knownhosts file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHostKeyCheck(tt.s, warn)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
	assert.Empty(t, *got, "an error is not the unchecked default")
}

func TestKnownKeyAlgorithms(t *testing.T) {
	const addr = testAddr
	ed1, ed2 := newEd25519Key(t).PublicKey(), newEd25519Key(t).PublicKey()
	ec := newECDSAKey(t).PublicKey()
	ca := newECDSAKey(t).PublicKey()
	// The library's host-certificate algorithms, in its order of preference: a certificate of any type fits a
	// @cert-authority line, whatever the authority's own type.
	certs := []string{
		ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSASHA512v01,
		ssh.CertAlgoECDSA256v01, ssh.CertAlgoECDSA384v01, ssh.CertAlgoECDSA521v01, ssh.CertAlgoED25519v01,
	}
	// lines joins known_hosts lines into one file.
	lines := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

	tests := []struct {
		name    string
		content []byte
		addr    string
		want    []string
	}{
		{name: "each type once, in file order", content: knownHostsFile(addr, ed1, ec, ed2), addr: addr, want: []string{ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256}},
		{name: "only the host's lines count", content: append(knownHostsFile("192.0.2.1:22", ec), knownHostsFile(addr, ed1)...), addr: addr, want: []string{ssh.KeyAlgoED25519}},
		{name: "an unknown host asks for nothing in particular", content: knownHostsFile("192.0.2.1:22", ed1), addr: addr, want: nil},
		{
			// An unbracketed IPv6 address has no port DialAddr can see: the callback cannot split it, nor can the dial.
			name: "an address the callback cannot split asks for nothing in particular", content: knownHostsFile(addr, ed1), addr: "::1", want: nil,
		},
		{name: "an authority asks for the certificate algorithms", content: certAuthorityLine(addr, ca), addr: addr, want: certs},
		{
			name:    "an authority counts where it stands in the file",
			content: lines(knownHostsFile(addr, ec), certAuthorityLine(addr, ca), knownHostsFile(addr, ed1)),
			addr:    addr,
			want:    append(append([]string{ssh.KeyAlgoECDSA256}, certs...), ssh.KeyAlgoED25519),
		},
		{
			// knownhosts numbers every line of the file: a miscount would read the authority's line as the next one.
			name:    "comments and blank lines do not shift the lines",
			content: lines([]byte("# venom test\n\n"), certAuthorityLine(addr, ca), knownHostsFile(addr, ed1)),
			addr:    addr,
			want:    append(slices.Clone(certs), ssh.KeyAlgoED25519),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback, err := knownhosts.New(writeFile(t, t.TempDir(), "known_hosts", tt.content))
			require.NoError(t, err)
			assert.Equal(t, tt.want, knownKeyAlgorithms(callback, tt.addr, strings.Split(string(tt.content), "\n")))
		})
	}
}

func TestLineAlgorithms(t *testing.T) {
	// The file changed between the two reads: a line knownhosts numbers past the end of the executor's copy is taken
	// for a host key rather than read out of range.
	key := newEd25519Key(t).PublicKey()
	assert.Equal(t, []string{ssh.KeyAlgoED25519}, lineAlgorithms(nil, knownhosts.KnownKey{Key: key, Line: 1}))
}

func TestDialAddr(t *testing.T) {
	assert.Equal(t, "venom.test:22", DialAddr("venom.test"), "no port: the default one")
	assert.Equal(t, "127.0.0.1:2222", DialAddr("127.0.0.1:2222"), "a port is kept")
}

func TestKeyAlgorithms(t *testing.T) {
	assert.Equal(t, []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA}, keyAlgorithms(ssh.KeyAlgoRSA))
	assert.Equal(t, []string{ssh.KeyAlgoED25519}, keyAlgorithms(ssh.KeyAlgoED25519))
	assert.Equal(t, []string{ssh.KeyAlgoECDSA256}, keyAlgorithms(ssh.KeyAlgoECDSA256))
}

// writeKey writes a fresh Ed25519 private key, protected by passphrase when it is not empty, and returns its path and
// the public key.
func writeKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "venom test")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "venom test", []byte(passphrase))
	}
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return writeFile(t, t.TempDir(), "id_ed25519", pem.EncodeToMemory(block)), sshPub
}

func TestPrivateKey(t *testing.T) {
	const passphrase = "correct horse"
	plain, plainPub := writeKey(t, "")
	locked, lockedPub := writeKey(t, passphrase)

	t.Run("a plain key loads", func(t *testing.T) {
		key, err := PrivateKey(plain, "")
		require.NoError(t, err)
		assert.Equal(t, plainPub.Marshal(), key.PublicKey().Marshal())
	})
	t.Run("a plain key loads with a passphrase too", func(t *testing.T) {
		key, err := PrivateKey(plain, passphrase)
		require.NoError(t, err)
		assert.Equal(t, plainPub.Marshal(), key.PublicKey().Marshal())
	})
	t.Run("the right passphrase unlocks the key", func(t *testing.T) {
		key, err := PrivateKey(locked, passphrase)
		require.NoError(t, err)
		assert.Equal(t, lockedPub.Marshal(), key.PublicKey().Marshal())
	})
	t.Run("the wrong passphrase is refused without echoing it", func(t *testing.T) {
		_, err := PrivateKey(locked, "wrong horse")
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "wrong horse")
		assert.NotContains(t, err.Error(), passphrase)
		assert.NotContains(t, err.Error(), locked)
	})
	t.Run("a missing passphrase is told apart", func(t *testing.T) {
		_, err := PrivateKey(locked, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "passphrase")
		assert.NotContains(t, err.Error(), passphrase)
	})
	t.Run("a file that is not a key carries no content in the error", func(t *testing.T) {
		junk := writeFile(t, t.TempDir(), "junk", []byte("secret-looking-content"))
		for _, p := range []string{"", passphrase} {
			_, err := PrivateKey(junk, p)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret-looking-content")
			assert.NotContains(t, err.Error(), passphrase)
		}
	})
	t.Run("an unreadable file is an error", func(t *testing.T) {
		_, err := PrivateKey(filepath.Join(t.TempDir(), "absent"), "")
		require.Error(t, err)
	})
	t.Run("the environment is expanded in the path", func(t *testing.T) {
		t.Setenv("VENOM_TEST_KEY", plain)
		_, err := PrivateKey("$VENOM_TEST_KEY", "")
		require.NoError(t, err)
	})
}
