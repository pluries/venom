package web

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of FR-08, the trace of a failing step (docs/feat/2048/research.md R8, R9; validation.md V7-V9), on the
// fake seam: the tracing calls as the fake logs them. Their real-browser counterpart is in web_playwright_test.go.

const (
	traceStart   = "Tracing Start screenshots=true snapshots=true"
	traceDiscard = "Tracing StopChunk"
	nextChunk    = "Tracing StartChunk"
	traceOff     = "web - the trace is turned off for this test case: "
	savedIn      = "Trace of the failing step is saved in "
)

// errStepFailure is a step's own failure, as opposed to the trace's.
var errStepFailure = errors.New("synthetic failure")

// tracedFake is a new fake with a session open on it, web.trace on (openFake's working directory and reset).
func tracedFake(t *testing.T) (*fakePW, context.Context) {
	t.Helper()
	pw := withFake(t)
	return pw, openFakeWith(t, pw, map[string]any{"trace": true})
}

// goClick is a click on #go; failGoClick makes it fail with errStepFailure.
func goClick() venom.TestStep {
	return action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#go")}})
}

func failGoClick(pw *fakePW) { pw.errs["Click css=#go first"] = errStepFailure }

// tracePath is where a failing step's trace goes: page.trace.zip in the working directory, as an absolute path.
func tracePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs("page.trace.zip")
	require.NoError(t, err)
	return path
}

// traceSave is the call that saves a failing step's chunk to tracePath.
func traceSave(t *testing.T) string {
	t.Helper()
	return traceDiscard + " " + tracePath(t)
}

// tracingCalls are the tracing calls in seen.
func tracingCalls(seen []string) []string {
	var calls []string
	for _, c := range seen {
		if strings.HasPrefix(c, "Tracing ") {
			calls = append(calls, c)
		}
	}
	return calls
}

func TestSetup_trace(t *testing.T) {
	t.Run("web.trace starts tracing with screenshots and snapshots, once the context is set up and before the page opens", func(t *testing.T) {
		pw := withFake(t)
		ctx, err := setupFake(t, map[string]any{"trace": true})
		require.NoError(t, err)
		assert.Equal(t, []string{
			"Run", "Launch chromium", "NewContext", "OnDialog",
			"OnRequest", "OnRequestFailed", "OnFrameNavigated",
			"SetDefaultTimeout 30000", "SetDefaultNavigationTimeout 180000", traceStart, "NewPage",
		}, pw.seen())
		assert.NotNil(t, getWebCtx(ctx).tracing)
	})

	t.Run("without web.trace, or with it false, nothing is traced", func(t *testing.T) {
		// R9: tracing is opt-in; TestSetup_session and TestRun_actions pin the exact calls without it.
		for name, vars := range map[string]map[string]any{"unset": nil, "false": {"trace": false}} {
			t.Run(name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFakeWith(t, pw, vars)
				assert.Nil(t, getWebCtx(ctx).tracing)
				_, err := webRun(ctx, t, venom.TestStep{})
				require.NoError(t, err)
				failGoClick(pw)
				_, err = webRun(ctx, t, goClick())
				require.Error(t, err)
				assert.Empty(t, tracingCalls(pw.seen()))
			})
		}
	})

	t.Run("a refused start fails Setup and closes the context, the browser and the driver", func(t *testing.T) {
		pw := withFake(t)
		pw.errs[traceStart] = errors.New("synthetic failure")
		ctx, err := setupFake(t, map[string]any{"trace": true})
		require.EqualError(t, err, "unable to start the trace: synthetic failure")
		assert.Nil(t, getWebCtx(ctx))
		calls := pw.seen()
		assert.Equal(t, []string{traceStart, "Close context", "Close browser", "Stop driver"}, calls[slices.Index(calls, traceStart):])
	})
}

func TestRun_tracePassing(t *testing.T) {
	t.Run("a passing step discards its chunk, then opens the next one, after the page read", func(t *testing.T) {
		pw, ctx := tracedFake(t)
		var got Result
		var err error
		logged := stderrOf(t, func() { got, err = webRun(ctx, t, venom.TestStep{}) })
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Equal(t, append(pageTail(), traceDiscard, nextChunk), pw.seen())
		assert.NotContains(t, logged, savedIn)
		assert.NoFileExists(t, "page.dump.html")

		pw.reset()
		_, err = webRun(ctx, t, venom.TestStep{})
		require.NoError(t, err)
		assert.Equal(t, []string{traceDiscard, nextChunk}, tracingCalls(pw.seen()), "every step has its chunk")
	})
}

