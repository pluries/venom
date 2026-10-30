package imap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// captureWarnings replaces the warning sink for the test and returns what was warned.
func captureWarnings(t *testing.T) *[]string {
	t.Helper()
	resetInsecureWarning()
	var got []string
	old := warnf
	warnf = func(_ context.Context, format string, _ ...interface{}) { got = append(got, format) }
	t.Cleanup(func() { warnf = old; resetInsecureWarning() })
	return &got
}

// TestRunWarnsOnceWhenTLSIsNotVerified pins #2298: withtls without verifytls says so once per run, over several steps.
func TestRunWarnsOnceWhenTLSIsNotVerified(t *testing.T) {
	got := captureWarnings(t)
	cert := serverTLS(selfSignedCert(t, "127.0.0.1"))

	for range 3 {
		auth := map[string]any{"withtls": true, "host": "127.0.0.1", "port": startIMAP(t, cert, "IMAP4rev1")}
		_, err := Executor{}.Run(context.Background(), venom.TestStep{"auth": auth})
		require.Error(t, err, "the fake server is not a real IMAP server; the step fails after the warning")
	}
	require.Len(t, *got, 1)
	assert.Contains(t, (*got)[0], "TLS certificate")
}

func TestRunDoesNotWarnWhenTLSIsVerifiedOrOff(t *testing.T) {
	got := captureWarnings(t)
	cert := serverTLS(selfSignedCert(t, "127.0.0.1"))

	_, err := Executor{}.Run(context.Background(), venom.TestStep{"auth": map[string]any{
		"withtls": true, "verifytls": true, "host": "127.0.0.1", "port": startIMAP(t, cert, "IMAP4rev1")}})
	require.Error(t, err, "the self-signed certificate is refused")
	AuthConfig{}.warnInsecureTLS(context.Background()) // withtls off: nothing to verify
	assert.Empty(t, *got)
}
