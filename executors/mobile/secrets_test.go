package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyFragment is a piece of the key file's body, which the report, the log and the console must never carry.
func keyFragment(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r", ""), "\n")
	require.Greater(t, len(lines), 3)
	require.GreaterOrEqual(t, len(lines[1]), 24)
	return lines[1][:24]
}

// failingSetups runs the Setups that fail on the SSH side and returns the text of every error they return.
func failingSetups(t *testing.T, f *fakeServer, s *fakeSSH, other *fakeSSH, phrase string) []string {
	t.Helper()
	var texts []string
	for name, edit := range map[string]func(map[string]interface{}){
		"wrong passphrase":   func(v map[string]interface{}) { v["ssh.passphrase"] = phrase + " wrong" },
		"missing passphrase": func(v map[string]interface{}) { delete(v, "ssh.passphrase") },
		"other host key":     func(v map[string]interface{}) { v["ssh.hostkey"] = authorizedKey(other.hostKey) },
	} {
		vars := sshVars(t, f, s)
		vars["ssh.passphrase"] = phrase
		edit(vars)
		err := setupErr(vars)
		require.Error(t, err, name)
		texts = append(texts, err.Error())
	}
	return texts
}

// emittedBySuccessfulRun runs Setup, a failing step, a passing step and TearDown, and returns what the engine would
// put in a report: the results as JSON, the facts line and every error text.
func emittedBySuccessfulRun(t *testing.T, vars map[string]interface{}) []string {
	t.Helper()
	withAction(t, "fail", func(context.Context, *session, Executor, *Result) error { return errors.New("synthetic failure") })
	withAction(t, "pass", func(context.Context, *session, Executor, *Result) error { return nil })
	var out []string
	cfg, err := readConfig(testCtx(vars))
	require.NoError(t, err)
	out = append(out, fmt.Sprintf("%v %+v", cfg.ssh, cfg.ssh))

	ctx := openCase(t, vars)
	for _, action := range []string{"fail", "pass"} {
		raw, err := json.Marshal(runStep(ctx, t, action))
		require.NoError(t, err)
		out = append(out, string(raw))
	}
	out = append(out, getSession(ctx).facts.describe())
	if err := (Executor{}).TearDown(ctx); err != nil {
		out = append(out, err.Error())
	}
	return out
}

// The passphrase and the key never appear anywhere. The context carries no declared secret on purpose: venom's
// masking by value would hide a leak, and the claim is stronger, the executor never emits them at all.
func TestAC6_PassphraseAndKeyAppearNowhere(t *testing.T) {
	logs := captureLogs(t)
	resetMobileHostKeyWarning() // the host key warning goes through the log of this test
	t.Cleanup(resetMobileHostKeyWarning)
	phrase := "synthetic phrase for ac6"
	f, s, other := newFakeServer(t), startFakeSSHWith(t, phrase), startFakeSSH(t)
	vars := sshVars(t, f, s)
	vars["ssh.passphrase"] = phrase

	emitted := emittedBySuccessfulRun(t, vars)
	emitted = append(emitted, failingSetups(t, f, s, other, phrase)...)
	require.NoError(t, Executor{}.TearDown(openCase(t, without(vars, "ssh.hostkey"))),
		"a run without a host key setting still opens, with a warning")
	emitted = append(emitted, logs.text())

	all := strings.Join(emitted, "\n")
	assert.NotContains(t, all, phrase)
	assert.NotContains(t, all, keyFragment(t, s.keyPath))
	assert.Contains(t, logs.text(), "mobile session opened", "the log was captured")
	assert.Contains(t, logs.text(), "host key", "the host key warning was captured")
}
