package ssh

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ovh/venom"
)

// Test fixtures for ssh_test.go: an in-process SSH server on loopback and keys generated at test time (nothing is
// committed).

// testUser is the only user the fake server lets in, with the client key of its fixture.
const testUser = "venom"

// loopback is the host the fake server listens on, and the one principal of the host certificates it presents.
const loopback = "127.0.0.1"

// fakeServer is a running SSH server and the number of TCP connections it accepted.
type fakeServer struct {
	addr     string
	accepted atomic.Int32
}

// startServer serves SSH on loopback until the test ends. It presents hostKeys (one per algorithm) and lets testUser
// in with clientKey. An exec request of the command "false" writes "false: failed" to stderr and exits 1; any other
// command writes "ran: <command>" to stdout and exits 0.
func startServer(t *testing.T, clientKey ssh.PublicKey, hostKeys ...ssh.Signer) *fakeServer {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == testUser && bytes.Equal(key.Marshal(), clientKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("key refused for %q", conn.User())
		},
	}
	for _, k := range hostKeys {
		cfg.AddHostKey(k)
	}
	lis, err := net.Listen("tcp", net.JoinHostPort(loopback, "0"))
	require.NoError(t, err)
	f := &fakeServer{addr: lis.Addr().String()}

	var (
		mu    sync.Mutex
		conns []net.Conn
		serve sync.WaitGroup
	)
	accepting := make(chan struct{})
	go func() {
		defer close(accepting)
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			f.accepted.Add(1)
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			serve.Add(1)
			go func() {
				defer serve.Done()
				serveConn(c, cfg)
			}()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-accepting
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
		serve.Wait()
	})
	return f
}

// serveConn runs the SSH handshake on c and serves its session channels until the client hangs up.
func serveConn(c net.Conn, cfg *ssh.ServerConfig) {
	defer c.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return // the client refused the host key, or the server refused the client
	}
	discarded := make(chan struct{})
	go func() {
		defer close(discarded)
		ssh.DiscardRequests(reqs)
	}()
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			if err := newCh.Reject(ssh.UnknownChannelType, "sessions only"); err != nil {
				break
			}
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			break
		}
		serveSession(ch, chReqs)
	}
	sconn.Close()
	<-discarded
}

// serveSession answers the session's exec request (see startServer), then closes the channel and drains the requests
// until the client closes its side.
func serveSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			if err := req.Reply(false, nil); err != nil {
				return
			}
			continue
		}
		if err := answerExec(ch, req); err != nil {
			return
		}
		ch.Close()
	}
}

// answerExec runs the fake command of an exec request on ch: its output, then its exit status (see startServer).
func answerExec(ch ssh.Channel, req *ssh.Request) error {
	var payload struct{ Command string }
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		return err
	}
	if err := req.Reply(true, nil); err != nil {
		return err
	}
	status := uint32(0)
	if payload.Command == "false" {
		status = 1
		fmt.Fprint(ch.Stderr(), "false: failed\n")
	} else {
		fmt.Fprintf(ch, "ran: %s\n", payload.Command)
	}
	_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
	return err
}

// newEd25519Key returns a fresh Ed25519 key as an SSH signer.
func newEd25519Key(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

// newECDSAKey returns a fresh ECDSA key on curve as an SSH signer.
func newECDSAKey(t *testing.T, curve elliptic.Curve) ssh.Signer {
	t.Helper()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer
}

// hostCert returns key presented as a host certificate for loopback, signed by ca and valid forever.
func hostCert(t *testing.T, ca, key ssh.Signer) ssh.Signer {
	t.Helper()
	cert := &ssh.Certificate{
		Key:             key.PublicKey(),
		CertType:        ssh.HostCert,
		KeyId:           "venom test host",
		ValidPrincipals: []string{loopback},
		ValidBefore:     ssh.CertTimeInfinity,
	}
	require.NoError(t, cert.SignCert(rand.Reader, ca))
	signer, err := ssh.NewCertSigner(cert, key)
	require.NoError(t, err)
	return signer
}

// writeClientKey writes a fresh Ed25519 private key, in OpenSSH format, into a temporary directory. It returns the
// file's path, for the step's privatekey, and the public key the server is to accept.
func writeClientKey(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(priv, "venom test client")
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return writeFile(t, t.TempDir(), "id_ed25519", pem.EncodeToMemory(block)), signer.PublicKey()
}

// writeFile writes content to dir/name and returns the path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// knownHostsFile is a known_hosts file listing keys for addr.
func knownHostsFile(addr string, keys ...ssh.PublicKey) []byte {
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(knownhosts.Line([]string{addr}, k) + "\n")
	}
	return []byte(b.String())
}

// certAuthorityLine is a known_hosts line trusting ca to sign the host certificates of addr.
func certAuthorityLine(addr string, ca ssh.PublicKey) []byte {
	return []byte("@cert-authority " + knownhosts.Line([]string{addr}, ca) + "\n")
}

// authorizedKey is key in authorized_keys format, the form of the step's hostkey.
func authorizedKey(key ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

// workdirCtx is the context a step gets inside a suite whose workdir is dir.
func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// sshRun runs one step, which must yield a Result; the Result's timing is checked, then zeroed so a whole struct can
// be compared.
func sshRun(ctx context.Context, t *testing.T, step venom.TestStep) Result {
	venom.InitTestLogger(t) // the no-host-key warning logs through venom's package logger, nil until this is called.
	t.Helper()
	got, err := Executor{}.Run(ctx, step)
	require.NoError(t, err)
	res, ok := got.(Result)
	require.Truef(t, ok, "want a Result, got %T: %#v", got, got)
	assert.Greater(t, res.TimeSeconds, 0.0)
	res.TimeSeconds = 0
	return res
}

// fixture is a fake server holding an ECDSA P-256 and an Ed25519 host key, as a stock OpenSSH server does, and the
// client key it accepts.
type fixture struct {
	srv        *fakeServer
	ecdsaKey   ssh.Signer
	ed25519Key ssh.Signer
	clientKey  string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f, clientPub := newFixtureKeys(t)
	f.srv = startServer(t, clientPub, f.ecdsaKey, f.ed25519Key)
	return f
}

// newCertFixture is a fixture whose server also presents a host certificate, its Ed25519 key signed by ca, as a host
// of an SSH certificate authority does.
func newCertFixture(t *testing.T, ca ssh.Signer) fixture {
	t.Helper()
	f, clientPub := newFixtureKeys(t)
	f.srv = startServer(t, clientPub, f.ecdsaKey, f.ed25519Key, hostCert(t, ca, f.ed25519Key))
	return f
}

// newFixtureKeys is a fixture's keys, its server not started yet, and the public client key the server is to accept.
func newFixtureKeys(t *testing.T) (fixture, ssh.PublicKey) {
	t.Helper()
	keyPath, clientPub := writeClientKey(t)
	return fixture{ecdsaKey: newECDSAKey(t, elliptic.P256()), ed25519Key: newEd25519Key(t), clientKey: keyPath}, clientPub
}

// step runs command on the fixture's server as testUser, extra keys merged in.
func (f fixture) step(command string, extra venom.TestStep) venom.TestStep {
	step := venom.TestStep{"host": f.srv.addr, "user": testUser, "privatekey": f.clientKey, "command": command}
	for k, v := range extra {
		step[k] = v
	}
	return step
}
