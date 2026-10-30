package mobile

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	sourcePath  = "/session/{id}/source"
	sessionRoot = "/session"
	sessionByID = "/session/{id}"
)

func TestFR01_ExecutorShape(t *testing.T) {
	assert.Equal(t, "mobile", Name)
	_, ok := New().(venom.ExecutorWithSetup)
	assert.True(t, ok, "mobile has a Setup and a TearDown")
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
}

func TestFR13_DefaultAssertion(t *testing.T) {
	got := Executor{}.GetDefaultAssertions()
	require.Len(t, got.Assertions, 1)
	assert.Equal(t, venom.Assertion("result.err ShouldBeEmpty"), got.Assertions[0])
}

func TestAC1_OneSessionPerCaseClosedAfterFailingStep(t *testing.T) {
	f := newFakeServer(t)
	f.on("GET "+sourcePath, fail(http.StatusNotFound, "no such element", "cannot find it"))
	withAction(t, "boom", callAction(http.MethodGet, "/source"))

	ctx := openCase(t, f.baseVars())
	failing := runStep(ctx, t, "boom")
	assert.Contains(t, failing.Err, "no such element")
	assert.Equal(t, 0, f.count(http.MethodDelete, sessionByID), "still open between the steps")
	require.NoError(t, Executor{}.TearDown(ctx))

	assert.Equal(t, 1, f.count(http.MethodPost, sessionRoot))
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID))
}

