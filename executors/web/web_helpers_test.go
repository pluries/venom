package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Test fixtures for web_test.go: a Go fake of the seam (page.go) standing in for Playwright. It logs every call the
// executor makes, answers from a small page model, and can open dialogs the way a page does. No browser, no driver
// process, no network (NFR-04): runPlaywright is swapped for the fake by withFake.

const (
	pageTitle  = "Synthetic page"
	pageURL    = "https://example.test/start"
	pageSource = "<html><head><title>Synthetic page</title></head><body>synthetic</body></html>"
)

// fakeNode is one element a selector matches.
type fakeNode struct {
	text     string
	value    any
	multiple bool // a file input accepting several files
	frame    bool // an iframe
}

// fakePW is the state shared by the fake driver, browser, context, pages, frames and elements, and the log of the
// calls they receive, as "Method target[ args][ timeout=ms]".
type fakePW struct {
	mu    sync.Mutex
	calls []string
	// errs makes a call fail; the key is the call as logged, without its timeout.
	errs map[string]error
	// hooks run inside a call, after it is logged: a page's script running, e.g. opening a dialog.
	hooks map[string]func()
	// results are what a frame's Evaluate returns, as playwright-go decodes it, keyed as errs; nil, undefined, for
	// any other.
	results map[string]any
	// nodes are the elements each selector matches, by the element's logged name.
	nodes map[string][]fakeNode
	// runErr makes runPlaywright fail.
	runErr error

	browserName    string
	launchOptions  playwright.BrowserTypeLaunchOptions
	contextOptions playwright.BrowserNewContextOptions
	handler        func(dialog)
	// onRequest, onRequestFailed and onNavigated are the context's navigation handlers (R16).
	onRequest       func(request)
	onRequestFailed func(request)
	onNavigated     func(frame)
	// routes are the context's mocks, in the order they were registered; a new context has none, as Playwright's.
	routes []routeEntry
	// navigating is, per frame, its navigation in flight as the browser has it, and commits counts each frame's
	// commits: Playwright holds an evaluation sent to a navigating frame until the navigation ends (R16). navEnded is
	// closed, then replaced, when one ends.
	navigating map[frame]*fakeRequest
	commits    map[frame]int
	navEnded   chan struct{}
	pageList   []*fakePage
	// iframes are the frames entered with selectFrame, one object per selector, as Playwright keeps one per frame.
	iframes map[string]*fakeFrame
	// closed is closed with the context: a call blocked in a hook returns then, as on a closed target.
	closed    chan struct{}
	closeOnce sync.Once
	dialogs   int
}

// withFake makes Setup use a new fake until the test ends.
func withFake(t *testing.T) *fakePW {
	t.Helper()
	pw := &fakePW{
		errs:       map[string]error{},
		hooks:      map[string]func(){},
		results:    map[string]any{},
		closed:     make(chan struct{}),
		iframes:    map[string]*fakeFrame{},
		navigating: map[frame]*fakeRequest{},
		commits:    map[frame]int{},
		navEnded:   make(chan struct{}),
		nodes: map[string][]fakeNode{
			"css=.item":                  {{text: "first", value: "v1"}},
			"xpath=//div[@class='item']": {{text: "first", value: "v1"}},
		},
	}
	prev := runPlaywright
	runPlaywright = func() (driver, error) {
		if err := pw.call("Run", nil); err != nil {
			return nil, err
		}
		return fakeDriver{pw}, pw.runErr
	}
	t.Cleanup(func() { runPlaywright = prev })
	// A call still blocked in a hook returns at the end of the test, as on a closed target.
	t.Cleanup(func() { pw.closeOnce.Do(func() { close(pw.closed) }) })
	return pw
}

// call logs a call, runs its hook, then returns its configured error.
func (pw *fakePW) call(name string, timeout *float64) error {
	pw.mu.Lock()
	logged := name
	if timeout != nil {
		logged += fmt.Sprintf(" timeout=%g", *timeout)
	}
	pw.calls = append(pw.calls, logged)
	hook, err := pw.hooks[name], pw.errs[name]
	pw.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

// seen returns the calls received since the last reset.
func (pw *fakePW) seen() []string {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	return append([]string(nil), pw.calls...)
}

func (pw *fakePW) reset() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.calls = nil
}

