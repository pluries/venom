package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

// Playwright's browser names.
const (
	chromium = "chromium"
	firefox  = "firefox"
	webkit   = "webkit"
)

const (
	// playwrightVersion is the playwright-go module version in go.mod (a test keeps them equal): its driver and
	// browser builds are the ones installCommand installs.
	playwrightVersion = "v0.6201.1"
	// actionTimeout bounds an action's wait for its element when the step sets no syncTimeout (FR-04, R15).
	actionTimeout = 30_000.0
	// defaultNavigationTimeout is web.timeout's default, in seconds.
	defaultNavigationTimeout = 180
)

// setupConfig is what Setup reads from the suite variables (web.*), mapped onto Playwright (FR-10, R5).
type setupConfig struct {
	browser           string
	launch            playwright.BrowserTypeLaunchOptions
	context           playwright.BrowserNewContextOptions
	navigationTimeout float64 // milliseconds
	// trace is web.trace, off by default: a failing step leaves its Playwright trace (FR-08,
	// docs/feat/2048/research.md R9).
	trace bool
}

func readSetupConfig(ctx context.Context) setupConfig {
	name := browserName(venom.StringVarFromCtx(ctx, "web.driver"))
	warnInertVariables(ctx, name)
	return setupConfig{
		browser:           name,
		launch:            launchOptions(ctx, name),
		context:           contextOptions(ctx),
		navigationTimeout: navigationTimeout(ctx),
		trace:             venom.BoolVarFromCtx(ctx, "web.trace"),
	}
}

// browserName maps web.driver onto Playwright's browsers; any other value is chrome, as it always was.
func browserName(driver string) string {
	switch driver {
	case "gecko":
		return firefox
	case "webkit":
		return webkit
	default:
		return chromium
	}
}

func launchOptions(ctx context.Context, name string) playwright.BrowserTypeLaunchOptions {
	o := playwright.BrowserTypeLaunchOptions{
		Args: browserArgs(name, venom.StringSliceVarFromCtx(ctx, "web.args")),
		// Playwright's own default is headless; the executor's has always been a visible browser.
		Headless: playwright.Bool(venom.BoolVarFromCtx(ctx, "web.headless")),
	}
	if proxy := venom.StringVarFromCtx(ctx, "web.proxy"); proxy != "" {
		o.Proxy = &playwright.Proxy{Server: proxy}
	}
	if name == chromium {
		// The OS sandbox stays on, as chromedriver left it (R18): Playwright turns it off unless asked to keep it, and a
		// suite that passes --no-sandbox in web.args still turns it off.
		o.ChromiumSandbox = playwright.Bool(true)
		if binary := venom.StringVarFromCtx(ctx, "web.binaryPath"); binary != "" {
			o.ExecutablePath = &binary
		}
	}
	if prefs := venom.StringMapInterfaceVarFromCtx(ctx, "web.prefs"); len(prefs) > 0 && name == firefox {
		o.FirefoxUserPrefs = prefs
	}
	return o
}

// browserArgs strips one pair of surrounding double quotes from each argument, as the WebDriver client did. For
// Chromium it also adds the "--" chromedriver added to a switch written without a dash ("headless", "mute-audio"):
// Playwright passes arguments as they are, and Chromium reads a bare word as a URL to open.
func browserArgs(name string, args []string) []string {
	var out []string
	for _, a := range args {
		if len(a) > 2 && a[0] == '"' && a[len(a)-1] == '"' {
			a = a[1 : len(a)-1]
		}
		if name == chromium && !strings.HasPrefix(a, "-") {
			a = "--" + a
		}
		out = append(out, a)
	}
	return out
}

