package imap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yesnault/go-imap/imap"
)

// TestConnectDoesNotRaceOnTheLogMask is the repro of #2326: connect set the client's log mask while go-imap's receiver
// goroutine could still read it. The race detector reports it when the connection gets past the greeting, which the
// fake server does not stop (no STARTTLS). The test is meant to run under -race, as the build does.
func TestConnectDoesNotRaceOnTheLogMask(t *testing.T) {
	cert := serverTLS(selfSignedCert(t, "127.0.0.1"))
	for range 20 {
		port := startIMAP(t, cert, "IMAP4rev1")
		_, err := Executor{Auth: AuthConfig{WithTLS: true}}.connect(context.Background(), "", "127.0.0.1", port, "user", "pass")
		require.ErrorContains(t, err, "unable to login", "the fake server refuses every command")
	}
}

// A session that logs in runs the second SetLogMask, after the login command completed.
func TestConnectWithALoginDoesNotRaceOnTheLogMask(t *testing.T) {
	cert := serverTLS(selfSignedCert(t, "127.0.0.1"))
	for range 20 {
		port := startIMAP(t, cert, "IMAP4rev1 X-TEST-LOGIN-OK")
		c, err := Executor{Auth: AuthConfig{WithTLS: true}}.connect(context.Background(), "", "127.0.0.1", port, "user", "pass")
		require.NoError(t, err)
		require.NotNil(t, c)
		// The receiver goroutine only stops on Logout; Close(false) sends the CLOSE command and leaves it running.
		_, err = c.Logout(imapClientTimeout)
		require.NoError(t, err)
	}
}

// The mask is applied from the first byte of every connection, the login included. The password travels in the LOGIN
// command: LogCmd logs the command line and LogRaw the raw stream, so neither may ever be part of the mask. Widening the
// mask for a debug mode needs a different design (a mask that is lowered around the login without a data race).
func TestLogMaskNeverIncludesTheLoginCommand(t *testing.T) {
	assert.Zero(t, imapLogMask&(imap.LogCmd|imap.LogRaw), "the login command, which carries the password, must never be logged")
	assert.Equal(t, imapLogMask, imap.DefaultLogMask, "new clients start with the executor's mask")
}
