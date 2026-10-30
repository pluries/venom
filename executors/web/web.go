package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

var (
	_ venom.Executor          = new(Executor)
	_ venom.ExecutorWithSetup = new(Executor)
)

// Key of context element in testsuite file
const (
	Name       = "web"
	ContextKey = venom.ContextKey("webContext")
)

const (
	// dumpName is the name of the page dump: the WebDriver client's session always named itself "page" (D112).
	dumpName = "page"
	// Locator strategies of a find element, named as WebDriver names them.
	cssLocator   = "css selector"
	xpathLocator = "xpath"
)

// Scripts run on an element.
const (
	// jsValue reads the value DOM property, as WebDriver's Get Element Property does (R7).
	jsValue = "el => el.value"
	// jsMultiple tells a file input that accepts several files.
	jsMultiple = "el => el.multiple === true"
	// jsCaretToEnd focuses a field that is not focused yet and puts the caret after its content, as WebDriver's
	// Element Send Keys does: Playwright's own focus puts the caret of an input first, so the text would be prepended.
	jsCaretToEnd = `el => {
  if (el.getRootNode().activeElement === el) return;
  el.focus();
  if (typeof el.value === 'string') {
    try { el.setSelectionRange(el.value.length, el.value.length); } catch (e) {}
  } else if (el.isContentEditable) {
    const s = el.ownerDocument.getSelection(); s.selectAllChildren(el); s.collapseToEnd();
  }
}`
)

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Executor struct
type Executor struct {
	Action     Action `json:"action,omitempty" yaml:"action"`
	Screenshot string `json:"screenshot,omitempty" yaml:"screenshot"`
	Mock       []Mock `json:"mock,omitempty" yaml:"mock"`
}

// Result represents a step result
type Result struct {
	Find        int     `json:"find,omitempty" yaml:"find,omitempty"`
	HTML        string  `json:"html,omitempty" yaml:"html,omitempty"`
	TimeSeconds float64 `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
	Title       string  `json:"title,omitempty" yaml:"title,omitempty"`
	URL         string  `json:"url,omitempty" yaml:"url,omitempty"`
	Text        string  `json:"text,omitempty" yaml:"text,omitempty"`
	Value       string  `json:"value,omitempty" yaml:"value,omitempty"`
	// ValueJSON is the typed value of an execute script, as bodyjson is http's (docs/feat/2048/research.md R6).
	ValueJSON interface{} `json:"valuejson,omitempty" yaml:"valuejson,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// Setup starts Playwright and opens the suite's browser session (FR-03).
func (Executor) Setup(ctx context.Context, _ venom.H) (context.Context, error) {
	venom.Info(ctx, "Setup")
	webCtx, err := openSession(ctx, readSetupConfig(ctx))
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, ContextKey, webCtx), nil
}

func getWebCtx(ctx context.Context) *WebContext {
	i := ctx.Value(ContextKey)
	if i == nil {
		return nil
	}
	webCtx, ok := i.(*WebContext)
	if !ok {
		// ContextKey is exported (venom.ContextKey), so in theory a caller outside this package
		// could store a different type under it; degrade the same way as the "not set" case above
		// rather than panic.
		return nil
	}
	return webCtx
}

// TearDown closes the browser session and stops Playwright.
func (Executor) TearDown(ctx context.Context) error {
	venom.Info(ctx, "TearDown")
	return getWebCtx(ctx).close()
}

// Run execute TestStep
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	webCtx := getWebCtx(ctx)
	webCtx.mu.Lock()
	defer webCtx.mu.Unlock()

	result, err := runStep(ctx, webCtx, step)
	// Every way the step ends ends its trace chunk (FR-08).
	webCtx.endTrace(ctx, err != nil)
	if err != nil {
		// An untyped nil: a (*Result)(nil) in the interface would not be nil to venom.
		return nil, err
	}
	return result, nil
}