func TestRun_traceFailing(t *testing.T) {
	t.Run("a failing step saves its chunk next to the page dump, and keeps its own error", func(t *testing.T) {
		pw, ctx := tracedFake(t)
		failGoClick(pw)
		var got interface{}
		var err error
		logged := stderrOf(t, func() { got, err = Executor{}.Run(ctx, goClick()) })
		require.EqualError(t, err, errStepFailure.Error())
		if got != nil {
			t.Errorf("a failed step returns an untyped nil, got %#v", got)
		}
		calls := pw.seen()
		require.GreaterOrEqual(t, len(calls), 3)
		assert.Equal(t, []string{"Content page1", traceSave(t), nextChunk}, calls[len(calls)-3:], "the dump, then the trace")
		assert.FileExists(t, "page.dump.html")
		assert.Contains(t, logged, savedIn+tracePath(t))
		assert.Contains(t, logged, "Content of the HTML page is saved in page.dump.html")
	})

	t.Run("the trace holds what was typed: only its owner reads it, as the page dump, even over an earlier run's file", func(t *testing.T) {
		for name, earlier := range map[string]bool{"a new file": false, "an earlier run's file": true} {
			t.Run(name, func(t *testing.T) {
				pw, ctx := tracedFake(t)
				if earlier {
					require.NoError(t, os.WriteFile("page.trace.zip", nil, 0o600))
					require.NoError(t, os.Chmod("page.trace.zip", 0o644), "the mode the driver's own create leaves")
				}
				failGoClick(pw)
				_, err := webRun(ctx, t, goClick())
				require.EqualError(t, err, errStepFailure.Error())
				info, err := os.Stat("page.trace.zip")
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			})
		}
	})

	t.Run("a trace file that cannot be created turns tracing off; the step keeps its own error", func(t *testing.T) {
		pw, ctx := tracedFake(t)
		require.NoError(t, os.Mkdir("page.trace.zip", 0o700))
		failGoClick(pw)
		var err error
		logged := stderrOf(t, func() { _, err = webRun(ctx, t, goClick()) })
		require.EqualError(t, err, errStepFailure.Error())
		assert.Contains(t, logged, traceOff+"unable to create the trace file: ")
		assert.NotContains(t, tracingCalls(pw.seen()), traceSave(t), "no chunk is saved to a file venom did not create")
		assert.Nil(t, getWebCtx(ctx).tracing)
	})
}

// failurePath is a way a step fails: the step, what makes it fail, its error and whether page.dump.html is written.
type failurePath struct {
	name     string
	step     venom.TestStep
	prepare  func(pw *fakePW)
	wantErr  string
	wantDump bool
}

// failurePaths are the ways a step fails, from its decoding to its page read.
func failurePaths(t *testing.T) []failurePath {
	t.Helper()
	// The screenshot is written into a directory that does not exist: the error is the one os.WriteFile gives there.
	missing := filepath.Join(t.TempDir(), "missing", "shot.png")
	errWrite := os.WriteFile(missing, nil, 0o600)
	require.Error(t, errWrite)
	shot := venom.TestStep{"screenshot": filepath.Join(t.TempDir(), "shot.png")}
	return []failurePath{
		{
			name:    "the step does not decode",
			step:    venom.TestStep{"mock": "not a list"},
			wantErr: "1 error(s) decoding:\n\n* 'Mock': source data must be an array or slice, got string",
		},
		{
			name:     "an invalid mock",
			step:     mocked(venom.TestStep{}, map[string]interface{}{"status": 200}),
			wantErr:  "mock 1 of 1: url is required",
			wantDump: true,
		},
		{
			name:     "the action",
			step:     goClick(),
			prepare:  failGoClick,
			wantErr:  errStepFailure.Error(),
			wantDump: true,
		},
		{
			// FR-09: no dump while the page navigates; the trace is saved all the same.
			name: "the action, while the page navigates",
			step: goClick(),
			prepare: func(pw *fakePW) {
				failGoClick(pw)
				pw.hooks["Click css=#go first"] = func() { pw.requestNavigation(pw.mainFrame(), "nav") }
			},
			wantErr: errStepFailure.Error(),
		},
		{
			name:    "the screenshot",
			step:    shot,
			prepare: func(pw *fakePW) { pw.errs["Screenshot page1"] = errStepFailure },
			wantErr: errStepFailure.Error(),
		},
		{
			name:    "the screenshot's write",
			step:    venom.TestStep{"screenshot": missing},
			wantErr: errWrite.Error(),
		},
		{
			name:    "the page dump after the screenshot",
			step:    shot,
			prepare: func(pw *fakePW) { pw.errs["Content page1"] = errStepFailure },
			wantErr: errStepFailure.Error(),
		},
		{
			name:    "the page read",
			step:    venom.TestStep{},
			prepare: func(pw *fakePW) { pw.errs["Title page1"] = errStepFailure },
			wantErr: errStepFailure.Error(),
		},
	}
}

