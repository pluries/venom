package ssh

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/ovh/venom"
)

// Tests for the ssh executor's host-key checks (#2015): the default, which checks nothing, and the two opt-ins,
// knownhosts and hostkey, against the in-process server of ssh_helpers_test.go.

// ran is the Result of the command "echo venom" on the fake server.
var ran = Result{Systemout: "ran: echo venom", Code: "0"}

// checkResult checks that res is ran when wantErr is empty, else a connection failure whose Err matches the regexp
// wantErr and nothing else.
func checkResult(t *testing.T, wantErr string, res Result) {
	t.Helper()
	if wantErr == "" {
		assert.Equal(t, ran, res)
		return
	}
	assert.Regexp(t, wantErr, res.Err)
	res.Err = ""
	assert.Equal(t, Result{}, res)
}

func TestRun_noHostKeyCheck(t *testing.T) {
	// Neither knownhosts nor hostkey: whatever key the server presents is accepted, as before #2015.
	f := newFixture(t)

	tests := []struct {
		name    string
		command string
		want    Result
	}{
		{name: "a command that succeeds answers code 0 and its output", command: "echo venom", want: ran},
		{name: "a failing command answers its exit code and stderr", command: "false", want: Result{Systemerr: "false: failed", Code: "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sshRun(context.Background(), t, f.step(tt.command, nil)))
		})
	}
}

func TestRun_knownHosts(t *testing.T) {
	f := newFixture(t)
	other := newEd25519Key(t)

	tests := []struct {
		name    string
		content []byte
		wantErr string
	}{
		{
			// The library's own preference is ECDSA: without asking for the listed type, the server presents its
			// ECDSA key and the check fails.
			name:    "the server's Ed25519 key alone connects",
			content: knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey()),
		},
		{name: "the server's ECDSA key alone connects", content: knownHostsFile(f.srv.addr, f.ecdsaKey.PublicKey())},
		{
			name:    "both server keys connect",
			content: knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey(), f.ecdsaKey.PublicKey()),
		},
		{
			name:    "another key for the host is a mismatch",
			content: knownHostsFile(f.srv.addr, other.PublicKey()),
			wantErr: `^ssh: handshake failed: knownhosts: key mismatch$`,
		},
		{
			name:    "a file without the host makes it unknown",
			content: knownHostsFile("192.0.2.1:22", f.ed25519Key.PublicKey()),
			wantErr: `^ssh: handshake failed: knownhosts: key is unknown$`,
		},
		{
			// A @cert-authority line's type is the authority's: asking for an ECDSA key would get the server's
			// ECDSA key, which no line lists.
			name: "an ECDSA authority before the host's Ed25519 key, without certificate, connects with that key",
			content: append(certAuthorityLine(f.srv.addr, newECDSAKey(t, elliptic.P256()).PublicKey()),
				knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey())...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), "known_hosts", tt.content)
			res := sshRun(context.Background(), t, f.step("echo venom", venom.TestStep{"knownhosts": path}))
			checkResult(t, tt.wantErr, res)
		})
	}
}

func TestRun_knownHostsCertAuthority(t *testing.T) {
	// The server presents a host certificate, its Ed25519 key signed by an ECDSA authority, beside its two plain keys.
	// A @cert-authority line lists the authority's key, of a type the server also holds as a plain key: asking for
	// that type, as for a host key, would get the plain key and a mismatch.
	ca := newECDSAKey(t, elliptic.P256())
	f := newCertFixture(t, ca)

	tests := []struct {
		name    string
		content []byte
		wantErr string
	}{
		{name: "a certificate signed by the listed authority connects", content: certAuthorityLine(f.srv.addr, ca.PublicKey())},
		{
			name:    "a certificate signed by another authority is refused",
			content: certAuthorityLine(f.srv.addr, newECDSAKey(t, elliptic.P256()).PublicKey()),
			wantErr: `^ssh: handshake failed: ssh: no authorities for hostname: ` + regexp.QuoteMeta(f.srv.addr) + `$`,
		},
		{
			name: "the authority, of another type, before the host's own key connects",
			content: append(certAuthorityLine(f.srv.addr, ca.PublicKey()),
				knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey())...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, t.TempDir(), "known_hosts", tt.content)
			res := sshRun(context.Background(), t, f.step("echo venom", venom.TestStep{"knownhosts": path}))
			checkResult(t, tt.wantErr, res)
		})
	}
}

