package mobile

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countPlainHTTPWarnings replaces the warning seam and re-arms the once-per-run limit, and returns the lines warned.
func countPlainHTTPWarnings(t *testing.T) *[]string {
	t.Helper()
	var lines []string
	shorten(t, &warnf, func(_ context.Context, format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, args...))
	})
	resetHTTPWarning()
	t.Cleanup(resetHTTPWarning)
	return &lines
}

// urlVars are the settings of a case against address, with an SSH tunnel when tunnel is set.
func urlVars(t *testing.T, address string, tunnel bool) map[string]interface{} {
	t.Helper()
	vars := map[string]interface{}{"url": address, "platform": "android", "device": testDevice}
	if tunnel {
		hostKey, _ := newSSHKey(t)
		vars["ssh.host"], vars["ssh.user"], vars["ssh.key"] = "mac.example.test", sshTestUser, "key"
		vars["ssh.hostkey"] = authorizedKey(hostKey.PublicKey())
	}
	return vars
}

func TestFR17_PlainHTTPToAnotherHostWarnsOnce(t *testing.T) {
	tests := []struct {
		name    string
		address string
		tunnel  bool
		warns   bool
	}{
		{"plain http to another host", "http://10.0.0.5:4723", false, true},
		{"plain http, upper case name", "http://Appium.Example.Test:4723", false, true},
		{"loopback address", "http://127.0.0.1:4723", false, false},
		{"other 127.0.0.0/8 address", "http://127.1.2.3:4723", false, false},
		{"localhost", "http://localhost:4723", false, false},
		{"localhost, upper case", "http://LOCALHOST:4723", false, false},
		{"IPv6 loopback", "http://[::1]:4723", false, false},
		{"https to another host", "https://10.0.0.5:4723", false, false},
		{"plain http with an SSH tunnel", "http://10.0.0.5:4723", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := countPlainHTTPWarnings(t)

			_, err := readConfig(testCtx(urlVars(t, tt.address, tt.tunnel)))
			require.NoError(t, err)
			if !tt.warns {
				assert.Empty(t, *lines)
				return
			}
			require.Len(t, *lines, 1)
			assert.Contains(t, (*lines)[0], "not encrypted")
			assert.Contains(t, (*lines)[0], "no password")
			assert.Contains(t, (*lines)[0], strings.TrimPrefix(tt.address, "http://"))
		})
	}
}

func TestFR17_TwoRunsOfTheSettingsWarnOnce(t *testing.T) {
	lines := countPlainHTTPWarnings(t)

	for range 2 {
		_, err := readConfig(testCtx(urlVars(t, "http://10.0.0.5:4723", false)))
		require.NoError(t, err)
	}
	assert.Len(t, *lines, 1, "one warning per run")
}

func TestFR17_RefusedSettingsDoNotWarn(t *testing.T) {
	lines := countPlainHTTPWarnings(t)

	_, err := readConfig(testCtx(urlVars(t, "http://user:pass@10.0.0.5:4723", false)))
	require.ErrorContains(t, err, "credentials")
	assert.Empty(t, *lines)
}