func TestRun_traceFailurePaths(t *testing.T) {
	t.Run("every way a step fails saves its chunk", func(t *testing.T) {
		for _, tt := range failurePaths(t) {
			t.Run(tt.name, func(t *testing.T) {
				pw, ctx := tracedFake(t)
				if tt.prepare != nil {
					tt.prepare(pw)
				}
				_, err := webRun(ctx, t, tt.step)
				require.EqualError(t, err, tt.wantErr)
				calls := pw.seen()
				require.GreaterOrEqual(t, len(calls), 2)
				assert.Equal(t, []string{traceSave(t), nextChunk}, calls[len(calls)-2:])
				assert.Equal(t, []string{traceSave(t), nextChunk}, tracingCalls(calls), "one chunk per step")
				if tt.wantDump {
					assert.FileExists(t, "page.dump.html")
				} else {
					assert.NoFileExists(t, "page.dump.html")
				}
			})
		}
	})
}

func TestRun_traceDialog(t *testing.T) {
	t.Run("a step that leaves a confirm open passes, its chunk ends with the dialog still pending", func(t *testing.T) {
		// R8: tracing calls are not page commands: they neither dismiss the dialog nor wait on it.
		pw, ctx := tracedFake(t)
		clicked := newFlag()
		pw.hooks["Click css=#c first"] = pw.blockOnDialog("confirm", 1, clicked)
		got, err := webRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}}))
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		assert.Equal(t, []string{"Click css=#c first", traceDiscard, nextChunk}, pw.seen(), "no Dismiss")
		assert.False(t, clicked.isSet())

		pw.reset()
		got, err = webRun(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.True(t, clicked.isSet(), "the next step answered the dialog the click left open")
		assert.Equal(t, append(settled("Accept confirm1"), traceDiscard, nextChunk), pw.seen())
	})
}

// failStep makes the step without action fail on its own, at its page read, when fails is set.
func failStep(pw *fakePW, fails bool) {
	if fails {
		pw.errs["Title page1"] = errStepFailure
	}
}

