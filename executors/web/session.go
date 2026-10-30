package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

// maxPendingDialogs bounds the dialogs waiting for the executor. A page's script is blocked while its dialog is open,
// so there is at most one per page; a dialog beyond the bound is dismissed rather than left blocking its page.
const maxPendingDialogs = 16

// The limits of the page calls Playwright sends without a timeout of its own (R17), develop's: variables, so that the
// unit tests can shorten them.
var (
	// readTimeout bounds a read of the page (its title, its source, a count, a text, a value) and the other calls with
	// no timeout: the WebDriver client's HTTP timeout.
	readTimeout = 60 * time.Second
	// scriptTimeout bounds execute: WebDriver's script timeout.
	scriptTimeout = 30 * time.Second
)

var (
	// errUnexpectedAlert is WebDriver's "dismiss and notify" answer to a command sent while a dialog is open.
	errUnexpectedAlert = errors.New("unexpected alert open: the dialog was dismissed; answer it with confirmPopup or cancelPopup")
	errNoAlert         = errors.New("no such alert: no dialog is open")
	// errDialogOpened reports that a dialog opened before the operation finished; the operation stays in flight.
	errDialogOpened = errors.New("a dialog opened before the operation finished")
	// errNoAnswer reports a call that did not return within its limit (R17). It is left in flight: TearDown's close
	// ends it.
	errNoAnswer = errors.New("the browser did not answer")
	// errStopped reports a call whose wait its limit's stop ended; it is left in flight, as on errNoAnswer.
	errStopped = errors.New("the wait for the call was stopped")
)

// callLimit bounds a page call on the Go side (R17). The zero value, ownTimeout, leaves the call to the timeout
// Playwright gives it: actions, navigations and load waits.
type callLimit struct {
	call    string
	timeout time.Duration
	// stop, when set, ends the wait once it is closed.
	stop <-chan struct{}
	// replaces marks a call that ends what an abandoned call works on, a navigation: it does not wait for it.
	replaces bool
}

var ownTimeout = callLimit{}

// read is the limit of a call Playwright sends without a timeout.
func read(call string) callLimit { return callLimit{call: call, timeout: readTimeout} }

// script is the limit of execute.
func script() callLimit { return callLimit{call: "the execute script", timeout: scriptTimeout} }

// elementRead is the limit of a script on an element the step waits for: the wait keeps the step's syncTimeout
// (seconds), and the script has readTimeout after it.
func elementRead(call string, syncTimeout int64) callLimit {
	l := read(call)
	if syncTimeout > 0 {
		l.timeout += time.Duration(syncTimeout) * time.Second
	}
	return l
}

// expiry receives once the limit is over; nil, never ready, for ownTimeout.
func (l callLimit) expiry() <-chan time.Time {
	if l.timeout <= 0 {
		return nil
	}
	return time.After(l.timeout)
}

func (l callLimit) expired() error {
	return fmt.Errorf("%w: %s did not return within %gs", errNoAnswer, l.call, l.timeout.Seconds())
}

// interrupted is an operation a dialog interrupted, in flight until the dialog is answered.
type interrupted struct {
	finished <-chan struct{} // closed when the operation returns; nil when there is none
	limit    callLimit
}

