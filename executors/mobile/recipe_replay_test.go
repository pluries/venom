package mobile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Spec 014, US4-AS2, US4-AS3, FR-006: a mobile recipe that cannot reach a host, an app or a session fails, names the
// cause and is never a pass. These tests drive the engine (Parse, Process) with the mobile executor registered, as
// `venom run` does, against the fake automation server. They pin what the engine does today; none of them is red.

const replaySuite = `name: Recipe replay
testcases:
- name: US4-AS1 first
  steps:
  - type: mobile
    action: back
- name: US4-AS1 second
  steps:
  - type: mobile
    action: back
- name: US4-AS1 third
  steps:
  - type: mobile
    action: back
`

// replayTestcases is the number of testcases of replaySuite.
const replayTestcases = 3

// replay runs replaySuite through the engine with the mobile settings given and returns the venom that ran it.
func replay(t *testing.T, mobileVars map[string]any, stopOnFailure bool) *venom.Venom {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "recipe.yml")
	require.NoError(t, os.WriteFile(file, []byte(replaySuite), 0o600))

	v := venom.New()
	v.RegisterExecutorBuiltin(Name, New())
	v.OutputDir = dir
	v.StopOnFailure = stopOnFailure
	v.AddVariables(map[string]any{"mobile": mobileVars})
	require.NoError(t, v.Parse(context.Background(), []string{file}))
	require.NoError(t, v.Process(context.Background(), []string{file}))
	require.Len(t, v.Tests.TestSuites, 1)
	require.Len(t, v.Tests.TestSuites[0].TestCases, replayTestcases)
	return v
}

// failureText is every error of every step of a testcase, as the report shows them.
func failureText(tc venom.TestCase) string {
	var parts []string
	for _, step := range tc.TestStepResults {
		for _, e := range step.Errors {
			parts = append(parts, e.Value)
		}
	}
	return strings.Join(parts, "\n")
}

// requireNeverPass checks that the run is a failure and that testcases from index first on failed with a text that
// contains every wanted fragment.
func requireNeverPass(t *testing.T, v *venom.Venom, first int, wanted ...string) {
	t.Helper()
	assert.Equal(t, venom.StatusFail, v.Tests.Status, "a run that could not run its testcases is never a pass")
	assert.Equal(t, venom.StatusFail, v.Tests.TestSuites[0].Status)
	for i := first; i < replayTestcases; i++ {
		tc := v.Tests.TestSuites[0].TestCases[i]
		assert.Equal(t, venom.StatusFail, tc.Status, tc.Name)
		text := failureText(tc)
		for _, w := range wanted {
			assert.Contains(t, text, w, tc.Name)
		}
	}
}

// US4-AS2: no reachable host.
func TestRecipeReplayNoReachableHost(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	url := gone.URL
	gone.Close() // nothing listens on this address any more

	v := replay(t, map[string]any{"url": url, "platform": "android", "device": testDevice}, false)

	requireNeverPass(t, v, 0, "cannot open a session", testDevice)
	assert.Zero(t, v.Tests.TestSuites[0].NbTestcasesPass)
}

// US4-AS2: the host refuses the session because the app is not on it.
func TestRecipeReplayAppMissingOnTheHost(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST /session", fail(http.StatusInternalServerError, "session not created", "the app cannot be found on the device"))

	v := replay(t, f.baseVars(), false)

	requireNeverPass(t, v, 0, "cannot open a session", "session not created")
	assert.Zero(t, v.Tests.TestSuites[0].NbTestcasesPass)
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/back"), "no step runs without a session")
}

// US4-AS2: the app file the recipe names does not exist, so the run stops before any request.
func TestRecipeReplayAppFileMissing(t *testing.T) {
	f := newFakeServer(t)

	v := replay(t, f.withVars(map[string]any{"app": "no-such-app.apk"}), false)

	requireNeverPass(t, v, 0, "no-such-app.apk")
	assert.Zero(t, f.total(), "a missing app is refused before any request")
}

