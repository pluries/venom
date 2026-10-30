package smtp

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

func smtpStep(port string, extra venom.TestStep) venom.TestStep {
	step := venom.TestStep{
		"withtls": true, "host": "127.0.0.1", "port": port,
		"from": "venom@example.org", "to": "alice@example.org", "subject": "warn", "body": "Hello Alice",
	}
	for k, v := range extra {
		step[k] = v
	}
	return step
}

// TestRunWarnsOnceWhenTLSIsNotVerified pins #2298: withtls without verifytls says so once per run, over several steps,
// and the mail is still sent.
func TestRunWarnsOnceWhenTLSIsNotVerified(t *testing.T) {
	venom.InitTestLogger(t)
	got := captureWarnings(t)
	srv := startSMTP(t, serverTLS(selfSignedCert(t, "127.0.0.1")))

	for range 3 {
		_, err := Executor{}.Run(context.Background(), smtpStep(srv.port, nil))
		require.NoError(t, err)
	}
	assert.Len(t, srv.received(), 3)
	require.Len(t, *got, 1)
	assert.Contains(t, (*got)[0], "TLS certificate")
}

func TestRunDoesNotWarnWhenTLSIsVerifiedOrOff(t *testing.T) {
	venom.InitTestLogger(t)
	got := captureWarnings(t)
	srv := startSMTP(t, serverTLS(selfSignedCert(t, "127.0.0.1")))

	_, err := Executor{}.Run(context.Background(), smtpStep(srv.port, venom.TestStep{"verifytls": true}))
	require.Error(t, err, "the self-signed certificate is refused")
	(&Executor{}).warnInsecureTLS(context.Background()) // withtls off: nothing to verify
	assert.Empty(t, *got)
}
