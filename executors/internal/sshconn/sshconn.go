// Package sshconn is the one place an executor turns its SSH settings into a connection: how the server's host key is
// checked, how a private key is loaded, how the server is dialled (#2397). It takes plain values, not an executor, so
// the ssh executor and the mobile executor's tunnel share it and cannot drift apart.
package sshconn

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// WarnFunc reports a warning; it has the shape of a printf, as venom.Warn bound to a context.
type WarnFunc func(format string, args ...any)

// insecureWarning limits the host key warning to one per run (#2298).
var insecureWarning sync.Once

// ResetInsecureWarning re-arms the warning, for tests only.
func ResetInsecureWarning() { insecureWarning = sync.Once{} }

// HostKeyCheck is how the client authenticates the server. Callback checks the key the server presents; Algorithms,
// when set, asks the server for a key of a type the callback knows. Without them the library prefers ECDSA to
// Ed25519, so a server holding both presents its ECDSA key and a check that only knows the Ed25519 one fails.
type HostKeyCheck struct {
	Callback   ssh.HostKeyCallback
	Algorithms []string
}

// HostKeySettings are the values a host key check is built from. KnownHosts and HostKey are pointers so that a value
// set to nothing is told from one not set.
type HostKeySettings struct {
	// KnownHosts is the path of an OpenSSH known_hosts file; relative to Workdir.
	KnownHosts *string
	// HostKey is the one public key, in authorized_keys format, the host must present. Exclusive with KnownHosts.
	HostKey *string
	// Workdir is the folder a relative KnownHosts path is read from.
	Workdir string
	// Host is the server, with or without a port.
	Host string
	// WarnOnce limits the warning of an unchecked host key to one per run; nil is the package's own, shared by the
	// callers that name none. A caller whose settings have other names (the mobile tunnel) passes its own, so that
	// its warning is neither lost to nor repeated by another caller's.
	WarnOnce *sync.Once
	// WarnMessage is the warning text; empty is DefaultInsecureMessage.
	WarnMessage string
}

// DefaultInsecureMessage is the warning of an unchecked host key for the ssh executor's settings.
const DefaultInsecureMessage = "ssh: the server's host key is not checked (set knownhosts or hostkey on the step to check it)"

// NewHostKeyCheck returns the check the settings ask for: the keys a known_hosts file lists for the host (KnownHosts),
// or one pinned key (HostKey). Settings that set neither are not checked at all, and warn says so, once per run. A key
// set to blanks, such as a variable that expands to nothing, is an error: taken as unset, it would turn the check off
// without a word.
func NewHostKeyCheck(s HostKeySettings, warn WarnFunc) (HostKeyCheck, error) {
	switch {
	case s.KnownHosts != nil && s.HostKey != nil:
		return HostKeyCheck{}, errors.New("knownhosts and hostkey are exclusive: set one of them, not both")
	case s.KnownHosts != nil && strings.TrimSpace(*s.KnownHosts) == "":
		return HostKeyCheck{}, errors.New("knownhosts is set but empty")
	case s.HostKey != nil && strings.TrimSpace(*s.HostKey) == "":
		return HostKeyCheck{}, errors.New("hostkey is set but empty")
	case s.KnownHosts != nil:
		path := *s.KnownHosts
		if !filepath.IsAbs(path) {
			path = filepath.Join(s.Workdir, path)
		}
		return knownHostsCheck(path, DialAddr(s.Host))
	case s.HostKey != nil:
		return fixedHostKeyCheck(*s.HostKey)
	}
	once, message := s.WarnOnce, s.WarnMessage
	if once == nil {
		once = &insecureWarning
	}
	if message == "" {
		message = DefaultInsecureMessage
	}
	once.Do(func() { warn(message) })
	//nolint:gosec // G106: reached only when the suite sets neither knownhosts nor hostkey — the default kept for ephemeral test hosts whose key changes every run
	return HostKeyCheck{Callback: ssh.InsecureIgnoreHostKey()}, nil
}

// knownHostsCheck checks the server's key against the OpenSSH known_hosts file at path, asking the server at addr for
// a key of a type the file lists for it. The callback verifies host certificates signed by a @cert-authority of the
// file too.
func knownHostsCheck(path, addr string) (HostKeyCheck, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return HostKeyCheck{}, fmt.Errorf("knownhosts file: %w", err)
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return HostKeyCheck{}, fmt.Errorf("knownhosts file: %w", err)
	}
	lines := strings.Split(string(content), "\n")
	return HostKeyCheck{Callback: callback, Algorithms: knownKeyAlgorithms(callback, addr, lines)}, nil
}

