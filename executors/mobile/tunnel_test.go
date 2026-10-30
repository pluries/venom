package mobile

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/ovh/venom/executors/internal/sshconn"
)

// sshVars are the settings of a test case that reaches f through the fake SSH server s, host key pinned.
func sshVars(t *testing.T, f *fakeServer, s *fakeSSH) map[string]interface{} {
	host, port, err := net.SplitHostPort(s.addr)
	require.NoError(t, err)
	return f.withVars(map[string]interface{}{
		"ssh.host":    host,
		"ssh.port":    port,
		"ssh.user":    sshTestUser,
		"ssh.key":     s.keyPath,
		"ssh.hostkey": authorizedKey(s.hostKey),
	})
}

// without returns vars minus the named keys.
func without(vars map[string]interface{}, names ...string) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range vars {
		out[k] = v
	}
	for _, n := range names {
		delete(out, n)
	}
	return out
}

// tunnelGoroutines counts the goroutines of the tunnels of this package, found in the stacks.
func tunnelGoroutines() int {
	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	return strings.Count(string(buf), "mobile.(*tunnel).")
}

// requireTunnelGone waits for the tunnel's goroutines to end: close waits for them, the stack dump only has to see it.
func requireTunnelGone(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool { return tunnelGoroutines() == 0 }, 5*time.Second, 10*time.Millisecond)
}

func setupErr(vars map[string]interface{}) error {
	_, err := Executor{}.Setup(testCtx(vars), nil)
	return err
}

func TestAC1_TunnelCarriesTheSessionAndClosesAtTearDown(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	ctx := openCase(t, sshVars(t, f, s))

	local := getSession(ctx).c.base
	assert.NotEqual(t, f.srv.URL, local)
	assert.True(t, strings.HasPrefix(local, "http://127.0.0.1:"), local)
	assert.Equal(t, 1, f.count(http.MethodPost, sessionRoot), "the server answered through the tunnel")
	u, err := url.Parse(f.srv.URL)
	require.NoError(t, err)
	assert.Contains(t, s.dialed(), u.Host, "the SSH server dialled the port of mobile.url on its side")

	require.NoError(t, Executor{}.TearDown(ctx))
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionRoot+"/{id}"))
	localHost, err := url.Parse(local)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", localHost.Host, time.Second)
	if err == nil {
		_ = conn.Close()
	}
	require.Error(t, err, "the local port refuses connections once the case ended")
	requireTunnelGone(t)
}

