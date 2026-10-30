package sshconn

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// silentHost accepts TCP connections and never speaks, as a host that is up but does not answer SSH.
func silentHost(t *testing.T) string {
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

func TestDialContext_SilentHostTimesOut(t *testing.T) {
	check, err := NewHostKeyCheck(HostKeySettings{Host: testAddr, WarnOnce: &sync.Once{}}, func(string, ...any) {})
	require.NoError(t, err)

	start := time.Now()
	_, err = DialContext(context.Background(), silentHost(t), "u", []ssh.AuthMethod{}, check, 200*time.Millisecond)

	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout(), "%v", err)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestDialContext_CancelAbortsTheHandshake(t *testing.T) {
	check, err := NewHostKeyCheck(HostKeySettings{Host: testAddr, WarnOnce: &sync.Once{}}, func(string, ...any) {})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err = DialContext(ctx, silentHost(t), "u", []ssh.AuthMethod{}, check, time.Minute)

	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestNewHostKeyCheck_OwnOnceAndMessage(t *testing.T) {
	warn, got := warnings(t)
	own, mine := &sync.Once{}, "mine: not checked"

	_, err := NewHostKeyCheck(HostKeySettings{Host: testAddr}, warn)
	require.NoError(t, err)
	for range 2 {
		_, err = NewHostKeyCheck(HostKeySettings{Host: testAddr, WarnOnce: own, WarnMessage: mine}, warn)
		require.NoError(t, err)
	}

	assert.Equal(t, []string{DefaultInsecureMessage, mine}, *got, "each once has its own single warning")
}
