package web

import "github.com/mxschmitt/playwright-go"

// The seam between the executor and Playwright (docs/feat/2047/research.md R4): only the operations the executor
// calls. Methods named like Playwright's keep Playwright's exact signature, so the adapter (playwright.go) satisfies
// them by embedding the Playwright object; the lower-case ones return another seam type and are the adapter's
// one-line wrappers. The unit tests implement the same interfaces with a Go fake (web_helpers_test.go): no browser,
// no driver process, no network.

// runPlaywright starts the Playwright driver. Unit tests replace it with the fake.
var runPlaywright = startPlaywright

// driver is the running Playwright driver process.
type driver interface {
	// launch starts one browser; name is chromium, firefox or webkit.
	launch(name string, options playwright.BrowserTypeLaunchOptions) (browser, error)
	Stop() error
}

type browser interface {
	newContext(options playwright.BrowserNewContextOptions) (browserContext, error)
	Close(options ...playwright.BrowserCloseOptions) error
}

// browserContext is the suite's browser session (FR-03): its pages, cookies, default timeouts, dialogs and navigations.
type browserContext interface {
	// onDialog registers the handler of every dialog any page of the context opens.
	onDialog(handler func(dialog))
	// onRequest, onRequestFailed and onFrameNavigated register the handlers of every request any page of the context
	// sends, of those that fail, and of every navigation a frame commits (R16). Handlers run on Playwright's goroutine.
	onRequest(handler func(request))
	onRequestFailed(handler func(request))
	onFrameNavigated(handler func(frame))
	// route stubs every request of the context whose URL matches pattern, a Playwright glob, with handler; handlers
	// run on a goroutine of Playwright's (docs/feat/2048/research.md R4).
	route(pattern string, handler func(route)) error
	// tracing is the context's trace recorder (FR-08).
	tracing() tracing
	newPage() (page, error)
	pages() []page
	SetDefaultTimeout(timeout float64)
	SetDefaultNavigationTimeout(timeout float64)
	ClearCookies(options ...playwright.BrowserContextClearCookiesOptions) error
	Close(options ...playwright.BrowserContextCloseOptions) error
}

// page is one tab of the context. Navigation, title, URL and screenshots are top-level, as in WebDriver.
type page interface {
	mainFrame() frame
	Title() (string, error)
	URL() string
	Screenshot(options ...playwright.PageScreenshotOptions) ([]byte, error)
	Goto(url string, options ...playwright.PageGotoOptions) (playwright.Response, error)
	GoBack(options ...playwright.PageGoBackOptions) (playwright.Response, error)
	GoForward(options ...playwright.PageGoForwardOptions) (playwright.Response, error)
	Reload(options ...playwright.PageReloadOptions) (playwright.Response, error)
	BringToFront() error
}

// frame is a document: the page's main frame or an iframe entered with selectFrame (R10).
type frame interface {
	locator(selector string) element
	Content() (string, error)
	Evaluate(expression string, arg ...any) (any, error)
	WaitForLoadState(options ...playwright.FrameWaitForLoadStateOptions) error
}

// element is a Playwright locator: lazy, resolved again by every operation.
type element interface {
	first() element
	// contentFrame is the document of an iframe element; nil, and no error, for any other element.
	contentFrame(options playwright.LocatorElementHandleOptions) (frame, error)
	Count() (int, error)
	Click(options ...playwright.LocatorClickOptions) error
	PressSequentially(text string, options ...playwright.LocatorPressSequentiallyOptions) error
	Press(key string, options ...playwright.LocatorPressOptions) error
	SelectOption(values playwright.SelectOptionValues, options ...playwright.LocatorSelectOptionOptions) ([]string, error)
	SetInputFiles(files any, options ...playwright.LocatorSetInputFilesOptions) error
	InnerText(options ...playwright.LocatorInnerTextOptions) (string, error)
	Evaluate(expression string, arg any, options ...playwright.LocatorEvaluateOptions) (any, error)
}

// request is a request a page sends: a document the frame navigates to, or a resource.
type request interface {
	IsNavigationRequest() bool
	// frame is the frame that sent it.
	frame() frame
}

// dialog is an alert, confirm, prompt or beforeunload dialog; playwright.Dialog satisfies it as is.
type dialog interface {
	Accept(promptText ...string) error
	Dismiss() error
}

// route is a request a mock intercepted (FR-06); playwright.Route satisfies it as is.
type route interface {
	Fulfill(options ...playwright.RouteFulfillOptions) error
}

// tracing records the context's actions, DOM snapshots and network into chunks, each saved or discarded (FR-08);
// playwright.Tracing satisfies it as is.
type tracing interface {
	Start(options ...playwright.TracingStartOptions) error
	StartChunk(options ...playwright.TracingStartChunkOptions) error
	StopChunk(path ...string) error
}