func (pw *fakePW) node(name string) (fakeNode, bool) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	nodes := pw.nodes[name]
	if len(nodes) == 0 {
		return fakeNode{}, false
	}
	return nodes[0], true
}

// openDialog shows a dialog, as a page's script does: the context's handler receives it.
func (pw *fakePW) openDialog(kind string) *fakeDialog {
	pw.mu.Lock()
	pw.dialogs++
	d := &fakeDialog{pw: pw, name: fmt.Sprintf("%s%d", kind, pw.dialogs), answered: make(chan struct{})}
	handler := pw.handler
	pw.mu.Unlock()
	handler(d)
	return d
}

// blockOnDialog is a hook: the call opens count dialogs, each once the previous one is answered, and returns once the
// last one is, as a click whose handler calls confirm() does; finished is set when it returns.
func (pw *fakePW) blockOnDialog(kind string, count int, finished *chanFlag) func() {
	return func() {
		for range count {
			d := pw.openDialog(kind)
			select {
			case <-d.answered:
			case <-pw.closed:
				finished.set()
				return
			}
		}
		finished.set()
	}
}

// fakeRequest is a request a frame sends; name tells them apart in the log.
type fakeRequest struct {
	name       string
	navigation bool
	from       frame
}

func (r *fakeRequest) IsNavigationRequest() bool { return r.navigation }
func (r *fakeRequest) frame() frame              { return r.from }

// requestNavigation makes f send a navigation request, as a script's location.href does: the navigation is then in
// flight, requested but not committed.
func (pw *fakePW) requestNavigation(f frame, name string) *fakeRequest {
	return pw.send(pw.startNavigation(f, name))
}

// startNavigation starts f's navigation in the browser only; send delivers its request event later, as Playwright's
// event can come after the result of the script that started the navigation.
func (pw *fakePW) startNavigation(f frame, name string) *fakeRequest {
	r := &fakeRequest{name: name, navigation: true, from: f}
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.navigating[f] = r
	return r
}

// endNavigation ends f's navigation in the browser; pw.mu is held.
func (pw *fakePW) endNavigation(f frame) {
	delete(pw.navigating, f)
	close(pw.navEnded)
	pw.navEnded = make(chan struct{})
}

// held blocks an evaluation, logged as name, while f navigates, as Playwright does, and reports whether the
// navigation committed meanwhile: the evaluation then fails. After a failed navigation it runs. The hook of
// "<name> held" runs once the evaluation is held.
func (pw *fakePW) held(f frame, name string) bool {
	pw.mu.Lock()
	if pw.navigating[f] == nil {
		pw.mu.Unlock()
		return false
	}
	pw.calls = append(pw.calls, name+" held")
	commits := pw.commits[f]
	if hook := pw.hooks[name+" held"]; hook != nil {
		pw.mu.Unlock()
		hook()
		pw.mu.Lock()
	}
	for pw.navigating[f] != nil {
		ended := pw.navEnded
		pw.mu.Unlock()
		select {
		case <-ended:
		case <-pw.closed:
			return true
		}
		pw.mu.Lock()
	}
	committed := pw.commits[f] != commits
	pw.mu.Unlock()
	return committed
}

// sendResource makes f request a resource, an image or an XHR: no navigation.
func (pw *fakePW) sendResource(f frame, name string) *fakeRequest {
	return pw.send(&fakeRequest{name: name, from: f})
}

func (pw *fakePW) send(r *fakeRequest) *fakeRequest {
	pw.mu.Lock()
	pw.calls = append(pw.calls, "request "+r.name)
	handler := pw.onRequest
	pw.mu.Unlock()
	handler(r)
	return r
}

// commitNavigation commits f's navigation: the new document is there.
func (pw *fakePW) commitNavigation(f *fakeFrame) {
	pw.mu.Lock()
	pw.calls = append(pw.calls, "commit "+f.name)
	pw.commits[f]++
	pw.endNavigation(f)
	handler := pw.onNavigated
	pw.mu.Unlock()
	handler(f)
}

