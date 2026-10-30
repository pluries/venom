package web

import (
	"context"
	"fmt"

	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

// Network mocks (FR-06). Research ids here are docs/feat/2048/research.md's, except R17 (docs/feat/2047/research.md).

// checkMocks refuses a step's mocks before any is registered: each needs a url whose braces balance, and its status is
// either unset, Playwright's 200, or an HTTP status (R5). Entries are counted from 1, as the message says ("mock 2 of
// 3").
func checkMocks(mocks []Mock) error {
	for i, m := range mocks {
		if m.URL == "" {
			return fmt.Errorf("mock %d of %d: url is required", i+1, len(mocks))
		}
		if !balancedBraces(m.URL) {
			return fmt.Errorf("mock %d of %d (%s): url has an unbalanced { or }", i+1, len(mocks), m.URL)
		}
		if m.Status != 0 && (m.Status < 100 || m.Status > 599) {
			return fmt.Errorf("mock %d of %d (%s): status %d is not an HTTP status (100-599)", i+1, len(mocks), m.URL, m.Status)
		}
	}
	return nil
}

// balancedBraces reports whether every { of a glob is closed and no } comes before its {. playwright-go turns them
// into a regular expression's ( and ) and compiles it with regexp.MustCompile, which panics the run on an unbalanced
// one; every other character it escapes. A \ escapes the character after it, as playwright-go reads it.
func balancedBraces(glob string) bool {
	depth := 0
	runes := []rune(glob)
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// addMocks registers the step's mocks on the context, in their order: they answer until the test case ends, and a
// later mock whose pattern also matches answers first, as Playwright tries the newest route first (R1). A registration
// is a page command: a pending dialog is dismissed and fails the step, since registering while one is open hangs (R3),
// and the read limit bounds a browser that does not answer (R17).
func (w *WebContext) addMocks(ctx context.Context, mocks []Mock) error {
	if err := checkMocks(mocks); err != nil {
		return err
	}
	for i, m := range mocks {
		handler := fulfil(ctx, m)
		if err := w.do(read("the mock registration"), func() error { return w.browserCtx.route(m.URL, handler) }); err != nil {
			return fmt.Errorf("mock %d of %d (%s): unable to register it: %w", i+1, len(mocks), m.URL, err)
		}
	}
	return nil
}

// fulfil is the handler of m. It runs on Playwright's goroutine, maybe after its step has returned: a failure has no
// step to fail, so it is logged, and the handler never panics, which would fail Playwright's whole connection (R4).
func fulfil(ctx context.Context, m Mock) func(route) {
	return func(r route) {
		if err := r.Fulfill(fulfilOptions(m)); err != nil {
			venom.Warning(ctx, "web - the mock of %s could not answer: %v", m.URL, err)
		}
	}
}

// fulfilOptions is m's response. An unset field is left to Playwright: status 200, no content type, no header, and
// the empty body it sends anyway (R5).
func fulfilOptions(m Mock) playwright.RouteFulfillOptions {
	var o playwright.RouteFulfillOptions
	if m.Status != 0 {
		o.Status = playwright.Int(m.Status)
	}
	if m.ContentType != "" {
		o.ContentType = playwright.String(m.ContentType)
	}
	if len(m.Headers) > 0 {
		o.Headers = m.Headers
	}
	if m.Body != "" {
		o.Body = m.Body
	}
	return o
}