func TestRun_knownHostsPath(t *testing.T) {
	f := newFixture(t)
	content := knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey())
	suiteDir := t.TempDir()
	writeFile(t, suiteDir, filepath.Join("keys", "known_hosts"), content)
	elsewhere := writeFile(t, t.TempDir(), "known_hosts", content)

	tests := []struct {
		name       string
		knownHosts string
		wantErr    string
	}{
		{name: "a relative path is read from the suite's workdir", knownHosts: filepath.Join("keys", "known_hosts")},
		{name: "an absolute path is kept as is", knownHosts: elsewhere},
		{
			name:       "a missing file is an error naming the resolved path",
			knownHosts: "missing_known_hosts",
			wantErr: `^knownhosts file: open ` + regexp.QuoteMeta(filepath.Join(suiteDir, "missing_known_hosts")) +
				`: no such file or directory$`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sshRun(workdirCtx(suiteDir), t, f.step("echo venom", venom.TestStep{"knownhosts": tt.knownHosts}))
			checkResult(t, tt.wantErr, res)
		})
	}
}

func TestRun_hostKey(t *testing.T) {
	f := newFixture(t)

	tests := []struct {
		name    string
		hostKey string
		wantErr string
	}{
		{
			// As with knownhosts: the library alone would negotiate the server's ECDSA key.
			name:    "the server's Ed25519 key connects",
			hostKey: authorizedKey(f.ed25519Key.PublicKey()),
		},
		{name: "the server's ECDSA key connects", hostKey: authorizedKey(f.ecdsaKey.PublicKey())},
		{name: "a trailing comment is allowed", hostKey: authorizedKey(f.ed25519Key.PublicKey()) + " root@venom-test"},
		{
			name:    "another key is a mismatch",
			hostKey: authorizedKey(newEd25519Key(t).PublicKey()),
			wantErr: `^ssh: handshake failed: ssh: host key mismatch$`,
		},
		{
			name:    "a key of a type the server does not hold cannot be negotiated",
			hostKey: authorizedKey(newECDSAKey(t, elliptic.P384()).PublicKey()),
			wantErr: `^ssh: handshake failed: ssh: no common algorithm for host key; we offered: \["ecdsa-sha2-nistp384"\], peer offered: `,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := sshRun(context.Background(), t, f.step("echo venom", venom.TestStep{"hostkey": tt.hostKey}))
			checkResult(t, tt.wantErr, res)
		})
	}
}

func TestRun_hostKeyRSA(t *testing.T) {
	// An RSA key's type is ssh-rsa, but a server that refuses SHA-1 signatures, as OpenSSH does by default since 8.8,
	// presents it only under the SHA-2 algorithm names: asking for ssh-rsa alone would find no common algorithm.
	keyPath, clientPub := writeClientKey(t)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(rsaKey)
	require.NoError(t, err)
	algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
	require.True(t, ok, "an RSA signer signs with a chosen algorithm")
	hostKey, err := ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512})
	require.NoError(t, err)
	srv := startServer(t, clientPub, hostKey)

	step := venom.TestStep{
		"host": srv.addr, "user": testUser, "privatekey": keyPath, "command": "echo venom",
		"hostkey": authorizedKey(hostKey.PublicKey()),
	}
	assert.Equal(t, ran, sshRun(context.Background(), t, step))
}