// failRequest fails r: an aborted or replaced navigation.
func (pw *fakePW) failRequest(r *fakeRequest) {
	pw.mu.Lock()
	pw.calls = append(pw.calls, "fail "+r.name)
	if pw.navigating[r.from] == r {
		pw.endNavigation(r.from)
	}
	handler := pw.onRequestFailed
	pw.mu.Unlock()
	handler(r)
}

// mainFrame is the main frame of the first page.
func (pw *fakePW) mainFrame() *fakeFrame {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	return pw.pageList[0].main
}

// iframe is the frame selectFrame entered with the selector.
func (pw *fakePW) iframe(selector string) *fakeFrame {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	return pw.iframes[selector]
}

// chanFlag records that an in-flight call has finished.
type chanFlag struct{ done chan struct{} }

func newFlag() *chanFlag { return &chanFlag{done: make(chan struct{})} }

func (f *chanFlag) set() { close(f.done) }

func (f *chanFlag) isSet() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

type fakeDialog struct {
	pw       *fakePW
	name     string
	answered chan struct{}
	once     sync.Once
}

func (d *fakeDialog) answer(verb string) error {
	err := d.pw.call(verb+" "+d.name, nil)
	d.once.Do(func() { close(d.answered) })
	return err
}

func (d *fakeDialog) Accept(...string) error { return d.answer("Accept") }
func (d *fakeDialog) Dismiss() error         { return d.answer("Dismiss") }

type fakeDriver struct{ pw *fakePW }

func (d fakeDriver) launch(name string, o playwright.BrowserTypeLaunchOptions) (browser, error) {
	d.pw.browserName, d.pw.launchOptions = name, o
	if err := d.pw.call("Launch "+name, nil); err != nil {
		return nil, err
	}
	return fakeBrowser(d), nil
}

func (d fakeDriver) Stop() error { return d.pw.call("Stop driver", nil) }

type fakeBrowser struct{ pw *fakePW }

func (b fakeBrowser) newContext(o playwright.BrowserNewContextOptions) (browserContext, error) {
	b.pw.contextOptions = o
	if err := b.pw.call("NewContext", nil); err != nil {
		return nil, err
	}
	b.pw.mu.Lock()
	b.pw.routes = nil
	b.pw.mu.Unlock()
	return fakeContext(b), nil
}

func (b fakeBrowser) Close(...playwright.BrowserCloseOptions) error {
	return b.pw.call("Close browser", nil)
}

type fakeContext struct{ pw *fakePW }

func (c fakeContext) onDialog(handler func(dialog)) {
	c.pw.mu.Lock()
	c.pw.handler = handler
	c.pw.mu.Unlock()
	_ = c.pw.call("OnDialog", nil) //nolint:errcheck // the fake never fails a void call
}

func (c fakeContext) onRequest(handler func(request)) {
	c.pw.mu.Lock()
	c.pw.onRequest = handler
	c.pw.mu.Unlock()
	_ = c.pw.call("OnRequest", nil) //nolint:errcheck // the fake never fails a void call
}

func (c fakeContext) onRequestFailed(handler func(request)) {
	c.pw.mu.Lock()
	c.pw.onRequestFailed = handler
	c.pw.mu.Unlock()
	_ = c.pw.call("OnRequestFailed", nil) //nolint:errcheck // the fake never fails a void call
}

func (c fakeContext) onFrameNavigated(handler func(frame)) {
	c.pw.mu.Lock()
	c.pw.onNavigated = handler
	c.pw.mu.Unlock()
	_ = c.pw.call("OnFrameNavigated", nil) //nolint:errcheck // the fake never fails a void call
}

// route adds handler after the context's other routes, logged as "Route <pattern>"; a refused one is not added.
func (c fakeContext) route(pattern string, handler func(route)) error {
	if err := c.pw.call("Route "+pattern, nil); err != nil {
		return err
	}
	c.pw.mu.Lock()
	defer c.pw.mu.Unlock()
	c.pw.routes = append(c.pw.routes, routeEntry{pattern: pattern, handler: handler})
	return nil
}