// runStep decodes the step, runs its action, takes its screenshot and reads the page; the time it reports covers all
// of it, not the trace.
func runStep(ctx context.Context, webCtx *WebContext, step venom.TestStep) (*Result, error) {
	start := time.Now()

	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	result, err := e.runAction(ctx, webCtx)
	if err != nil {
		if errg := generateErrorHTMLFile(ctx, webCtx, dumpName); errg != nil {
			warnDumpFailed(ctx, dumpName, errg)
		}
		return nil, err
	}

	// take a screenshot
	if e.Screenshot != "" {
		if err := takeScreenshot(webCtx, e.Screenshot); err != nil {
			return nil, err
		}
		if err := generateErrorHTMLFile(ctx, webCtx, dumpName); err != nil {
			venom.Warn(ctx, "Error while generating the HTML file: %v", err)
			return nil, err
		}
	}

	if err := readPage(webCtx, result); err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	return result, nil
}

// readPage reads the title and the URL of the page, left empty while a dialog waits for an answer, as they were under
// WebDriver.
func readPage(webCtx *WebContext, result *Result) error {
	result.Title = ""
	if !webCtx.dialogPending() {
		title, err := run(webCtx, read("the title read"), webCtx.page.Title)
		if err != nil && !errors.Is(err, errDialogOpened) {
			return err
		}
		result.Title = title
	}
	result.URL = ""
	if !webCtx.dialogPending() {
		result.URL = webCtx.page.URL()
	}
	return nil
}

func (e Executor) runAction(ctx context.Context, webCtx *WebContext) (*Result, error) {
	r := &Result{}
	// The step's mocks answer from its first request on, the action's included (FR-06).
	if err := webCtx.addMocks(ctx, e.Mock); err != nil {
		return nil, err
	}
	// A navigation still in flight would hold the action's first call to the frame (R19).
	if e.Action.callsFrame() {
		if err := webCtx.awaitPending(); err != nil {
			return nil, err
		}
	}
	if err := e.performAction(ctx, webCtx, r); err != nil {
		return nil, err
	}
	// Whatever the action, the page is read once it has stopped navigating, as a WebDriver read waited (R16).
	if err := webCtx.settle(); err != nil {
		return nil, err
	}
	return r, nil
}

// performAction runs the first action set on the step, in the order below; only find and execute write to r.
func (e Executor) performAction(ctx context.Context, webCtx *WebContext, r *Result) error {
	switch {
	case e.Action.Click != nil:
		return click(ctx, webCtx, e.Action.Click)
	case e.Action.Fill != nil:
		return fill(ctx, webCtx, e.Action.Fill)
	case e.Action.Find != nil:
		return findAndRead(ctx, webCtx, e.Action.Find, r)
	case e.Action.Navigate != nil:
		return navigate(ctx, webCtx, e.Action.Navigate)
	case e.Action.Wait != 0:
		sleepSeconds(e.Action.Wait)
		return nil
	case e.Action.ConfirmPopup:
		return webCtx.answerDialog(true)
	case e.Action.CancelPopup:
		return webCtx.answerDialog(false)
	case e.Action.Select != nil:
		return selectOption(ctx, webCtx, e.Action.Select)
	case e.Action.UploadFile != nil:
		return uploadFiles(ctx, webCtx, e.Action.UploadFile)
	case e.Action.SelectFrame != nil:
		return selectFrame(ctx, webCtx, e.Action.SelectFrame)
	case e.Action.SelectRootFrame:
		return selectParentFrame(webCtx)
	case e.Action.NextWindow:
		return nextWindow(webCtx)
	case e.Action.HistoryAction != "":
		return historyAction(webCtx, e.Action.HistoryAction)
	case e.Action.Execute != nil:
		return executeScript(ctx, webCtx, e.Action.Execute, r)
	}
	return nil
}