// WebContext is the suite's browser session: the Playwright objects Setup opened, the current page and frame, the
// dialog and navigation state (docs/feat/2047/research.md R6, R10, R16), and the trace recorder when web.trace is on.
type WebContext struct { //nolint:revive // exported name kept for module compatibility (plugins, library users, upstream merges)
	// mu is held for the whole of a Run and of TearDown: a step venom's timeout abandoned goes on, and the next step
	// or TearDown waits for it (R17). The in-flight operations and Playwright's event handlers never take it.
	mu sync.Mutex

	driver     driver
	browser    browser
	browserCtx browserContext
	page       page
	// frames is the current frame and the frames entered to reach it: frames[0] is the page's main frame.
	frames []frame

	// nav follows the frames' navigations, from the context's events; navigationTimeout (web.timeout) bounds the wait
	// for them.
	nav               *navigations
	navigationTimeout time.Duration

	// dialogs receives the dialogs the pages open, on Playwright's goroutine. Everything below is only touched with mu
	// held.
	dialogs chan dialog
	// pending is the dialog the next confirmPopup or cancelPopup answers.
	pending dialog
	// inflight is the operation the pending dialog interrupted.
	inflight interrupted
	// abandoned is the call a limit gave up on, still running; nil when there is none. The next call waits for it,
	// within its own limit (R17).
	abandoned <-chan struct{}
	// closing is closed when TearDown starts, so that a step waiting for the abandoned call does not hold mu against
	// the close that ends that call; closeOnce guards it. Both are nil in a WebContext not made by openSession.
	closing   chan struct{}
	closeOnce sync.Once

	// tracing records the step's chunk of the trace (FR-08); nil when web.trace is off, or once a tracing call failed.
	tracing tracing
}

// openSession starts Playwright, launches the browser and opens the suite's context and page. Whatever it opened is
// closed again when a later step fails: the engine calls no TearDown after a failed Setup.
func openSession(ctx context.Context, cfg setupConfig) (*WebContext, error) {
	w := &WebContext{
		dialogs:           make(chan dialog, maxPendingDialogs),
		closing:           make(chan struct{}),
		nav:               newNavigations(),
		navigationTimeout: time.Duration(cfg.navigationTimeout) * time.Millisecond,
	}
	d, err := runPlaywright()
	if err != nil {
		return nil, fmt.Errorf("unable to start Playwright (install it with: %s): %w", installCommand(cfg.browser), err)
	}
	w.driver = d
	b, err := d.launch(cfg.browser, cfg.launch)
	if err != nil {
		return nil, w.abort(launchError(cfg.browser, err))
	}
	w.browser = b
	c, err := b.newContext(cfg.context)
	if err != nil {
		return nil, w.abort(fmt.Errorf("unable to open a browser context: %w", err))
	}
	w.browserCtx = c
	c.onDialog(func(d dialog) { w.onDialog(ctx, d) })
	w.nav.listen(c)
	c.SetDefaultTimeout(actionTimeout)
	c.SetDefaultNavigationTimeout(cfg.navigationTimeout)
	if cfg.trace {
		// Start opens the first chunk, which the first step ends (docs/feat/2048/research.md R8).
		tr := c.tracing()
		if err := tr.Start(playwright.TracingStartOptions{Screenshots: playwright.Bool(true), Snapshots: playwright.Bool(true)}); err != nil {
			return nil, w.abort(fmt.Errorf("unable to start the trace: %w", err))
		}
		w.tracing = tr
	}
	p, err := c.newPage()
	if err != nil {
		return nil, w.abort(fmt.Errorf("unable to open a page: %w", err))
	}
	w.switchTo(p)
	return w, nil
}

// abort closes what openSession opened so far and returns err with any close error.
func (w *WebContext) abort(err error) error {
	return errors.Join(err, w.closeAll())
}