func (c fakeContext) tracing() tracing { return fakeTracing(c) }

// routeEntry is a route of the context: the pattern it was registered with and the executor's handler.
type routeEntry struct {
	pattern string
	handler func(route)
}

// intercept sends a request for url, as a page does, and returns the route the newest matching handler received,
// Playwright's order (docs/feat/2048/research.md R1); nil when no handler matches: the request would reach the
// network. A pattern matches the url it equals: the fake does not reimplement Playwright's globs.
func (pw *fakePW) intercept(url string) *fakeRoute {
	pw.mu.Lock()
	var handler func(route)
	for _, e := range slices.Backward(pw.routes) {
		if e.pattern == url {
			handler = e.handler
			break
		}
	}
	pw.mu.Unlock()
	if handler == nil {
		return nil
	}
	r := &fakeRoute{pw: pw, url: url}
	handler(r)
	return r
}

// fakeRoute is an intercepted request. Its Fulfill is logged, and kept in fulfilled, as
// "Fulfill <url> status=<n|default> contentType=<type|none> headers=<map|none> body=<Go literal|none>": "default" and
// "none" are an option left unset, Playwright's 200 and no content type, no header, no body.
type fakeRoute struct {
	pw        *fakePW
	url       string
	fulfilled string
}

func (r *fakeRoute) Fulfill(o ...playwright.RouteFulfillOptions) error {
	var opts playwright.RouteFulfillOptions
	if len(o) > 0 {
		opts = o[0]
	}
	status, contentType, headers, body := "default", "none", "none", "none"
	if opts.Status != nil {
		status = strconv.Itoa(*opts.Status)
	}
	if opts.ContentType != nil {
		contentType = *opts.ContentType
	}
	if opts.Headers != nil {
		headers = fmt.Sprint(opts.Headers) // fmt sorts a map by its keys
	}
	if opts.Body != nil {
		body = fmt.Sprintf("%#v", opts.Body)
	}
	r.fulfilled = fmt.Sprintf("Fulfill %s status=%s contentType=%s headers=%s body=%s", r.url, status, contentType, headers, body)
	return r.pw.call(r.fulfilled, nil)
}

// fakeTracing is the context's trace recorder, logged as "Tracing Start screenshots=<bool|unset> snapshots=<bool|unset>",
// "Tracing StartChunk", and "Tracing StopChunk" (discarded) or "Tracing StopChunk <path>" (written there).
type fakeTracing struct{ pw *fakePW }

func (tr fakeTracing) Start(o ...playwright.TracingStartOptions) error {
	var opts playwright.TracingStartOptions
	if len(o) > 0 {
		opts = o[0]
	}
	return tr.pw.call(fmt.Sprintf("Tracing Start screenshots=%s snapshots=%s", optionalBool(opts.Screenshots), optionalBool(opts.Snapshots)), nil)
}

func (tr fakeTracing) StartChunk(...playwright.TracingStartChunkOptions) error {
	return tr.pw.call("Tracing StartChunk", nil)
}

func (tr fakeTracing) StopChunk(path ...string) error {
	name := "Tracing StopChunk"
	if len(path) > 0 {
		name += " " + path[0]
	}
	return tr.pw.call(name, nil)
}

// optionalBool is a Playwright *bool option as logged: its value, or "unset".
func optionalBool(b *bool) string {
	if b == nil {
		return "unset"
	}
	return strconv.FormatBool(*b)
}

func (c fakeContext) newPage() (page, error) {
	if err := c.pw.call("NewPage", nil); err != nil {
		return nil, err
	}
	return c.pw.addPage(), nil
}

// addPage opens a tab, as window.open does.
func (pw *fakePW) addPage() *fakePage {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	p := &fakePage{pw: pw, name: fmt.Sprintf("page%d", len(pw.pageList)+1)}
	p.main = &fakeFrame{pw: pw, name: p.name}
	pw.pageList = append(pw.pageList, p)
	return p
}