// callsFrame reports whether the action performAction runs, in the same order, starts with a call to the current
// frame: a find, an action on an element, a script. Navigating, answering a dialog, switching window or frame and
// waiting do not.
func (a Action) callsFrame() bool {
	switch {
	case a.Click != nil, a.Fill != nil, a.Find != nil:
		return true
	case a.Navigate != nil, a.Wait != 0, a.ConfirmPopup, a.CancelPopup:
		return false
	case a.Select != nil, a.UploadFile != nil, a.SelectFrame != nil:
		return true
	default:
		return !a.SelectRootFrame && !a.NextWindow && a.HistoryAction == "" && a.Execute != nil
	}
}

// sleepSeconds waits the requested seconds; zero or a negative value returns at once.
func sleepSeconds(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// stepTimeout is the step's syncTimeout in Playwright's milliseconds; nil, the context's 30 s, when the step sets none.
func stepTimeout(syncTimeout int64) *float64 {
	if syncTimeout <= 0 {
		return nil
	}
	return playwright.Float(float64(syncTimeout) * 1000)
}

// click clicks the element once it is actionable, then waits the requested seconds.
func click(ctx context.Context, webCtx *WebContext, c *Click) error {
	elt, err := findOne(ctx, webCtx, c.Find)
	if err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error { return elt.Click(playwright.LocatorClickOptions{Timeout: stepTimeout(c.SyncTimeout)}) }); err != nil {
		return err
	}
	if err := webCtx.settle(); err != nil {
		return err
	}
	sleepSeconds(c.Wait)
	return nil
}

// fill types each text, then its optional key, into the element it names.
func fill(ctx context.Context, webCtx *WebContext, fills []Fill) error {
	for _, f := range fills {
		if err := fillOne(ctx, webCtx, f); err != nil {
			return err
		}
	}
	return nil
}

// fillOne types after the field's content, as WebDriver's Send Keys did (R9), then presses the key if Playwright has it.
func fillOne(ctx context.Context, webCtx *WebContext, f Fill) error {
	elt, err := findOne(ctx, webCtx, f.Find)
	if err != nil {
		return err
	}
	t := stepTimeout(f.SyncTimeout)
	if err := webCtx.do(elementRead("the caret move", f.SyncTimeout), func() error {
		_, err := elt.Evaluate(jsCaretToEnd, nil, playwright.LocatorEvaluateOptions{Timeout: t})
		return err
	}); err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error {
		return elt.PressSequentially(f.Text, playwright.LocatorPressSequentiallyOptions{Timeout: t})
	}); err != nil {
		return err
	}
	if f.Key != nil {
		if key, ok := playwrightKey(ctx, *f.Key); ok {
			if err := webCtx.do(ownTimeout, func() error { return elt.Press(key, playwright.LocatorPressOptions{Timeout: t}) }); err != nil {
				return err
			}
		}
	}
	return webCtx.settle()
}

// findAndRead counts the matching elements and reads the text and value of the first one.
func findAndRead(ctx context.Context, webCtx *WebContext, findElement interface{}, r *Result) error {
	elts, err := find(ctx, webCtx, findElement, r)
	if err != nil {
		return err
	} else if r.Find > 0 {
		return readElement(webCtx, elts.first(), r)
	}
	return nil
}

// navigate optionally resets the browser, then opens the URL.
func navigate(ctx context.Context, webCtx *WebContext, n *Navigate) error {
	if n.Reset {
		if err := reset(ctx, webCtx); err != nil {
			return err
		}
	}
	return webCtx.goTo(n.URL)
}

// reset answers a pending dialog (accept, whatever it answers), deletes the cookies and opens about:blank (R11).
func reset(ctx context.Context, webCtx *WebContext) error {
	if webCtx.dialogPending() {
		if err := webCtx.resolveDialog(true); err != nil {
			venom.Debug(ctx, "web - reset: accepting the dialog failed: %v", err)
		}
	}
	if err := webCtx.do(read("the cookie reset"), func() error { return webCtx.browserCtx.ClearCookies() }); err != nil {
		return fmt.Errorf("unable to reset the browser: %w", err)
	}
	if err := webCtx.goTo("about:blank"); err != nil {
		return fmt.Errorf("unable to reset the browser: %w", err)
	}
	return nil
}