func TestRun_traceCallFailures(t *testing.T) {
	t.Run("a tracing call that fails turns tracing off for the test case with one warning; the step's outcome stands", func(t *testing.T) {
		tests := []struct {
			name string
			// failing is the tracing call that fails, as the fake logs it.
			failing func(t *testing.T) string
			// stepFails makes the step fail on its own: its chunk is the one saved.
			stepFails bool
			// wantCalls are the tracing calls of the step.
			wantCalls func(t *testing.T) []string
		}{
			{
				name:      "the discard of a passing step's chunk",
				failing:   func(*testing.T) string { return traceDiscard },
				wantCalls: func(*testing.T) []string { return []string{traceDiscard} },
			},
			{
				name:      "the save of a failing step's chunk",
				failing:   traceSave,
				stepFails: true,
				wantCalls: func(t *testing.T) []string { return []string{traceSave(t)} },
			},
			{
				name:      "the next chunk's start",
				failing:   func(*testing.T) string { return nextChunk },
				wantCalls: func(*testing.T) []string { return []string{traceDiscard, nextChunk} },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pw, ctx := tracedFake(t)
				pw.errs[tt.failing(t)] = errors.New("synthetic trace failure")
				failStep(pw, tt.stepFails)
				wantCalls := tt.wantCalls(t)
				var got Result
				var err, errPass, errFail error
				var calls, after []string
				logged := stderrOf(t, func() {
					got, err = webRun(ctx, t, venom.TestStep{})
					calls = pw.seen()
					// The following steps, one passing, one failing.
					pw.reset()
					delete(pw.errs, "Title page1")
					_, errPass = webRun(ctx, t, venom.TestStep{})
					failGoClick(pw)
					_, errFail = webRun(ctx, t, goClick())
					after = pw.seen()
				})
				assert.Equal(t, wantCalls, tracingCalls(calls))
				require.NoError(t, errPass)
				require.EqualError(t, errFail, errStepFailure.Error())
				if tt.stepFails {
					require.EqualError(t, err, errStepFailure.Error(), "the step's own error, not the trace's")
					assert.NotContains(t, logged, savedIn)
				} else {
					require.NoError(t, err)
					assert.Equal(t, pageResult(), got)
				}
				assert.Equal(t, 1, strings.Count(logged, traceOff+"synthetic trace failure"), "one warning in:\n%s", logged)
				assert.Equal(t, 1, strings.Count(logged, traceOff))
				assert.Empty(t, tracingCalls(after), "no tracing call in the following steps")
				assert.Nil(t, getWebCtx(ctx).tracing)
			})
		}
	})
}

func TestRun_traceBounded(t *testing.T) {
	t.Run("a tracing call the browser never answers is bounded by the read limit, then tracing is off", func(t *testing.T) {
		tests := []struct {
			name      string
			blocking  func(t *testing.T) string
			stepFails bool
			wantLimit string
		}{
			{"the discard", func(*testing.T) string { return traceDiscard }, false, "the trace discard"},
			{"the save", traceSave, true, "the trace save"},
			{"the next start", func(*testing.T) string { return nextChunk }, false, "the trace start"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				shortLimits(t)
				pw, ctx := tracedFake(t)
				// The call returns at the end of the test, as on a closed target: no goroutine outlives it.
				pw.hooks[tt.blocking(t)] = blockUntilClosed(pw)
				failStep(pw, tt.stepFails)
				var err error
				var elapsed time.Duration
				logged := stderrOf(t, func() {
					start := time.Now()
					_, err = webRun(ctx, t, venom.TestStep{})
					elapsed = time.Since(start)
				})
				if tt.stepFails {
					require.EqualError(t, err, errStepFailure.Error())
				} else {
					require.NoError(t, err)
				}
				assert.Less(t, elapsed, 2*time.Second, "the step is not held beyond the limit")
				assert.Contains(t, logged, traceOff+"the browser did not answer: "+tt.wantLimit+" did not return within 0.05s")
				assert.Nil(t, getWebCtx(ctx).tracing)

				pw.reset()
				delete(pw.errs, "Title page1")
				_, err = webRun(ctx, t, venom.TestStep{})
				require.NoError(t, err)
				assert.Empty(t, tracingCalls(pw.seen()))
			})
		}
	})
}

func TestRun_traceUnnamedFile(t *testing.T) {
	t.Run("a trace file that cannot be named turns tracing off", func(t *testing.T) {
		pw, ctx := tracedFake(t)
		dir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Remove(dir))
		if _, err := os.Getwd(); err == nil {
			t.Skip("this OS still names a working directory that was removed")
		}
		failGoClick(pw)
		var errStep error
		logged := stderrOf(t, func() { _, errStep = webRun(ctx, t, goClick()) })
		require.EqualError(t, errStep, errStepFailure.Error())
		assert.Contains(t, logged, traceOff+"unable to name the trace file: ")
		assert.Empty(t, tracingCalls(pw.seen()), "no chunk ended, no chunk started")
		assert.Nil(t, getWebCtx(ctx).tracing)
	})
}

func TestTearDown_trace(t *testing.T) {
	t.Run("TearDown closes the context with its chunk open: no tracing call", func(t *testing.T) {
		pw, ctx := tracedFake(t)
		_, err := webRun(ctx, t, venom.TestStep{})
		require.NoError(t, err)
		pw.reset()
		require.NoError(t, Executor{}.TearDown(ctx))
		assert.Equal(t, []string{"Close context", "Close browser", "Stop driver"}, pw.seen())
	})
}