func (c fakeContext) pages() []page {
	_ = c.pw.call("Pages", nil) //nolint:errcheck // the fake never fails a void call
	c.pw.mu.Lock()
	defer c.pw.mu.Unlock()
	var all []page
	for _, p := range c.pw.pageList {
		all = append(all, p)
	}
	return all
}

func (c fakeContext) SetDefaultTimeout(timeout float64) {
	_ = c.pw.call(fmt.Sprintf("SetDefaultTimeout %g", timeout), nil) //nolint:errcheck // the fake never fails a void call
}

func (c fakeContext) SetDefaultNavigationTimeout(timeout float64) {
	_ = c.pw.call(fmt.Sprintf("SetDefaultNavigationTimeout %g", timeout), nil) //nolint:errcheck // the fake never fails a void call
}

func (c fakeContext) ClearCookies(...playwright.BrowserContextClearCookiesOptions) error {
	return c.pw.call("ClearCookies", nil)
}

func (c fakeContext) Close(...playwright.BrowserContextCloseOptions) error {
	c.pw.closeOnce.Do(func() { close(c.pw.closed) })
	return c.pw.call("Close context", nil)
}

// fakePage's main frame has the page's name: its elements are named by their selector alone.
type fakePage struct {
	pw   *fakePW
	name string
	main *fakeFrame
}

func (p *fakePage) mainFrame() frame { return p.main }

func (p *fakePage) Title() (string, error) { return pageTitle, p.pw.call("Title "+p.name, nil) }

func (p *fakePage) URL() string {
	_ = p.pw.call("URL "+p.name, nil) //nolint:errcheck // URL cannot fail
	return pageURL
}

func (p *fakePage) Screenshot(...playwright.PageScreenshotOptions) ([]byte, error) {
	return []byte("synthetic-png"), p.pw.call("Screenshot "+p.name, nil)
}

// Goto aborts the navigation the page's main frame has in flight, as the browser does (net::ERR_ABORTED).
func (p *fakePage) Goto(url string, _ ...playwright.PageGotoOptions) (playwright.Response, error) {
	p.pw.mu.Lock()
	pending := p.pw.navigating[p.main]
	p.pw.mu.Unlock()
	if pending != nil {
		p.pw.failRequest(pending)
	}
	return nil, p.pw.call("Goto "+p.name+" "+url, nil)
}

func (p *fakePage) GoBack(...playwright.PageGoBackOptions) (playwright.Response, error) {
	return nil, p.pw.call("GoBack "+p.name, nil)
}

func (p *fakePage) GoForward(...playwright.PageGoForwardOptions) (playwright.Response, error) {
	return nil, p.pw.call("GoForward "+p.name, nil)
}

func (p *fakePage) Reload(...playwright.PageReloadOptions) (playwright.Response, error) {
	return nil, p.pw.call("Reload "+p.name, nil)
}

func (p *fakePage) BringToFront() error { return p.pw.call("BringToFront "+p.name, nil) }

// fakeFrame is a page's main frame, or an iframe named by the selector that found it.
type fakeFrame struct {
	pw     *fakePW
	name   string
	iframe bool
}

func (f *fakeFrame) locator(selector string) element {
	name := selector
	if f.iframe {
		name = f.name + " >> " + selector
	}
	return &fakeElement{pw: f.pw, frame: f, name: name}
}

// Content is held while the frame navigates, as Evaluate is.
func (f *fakeFrame) Content() (string, error) {
	if f.pw.held(f, "Content "+f.name) {
		return "", errContextDestroyed
	}
	return pageSource, f.pw.call("Content "+f.name, nil)
}

// errContextDestroyed is Playwright's failure of an evaluation its frame's navigation replaced.
var errContextDestroyed = errors.New("execution context was destroyed, most likely because of a navigation")

// Evaluate is held while the frame navigates (held). It returns its result once its hook has run: a script that opens
// a dialog returns when the dialog is answered.
func (f *fakeFrame) Evaluate(expression string, arg ...any) (any, error) {
	name := "Evaluate " + f.name + " flushTimers"
	if expression != jsFlushTimers {
		name = fmt.Sprintf("Evaluate %s %s %#v", f.name, expression, arg[0])
	}
	if f.pw.held(f, name) {
		return nil, errContextDestroyed
	}
	err := f.pw.call(name, nil)
	f.pw.mu.Lock()
	defer f.pw.mu.Unlock()
	return f.pw.results[name], err
}

