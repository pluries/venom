package web

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of R16 (waiting for the navigations a step starts, as chromedriver did) and R17 (limits on the calls
// Playwright sends without a timeout), on the fake seam (docs/feat/2047/research.md). Their real-browser counterparts
// are in web_playwright_test.go.

const navScript = "location.href='https://example.test/next'"

func executeStep(command string) venom.TestStep {
	return action(map[string]interface{}{"execute": map[string]interface{}{"command": command}})
}

// executeCall is the call an execute step sends to frame, as the fake logs it.
func executeCall(frame, command string) string {
	return fmt.Sprintf("Evaluate %s (args) => (function(){\n%s\n}).apply(null, args) []string{}", frame, command)
}

// later runs fn on its own goroutine after a short delay: an event of the page that arrives while the step waits for
// the navigation. The assertions hold whether it arrives before or during that wait; the delay only makes the wait
// itself the usual case.
func later(fn func()) {
	go func() {
		time.Sleep(20 * time.Millisecond)
		fn()
	}()
}

// openFakeWith is openFake with suite variables.
func openFakeWith(t *testing.T, pw *fakePW, vars map[string]any) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())
	ctx, err := setupFake(t, vars)
	require.NoError(t, err)
	pw.reset()
	return ctx
}

// assertAwaited checks that the step waited for its navigation to end, with event, before it waited for the frame's
// load, and that it sent no evaluation to the navigating frame: Playwright would have held it until the commit, beyond
// web.timeout.
func assertAwaited(t *testing.T, seen []string, frame, event string) {
	t.Helper()
	end, load := slices.Index(seen, event), slices.Index(seen, "WaitForLoadState "+frame)
	require.NotEqual(t, -1, end, "no %q in %v", event, seen)
	assert.Less(t, end, load, "the load is awaited once the navigation ended: %v", seen)
	for _, c := range seen {
		assert.NotContains(t, c, " held", "an evaluation was sent to the navigating frame: %v", seen)
	}
}

