package mobile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ovh/venom/executors/internal/sshconn"
)

// tunnel forwards a local loopback port to the automation server over SSH, so the server can listen on its host's
// loopback only (FR-03). Every goroutine it starts has an owner: the accept loop ends when the listener closes, a
// forwarded connection ends when either side closes, and close waits for all of them.
type tunnel struct {
	client  *ssh.Client
	lis     net.Listener
	target  string // host:port the SSH server dials, on its own side
	baseURL string // the server's URL with the local end of the tunnel as its host

	wg     sync.WaitGroup
	once   sync.Once
	closed error
}

// openTunnel connects to the SSH server and listens on a free loopback port. serverURL is mobile.url: the tunnel
// reaches that port on the SSH server's loopback and the base URL keeps its scheme and path. The connection and the
// handshake are bounded by timeout and by ctx: a host that accepts and never answers is an error, not a hang.
func openTunnel(ctx context.Context, s *sshSettings, serverURL string, timeout time.Duration) (*tunnel, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("mobile.url: %w", err)
	}
	signer, err := sshconn.PrivateKey(s.key, s.passphrase)
	if err != nil {
		return nil, fmt.Errorf("cannot load the private key of mobile.ssh.key: %w", err)
	}
	client, err := sshconn.DialContext(ctx, s.addr, s.user, []ssh.AuthMethod{ssh.PublicKeys(signer)}, s.check, timeout)
	if err != nil {
		return nil, fmt.Errorf("cannot open the SSH connection to %s: %w", s.addr, err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("cannot listen for the SSH tunnel: %w", err), client.Close())
	}
	t := &tunnel{client: client, lis: lis, target: net.JoinHostPort("127.0.0.1", portOf(u))}
	local := *u
	local.Host = lis.Addr().String()
	t.baseURL = local.String()
	t.wg.Add(1)
	go t.accept()
	return t, nil
}

// portOf is the port of a URL, the scheme's default when it has none.
func portOf(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// accept hands each local connection to forward, until the listener closes.
func (t *tunnel) accept() {
	defer t.wg.Done()
	for {
		conn, err := t.lis.Accept()
		if err != nil {
			return
		}
		t.wg.Add(1) // safe: the accept loop still holds its own count
		go t.forward(conn)
	}
}

// forward joins one local connection to the server's port through a direct-tcpip channel.
func (t *tunnel) forward(local net.Conn) {
	defer t.wg.Done()
	remote, err := t.client.Dial("tcp", t.target)
	if err != nil {
		_ = local.Close()
		return
	}
	relay(local, remote)
}

// relay copies both ways until either side closes, then closes both. It returns when both copies are done.
func relay(a, b io.ReadWriteCloser) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		copyThenClose(b, a)
	}()
	copyThenClose(a, b)
	<-done
}

func copyThenClose(dst, src io.ReadWriteCloser) {
	_, _ = io.Copy(dst, src) //nolint:errcheck // a closed side ends the copy; the error says nothing the close does not
	_ = dst.Close()
	_ = src.Close()
}

// close stops the tunnel: no new connection, the SSH connection and with it every forwarded one, then it waits for
// the goroutines. It is idempotent and returns the first call's result.
func (t *tunnel) close() error {
	t.once.Do(func() {
		var errs []error
		if err := t.lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
		if err := t.client.Close(); err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
			errs = append(errs, err)
		}
		t.wg.Wait()
		t.closed = errors.Join(errs...)
	})
	return t.closed
}

// closers are the things a session opened besides the device session, closed in reverse order of opening (S6).
type closers []func() error

// run closes everything, last opened first, and reports every failure. The list is emptied: a second run is a no-op.
func (c *closers) run() error {
	var errs []error
	for i := len(*c) - 1; i >= 0; i-- {
		if err := (*c)[i](); err != nil {
			errs = append(errs, err)
		}
	}
	*c = nil
	return errors.Join(errs...)
}