func (f *fakeFrame) WaitForLoadState(...playwright.FrameWaitForLoadStateOptions) error {
	return f.pw.call("WaitForLoadState "+f.name, nil)
}

// fakeElement is a locator of frame; its first() is logged as "<selector> first".
type fakeElement struct {
	pw      *fakePW
	frame   *fakeFrame
	name    string
	isFirst bool
}

// call is pw.call, held while the element's frame navigates, as Playwright holds a locator call (R19).
func (e *fakeElement) call(name string, timeout *float64) error {
	if e.pw.held(e.frame, name) {
		return errContextDestroyed
	}
	return e.pw.call(name, timeout)
}

func (e *fakeElement) target() string {
	if e.isFirst {
		return e.name + " first"
	}
	return e.name
}

func (e *fakeElement) first() element {
	return &fakeElement{pw: e.pw, frame: e.frame, name: e.name, isFirst: true}
}

func (e *fakeElement) contentFrame(o playwright.LocatorElementHandleOptions) (frame, error) {
	if err := e.call("ContentFrame "+e.target(), o.Timeout); err != nil {
		return nil, err
	}
	if n, found := e.pw.node(e.name); !found || !n.frame {
		return nil, nil
	}
	e.pw.mu.Lock()
	defer e.pw.mu.Unlock()
	f, known := e.pw.iframes[e.name]
	if !known {
		f = &fakeFrame{pw: e.pw, name: e.name, iframe: true}
		e.pw.iframes[e.name] = f
	}
	return f, nil
}

func (e *fakeElement) Count() (int, error) {
	err := e.call("Count "+e.target(), nil)
	e.pw.mu.Lock()
	defer e.pw.mu.Unlock()
	return len(e.pw.nodes[e.name]), err
}

func (e *fakeElement) Click(o ...playwright.LocatorClickOptions) error {
	return e.call("Click "+e.target(), o[0].Timeout)
}

func (e *fakeElement) PressSequentially(text string, o ...playwright.LocatorPressSequentiallyOptions) error {
	return e.call("PressSequentially "+e.target()+" "+text, o[0].Timeout)
}

func (e *fakeElement) Press(key string, o ...playwright.LocatorPressOptions) error {
	return e.call("Press "+e.target()+" "+key, o[0].Timeout)
}

func (e *fakeElement) SelectOption(values playwright.SelectOptionValues, o ...playwright.LocatorSelectOptionOptions) ([]string, error) {
	return nil, e.call("SelectOption "+e.target()+" label="+strings.Join(*values.Labels, ","), o[0].Timeout)
}

func (e *fakeElement) SetInputFiles(files any, o ...playwright.LocatorSetInputFilesOptions) error {
	return e.call(fmt.Sprintf("SetInputFiles %s %v", e.target(), files), o[0].Timeout)
}

func (e *fakeElement) InnerText(...playwright.LocatorInnerTextOptions) (string, error) {
	if err := e.call("InnerText "+e.target(), nil); err != nil {
		return "", err
	}
	n, found := e.pw.node(e.name)
	if !found {
		return "", errors.New("no element")
	}
	return n.text, nil
}

// scriptNames shortens the element scripts in the log.
var scriptNames = map[string]string{jsValue: "value", jsMultiple: "multiple", jsCaretToEnd: "caretToEnd"}

func (e *fakeElement) Evaluate(expression string, _ any, o ...playwright.LocatorEvaluateOptions) (any, error) {
	var timeout *float64
	if len(o) > 0 {
		timeout = o[0].Timeout
	}
	if err := e.call("Evaluate "+e.target()+" "+scriptNames[expression], timeout); err != nil {
		return nil, err
	}
	n, _ := e.pw.node(e.name)
	switch expression {
	case jsValue:
		return n.value, nil
	case jsMultiple:
		return n.multiple, nil
	default:
		return nil, nil
	}
}