func TestFR15_ServerFailureIsAStepFailureAndTheSessionGoesOn(t *testing.T) {
	f := newFakeServer(t)
	f.on("GET "+sourcePath, fail(http.StatusNotFound, "no such element", "cannot find it"))
	withAction(t, "boom", callAction(http.MethodGet, "/source"))
	withAction(t, "fine", func(context.Context, *session, Executor, *Result) error { return nil })

	ctx := openCase(t, f.baseVars())
	failing := runStep(ctx, t, "boom")
	assert.Equal(t, "action boom: no such element: cannot find it", failing.Err)
	assert.NotContains(t, failing.Err, "Trace.java", "a stack trace never reaches the report")

	assert.Empty(t, runStep(ctx, t, "fine").Err)
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestFR09_UnknownActionIsAGoErrorListingTheSupportedOnes(t *testing.T) {
	f := newFakeServer(t)
	withAction(t, "known", callAction(http.MethodGet, "/source"))
	ctx := openCase(t, f.baseVars())

	_, err := Executor{}.Run(ctx, venom.TestStep{"action": "dance"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown action "dance"`)
	assert.Contains(t, err.Error(), "known")
}

func TestFR13_ResultRepeatsTheRunFacts(t *testing.T) {
	f := newFakeServer(t)
	withAction(t, "fine", func(context.Context, *session, Executor, *Result) error { return nil })
	ctx := openCase(t, f.withVars(map[string]interface{}{"osVersion": "13", "appId": "org.example.app"}))

	result := runStep(ctx, t, "fine")
	assert.Equal(t, "android", result.Platform)
	assert.Equal(t, testDevice, result.Device)
	assert.Equal(t, "14", result.OSVersion, "the version the server reports wins")
	assert.Equal(t, "org.example.app", result.AppID)
	assert.Empty(t, result.Err)
}

func TestFR02_RunWithoutSessionOrAfterCloseFailsClearly(t *testing.T) {
	_, err := Executor{}.Run(context.Background(), venom.TestStep{"action": "x"})
	require.ErrorContains(t, err, "no mobile session")

	f := newFakeServer(t)
	ctx := openCase(t, f.baseVars())
	require.NoError(t, Executor{}.TearDown(ctx))
	_, err = Executor{}.Run(ctx, venom.TestStep{"action": "x"})
	require.ErrorContains(t, err, "already closed")
	require.NoError(t, Executor{}.TearDown(ctx), "a second TearDown is harmless")
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID))
}

func TestFR02_ForeignContextValueIsNoSession(t *testing.T) {
	assert.Nil(t, getSession(context.WithValue(context.Background(), ContextKey, "not a session")))
	require.NoError(t, Executor{}.TearDown(context.Background()))
}

func TestFR02_CloseSurvivesACancelledRun(t *testing.T) {
	f := newFakeServer(t)
	runCtx, cancel := context.WithCancel(testCtx(f.baseVars()))
	ctx, err := Executor{}.Setup(runCtx, nil)
	require.NoError(t, err)

	cancel() // Ctrl-C
	require.NoError(t, Executor{}.TearDown(ctx))
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID))
}

func TestFR02_CloseFailures(t *testing.T) {
	t.Run("a session the server forgot is closed", func(t *testing.T) {
		f := newFakeServer(t)
		f.on("DELETE "+sessionByID, fail(http.StatusNotFound, "invalid session id", "gone"))
		require.NoError(t, Executor{}.TearDown(openCase(t, f.baseVars())))
	})
	t.Run("any other failure is reported", func(t *testing.T) {
		f := newFakeServer(t)
		f.on("DELETE "+sessionByID, fail(http.StatusInternalServerError, "unknown error", "disk full"))
		err := Executor{}.TearDown(openCase(t, f.baseVars()))
		require.ErrorContains(t, err, "disk full")
	})
}

func TestFR16_CallHangFailsTheStepAtItsLimit(t *testing.T) {
	shorten(t, &callTimeout, 150*time.Millisecond)
	shorten(t, &diagnosticsTimeout, 150*time.Millisecond)
	f := newFakeServer(t)
	f.on("GET "+sourcePath, f.hang())
	withAction(t, "slow", callAction(http.MethodGet, "/source"))

	ctx := openCase(t, f.baseVars())
	within(t, 10*time.Second, func() {
		result := runStep(ctx, t, "slow")
		assert.Contains(t, result.Err, "no answer from the automation server within 150ms")
	})
	require.NoError(t, Executor{}.TearDown(ctx))
}

// defaults are read at package initialisation, before any test shortens them.
var defaults = struct{ call, open time.Duration }{callTimeout, sessionOpenTimeout}

func TestAC10_DefaultLimits(t *testing.T) {
	assert.Equal(t, 60*time.Second, defaults.call)
	assert.Equal(t, 180*time.Second, defaults.open)
	assert.Equal(t, 120, defaultIdleTimeout)
}

func TestFR16_SessionOpenHangFailsAtItsLimit(t *testing.T) {
	shorten(t, &sessionOpenTimeout, 150*time.Millisecond)
	f := newFakeServer(t)
	f.on("POST "+sessionRoot, f.hang())

	within(t, 10*time.Second, func() {
		_, err := Executor{}.Setup(testCtx(f.baseVars()), nil)
		require.ErrorContains(t, err, "no answer from the automation server within 150ms")
		assert.Contains(t, err.Error(), testDevice)
	})
}

func TestFR02_TearDownStopsACallInFlight(t *testing.T) {
	f := newFakeServer(t)
	entered := make(chan struct{})
	hang := f.hang()
	f.on("GET "+sourcePath, func(w http.ResponseWriter, r *http.Request, body []byte) {
		close(entered)
		hang(w, r, body)
	})
	withAction(t, "slow", callAction(http.MethodGet, "/source"))
	ctx := openCase(t, f.baseVars())

	type outcome struct {
		out interface{}
		err error
	}
	stepDone := make(chan outcome, 1)
	go func() {
		out, err := Executor{}.Run(ctx, venom.TestStep{"action": "slow"})
		stepDone <- outcome{out, err}
	}()
	<-entered

	within(t, 10*time.Second, func() { require.NoError(t, Executor{}.TearDown(ctx)) })
	done := <-stepDone
	require.NoError(t, done.err)
	result, ok := done.out.(Result)
	require.True(t, ok)
	assert.Contains(t, result.Err, "cancelled")
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID))
}

func TestFR02_ServerOfAnotherPlatformIsClosedAgain(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST "+sessionRoot, reply(http.StatusOK, `{"sessionId":"s9","capabilities":{"platformName":"iOS"}}`))

	_, err := Executor{}.Setup(testCtx(f.baseVars()), nil)
	require.ErrorContains(t, err, "expected android")
	assert.Equal(t, 1, f.count(http.MethodDelete, sessionByID), "Setup leaves no half-open session")
}

func TestNFR09_LogsCarryNoBody(t *testing.T) {
	logs := captureLogs(t)
	f := newFakeServer(t)
	f.on("POST /session/{id}/value", reply(http.StatusOK, `null`))
	secret := "typed-" + t.Name()
	withAction(t, "type", func(ctx context.Context, s *session, _ Executor, _ *Result) error {
		_, err := s.call(ctx, http.MethodPost, "/value", map[string]string{"text": secret})
		return err
	})

	ctx := openCase(t, f.baseVars())
	assert.Empty(t, runStep(ctx, t, "type").Err)
	require.NoError(t, Executor{}.TearDown(ctx))

	text := logs.text()
	assert.Contains(t, text, "mobile POST /session/"+testSessionID+"/value")
	assert.NotContains(t, text, secret)
}

func TestNFR09_DashVLogsEachCallAtInfo(t *testing.T) {
	logs := captureLogs(t)
	logrus.SetLevel(logrus.InfoLevel) // -v: Info, not Debug
	f := newFakeServer(t)
	f.on("POST /session/{id}/value", reply(http.StatusOK, `null`))
	secret := "typed-" + t.Name()
	withAction(t, "type", func(ctx context.Context, s *session, _ Executor, _ *Result) error {
		_, err := s.call(ctx, http.MethodPost, "/value", map[string]string{"text": secret})
		return err
	})

	ctx := openCase(t, f.baseVars())
	assert.Empty(t, runStep(ctx, t, "type").Err)
	require.NoError(t, Executor{}.TearDown(ctx))

	text := logs.text()
	assert.Regexp(t, `mobile POST /session/`+testSessionID+`/value \d+ms`, text)
	assert.NotContains(t, text, secret)
	assert.Equal(t, 1, strings.Count(text, "mobile session opened"), "the facts line is logged once")
}

func TestFR15_RedirectIsNotFollowed(t *testing.T) {
	f := newFakeServer(t)
	f.on("GET "+sourcePath, func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	})
	withAction(t, "go", callAction(http.MethodGet, "/source"))
	ctx := openCase(t, f.baseVars())

	assert.Contains(t, runStep(ctx, t, "go").Err, "http 302")
	assert.Equal(t, 0, f.count(http.MethodGet, "/elsewhere"))
	require.NoError(t, Executor{}.TearDown(ctx))
}

func TestFR15_UsageErrorFromAnActionStaysAGoError(t *testing.T) {
	f := newFakeServer(t)
	withAction(t, "bad", func(context.Context, *session, Executor, *Result) error {
		return usagef("two locators")
	})
	ctx := openCase(t, f.baseVars())

	_, err := Executor{}.Run(ctx, venom.TestStep{"action": "bad"})
	var misuse *usageError
	require.True(t, errors.As(err, &misuse))
}