// US4-AS2: the server cannot create the session.
func TestRecipeReplaySessionCannotBeCreated(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST /session", fail(http.StatusInternalServerError, "session not created", "Could not start a new session"))

	v := replay(t, f.baseVars(), false)

	requireNeverPass(t, v, 0, "cannot open a session", "Could not start a new session")
	assert.Zero(t, v.Tests.TestSuites[0].NbTestcasesPass)
}

// dropAfterFirstSession is a POST /session handler that opens one session and then drops every connection, like a
// host that goes away after the first testcase.
func dropAfterFirstSession(f *fakeServer) handler {
	var opened atomic.Int32
	ok := reply(http.StatusOK, `{"sessionId":"`+testSessionID+`","capabilities":{"platformName":"Android"}}`)
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		if opened.Add(1) > 1 {
			panic(http.ErrAbortHandler)
		}
		ok(w, r, body)
	}
}

// US4-AS3: the host drops after the first testcase.
func TestRecipeReplayHostDropsAfterTheFirstTestcase(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST /session", dropAfterFirstSession(f))
	f.on("POST /session/{id}/back", reply(http.StatusOK, `null`))

	v := replay(t, f.baseVars(), false)

	ts := v.Tests.TestSuites[0]
	assert.Equal(t, venom.StatusPass, ts.TestCases[0].Status, "the testcase that ran before the drop passes")
	requireNeverPass(t, v, 1, "cannot open a session")
	assert.Equal(t, 1, ts.NbTestcasesPass)
	assert.Equal(t, replayTestcases-1, ts.NbTestcasesFail)
}

// US4-AS3, FR-006, O-1: does the engine report a COUNT of testcases that did not run?
//
// Observed answer (T041, decides T044): NO dedicated "not run" count. Without --stop-on-failure the testcases after
// the drop are not "not run": the engine runs each one, its Setup fails, and it is counted in NbTestcasesFail with the
// others; NbTestcasesSkip stays 0. With --stop-on-failure the testcases after the first failure are marked skipped
// (reason "stop-on-failure: enabled") and counted in NbTestcasesSkip; that count exists, but it also counts testcases
// skipped by a `skip:` list or by any other failure, so it does not say "the host dropped".
func TestRecipeReplayNotRunCountOfTheEngine(t *testing.T) {
	t.Run("default run: later testcases run and fail, nothing is skipped", func(t *testing.T) {
		f := newFakeServer(t)
		f.on("POST /session", dropAfterFirstSession(f))
		f.on("POST /session/{id}/back", reply(http.StatusOK, `null`))

		v := replay(t, f.baseVars(), false)

		ts := v.Tests.TestSuites[0]
		assert.Equal(t, 1, ts.NbTestcasesPass)
		assert.Equal(t, 2, ts.NbTestcasesFail)
		assert.Zero(t, ts.NbTestcasesSkip)
		assert.Equal(t, 1, v.Tests.NbTestsuitesFail)
	})

	t.Run("stop on failure: the rest is counted as skipped, the run still fails", func(t *testing.T) {
		f := newFakeServer(t)
		f.on("POST /session", fail(http.StatusInternalServerError, "session not created", "Could not start a new session"))

		v := replay(t, f.baseVars(), true)

		ts := v.Tests.TestSuites[0]
		assert.Equal(t, venom.StatusFail, v.Tests.Status)
		assert.Equal(t, venom.StatusFail, ts.TestCases[0].Status)
		assert.Equal(t, 1, ts.NbTestcasesFail)
		assert.Equal(t, replayTestcases-1, ts.NbTestcasesSkip)
		for _, tc := range ts.TestCases[1:] {
			assert.Equal(t, venom.StatusSkip, tc.Status, tc.Name)
			require.NotEmpty(t, tc.Skipped, tc.Name)
			assert.Contains(t, tc.Skipped[0].Value, "stop-on-failure", tc.Name)
		}
	})
}
