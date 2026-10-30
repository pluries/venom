package mobile

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/sshconn"
)

const (
	defaultSSHPort = 22
	maxPort        = 65535
)

// warnf is venom.Warn; a test replaces it.
var warnf = venom.Warn

// hostKeyWarning limits the host key warning of the tunnel to one per run. It is the tunnel's own, apart from the ssh
// executor's, and the text names the mobile.ssh.* settings.
var hostKeyWarning sync.Once

const hostKeyWarningText = "mobile: the SSH server's host key is not checked (set mobile.ssh.knownhosts or mobile.ssh.hostkey to check it)"

// resetMobileHostKeyWarning re-arms the warning, for tests only.
func resetMobileHostKeyWarning() { hostKeyWarning = sync.Once{} }

// sshSettings are the mobile.ssh.* settings: how venom reaches the host that runs the automation server (FR-03).
// Key authentication only (S4). The passphrase is never printed, logged or put in an error.
type sshSettings struct {
	addr       string // host:port of the SSH server
	user       string
	key        string // path of the private key
	passphrase string
	check      sshconn.HostKeyCheck
}

// String names the target only, so that the settings cannot leak a passphrase through a %v.
func (s *sshSettings) String() string { return s.user + "@" + s.addr }

// readSSH reads the mobile.ssh.* settings. It returns nil settings when mobile.ssh.host is not set: SSH is used if
// and only if the host is set. Every refusal happens here, before any connection.
func readSSH(ctx context.Context) (*sshSettings, error) {
	host := strings.TrimSpace(venom.StringVarFromCtx(ctx, "mobile.ssh.host"))
	if host == "" {
		return nil, nil //nolint:nilnil // no host means no tunnel; nil settings are the answer, not an error
	}
	port, err := sshPort(ctx)
	if err != nil {
		return nil, err
	}
	s := &sshSettings{
		addr:       net.JoinHostPort(host, strconv.Itoa(port)),
		user:       venom.StringVarFromCtx(ctx, "mobile.ssh.user"),
		key:        venom.StringVarFromCtx(ctx, "mobile.ssh.key"),
		passphrase: venom.StringVarFromCtx(ctx, "mobile.ssh.passphrase"),
	}
	if err := s.requireLogin(); err != nil {
		return nil, err
	}
	if s.check, err = hostKeyCheck(ctx, s.addr); err != nil {
		return nil, err
	}
	return s, nil
}

// requireLogin refuses a host without the user and the private key that log in to it.
func (s *sshSettings) requireLogin() error {
	if s.user == "" {
		return usagef("mobile.ssh.user is not set: mobile.ssh.host needs a user")
	}
	if s.key == "" {
		return usagef("mobile.ssh.key is not set: mobile.ssh.host needs the path of a private key (no password login)")
	}
	return nil
}

// sshPort reads mobile.ssh.port, 22 when unset.
func sshPort(ctx context.Context) (int, error) {
	port := venom.IntVarFromCtx(ctx, "mobile.ssh.port")
	if port == 0 {
		return defaultSSHPort, nil
	}
	if port < 0 || port > maxPort {
		return 0, usagef("mobile.ssh.port must be between 1 and %d", maxPort)
	}
	return port, nil
}

// hostKeyCheck builds the host key check the suite asks for, as the ssh executor does (knownhosts and hostkey are
// exclusive; neither set warns once per run).
func hostKeyCheck(ctx context.Context, addr string) (sshconn.HostKeyCheck, error) {
	check, err := sshconn.NewHostKeyCheck(sshconn.HostKeySettings{
		KnownHosts:  optionalString(ctx, "mobile.ssh.knownhosts"),
		HostKey:     optionalString(ctx, "mobile.ssh.hostkey"),
		Workdir:     venom.StringVarFromCtx(ctx, "venom.testsuite.workdir"),
		Host:        addr,
		WarnOnce:    &hostKeyWarning,
		WarnMessage: hostKeyWarningText,
	}, func(format string, args ...any) { warnf(ctx, format, args...) })
	if err != nil {
		return check, usagef("mobile.ssh: %v", err)
	}
	return check, nil
}

// optionalString is a variable's value, nil when the variable is not set: a value set to nothing is told from none.
func optionalString(ctx context.Context, name string) *string {
	if venom.VarFromCtx(ctx, name) == nil {
		return nil
	}
	v := venom.StringVarFromCtx(ctx, name)
	return &v
}
