package sshconn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// openHost is an in-process SSH server that lets every client in without credentials, and the key it presents.
func openHost(t *testing.T) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	hostKey, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go serveOpenHost(lis, config)
	return lis.Addr().String(), hostKey
}

// serveOpenHost completes the handshake of each connection and refuses every channel and request.
func serveOpenHost(lis net.Listener, config *ssh.ServerConfig) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		go func() {
			defer func() { discardErr(conn.Close()) }()
			sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
			if err != nil {
				return
			}
			go ssh.DiscardRequests(reqs)
			for ch := range chans {
				discardErr(ch.Reject(ssh.Prohibited, "no channels"))
			}
			discardErr(sconn.Wait())
		}()
	}
}

// pinnedKey is the host key check of one fixed key.
func pinnedKey(key ssh.PublicKey) HostKeyCheck {
	return HostKeyCheck{Callback: ssh.FixedHostKey(key), Algorithms: keyAlgorithms(key.Type())}
}

func TestDial_ReachesAHostThatPresentsThePinnedKey(t *testing.T) {
	addr, key := openHost(t)
	client, err := Dial(addr, "venom", nil, pinnedKey(key))
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestDialContext_ReachesAHostAndLeavesNoDeadlineOnTheConnection(t *testing.T) {
	addr, key := openHost(t)
	client, err := DialContext(context.Background(), addr, "venom", nil, pinnedKey(key), 150*time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	// A deadline left on the connection would end it once the 150 ms timeout has passed; it must still answer after
	// 300 ms. The server refuses the channel, which is an answer.
	time.Sleep(300 * time.Millisecond)
	_, _, err = client.OpenChannel("session", nil)
	var open *ssh.OpenChannelError
	require.ErrorAs(t, err, &open)
	assert.Equal(t, ssh.Prohibited, open.Reason)
}

func TestDialContext_RefusesAHostWhoseKeyIsNotThePinnedOne(t *testing.T) {
	addr, _ := openHost(t)
	_, other := openHost(t)
	_, err := DialContext(context.Background(), addr, "venom", nil, pinnedKey(other), 5*time.Second)
	require.Error(t, err)
}

func TestDialContext_RefusesAnAddressNobodyListensOn(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	_, other := openHost(t)
	_, err = DialContext(context.Background(), addr, "venom", nil, pinnedKey(other), time.Second)
	require.Error(t, err)
}

// discardErr drops the error of a close that has nothing left to report.
func discardErr(error) {}