// webVarsCtx carries the test suite variables Setup reads, each name prefixed with "web.".
func webVarsCtx(vars map[string]any) context.Context {
	ctx := context.Background()
	for k, v := range vars {
		ctx = context.WithValue(ctx, venom.ContextKey("var.web."+k), v)
	}
	return ctx
}

// setupFake runs Setup against the fake.
func setupFake(t *testing.T, vars map[string]any) (context.Context, error) {
	t.Helper()
	venom.InitTestLogger(t)
	return Executor{}.Setup(webVarsCtx(vars), nil)
}

// openFake sets up a session on the fake, moves the test into a temporary working directory (a failing step writes
// page.dump.html into the cwd, D112) and forgets the Setup calls, so seen() only shows what the step sent.
func openFake(t *testing.T, pw *fakePW) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())
	ctx, err := setupFake(t, nil)
	require.NoError(t, err)
	pw.reset()
	return ctx
}

// webRun runs one step; a successful Run returns a *Result, whose timing is checked then zeroed.
func webRun(ctx context.Context, t *testing.T, step venom.TestStep) (Result, error) {
	t.Helper()
	got, err := Executor{}.Run(ctx, step)
	if err != nil {
		assert.Nil(t, got)
		return Result{}, err
	}
	res, isPtr := got.(*Result)
	require.Truef(t, isPtr, "want a *Result, got %T", got)
	assert.Greater(t, res.TimeSeconds, 0.0)
	out := *res
	out.TimeSeconds = 0
	return out, nil
}

// settleIn is a settle round with no navigation, in the named frame (R16): the zero-delay timers flushed, the load
// awaited.
func settleIn(frame string) []string {
	return []string{"Evaluate " + frame + " flushTimers", "WaitForLoadState " + frame}
}

// tailIn is what every successful Run does after the action when no dialog is open, the current frame named: it
// settles, then reads the page's title and URL.
func tailIn(frame string) []string {
	return append(settleIn(frame), "Title page1", "URL page1")
}

// pageTail is tailIn at the top-level frame of the first page.
func pageTail() []string {
	return tailIn("page1")
}

// pageResult is the Result of an action that writes nothing to it.
func pageResult() Result {
	return Result{Title: pageTitle, URL: pageURL}
}

// withTail appends pageTail to the calls of the action.
func withTail(calls ...string) []string {
	return append(calls, pageTail()...)
}

// settled appends the settle of an action that may navigate, then pageTail.
func settled(calls ...string) []string {
	return withTail(append(calls, settleIn("page1")...)...)
}

func findCSS(selector string) map[string]interface{} {
	return map[string]interface{}{"selector": selector, "locator": "CSS"}
}

func action(a map[string]interface{}) venom.TestStep {
	return venom.TestStep{"action": a}
}

// mocked is step with the mock entries beside its action, as a suite writes them.
func mocked(step venom.TestStep, mocks ...map[string]interface{}) venom.TestStep {
	entries := make([]interface{}, 0, len(mocks))
	for _, m := range mocks {
		entries = append(entries, m)
	}
	out := maps.Clone(step)
	if out == nil {
		out = venom.TestStep{}
	}
	out["mock"] = entries
	return out
}

// answer is what the newest mock matching url answers a request with; the test fails when none matches.
func (pw *fakePW) answer(t *testing.T, url string) string {
	t.Helper()
	r := pw.intercept(url)
	require.NotNil(t, r, "no mock matches %s", url)
	return r.fulfilled
}

// stderrOf runs fn with os.Stderr redirected and venom's test logger built on the redirect (its Out is os.Stderr at
// construction time), and returns what was written. venom's package-level logger has no test hook.
func stderrOf(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	out := make(chan string, 1)
	go func() {
		b, err := io.ReadAll(r)
		assert.NoError(t, err)
		out <- string(b)
	}()
	prevStderr := os.Stderr
	os.Stderr = w
	venom.InitTestLogger(t)

	fn()

	os.Stderr = prevStderr
	venom.InitTestLogger(t)
	require.NoError(t, w.Close())
	written := <-out
	require.NoError(t, r.Close())
	return written
}