// goTo opens url in the current page, at its top-level frame.
func (w *WebContext) goTo(url string) error {
	w.toMainFrame()
	limit := ownTimeout
	limit.replaces = true
	return w.do(limit, func() error {
		_, err := w.page.Goto(url)
		return err
	})
}

// selectOption selects the option whose visible text is s.Text (FR-13; D110 only found it), then waits the requested
// seconds.
func selectOption(ctx context.Context, webCtx *WebContext, s *Select) error {
	elt, err := findOne(ctx, webCtx, s.Find)
	if err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error {
		_, err := elt.SelectOption(playwright.SelectOptionValues{Labels: &[]string{s.Text}},
			playwright.LocatorSelectOptionOptions{Timeout: stepTimeout(s.SyncTimeout)})
		return err
	}); err != nil {
		return err
	}
	if err := webCtx.settle(); err != nil {
		return err
	}
	sleepSeconds(s.Wait)
	return nil
}

// uploadFiles sets each file on the upload element in turn, as each Send Keys did: a file replaces the selection of a
// single-file input and is added to that of a multiple one. Then it waits the requested seconds.
func uploadFiles(ctx context.Context, webCtx *WebContext, u *UploadFile) error {
	elt, err := findOne(ctx, webCtx, u.Find)
	if err != nil {
		return err
	}
	t := stepTimeout(u.SyncTimeout)
	multiple := false
	if len(u.Files) > 1 {
		v, err := run(webCtx, elementRead("the multiple check", u.SyncTimeout), func() (any, error) {
			return elt.Evaluate(jsMultiple, nil, playwright.LocatorEvaluateOptions{Timeout: t})
		})
		if err != nil {
			return err
		}
		multiple = v == true
	}
	for i, f := range u.Files {
		selection := []string{f}
		if multiple {
			selection = u.Files[:i+1]
		}
		if err := webCtx.do(ownTimeout, func() error {
			return elt.SetInputFiles(selection, playwright.LocatorSetInputFilesOptions{Timeout: t})
		}); err != nil {
			return err
		}
	}
	if err := webCtx.settle(); err != nil {
		return err
	}
	sleepSeconds(u.Wait)
	return nil
}

// selectFrame enters the document of the iframe element it names.
func selectFrame(ctx context.Context, webCtx *WebContext, s *SelectFrame) error {
	elt, err := findOne(ctx, webCtx, s.Find)
	if err != nil {
		return err
	}
	f, err := run(webCtx, elementRead("the frame lookup", s.SyncTimeout), func() (frame, error) {
		return elt.contentFrame(playwright.LocatorElementHandleOptions{Timeout: stepTimeout(s.SyncTimeout)})
	})
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("no such frame: the element found by %v is not a frame", s.Find)
	}
	webCtx.frames = append(webCtx.frames, f)
	return nil
}

// selectParentFrame is selectRootFrame: it goes back to the parent frame, not to the root, as it always did (R10). At
// the top-level frame it stays there.
func selectParentFrame(webCtx *WebContext) error {
	if err := webCtx.refuseWhilePending(); err != nil {
		return err
	}
	if len(webCtx.frames) > 1 {
		webCtx.frames = webCtx.frames[:len(webCtx.frames)-1]
	}
	return nil
}

// nextWindow switches to the next page of the context, the first one after the last (FR-11; D111 always failed).
func nextWindow(webCtx *WebContext) error {
	if err := webCtx.refuseWhilePending(); err != nil {
		return err
	}
	pages := webCtx.browserCtx.pages()
	current := -1
	for i, p := range pages {
		if p == webCtx.page {
			current = i
		}
	}
	if current == -1 {
		return fmt.Errorf("no such window: the current window is closed")
	}
	next := pages[(current+1)%len(pages)]
	if err := webCtx.do(read("the window switch"), next.BringToFront); err != nil {
		return err
	}
	webCtx.switchTo(next)
	return nil
}