// contextOptions sizes the viewport from web.width and web.height; without both, the page keeps the browser window's
// own size, as it did under WebDriver, instead of Playwright's 1280x720. Certificate errors are ignored, as the
// WebDriver session did (acceptInsecureCerts), unless web.verifyTls is set.
func contextOptions(ctx context.Context) playwright.BrowserNewContextOptions {
	o := playwright.BrowserNewContextOptions{IgnoreHttpsErrors: playwright.Bool(!venom.BoolVarFromCtx(ctx, "web.verifyTls"))}
	width, height := venom.IntVarFromCtx(ctx, "web.width"), venom.IntVarFromCtx(ctx, "web.height")
	if width > 0 && height > 0 {
		o.Viewport = &playwright.Size{Width: width, Height: height}
	} else {
		o.NoViewport = playwright.Bool(true)
	}
	return o
}

// navigationTimeout is web.timeout (seconds, default 180), the bound of page loads: navigate, historyAction, reset
// and the load an action starts.
func navigationTimeout(ctx context.Context) float64 {
	seconds := venom.IntVarFromCtx(ctx, "web.timeout")
	if seconds <= 0 {
		seconds = defaultNavigationTimeout
	}
	return float64(seconds) * 1000
}

const logLevelGone = "it set the log level of the WebDriver client, which is gone; venom's -v flags set venom's own"

// inertVariables are the variables Playwright cannot honour (R5): each is still accepted, and logs one warning.
var inertVariables = []struct {
	name   string
	set    func(ctx context.Context, browser string) bool
	reason string
}{
	{"driverPath", stringSet("web.driverPath"), "Playwright starts its own driver"},
	{"driverPort", stringSet("web.driverPort"), "Playwright talks to its driver over a pipe, not a port"},
	{"detach", boolSet("web.detach"), "Playwright has no detach mode: the browser closes at TearDown"},
	{"debug", boolSet("web.debug"), logLevelGone},
	{"logLevel", stringSet("web.logLevel"), logLevelGone},
	{"binaryPath", unlessBrowser(chromium, stringSet("web.binaryPath")), "Playwright drives its own build of Firefox and WebKit"},
	{"prefs", unlessBrowser(firefox, mapSet("web.prefs")), "only Firefox takes launch preferences"},
}

func warnInertVariables(ctx context.Context, browser string) {
	for _, v := range inertVariables {
		if v.set(ctx, browser) {
			venom.Warning(ctx, "web - %s is ignored: %s", v.name, v.reason)
		}
	}
}

func stringSet(name string) func(context.Context, string) bool {
	return func(ctx context.Context, _ string) bool { return venom.StringVarFromCtx(ctx, name) != "" }
}

func boolSet(name string) func(context.Context, string) bool {
	return func(ctx context.Context, _ string) bool { return venom.BoolVarFromCtx(ctx, name) }
}

func mapSet(name string) func(context.Context, string) bool {
	return func(ctx context.Context, _ string) bool {
		return len(venom.StringMapInterfaceVarFromCtx(ctx, name)) > 0
	}
}

// unlessBrowser is set only when the suite drives another browser than the one the variable applies to.
func unlessBrowser(applies string, set func(context.Context, string) bool) func(context.Context, string) bool {
	return func(ctx context.Context, browser string) bool { return browser != applies && set(ctx, browser) }
}

// launchError explains a launch failure: a browser that is not installed, or a Chromium sandbox this host cannot start
// (R20). Playwright recognises the second case, but its advice, its chromiumSandbox option, is not a suite variable.
func launchError(browser string, err error) error {
	if msg := err.Error(); strings.Contains(msg, "No usable sandbox") || strings.Contains(msg, "Chromium sandboxing failed") {
		return fmt.Errorf("unable to launch %s: its sandbox cannot start on this host, which restricts unprivileged user "+
			"namespaces (as Ubuntu 23.10 and later do): pass --no-sandbox in args, point binaryPath at a system Chrome, or "+
			"allow the namespaces (sysctl kernel.apparmor_restrict_unprivileged_userns=0): %w", browser, err)
	}
	return fmt.Errorf("unable to launch %s (install it with: %s): %w", browser, installCommand(browser), err)
}

// installCommand installs the driver and the browser, with the OS packages the browser needs (R14): Setup never
// downloads anything itself.
func installCommand(browser string) string {
	return "go run github.com/mxschmitt/playwright-go/cmd/playwright@" + playwrightVersion + " install --with-deps " + browser
}
