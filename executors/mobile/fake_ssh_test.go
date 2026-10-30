package mobile

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// An in-process SSH server on loopback, for the tunnel tests: a copy of the ssh executor's fixture (that one is in
// package ssh, not importable), extended with direct-tcpip. Keys are generated at test time, nothing is committed.

// sshTestUser is the only user the fake SSH server lets in.
const sshTestUser = "venom"

// fakeSSH is a running SSH server. It records the direct-tcpip targets it was asked to dial and the exec commands.
type fakeSSH struct {
	addr     string
	hostKey  ssh.PublicKey
	keyPath  string // the private key the server accepts, in OpenSSH format
	accepted atomic.Int32

	mu      sync.Mutex
	dials   []string
	execs   []string
	dirs    map[string]bool
	files   map[string][]byte
	rmFails bool
	rmHangs atomic.Bool
}

// startFakeSSH serves SSH on loopback until the test ends.
func startFakeSSH(t *testing.T) *fakeSSH {
	t.Helper()
	return startFakeSSHWith(t, "")
}

// startFakeSSHWith is startFakeSSH with a client key encrypted with phrase, when one is given.
func startFakeSSHWith(t *testing.T, phrase string) *fakeSSH {
	t.Helper()
	hostSigner, _ := newSSHKey(t)
	keyPath, clientPub := writeSSHClientKey(t, phrase)
	f := &fakeSSH{hostKey: hostSigner.PublicKey(), keyPath: keyPath}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if conn.User() == sshTestUser && bytes.Equal(key.Marshal(), clientPub.Marshal()) {
			return &ssh.Permissions{}, nil
		}
		return nil, fmt.Errorf("key refused for %q", conn.User())
	}}
	cfg.AddHostKey(hostSigner)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f.addr = lis.Addr().String()
	t.Cleanup(f.serve(lis, cfg))
	return f
}

// serve accepts connections until the returned stop function runs; stop waits for every goroutine.
func (f *fakeSSH) serve(lis net.Listener, cfg *ssh.ServerConfig) func() {
	var (
		mu    sync.Mutex
		conns []net.Conn
		wg    sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			f.accepted.Add(1)
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				f.serveConn(c, cfg)
			}()
		}
	}()
	return func() {
		_ = lis.Close()
		mu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		mu.Unlock()
		wg.Wait()
	}
}

// serveConn runs the SSH handshake on c and serves its channels until the client hangs up.
func (f *fakeSSH) serveConn(c net.Conn, cfg *ssh.ServerConfig) {
	defer c.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return // the client refused the host key, or the server refused the client
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ssh.DiscardRequests(reqs)
	}()
	for newCh := range chans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.serveChannel(newCh)
		}()
	}
	_ = sconn.Close()
	wg.Wait()
}

// serveChannel answers one channel: direct-tcpip is forwarded, session runs the exec handler, the rest is refused.
func (f *fakeSSH) serveChannel(newCh ssh.NewChannel) {
	switch newCh.ChannelType() {
	case "direct-tcpip":
		f.forward(newCh)
	case "session":
		f.session(newCh)
	default:
		ignoreErr(newCh.Reject(ssh.UnknownChannelType, "unsupported channel"))
	}
}

// forward dials the target of a direct-tcpip request, as the Mac's sshd does for the tunnel, and relays both ways.
func (f *fakeSSH) forward(newCh ssh.NewChannel) {
	var p struct {
		DestAddr string
		DestPort uint32
		OrigAddr string
		OrigPort uint32
	}
	if err := ssh.Unmarshal(newCh.ExtraData(), &p); err != nil {
		ignoreErr(newCh.Reject(ssh.ConnectionFailed, "bad request"))
		return
	}
	target := net.JoinHostPort(p.DestAddr, fmt.Sprint(p.DestPort))
	f.mu.Lock()
	f.dials = append(f.dials, target)
	f.mu.Unlock()
	conn, err := net.Dial("tcp", target)
	if err != nil {
		ignoreErr(newCh.Reject(ssh.ConnectionFailed, err.Error()))
		return
	}
	ch, reqs, err := newCh.Accept()
	if err != nil {
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	relay(ch, conn)
}

// session records the exec command of a session channel, emulates it in memory and answers with its exit status.
func (f *fakeSSH) session(newCh ssh.NewChannel) {
	ch, reqs, err := newCh.Accept()
	if err != nil {
		return
	}
	defer ch.Close()
	for req := range reqs {
		if req.Type != "exec" {
			ignoreErr(req.Reply(false, nil))
			continue
		}
		var payload struct{ Command string }
		if ssh.Unmarshal(req.Payload, &payload) != nil {
			ignoreErr(req.Reply(false, nil))
			return
		}
		f.mu.Lock()
		f.execs = append(f.execs, payload.Command)
		f.mu.Unlock()
		ignoreErr(req.Reply(true, nil))
		status := f.emulate(ch, payload.Command)
		ignoreErr2(ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status})))
		return
	}
}