// historyAction goes back, refreshes or goes forward; the value is matched case-insensitively.
func historyAction(webCtx *WebContext, action string) error {
	var move func() (playwright.Response, error)
	switch strings.ToLower(action) {
	case "back":
		move = func() (playwright.Response, error) { return webCtx.page.GoBack() }
	case "refresh":
		move = func() (playwright.Response, error) { return webCtx.page.Reload() }
	case "forward":
		move = func() (playwright.Response, error) { return webCtx.page.GoForward() }
	default:
		return fmt.Errorf("history action '%s' is invalid", action)
	}
	webCtx.toMainFrame()
	limit := ownTimeout
	limit.replaces = true
	return webCtx.do(limit, func() error {
		_, err := move()
		return err
	})
}

// executeScript runs the command as the body of a function, as WebDriver's Execute Script did (R8): return and
// arguments[i] keep working. The arguments are an empty list rather than null when there is none. A script that has
// not returned after scriptTimeout, a returned promise included, fails the step (R17). What it returns is r's value
// and valuejson (FR-07); a dialog that interrupts it leaves both empty, as a command that opens one has done its part.
func executeScript(ctx context.Context, webCtx *WebContext, x *Execute, r *Result) error {
	args := []string{}
	if len(x.Args) > 0 { // len() of a nil slice is 0 — the nil check was redundant (S1009)
		args = x.Args
	}
	body := "(args) => (function(){\n" + x.Command + "\n}).apply(null, args)"
	v, err := run(webCtx, script(), func() (any, error) { return webCtx.frame().Evaluate(body, args) })
	if err != nil && !errors.Is(err, errDialogOpened) {
		return err
	}
	if err := webCtx.settle(); err != nil {
		return err
	}
	text, typed, err := scriptValue(v)
	if err != nil {
		// A value venom cannot flatten would panic the run; the step passes, as it did before FR-07.
		venom.Warning(ctx, "web - execute: %v: result.value and result.valuejson are left empty", err)
		return nil
	}
	r.Value, r.ValueJSON = text, typed
	return nil
}

// find counts the elements matching findElement in the current frame, without waiting (R15), and reads the text and
// value of the only one.
func find(ctx context.Context, webCtx *WebContext, findElement interface{}, r *Result) (element, error) {
	elts, err := locate(ctx, webCtx, findElement)
	if err != nil {
		return nil, err
	}
	n, err := run(webCtx, read("the element count"), elts.Count)
	if err != nil {
		return nil, err
	}
	r.Find = n
	if r.Find == 1 {
		if err := readElement(webCtx, elts.first(), r); err != nil {
			return nil, err
		}
	}
	return elts, nil
}

// readElement reads the rendered text and the value property (R7), keeping the previous one when a read fails; a
// value that is not a string reads as "", as it did under WebDriver. A page that does not answer fails the step
// (R17).
func readElement(webCtx *WebContext, elt element, r *Result) error {
	text, err := run(webCtx, read("the element text read"), func() (string, error) { return elt.InnerText() })
	if err == nil {
		r.Text = text
	} else if errors.Is(err, errNoAnswer) {
		return err
	}
	value, err := run(webCtx, read("the element value read"), func() (any, error) { return elt.Evaluate(jsValue, nil) })
	if err == nil {
		r.Value = stringValue(value)
	} else if errors.Is(err, errNoAnswer) {
		return err
	}
	return nil
}

// stringValue is v when it is a string, else "" (undefined, a number, a boolean).
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// findOne is the first element matching findElement in the current frame, as WebDriver's Find Element: an action on it
// waits for it to be actionable (FR-04).
func findOne(ctx context.Context, webCtx *WebContext, findElement interface{}) (element, error) {
	elts, err := locate(ctx, webCtx, findElement)
	if err != nil {
		return nil, err
	}
	return elts.first(), nil
}