func TestRun_navigations(t *testing.T) {
	exec := executeCall("page1", navScript)

	t.Run("a navigation the script requests is awaited until it commits", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[exec] = func() {
			pw.requestNavigation(pw.mainFrame(), "nav")
			later(func() { pw.commitNavigation(pw.mainFrame()) })
		}

		res, err := webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assert.Equal(t, pageResult(), res)
		assertAwaited(t, pw.seen(), "page1", "commit page1")
		assert.Equal(t, pageTail(), pw.seen()[len(pw.seen())-4:], "the page is read once it has settled")
	})

	t.Run("a navigation whose request fails ends the wait", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[exec] = func() {
			nav := pw.requestNavigation(pw.mainFrame(), "nav")
			later(func() { pw.failRequest(nav) })
		}

		_, err := webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assertAwaited(t, pw.seen(), "page1", "fail nav")
	})

	t.Run("a redirect replaces the request: the first one's failure does not end the wait", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[exec] = func() {
			first := pw.requestNavigation(pw.mainFrame(), "nav")
			pw.requestNavigation(pw.mainFrame(), "redirect")
			pw.failRequest(first)
			later(func() { pw.commitNavigation(pw.mainFrame()) })
		}

		_, err := webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assertAwaited(t, pw.seen(), "page1", "commit page1")
	})

	t.Run("a navigation a zero-delay timer requests during the flush is awaited", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		var once sync.Once
		pw.hooks["Evaluate page1 flushTimers"] = func() {
			once.Do(func() {
				pw.requestNavigation(pw.mainFrame(), "timer")
				later(func() { pw.commitNavigation(pw.mainFrame()) })
			})
		}

		_, err := webRun(ctx, t, executeStep("return 1;"))
		require.NoError(t, err)
		assertAwaited(t, pw.seen(), "page1", "commit page1")
	})

	t.Run("a request event arriving after the flush was sent ends the flush's wait: the commit is awaited within web.timeout", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFakeWith(t, pw, map[string]any{"timeout": 1})
		prev := readTimeout
		readTimeout = 5 * time.Second
		t.Cleanup(func() { readTimeout = prev })
		var nav *fakeRequest
		pw.hooks[exec] = func() { nav = pw.startNavigation(pw.mainFrame(), "nav") }
		pw.hooks["Evaluate page1 flushTimers held"] = func() { pw.send(nav) }

		start := time.Now()
		_, err := webRun(ctx, t, executeStep(navScript))
		require.EqualError(t, err, "the page did not commit its navigation within web.timeout (1s)")
		assert.Less(t, time.Since(start), 4*time.Second, "within web.timeout, not the flush's read limit")
	})

	t.Run("in an iframe, the iframe's navigation is awaited", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.nodes["css=iframe"] = []fakeNode{{frame: true}}
		_, err := webRun(ctx, t, action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("iframe")}}))
		require.NoError(t, err)
		pw.hooks[executeCall("css=iframe", navScript)] = func() {
			pw.requestNavigation(pw.iframe("css=iframe"), "inner")
			later(func() { pw.commitNavigation(pw.iframe("css=iframe")) })
		}
		pw.reset()

		_, err = webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assertAwaited(t, pw.seen(), "css=iframe", "commit css=iframe")
	})

	t.Run("a resource request and another page's navigation do not hold the step", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[exec] = func() {
			pw.sendResource(pw.mainFrame(), "image")
			pw.requestNavigation(pw.addPage().main, "elsewhere")
		}

		_, err := webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assert.Equal(t, settled(exec, "request image", "request elsewhere"), pw.seen())
	})

	t.Run("a navigation that never commits fails its step after web.timeout, and the next steps until a navigate ends it", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFakeWith(t, pw, map[string]any{"timeout": 1})
		prev := readTimeout
		readTimeout = 2 * time.Second
		t.Cleanup(func() { readTimeout = prev })
		pw.hooks[exec] = func() { pw.requestNavigation(pw.mainFrame(), "nav") }

		start := time.Now()
		_, err := webRun(ctx, t, executeStep(navScript))
		require.EqualError(t, err, "the page did not commit its navigation within web.timeout (1s)")
		assert.Less(t, time.Since(start), 4*time.Second, "no page dump while the page navigates: Playwright would hold its read")
		assert.NoFileExists(t, "page.dump.html")

		pw.reset()
		_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.EqualError(t, err, "the page did not commit its navigation within web.timeout (1s)", "the browser still navigates")
		assert.NotContains(t, pw.seen(), "Count css=.item", "the find waits for the navigation before its first call (R19)")

		pw.reset()
		_, err = webRun(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}}))
		require.NoError(t, err)
		assert.Equal(t, "fail nav", pw.seen()[0], "the browser aborts the navigation for the new one")
	})

	t.Run("an action waits for the navigation its frame still has in flight before its first call", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.requestNavigation(pw.mainFrame(), "late")
		later(func() { pw.commitNavigation(pw.mainFrame()) })

		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.NoError(t, err)
		seen := pw.seen()
		assert.Less(t, slices.Index(seen, "commit page1"), slices.Index(seen, "Count css=.item"), "the count is sent once the navigation committed: %v", seen)
		for _, c := range seen {
			assert.NotContains(t, c, " held", "a call was sent to the navigating frame: %v", seen)
		}
	})

	t.Run("a dialog opening while the commit is awaited is left pending", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[exec] = func() {
			pw.requestNavigation(pw.mainFrame(), "nav")
			later(func() { pw.openDialog("confirm") })
		}

		res, err := webRun(ctx, t, executeStep(navScript))
		require.NoError(t, err)
		assert.Equal(t, Result{}, res, "no title nor URL while the dialog is open")
		assert.Equal(t, []string{exec, "request nav"}, pw.seen())
		assert.True(t, getWebCtx(ctx).dialogPending())
	})
}

func TestAction_callsFrame(t *testing.T) {
	// R19: the actions that wait for a navigation in flight before they run, in performAction's order.
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{"click", Action{Click: &Click{}}, true},
		{"fill", Action{Fill: []Fill{{}}}, true},
		{"find", Action{Find: ".item"}, true},
		{"navigate", Action{Navigate: &Navigate{}}, false},
		{"wait", Action{Wait: 1}, false},
		{"confirmPopup", Action{ConfirmPopup: true}, false},
		{"cancelPopup", Action{CancelPopup: true}, false},
		{"select", Action{Select: &Select{}}, true},
		{"uploadFile", Action{UploadFile: &UploadFile{}}, true},
		{"selectFrame", Action{SelectFrame: &SelectFrame{}}, true},
		{"selectRootFrame", Action{SelectRootFrame: true}, false},
		{"nextWindow", Action{NextWindow: true}, false},
		{"historyAction", Action{HistoryAction: "back"}, false},
		{"execute", Action{Execute: &Execute{}}, true},
		{"no action", Action{}, false},
		{"navigate runs before select", Action{Navigate: &Navigate{}, Select: &Select{}}, false},
		{"historyAction runs before execute", Action{HistoryAction: "back", Execute: &Execute{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.callsFrame())
		})
	}
}