// The commands the mobile executor sends, and nothing else is emulated: nothing runs for real, files live in memory.
var (
	mkdirCommand = regexp.MustCompile(`^mkdir -m 700 (/tmp/venom-mobile-[0-9a-f]{16})$`)
	catCommand   = regexp.MustCompile(`^cat > (/tmp/venom-mobile-[0-9a-f]{16}/app\.(?:apk|zip))$`)
	rmCommand    = regexp.MustCompile(`^rm -rf (/tmp/venom-mobile-[0-9a-f]{16})$`)
)

// emulate runs one command against the in-memory file system and returns its exit status. A command it does not know
// fails with 127, so a test notices a command the executor was not meant to send.
func (f *fakeSSH) emulate(ch ssh.Channel, cmd string) uint32 {
	if m := catCommand.FindStringSubmatch(cmd); m != nil {
		data, err := io.ReadAll(ch) // outside the lock: the stream can be long
		if err != nil {
			return 1
		}
		return f.store(m[1], data)
	}
	if rmCommand.MatchString(cmd) && f.rmHangs.Load() {
		_, _ = io.Copy(io.Discard, ch) //nolint:errcheck // stalls until the client closes the channel; the error says nothing more
		return 1
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dirs == nil {
		f.dirs, f.files = map[string]bool{}, map[string][]byte{}
	}
	if m := mkdirCommand.FindStringSubmatch(cmd); m != nil {
		f.dirs[m[1]] = true
		return 0
	}
	if m := rmCommand.FindStringSubmatch(cmd); m != nil {
		return f.remove(m[1])
	}
	return 127
}

// store keeps data at path, which fails when the folder does not exist.
func (f *fakeSSH) store(path string, data []byte) uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.dirs[path[:strings.LastIndex(path, "/")]] {
		return 1
	}
	f.files[path] = data
	return 0
}

// remove deletes a folder and its files. It fails when rmFails is set. The caller holds f.mu.
func (f *fakeSSH) remove(dir string) uint32 {
	if f.rmFails {
		return 1
	}
	delete(f.dirs, dir)
	for path := range f.files {
		if strings.HasPrefix(path, dir+"/") {
			delete(f.files, path)
		}
	}
	return 0
}

// commands is the exec commands received so far.
func (f *fakeSSH) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.execs...)
}

// storedFiles is a copy of the files the emulated host holds, by path.
func (f *fakeSSH) storedFiles() map[string][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string][]byte{}
	for p, b := range f.files {
		out[p] = b
	}
	return out
}

// folders is the folders the emulated host holds.
func (f *fakeSSH) folders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for d := range f.dirs {
		out = append(out, d)
	}
	return out
}

// failRemoval makes every later rm -rf fail.
func (f *fakeSSH) failRemoval() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rmFails = true
}

// dialed is the direct-tcpip targets received so far.
func (f *fakeSSH) dialed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.dials...)
}

// newSSHKey returns a fresh Ed25519 key as an SSH signer and as a raw private key.
func newSSHKey(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return signer, priv
}

// writeSSHClientKey writes a fresh private key into a temporary directory, encrypted with passphrase when one is
// given, and returns its path and the public key the server is to accept.
func writeSSHClientKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	signer, priv := newSSHKey(t)
	var block *pem.Block
	var err error
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "venom test client")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "venom test client", []byte(passphrase))
	}
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))
	return path, signer.PublicKey()
}

// authorizedKey is key in authorized_keys format, the form of mobile.ssh.hostkey.
func authorizedKey(key ssh.PublicKey) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
}

// ignoreErr drops the error of a fake server's reply: a client that hung up is not the test's concern.
func ignoreErr(error) {}

// ignoreErr2 is ignoreErr for a call that also returns a bool.
func ignoreErr2(bool, error) {}