// locate is every element matching findElement in the current frame. A CSS selector pierces open shadow roots
// (FR-05); an XPath does not.
func locate(ctx context.Context, webCtx *WebContext, findElement interface{}) (element, error) {
	selector, locator, err := readFindElement(ctx, findElement)
	if err != nil {
		return nil, err
	}
	engine := "css="
	if locator == xpathLocator {
		engine = "xpath="
	}
	return webCtx.frame().locator(engine + selector), nil
}

// Identify selector and locator from a find element
func readFindElement(ctx context.Context, findElement interface{}) (string, string, error) {
	selector := ""
	locator := cssLocator
	if find, ok := findElement.(string); ok {
		venom.Warning(ctx, "web - findElement : this find element syntax deprecated and will be not supported in next version")
		selector = find
	} else if find, ok := findElement.(map[string]interface{}); ok {
		selector, ok = find["selector"].(string)
		if !ok {
			return "", "", fmt.Errorf("invalid find element selector %v (must be a string)", find["selector"])
		}
		if find["locator"] == "XPATH" {
			locator = xpathLocator
		} else if find["locator"] != "CSS" {
			return "", "", fmt.Errorf("invalid find element locator '%s' (must be CSS or XPATH)", find["locator"])
		}
	} else {
		return "", "", fmt.Errorf("invalid find element structure %v\n (must be a string or a element with selector and locator attribute)", findElement)
	}

	return selector, locator, nil
}

// takeScreenshot writes the viewport of the page as PNG, whatever the file's extension, as WebDriver's screenshots
// were.
func takeScreenshot(webCtx *WebContext, filename string) error {
	png, err := run(webCtx, ownTimeout, func() ([]byte, error) { return webCtx.page.Screenshot() })
	if err != nil {
		return err
	}
	return os.WriteFile(filename, png, 0o600)
}

// generateErrorHTMLFile generates an HTML file in error case to identify clearly the error: the source of the current
// frame, as WebDriver's Get Page Source returned.
func generateErrorHTMLFile(ctx context.Context, webCtx *WebContext, name string) error {
	select {
	case <-webCtx.abandoned: // nil, never ready, when there is none
		webCtx.abandoned = nil
	default:
		if webCtx.abandoned != nil {
			// The call that did not answer still runs: the read would wait for it (R17).
			venom.Warning(ctx, "web - a browser call did not answer and is still running: %s.dump.html is not written", name)
			return nil
		}
	}
	if webCtx.nav.navigating(webCtx.frame()) {
		// Playwright would hold the read until the navigation ends (R16).
		warnNavigating(ctx, name)
		return nil
	}
	html, err := run(webCtx, read("the page source read"), webCtx.frame().Content)
	if err != nil {
		return err
	}
	filename := name + ".dump.html"
	venom.Info(ctx, "Content of the HTML page is saved in %s", filename)
	return os.WriteFile(filename, []byte(html), 0o600)
}

// contentWhileNavigating is Playwright's untyped Frame._content error while the document is replaced (driver 1.62.1).
const contentWhileNavigating = "Unable to retrieve content because the page is navigating"

// warnNavigating says the dump is skipped because the page navigates (FR-09).
func warnNavigating(ctx context.Context, name string) {
	venom.Warning(ctx, "web - the page is navigating: %s.dump.html is not written", name)
}

// warnDumpFailed logs why a failed action left no dump; a failed load replaces the document unseen by the tracker.
func warnDumpFailed(ctx context.Context, name string, err error) {
	if strings.Contains(err.Error(), contentWhileNavigating) {
		warnNavigating(ctx, name)
		return
	}
	venom.Warn(ctx, "Error while generating the HTML file: %v", err)
}