func TestAC2_WithoutSSHSettingsTheURLIsUsedAsGiven(t *testing.T) {
	f := newFakeServer(t)
	ctx := openCase(t, f.baseVars())

	sess := getSession(ctx)
	assert.Equal(t, f.srv.URL, sess.c.base)
	assert.Empty(t, sess.closers)
	assert.Zero(t, tunnelGoroutines())
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestAC3_HostKeyMismatchIsRefusedBeforeAnyRequest(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	other := startFakeSSH(t)
	vars := sshVars(t, f, s)
	vars["ssh.hostkey"] = authorizedKey(other.hostKey)

	err := setupErr(vars)
	require.ErrorContains(t, err, "host key")
	assert.Zero(t, f.total(), "nothing reached the automation server")
	assert.Empty(t, s.dialed())
	requireTunnelGone(t)
}

func TestAC3_NoHostKeySettingWarnsOncePerRun(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	warnings := 0
	shorten(t, &warnf, func(context.Context, string, ...interface{}) { warnings++ })
	resetMobileHostKeyWarning()
	t.Cleanup(resetMobileHostKeyWarning)
	vars := without(sshVars(t, f, s), "ssh.hostkey")

	for range 2 {
		require.NoError(t, Executor{}.TearDown(openCase(t, vars)))
	}
	assert.Equal(t, 1, warnings, "two sessions, one warning")
}

func TestAC3_HostKeyWarningIsTheTunnelsOwnAndNamesTheMobileSettings(t *testing.T) {
	var texts []string
	shorten(t, &warnf, func(_ context.Context, format string, _ ...interface{}) { texts = append(texts, format) })
	resetMobileHostKeyWarning()
	sshconn.ResetInsecureWarning()
	t.Cleanup(resetMobileHostKeyWarning)
	t.Cleanup(sshconn.ResetInsecureWarning)
	ctx := testCtx(map[string]interface{}{"ssh.host": "mac.example.test", "ssh.user": sshTestUser, "ssh.key": "k"})
	sshWarn := func(string, ...any) { texts = append(texts, "ssh executor") }

	_, err := sshconn.NewHostKeyCheck(sshconn.HostKeySettings{Host: "h:22"}, sshWarn)
	require.NoError(t, err)
	_, err = readSSH(ctx)
	require.NoError(t, err)
	_, err = readSSH(ctx)
	require.NoError(t, err)

	require.Len(t, texts, 2, "the ssh executor's warning does not use up the tunnel's, which still comes once")
	assert.Equal(t, "ssh executor", texts[0])
	assert.Contains(t, texts[1], "mobile.ssh.knownhosts")
	assert.Contains(t, texts[1], "mobile.ssh.hostkey")

	texts = nil
	_, err = readSSH(ctx)
	require.NoError(t, err)
	sshconn.ResetInsecureWarning()
	_, err = sshconn.NewHostKeyCheck(sshconn.HostKeySettings{Host: "h:22"}, sshWarn)
	require.NoError(t, err)
	assert.Equal(t, []string{"ssh executor"}, texts, "the tunnel's warning does not use up the ssh executor's")
}

// silentListener accepts TCP connections and never speaks, as a host that is up but does not answer SSH.
func silentListener(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		var held []net.Conn
		defer func() {
			for _, c := range held {
				_ = c.Close()
			}
		}()
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			held = append(held, c)
		}
	}()
	t.Cleanup(func() { _ = lis.Close() })
	return lis.Addr().String()
}

func TestFR03_SilentHostEndsAsATimeoutNotAHang(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	settings, err := readSSH(testCtx(sshVars(t, f, s)))
	require.NoError(t, err)
	settings.addr = silentListener(t)

	start := time.Now()
	_, err = openTunnel(context.Background(), settings, f.srv.URL, 200*time.Millisecond)

	require.ErrorContains(t, err, "cannot open the SSH connection")
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "a timeout, not another failure: %v", err)
	assert.Less(t, time.Since(start), 10*time.Second)
	requireTunnelGone(t)
}

func TestFR03_CancelledContextAbortsTheHandshakePromptly(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	settings, err := readSSH(testCtx(sshVars(t, f, s)))
	require.NoError(t, err)
	settings.addr = silentListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err = openTunnel(ctx, settings, f.srv.URL, time.Minute)

	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 10*time.Second, "well before the minute of the timeout")
}

func TestAC3_KnownHostsFile(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	other := startFakeSSH(t)
	listing := func(srv *fakeSSH) string {
		path := filepath.Join(t.TempDir(), "known_hosts")
		line := knownhosts.Line([]string{s.addr}, srv.hostKey) + "\n"
		require.NoError(t, os.WriteFile(path, []byte(line), 0o600))
		return path
	}
	vars := without(sshVars(t, f, s), "ssh.hostkey")

	vars["ssh.knownhosts"] = listing(s)
	require.NoError(t, Executor{}.TearDown(openCase(t, vars)), "the listed key matches")

	vars["ssh.knownhosts"] = listing(other)
	require.Error(t, setupErr(vars), "a key the file does not list is refused")
	requireTunnelGone(t)
}