// close dismisses a leftover dialog and lets the operation it interrupted finish, closes the context, the browser and
// the driver, then waits for an operation still in flight: once the driver is stopped, every call has returned. Each
// wait is bounded, so a frozen page cannot hold TearDown (R17).
func (w *WebContext) close() error {
	if w.closing != nil {
		w.closeOnce.Do(func() { close(w.closing) })
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var errDismiss error
	if w.dialogPending() {
		errDismiss = w.resolveDialog(false)
	}
	err := errors.Join(errDismiss, w.closeAll())
	if w.inflight.finished != nil {
		select {
		case <-w.inflight.finished:
		case <-time.After(readTimeout):
		}
		w.inflight = interrupted{}
	}
	return err
}

func (w *WebContext) closeAll() error {
	var errs []error
	if w.browserCtx != nil {
		errs = append(errs, w.browserCtx.Close())
	}
	if w.browser != nil {
		errs = append(errs, w.browser.Close())
	}
	errs = append(errs, w.driver.Stop())
	return errors.Join(errs...)
}

// onDialog runs on Playwright's goroutine: it hands the dialog over and never blocks.
func (w *WebContext) onDialog(ctx context.Context, d dialog) {
	select {
	case w.dialogs <- d:
	default:
		venom.Warning(ctx, "web - %d dialogs already wait for an answer: dismissing a new one", maxPendingDialogs)
		if err := d.Dismiss(); err != nil {
			venom.Warning(ctx, "web - dismissing the dialog failed: %v", err)
		}
	}
}

// dialogPending reports whether a dialog waits for confirmPopup or cancelPopup.
func (w *WebContext) dialogPending() bool {
	if w.pending == nil {
		select {
		case w.pending = <-w.dialogs:
		default:
		}
	}
	return w.pending != nil
}

// answerDialog is confirmPopup (accept) and cancelPopup: it answers the pending dialog, then lets the operation it
// interrupted finish and the navigation it started end, so the page has reacted before the step reads its title.
func (w *WebContext) answerDialog(accept bool) error {
	if !w.dialogPending() {
		return errNoAlert
	}
	if err := w.resolveDialog(accept); err != nil {
		return err
	}
	return w.settle()
}

func (w *WebContext) resolveDialog(accept bool) error {
	d := w.pending
	w.pending = nil
	var err error
	if accept {
		err = d.Accept()
	} else {
		err = d.Dismiss()
	}
	return errors.Join(err, w.awaitInflight())
}

// awaitInflight waits for the interrupted operation, within its limit, unless it opens another dialog first: that one
// is then pending.
func (w *WebContext) awaitInflight() error {
	op := w.inflight
	if op.finished == nil {
		return nil
	}
	w.inflight = interrupted{}
	select {
	case <-op.finished:
	case d := <-w.dialogs:
		w.pending = d
		w.inflight = op
	case <-op.limit.expiry():
		return op.limit.expired()
	}
	return nil
}

// refuseWhilePending is WebDriver's default user-prompt handling: a command sent while a dialog is open dismisses the
// dialog and fails.
func (w *WebContext) refuseWhilePending() error {
	if !w.dialogPending() {
		return nil
	}
	return errors.Join(errUnexpectedAlert, w.resolveDialog(false))
}

type outcome[T any] struct {
	value T
	err   error
}

// run is the one path of every call into the page (R6, R17). It first waits for a call an earlier limit left in flight,
// then refuses the call while a dialog is pending, then runs
// op on its own goroutine and returns its result; or errDialogOpened as soon as a dialog opens first: Playwright does
// not return from an action whose page shows a dialog, WebDriver did, and the dialog is answered by a later step; or
// errNoAnswer once the limit is over, or errStopped once its stop is closed, the call left in flight.
func run[T any](w *WebContext, limit callLimit, op func() (T, error)) (T, error) {
	var zero T
	if err := w.refuseWhilePending(); err != nil {
		return zero, err
	}
	if err := w.awaitAbandoned(limit); err != nil {
		return zero, err
	}
	res := make(chan outcome[T], 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		v, err := op()
		res <- outcome[T]{v, err}
	}()
	select {
	case o := <-res:
		return o.value, o.err
	case w.pending = <-w.dialogs:
		select {
		case o := <-res: // it finished as the dialog opened: nothing is left in flight
			return o.value, o.err
		default:
			w.inflight = interrupted{finished, limit}
			return zero, errDialogOpened
		}
	case <-limit.expiry():
		w.abandoned = finished
		return zero, limit.expired()
	case <-limit.stop: // left in flight by design (a timer flush): not remembered
		return zero, errStopped
	}
}

// awaitAbandoned holds the next call back until the call an earlier limit gave up on has returned: two calls must not
// overlap on the page. A call with a limit of its own waits at most that limit and then fails with it, the abandoned
// call staying remembered. A call with none (Playwright's own timeout applies to it) waits at most readTimeout, then
// goes on and forgets the abandoned call: a page that never answers must not hang the run. A navigation does not wait,
// and TearDown's close ends the wait too.
func (w *WebContext) awaitAbandoned(limit callLimit) error {
	if w.abandoned == nil || limit.replaces {
		return nil
	}
	expiry := limit.expiry()
	unlimited := expiry == nil
	if unlimited {
		expiry = time.After(readTimeout)
	}
	select {
	case <-w.abandoned:
		w.abandoned = nil
	case <-expiry:
		if !unlimited {
			return limit.expired()
		}
		venom.Warning(context.Background(), "web - a browser call that did not answer is still running: the step goes on after %gs", readTimeout.Seconds())
		w.abandoned = nil
	case <-limit.stop:
		return errStopped
	case <-w.closing:
		return errStopped
	}
	return nil
}

// do runs a command through run. A command a dialog interrupted has done its part, as a WebDriver command that opens a
// dialog returns: the step goes on, the next command meets the dialog.
func (w *WebContext) do(limit callLimit, op func() error) error {
	_, err := run(w, limit, func() (struct{}, error) { return struct{}{}, op() })
	if errors.Is(err, errDialogOpened) {
		return nil
	}
	return err
}

// bounded runs op within limit, outside run: tracing does not wait on a page's script, so a pending dialog must neither
// refuse nor interrupt it (docs/feat/2048/research.md R8). On expiry op is left in flight, as run leaves a call (R17).
func bounded(limit callLimit, op func() error) error {
	res := make(chan error, 1)
	go func() { res <- op() }()
	select {
	case err := <-res:
		return err
	case <-limit.expiry():
		return limit.expired()
	}
}

// endTrace ends the step's chunk of the trace, then opens the next one (FR-08, docs/feat/2048/research.md R8): the chunk
// of a failed step is saved next to its page dump, any other is discarded. A tracing call that fails turns tracing off
// for the rest of the test case, with a warning; the step's own result and error stand.
func (w *WebContext) endTrace(ctx context.Context, failed bool) {
	if w.tracing == nil {
		return
	}
	if err := w.rotateTrace(ctx, failed); err != nil {
		venom.Warning(ctx, "web - the trace is turned off for this test case: %v", err)
		w.tracing = nil
	}
}

// rotateTrace is endTrace's calls. The driver process writes the zip, so its path is made absolute here, in the
// working directory, where page.dump.html goes.
func (w *WebContext) rotateTrace(ctx context.Context, failed bool) error {
	tr := w.tracing
	if failed {
		path, err := filepath.Abs(dumpName + ".trace.zip")
		if err != nil {
			return fmt.Errorf("unable to name the trace file: %w", err)
		}
		if err := ownerOnly(path); err != nil {
			return fmt.Errorf("unable to create the trace file: %w", err)
		}
		if err := bounded(read("the trace save"), func() error { return tr.StopChunk(path) }); err != nil {
			return err
		}
		venom.Info(ctx, "Trace of the failing step is saved in %s", path)
	} else if err := bounded(read("the trace discard"), func() error { return tr.StopChunk() }); err != nil {
		return err
	}
	return bounded(read("the trace start"), func() error { return tr.StartChunk() })
}

// ownerOnly creates path, or empties the file an earlier run left there, readable by its owner only, as the page dump:
// the trace holds every text a step typed. The driver process then writes into it, which keeps that mode; its own
// create would leave 0644 under the usual umask.
func ownerOnly(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// frame is the current frame: finds, actions, execute and the page dump resolve in it (R10).
func (w *WebContext) frame() frame {
	return w.frames[len(w.frames)-1]
}

// switchTo makes p the current page, at its top-level frame (WebDriver's Switch To Window).
func (w *WebContext) switchTo(p page) {
	w.page = p
	w.toMainFrame()
}

// toMainFrame goes back to the top-level frame: a top-level navigation leaves any iframe.
func (w *WebContext) toMainFrame() {
	w.frames = []frame{w.page.mainFrame()}
}
