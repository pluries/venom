package ssh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
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

// TestHostKeyCheckWarnsOncePerRun pins #2298: a step that checks no host key says so, once per run, and still connects.
func TestHostKeyCheckWarnsOncePerRun(t *testing.T) {
	got := captureWarnings(t)

	for range 3 {
		check, err := Executor{Host: "127.0.0.1:22"}.newHostKeyCheck(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, check.Callback, "behaviour unchanged: the check still accepts any key")
	}
	require.Len(t, *got, 1)
	assert.Contains(t, (*got)[0], "host key")
}

func TestHostKeyCheckDoesNotWarnWhenAKeyIsChecked(t *testing.T) {
	got := captureWarnings(t)
	key := newEd25519Key(t).PublicKey()

	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(path, knownHostsFile("127.0.0.1:22", key), 0o600))
	_, err := Executor{Host: "127.0.0.1:22", KnownHosts: &path}.newHostKeyCheck(context.Background())
	require.NoError(t, err)

	hk := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	_, err = Executor{Host: "127.0.0.1:22", HostKey: &hk}.newHostKeyCheck(context.Background())
	require.NoError(t, err)

	assert.Empty(t, *got)
}