// shortLimits shortens the limits of R17 for the test.
func shortLimits(t *testing.T) {
	t.Helper()
	prevRead, prevScript := readTimeout, scriptTimeout
	readTimeout, scriptTimeout = 50*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { readTimeout, scriptTimeout = prevRead, prevScript })
}

// blockUntilClosed is a hook that never returns while the test runs: a page whose main thread is stuck.
func blockUntilClosed(pw *fakePW) func() {
	return func() { <-pw.closed }
}

func TestRun_callLimits(t *testing.T) {
	t.Run("a title read the page never answers fails the step, the call left in flight", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Title page1"] = blockUntilClosed(pw)

		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)
		assert.EqualError(t, err, "the browser did not answer: the title read did not return within 0.05s")
	})

	t.Run("an execute the page never finishes fails after the script limit", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[executeCall("page1", "return new Promise(() => {});")] = blockUntilClosed(pw)

		_, err := webRun(ctx, t, executeStep("return new Promise(() => {});"))
		require.ErrorIs(t, err, errNoAnswer)
		assert.EqualError(t, err, "the browser did not answer: the execute script did not return within 0.05s")
	})

	t.Run("a stuck timer flush fails the step", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Evaluate page1 flushTimers"] = blockUntilClosed(pw)

		_, err := webRun(ctx, t, executeStep("return 1;"))
		require.ErrorIs(t, err, errNoAnswer)
		assert.ErrorContains(t, err, "the timer flush did not return within 0.05s")
	})

	// A script that opens a dialog, then never returns once it is answered.
	stuckAfterDialog := func(pw *fakePW) func() {
		return func() {
			d := pw.openDialog("alert")
			select {
			case <-d.answered:
			case <-pw.closed:
				return
			}
			<-pw.closed
		}
	}

	t.Run("the script a dialog interrupted must finish within its limit once the dialog is answered", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[executeCall("page1", "alert('x');")] = stuckAfterDialog(pw)

		res, err := webRun(ctx, t, executeStep("alert('x');"))
		require.NoError(t, err)
		assert.Equal(t, Result{}, res)

		_, err = webRun(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
		require.ErrorIs(t, err, errNoAnswer)
		assert.ErrorContains(t, err, "the execute script did not return within 0.05s")
	})

	t.Run("TearDown is bounded when the interrupted call never returns", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks[executeCall("page1", "alert('x');")] = stuckAfterDialog(pw)
		_, err := webRun(ctx, t, executeStep("alert('x');"))
		require.NoError(t, err)

		done := make(chan error, 1)
		go func() { done <- Executor{}.TearDown(ctx) }()
		select {
		case err := <-done:
			require.ErrorIs(t, err, errNoAnswer, "the dismissed dialog's script is reported, the rest still closed")
			assert.Contains(t, pw.seen(), "Close context")
		case <-time.After(5 * time.Second):
			t.Fatal("TearDown did not return")
		}
	})

	t.Run("a step the engine abandoned does not overlap the next one", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		release := make(chan struct{})
		pw.hooks["Title page1"] = func() { <-release }

		first := make(chan error, 1)
		go func() {
			_, err := Executor{}.Run(ctx, action(map[string]interface{}{"find": findCSS(".item")}))
			first <- err
		}()
		require.Eventually(t, func() bool { return slices.Contains(pw.seen(), "Title page1") }, 5*time.Second, 5*time.Millisecond)

		second := make(chan error, 1)
		go func() {
			_, err := Executor{}.Run(ctx, action(map[string]interface{}{"find": findCSS(".none")}))
			second <- err
		}()
		assert.Never(t, func() bool { return slices.Contains(pw.seen(), "Count css=.none") }, 100*time.Millisecond, 5*time.Millisecond,
			"the next step waits for the abandoned one")

		close(release)
		require.NoError(t, <-first)
		require.NoError(t, <-second)
		assert.Contains(t, pw.seen(), "Count css=.none")
	})

	t.Run("a call whose limit expired is still running: the next step waits for it, within its own limit", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		release := make(chan struct{})
		var running, overlaps, blocked atomic.Int32
		pw.hooks["Title page1"] = func() {
			if running.Add(1) > 1 {
				overlaps.Add(1)
			}
			defer running.Add(-1)
			if blocked.Add(1) == 1 { // only the first read is stuck
				<-release
			}
		}

		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)

		_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer, "the next step's own limit passed while the abandoned call ran")
		assert.Zero(t, overlaps.Load(), "the next step's call started while the abandoned one still ran")
		assert.EqualValues(t, 1, blocked.Load(), "the next step sent nothing to the page")

		close(release)
		require.Eventually(t, func() bool { return running.Load() == 0 }, 5*time.Second, time.Millisecond)
		_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.NoError(t, err, "once the abandoned call finished, steps run normally")
	})

	t.Run("the next step's stop ends its wait for the abandoned call", func(t *testing.T) {
		stop := make(chan struct{})
		close(stop)
		w := &WebContext{abandoned: make(chan struct{})}
		err := w.awaitAbandoned(callLimit{call: "x", stop: stop})
		require.ErrorIs(t, err, errStopped)
		assert.NotNil(t, w.abandoned, "the abandoned call stays remembered")
	})

	t.Run("a call a stop ended is not remembered: it is left in flight by design", func(t *testing.T) {
		stop := make(chan struct{})
		close(stop)
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		w := &WebContext{}
		_, err := run(w, callLimit{call: "x", stop: stop}, func() (int, error) { <-release; return 0, nil })
		require.ErrorIs(t, err, errStopped)
		assert.Nil(t, w.abandoned)
	})

	t.Run("a navigate does not wait for an abandoned call", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		release := make(chan struct{})
		pw.hooks["Title page1"] = func() { <-release }
		// The navigation ends what the old document held: the abandoned call returns once it starts.
		pw.hooks["Goto page1 https://example.test/next"] = func() { close(release) }
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)

		_, err = webRun(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}}))
		require.NoError(t, err)
		assert.Contains(t, pw.seen(), "Goto page1 https://example.test/next")
	})

	t.Run("the failure dump is skipped while a call is abandoned", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Count css=.item"] = blockUntilClosed(pw)
		var err error
		logged := stderrOf(t, func() {
			_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		})
		require.ErrorIs(t, err, errNoAnswer)
		assert.Contains(t, logged, "page.dump.html is not written")
		assert.NotContains(t, pw.seen(), "Content page1")
		assert.NotContains(t, logged, dumpWarning)
		assert.NoFileExists(t, "page.dump.html")
	})

	// abandon expires a find whose count the page never answers: the call stays in flight until the test ends.
	abandon := func(t *testing.T) (*fakePW, context.Context) {
		t.Helper()
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Count css=.item"] = blockUntilClosed(pw)
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)
		return pw, ctx
	}
	// within fails the test, instead of hanging it, when a step does not return.
	within := func(t *testing.T, ctx context.Context, step venom.TestStep) error {
		t.Helper()
		done := make(chan error, 1)
		go func() { _, err := Executor{}.Run(ctx, step); done <- err }()
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("the step did not return")
			return nil
		}
	}

	t.Run("a call with no limit of its own waits for the abandoned call only within the read limit", func(t *testing.T) {
		pw, ctx := abandon(t)
		err := within(t, ctx, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}}))
		require.NoError(t, err, "the wait ends and the call goes on")
		assert.Contains(t, pw.seen(), "Click css=#c first")
	})

	t.Run("a history move does not wait for an abandoned call", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		release := make(chan struct{})
		pw.hooks["Count css=.item"] = func() { <-release }
		// The move ends what the old document held: the abandoned call returns once it starts.
		pw.hooks["Reload page1"] = func() { close(release) }
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)

		readTimeout = 10 * time.Second // the bound of a wait: only a call that does not wait passes within 5 s
		err = within(t, ctx, action(map[string]interface{}{"historyAction": "refresh"}))
		require.NoError(t, err)
		assert.Contains(t, pw.seen(), "Reload page1")
	})

	t.Run("a dialog opened after the call expired is dismissed, not waited for", func(t *testing.T) {
		pw, ctx := abandon(t)
		pw.openDialog("alert")
		err := within(t, ctx, action(map[string]interface{}{"find": findCSS(".other")}))
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.NotContains(t, pw.seen(), "Count css=.other")
	})

	t.Run("a finished abandoned call does not suppress the failure dump", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		w := getWebCtx(ctx)
		finished := make(chan struct{})
		close(finished)
		w.abandoned = finished

		require.NoError(t, generateErrorHTMLFile(ctx, w, "page"))
		assert.Contains(t, pw.seen(), "Content page1")
		assert.FileExists(t, "page.dump.html")
		assert.Nil(t, w.abandoned)
	})

	t.Run("TearDown does not wait for an abandoned call", func(t *testing.T) {
		shortLimits(t)
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Title page1"] = blockUntilClosed(pw)
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errNoAnswer)

		require.NoError(t, Executor{}.TearDown(ctx))
		assert.Contains(t, pw.seen(), "Close context")
	})
}