// knownKeyAlgorithms returns the host-key algorithms of the keys a known_hosts callback lists for addr, nil when it
// lists none; lines are the file's, to tell a certificate authority from a host key. The knownhosts package has no
// lookup, but a key no file can list (all zeros) makes the callback answer with a *knownhosts.KeyError whose Want holds
// every key it has for the host, certificate authorities included, each with its line.
func knownKeyAlgorithms(callback ssh.HostKeyCallback, addr string, lines []string) []string {
	probe, _ := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize))) //nolint:errcheck // an Ed25519 key of the right size always converts
	var keyErr *knownhosts.KeyError
	if !errors.As(callback(addr, &net.TCPAddr{}, probe), &keyErr) {
		return nil
	}
	var algorithms []string
	for _, known := range keyErr.Want {
		for _, algorithm := range lineAlgorithms(lines, known) {
			if !slices.Contains(algorithms, algorithm) {
				algorithms = append(algorithms, algorithm)
			}
		}
	}
	return algorithms
}

// lineAlgorithms returns the host-key algorithms under which a server presents a key the known_hosts line of known
// lets in. A @cert-authority line holds the key of the authority that signs host certificates: its type says nothing
// of the host's own key, so any certificate algorithm fits. known.Line counts every line of the file, blank lines and
// comments too. A line past the end of lines (the file changed between the two reads) counts as a host key: the
// algorithms only steer the negotiation, the callback still decides.
func lineAlgorithms(lines []string, known knownhosts.KnownKey) []string {
	if known.Line <= len(lines) {
		if fields := strings.Fields(lines[known.Line-1]); len(fields) > 0 && fields[0] == "@cert-authority" {
			return certAlgorithms()
		}
	}
	return keyAlgorithms(known.Key.Type())
}

// certAlgorithms returns the host-certificate algorithms the library supports, in its order of preference.
func certAlgorithms() []string {
	return slices.DeleteFunc(ssh.SupportedAlgorithms().HostKeys, func(algorithm string) bool {
		return !strings.HasSuffix(algorithm, "-cert-v01@openssh.com")
	})
}

// fixedHostKeyCheck checks the server's key against the one public key hostKey holds, in authorized_keys format.
func fixedHostKeyCheck(hostKey string) (HostKeyCheck, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	if err != nil {
		return HostKeyCheck{}, fmt.Errorf("hostkey is not a public key in authorized_keys format (\"ssh-ed25519 AAAA…\"): %w", err)
	}
	return HostKeyCheck{Callback: ssh.FixedHostKey(key), Algorithms: keyAlgorithms(key.Type())}, nil
}

// keyAlgorithms returns the host-key algorithms under which a server presents a key of type keyType: an RSA key is
// also signed with SHA-2, in the library's order of preference.
func keyAlgorithms(keyType string) []string {
	if keyType == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA}
	}
	return []string{keyType}
}

// DialAddr is host with the default port when it has none.
func DialAddr(host string) string {
	if !strings.Contains(host, ":") {
		return host + ":22"
	}
	return host
}

// Dial opens the SSH connection to host as u, authenticating with auth and checking the server's key with check.
func Dial(host, u string, auth []ssh.AuthMethod, check HostKeyCheck) (*ssh.Client, error) {
	return ssh.Dial("tcp", DialAddr(host), clientConfig(u, auth, check))
}

func clientConfig(u string, auth []ssh.AuthMethod, check HostKeyCheck) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:              u,
		Auth:              auth,
		HostKeyCallback:   check.Callback,
		HostKeyAlgorithms: check.Algorithms,
	}
}

// DialContext is Dial bounded: the TCP connect and the SSH handshake together must end within timeout (none when it is
// not positive), and cancelling ctx during either aborts it. A server that accepts the connection and never answers
// ends as a timeout error instead of a hang. The connection carries no deadline once it is open.
func DialContext(ctx context.Context, host, u string, auth []ssh.AuthMethod, check HostKeyCheck, timeout time.Duration) (*ssh.Client, error) {
	addr := DialAddr(host)
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := handshake(ctx, conn, addr, clientConfig(u, auth, check), timeout)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

// handshake runs the SSH handshake on conn under the deadline and the context. It leaves conn open; the caller closes
// it on error.
func handshake(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig, timeout time.Duration) (*ssh.Client, error) {
	if timeout > 0 {
		if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		stop()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	if !stop() {
		_ = c.Close()
		return nil, ctx.Err()
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = c.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// PrivateKey loads the private key in file, decrypted with passphrase when one is given. An empty file is
// $HOME/.ssh/id_rsa. The errors never carry the passphrase or the key's content.
func PrivateKey(file, passphrase string) (ssh.Signer, error) {
	// Default private key is $HOME/.ssh/id_rsa
	if file == "" {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		file = filepath.Join(usr.HomeDir, "/.ssh/id_rsa")
	} else {
		file = os.ExpandEnv(file)
	}

	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(buf)
	var missing *ssh.PassphraseMissingError
	if !errors.As(err, &missing) {
		return key, err
	}
	if passphrase == "" {
		// The library's own error, as the ssh executor has always returned it.
		return nil, err
	}
	key, err = ssh.ParsePrivateKeyWithPassphrase(buf, []byte(passphrase))
	if err != nil {
		return nil, errors.New("the private key cannot be decrypted with the passphrase given")
	}
	return key, nil
}