func TestFR03_SSHSettingsAreRefusedBeforeAnyConnection(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string]interface{})
		want string
	}{
		{"no user", func(v map[string]interface{}) { delete(v, "ssh.user") }, "mobile.ssh.user"},
		{"no key", func(v map[string]interface{}) { delete(v, "ssh.key") }, "mobile.ssh.key"},
		{"both host key settings", func(v map[string]interface{}) { v["ssh.knownhosts"] = "known_hosts" }, "exclusive"},
		{"empty host key", func(v map[string]interface{}) { v["ssh.hostkey"] = "" }, "hostkey is set but empty"},
		{"port out of range", func(v map[string]interface{}) { v["ssh.port"] = 70000 }, "mobile.ssh.port"},
		{"negative port", func(v map[string]interface{}) { v["ssh.port"] = -1 }, "mobile.ssh.port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, s := newFakeServer(t), startFakeSSH(t)
			vars := sshVars(t, f, s)
			tt.edit(vars)

			err := setupErr(vars)
			var usage *usageError
			require.ErrorAs(t, err, &usage)
			assert.Contains(t, err.Error(), tt.want)
			assert.Zero(t, f.total())
			assert.Zero(t, s.accepted.Load(), "no SSH connection was made")
		})
	}
}

func TestFR03_PortDefaultsTo22(t *testing.T) {
	settings, err := readSSH(testCtx(map[string]interface{}{
		"ssh.host": "mac.example.test", "ssh.user": sshTestUser, "ssh.key": "k",
	}))
	require.NoError(t, err)
	assert.Equal(t, "mac.example.test:22", settings.addr)
	assert.Equal(t, sshTestUser+"@mac.example.test:22", settings.String())
}

func TestNFR05_PassphraseOpensTheKeyAndNeverLeaks(t *testing.T) {
	phrase, wrong := "synthetic phrase one", "synthetic phrase two"
	f, s := newFakeServer(t), startFakeSSHWith(t, phrase)
	vars := sshVars(t, f, s)

	vars["ssh.passphrase"] = phrase
	require.NoError(t, Executor{}.TearDown(openCase(t, vars)))

	for name, given := range map[string]string{"wrong": wrong, "missing": ""} {
		vars["ssh.passphrase"] = given
		err := setupErr(vars)
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), phrase)
		assert.NotContains(t, err.Error(), wrong)
	}
}

func TestFR03_FailedSetupClosesTheTunnel(t *testing.T) {
	t.Run("the server refuses the session", func(t *testing.T) {
		f, s := newFakeServer(t), startFakeSSH(t)
		f.on("POST /session", fail(http.StatusInternalServerError, "session not created", "no device"))

		require.Error(t, setupErr(sshVars(t, f, s)))
		requireTunnelGone(t)
	})
	t.Run("the server opens another platform", func(t *testing.T) {
		f, s := newFakeServer(t), startFakeSSH(t)
		f.on("POST /session", reply(http.StatusOK, `{"sessionId":"x","capabilities":{"platformName":"iOS"}}`))

		require.Error(t, setupErr(sshVars(t, f, s)))
		assert.Equal(t, 1, f.count(http.MethodDelete, sessionRoot+"/{id}"), "the session closed through the tunnel first")
		requireTunnelGone(t)
	})
}

func TestFR03_ClosersRunInReverseOrderAndOnlyOnce(t *testing.T) {
	var order []int
	errFirst := errors.New("first failed")
	list := closers{
		func() error { order = append(order, 1); return errFirst },
		func() error { order = append(order, 2); return nil },
		func() error { order = append(order, 3); return nil },
	}

	err := list.run()
	require.ErrorIs(t, err, errFirst)
	assert.Equal(t, []int{3, 2, 1}, order)
	require.NoError(t, list.run())
	assert.Equal(t, []int{3, 2, 1}, order, "a second run does nothing")
}

func TestFR03_TunnelCloseIsIdempotent(t *testing.T) {
	f, s := newFakeServer(t), startFakeSSH(t)
	settings, err := readSSH(testCtx(sshVars(t, f, s)))
	require.NoError(t, err)
	tun, err := openTunnel(context.Background(), settings, f.srv.URL, time.Minute)
	require.NoError(t, err)

	require.NoError(t, tun.close())
	require.NoError(t, tun.close())
	requireTunnelGone(t)
}
