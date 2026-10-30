package web

import "github.com/mxschmitt/playwright-go"

// The Playwright implementation of the seam (page.go). Every call into playwright-go lives here, and only here: one
// delegation per method, no behaviour, so all the behaviour is in code the browser-free unit tests cover. The
// real-browser tests (web_playwright_test.go, build tag playwright) exercise this file.

// startPlaywright connects to an installed driver. playwright.Run never downloads: a missing driver is an error.
func startPlaywright() (driver, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	return pwDriver{pw, map[string]playwright.BrowserType{chromium: pw.Chromium, firefox: pw.Firefox, webkit: pw.WebKit}}, nil
}

type pwDriver struct {
	*playwright.Playwright
	types map[string]playwright.BrowserType
}

// A failed call returns its wrapper around a nil object with the error: callers never use the object then.

func (d pwDriver) launch(name string, o playwright.BrowserTypeLaunchOptions) (browser, error) {
	b, err := d.types[name].Launch(o)
	return pwBrowser{b}, err
}

type pwBrowser struct{ playwright.Browser }

func (b pwBrowser) newContext(o playwright.BrowserNewContextOptions) (browserContext, error) {
	c, err := b.NewContext(o)
	return pwContext{c}, err
}

type pwContext struct{ playwright.BrowserContext }

func (c pwContext) onDialog(handler func(dialog)) {
	c.OnDialog(func(d playwright.Dialog) { handler(d) })
}

func (c pwContext) onRequest(handler func(request)) {
	c.OnRequest(func(r playwright.Request) { handler(pwRequest{r}) })
}

func (c pwContext) onRequestFailed(handler func(request)) {
	c.OnRequestFailed(func(r playwright.Request) { handler(pwRequest{r}) })
}

func (c pwContext) onFrameNavigated(handler func(frame)) {
	c.OnFrameNavigated(func(f playwright.Frame) { handler(pwFrame{f}) })
}

func (c pwContext) route(pattern string, handler func(route)) error {
	return c.Route(pattern, func(r playwright.Route) { handler(r) })
}

func (c pwContext) tracing() tracing { return c.Tracing() }

// pwRequest and pwFrame compare equal when they wrap the same Playwright object: the navigation tracker keys on them.
type pwRequest struct{ playwright.Request }

func (r pwRequest) frame() frame { return pwFrame{r.Frame()} }

func (c pwContext) newPage() (page, error) {
	p, err := c.NewPage()
	return pwPage{p}, err
}

func (c pwContext) pages() []page {
	var all []page
	for _, p := range c.Pages() {
		all = append(all, pwPage{p})
	}
	return all
}

type pwPage struct{ playwright.Page }

func (p pwPage) mainFrame() frame { return pwFrame{p.MainFrame()} }

type pwFrame struct{ playwright.Frame }

func (f pwFrame) locator(selector string) element { return pwLocator{f.Locator(selector)} }

type pwLocator struct{ playwright.Locator }

func (l pwLocator) first() element { return pwLocator{l.First()} }

func (l pwLocator) contentFrame(o playwright.LocatorElementHandleOptions) (frame, error) {
	h, err := l.ElementHandle(o) //nolint:staticcheck // SA1019: Locator.ContentFrame is a FrameLocator, which has no source, script or load state; the handle is used at once, not kept
	if err != nil {
		return nil, err
	}
	f, err := h.ContentFrame()
	if f == nil {
		return nil, err
	}
	return pwFrame{f}, err
}
