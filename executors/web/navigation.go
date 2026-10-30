package web

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// Waiting for the navigations a step starts (docs/feat/2047/research.md R16). chromedriver waited for pending
// navigations after every command and before every read. Playwright's actions wait only for a navigation they request
// themselves, an evaluation waits for none, and a load-state wait returns at once while the old document is still
// there: a script's `location.href = …`, or a timer a click handler set, left the step reading the old page, or
// Playwright's "Loading …" placeholder title. And Playwright holds an evaluation sent while the frame has a navigation
// in flight until the navigation commits, then fails it ("execution context was destroyed"); one already running goes
// on (measured on Chromium 151 with driver 1.62.1).

// jsFlushTimers resolves once the timers the page set with no delay have run: a navigation one of them starts has then
// been requested.
const jsFlushTimers = "() => new Promise(resolve => setTimeout(resolve, 0))"

// navigations follows the navigations of the context's frames, from its request and navigation events: Playwright's
// goroutine writes it and never blocks, settle reads it.
type navigations struct {
	mu sync.Mutex
	// inFlight is, per frame, its latest navigation request that has neither committed nor failed.
	inFlight map[frame]request
	// watched is the frame settle waits for, the current frame; events counts its navigation events.
	watched frame
	events  uint64
	// changed is closed, then replaced, at every navigation event of the watched frame.
	changed chan struct{}
}

func newNavigations() *navigations {
	return &navigations{inFlight: map[frame]request{}, changed: make(chan struct{})}
}

// listen registers the tracker on the context's events, once for all its pages.
func (n *navigations) listen(c browserContext) {
	c.onRequest(n.requested)
	c.onRequestFailed(n.failed)
	c.onFrameNavigated(n.committed)
}

// requested records a navigation request; the request a redirect sends replaces the first one.
func (n *navigations) requested(r request) {
	if !r.IsNavigationRequest() {
		return
	}
	f := r.frame()
	n.mu.Lock()
	defer n.mu.Unlock()
	n.inFlight[f] = r
	n.touch(f)
}

// failed ends a navigation whose request failed: stopped, turned into a download, or replaced by another navigation.
func (n *navigations) failed(r request) {
	if !r.IsNavigationRequest() {
		return
	}
	f := r.frame()
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.inFlight[f] != r {
		return // a navigation a later one replaced: the later one goes on
	}
	delete(n.inFlight, f)
	n.touch(f)
}

// committed ends the frame's navigation: the frame shows the new document, or the same one at a new URL.
func (n *navigations) committed(f frame) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.inFlight, f)
	n.touch(f)
}

func (n *navigations) touch(f frame) {
	if f != n.watched {
		return
	}
	n.events++
	close(n.changed)
	n.changed = make(chan struct{})
}

// watch makes f the frame state reports on.
func (n *navigations) watch(f frame) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.watched = f
}

// state is the count of the watched frame's navigation events so far, whether one of its navigations is in flight,
// and a channel closed at its next navigation event.
func (n *navigations) state() (events uint64, inFlight bool, changed <-chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, inFlight = n.inFlight[n.watched]
	return n.events, inFlight, n.changed
}

// navigating reports whether f has a navigation in flight.
func (n *navigations) navigating(f frame) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, inFlight := n.inFlight[f]
	return inFlight
}

// settle waits until the current frame has stopped navigating, as chromedriver did after a command: it lets the
// frame's zero-delay timers run, waits for its navigation in flight to commit, then for its load, and goes again
// while the frame keeps navigating, all within web.timeout. A dialog that opens meanwhile ends the wait, the dialog
// pending (R6). A navigation that does not commit in time fails the step and stays in flight, as it does in the
// browser: the next steps fail the same way until it ends, and a navigate ends it (the browser aborts it).
func (w *WebContext) settle() error {
	deadline := time.Now().Add(w.navigationTimeout)
	w.nav.watch(w.frame())
	for {
		if time.Now().After(deadline) {
			return w.navigationTimedOut("stop navigating")
		}
		settled, err := w.settleOnce(deadline)
		if settled || err != nil {
			return err
		}
	}
}

// settleOnce is one round of settle; settled when no navigation event of the frame came meanwhile, or when a dialog is
// pending.
func (w *WebContext) settleOnce(deadline time.Time) (bool, error) {
	if w.dialogPending() {
		return true, nil
	}
	before, _, _ := w.nav.state()
	if err := w.flushTimers(); err != nil || w.dialogPending() {
		return true, err
	}
	if err := w.awaitCommit(deadline); err != nil || w.dialogPending() {
		return true, err
	}
	remaining := playwright.FrameWaitForLoadStateOptions{Timeout: playwright.Float(max(float64(time.Until(deadline).Milliseconds()), 1))}
	if err := w.do(ownTimeout, func() error { return w.frame().WaitForLoadState(remaining) }); err != nil {
		return true, err
	}
	after, inFlight, _ := w.nav.state()
	return after == before && !inFlight, nil
}

// flushTimers lets the current frame's zero-delay timers run. Playwright would hold the flush while the frame
// navigates, for as long as the navigation takes: it is not sent while a navigation is in flight, and its wait ends
// at the frame's first navigation event, the call left in flight, since that event can arrive after the flush was
// sent. awaitCommit then waits, within web.timeout. Its failure is no step failure: an evaluation that a navigation
// interrupts ("execution context was destroyed") is a navigation that happened, and any other failure is met by the
// next call on the page. A page that does not answer is one (R17).
func (w *WebContext) flushTimers() error {
	_, inFlight, changed := w.nav.state()
	if inFlight {
		return nil
	}
	limit := read("the timer flush")
	limit.stop = changed
	err := w.do(limit, func() error {
		_, err := w.frame().Evaluate(jsFlushTimers)
		return err
	})
	if errors.Is(err, errNoAnswer) {
		return err
	}
	return nil
}

// awaitCommit waits until the current frame has no navigation in flight, or until a dialog opens: it is then pending.
func (w *WebContext) awaitCommit(deadline time.Time) error {
	timeout := time.After(time.Until(deadline))
	for {
		_, inFlight, changed := w.nav.state()
		if !inFlight {
			return nil
		}
		select {
		case <-changed:
		case w.pending = <-w.dialogs:
			return nil
		case <-timeout:
			return w.navigationTimedOut("commit its navigation")
		}
	}
}

// awaitPending waits, before an action whose first call goes to the current frame, for the frame's navigation still
// in flight to commit, within web.timeout, as chromedriver waited before a command (R19): Playwright would hold that
// call until the navigation ends. A pending dialog is left to the action, which dismisses it and fails.
func (w *WebContext) awaitPending() error {
	if w.dialogPending() || !w.nav.navigating(w.frame()) {
		return nil
	}
	w.nav.watch(w.frame())
	return w.awaitCommit(time.Now().Add(w.navigationTimeout))
}

func (w *WebContext) navigationTimedOut(what string) error {
	return fmt.Errorf("the page did not %s within web.timeout (%gs)", what, w.navigationTimeout.Seconds())
}