func TestRun_hostKeyStepErrors(t *testing.T) {
	// A host-key setting the executor cannot use fails the step in result.err, like an unreadable privatekey, before
	// any connection is made.
	f := newFixture(t)
	dir := t.TempDir()
	valid := writeFile(t, dir, "known_hosts", knownHostsFile(f.srv.addr, f.ed25519Key.PublicKey()))
	malformed := writeFile(t, dir, "malformed_known_hosts", []byte("127.0.0.1 ssh-ed25519 not-base64!\n"))

	tests := []struct {
		name    string
		extra   venom.TestStep
		wantErr string
	}{
		{
			name:    "knownhosts and hostkey together",
			extra:   venom.TestStep{"knownhosts": valid, "hostkey": authorizedKey(f.ed25519Key.PublicKey())},
			wantErr: `^knownhosts and hostkey are exclusive: set one of them, not both$`,
		},
		{
			name:    "a hostkey that is not a public key",
			extra:   venom.TestStep{"hostkey": "not a key"},
			wantErr: `^hostkey is not a public key in authorized_keys format \("ssh-ed25519 AAAA…"\): ssh: no key found`,
		},
		{
			name:    "a malformed known_hosts file names the file and the line",
			extra:   venom.TestStep{"knownhosts": malformed},
			wantErr: `^knownhosts file: knownhosts: ` + regexp.QuoteMeta(malformed) + `:1: `,
		},
		// A key set to nothing, such as a variable that expands to nothing, must not fall back to no check at all.
		{name: "an empty hostkey", extra: venom.TestStep{"hostkey": ""}, wantErr: `^hostkey is set but empty$`},
		{name: "a hostkey of blanks", extra: venom.TestStep{"hostkey": "  "}, wantErr: `^hostkey is set but empty$`},
		{name: "an empty knownhosts", extra: venom.TestStep{"knownhosts": ""}, wantErr: `^knownhosts is set but empty$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := f.srv.accepted.Load()
			res := sshRun(context.Background(), t, f.step("echo venom", tt.extra))
			checkResult(t, tt.wantErr, res)
			assert.Equal(t, before, f.srv.accepted.Load(), "no connection is made")
		})
	}
}

func TestRun_noCommand(t *testing.T) {
	got, err := Executor{}.Run(context.Background(), venom.TestStep{"host": "127.0.0.1:1", "hostkey": "not a key"})
	assert.Nil(t, got)
	assert.EqualError(t, err, "invalid command")
}

func TestRun_stepKeys(t *testing.T) {
	// Run decodes the step with mapstructure, which matches a key to the Go field name in any case: the documented
	// keys are the lowercased field names. A snake_case key is silently ignored.
	tests := []struct {
		name string
		step venom.TestStep
		want Executor
	}{
		{
			name: "documented keys",
			step: venom.TestStep{"knownhosts": "keys/known_hosts", "hostkey": "ssh-ed25519 AAAA"},
			want: Executor{KnownHosts: new("keys/known_hosts"), HostKey: new("ssh-ed25519 AAAA")},
		},
		{
			name: "field-name keys",
			step: venom.TestStep{"KnownHosts": "keys/known_hosts", "hostKey": "ssh-ed25519 AAAA"},
			want: Executor{KnownHosts: new("keys/known_hosts"), HostKey: new("ssh-ed25519 AAAA")},
		},
		{
			// What tells a key set to nothing from a key not set: the first is an error, the second no check.
			name: "a key set to nothing is decoded as set",
			step: venom.TestStep{"hostkey": ""},
			want: Executor{HostKey: new("")},
		},
		{
			name: "snake_case keys are not decoded",
			step: venom.TestStep{"known_hosts": "keys/known_hosts", "host_key": "ssh-ed25519 AAAA"},
			want: Executor{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Executor
			require.NoError(t, mapstructure.Decode(tt.step, &got))
			assert.Equal(t, tt.want, got)
		})
	}
}
