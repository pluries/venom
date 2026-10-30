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

	"github.com/mitchellh/mapstructure"
	"github.com/mxschmitt/playwright-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of the web executor on a Go fake of the seam (web_helpers_test.go): no browser (NFR-04). They carry the
// expectations of the characterisation tests written on the WebDriver executor (#2047 Task 1, d35fe6a) over to the
// Playwright one: same scenarios, same result fields; the calls they pin are the seam's instead of WebDriver's. The
// deliberate changes are D110 (select now selects) and D111 (nextWindow now switches), research.md R12. The
// real-browser counterparts are web_playwright_test.go (build tag playwright) and the golden master.

func TestNew(t *testing.T) {
	assert.Equal(t, &Executor{}, New())
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
}

func TestSetup_launchOptions(t *testing.T) {
	tests := []struct {
		name        string
		vars        map[string]any
		wantBrowser string
		wantLaunch  playwright.BrowserTypeLaunchOptions
	}{
		{
			// R18: the OS sandbox is on, as chromedriver left it; Playwright's own default turns it off.
			name:        "chrome is the default driver, headful, sandboxed",
			wantBrowser: chromium,
			wantLaunch:  playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false), ChromiumSandbox: playwright.Bool(true)},
		},
		{
			name:        "an unknown driver name falls back to chrome",
			vars:        map[string]any{"driver": "safari"},
			wantBrowser: chromium,
			wantLaunch:  playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false), ChromiumSandbox: playwright.Bool(true)},
		},
		{
			// The WebDriver client stripped one pair of surrounding double quotes; chromedriver added "--" to a bare
			// switch.
			name:        "args and binaryPath reach chromium, quotes stripped, a bare switch prefixed",
			vars:        map[string]any{"args": []string{`"--lang=fr"`, "--mute-audio", "browser-test", "-single"}, "binaryPath": "/opt/synthetic/chrome"},
			wantBrowser: chromium,
			wantLaunch: playwright.BrowserTypeLaunchOptions{
				Args:            []string{"--lang=fr", "--mute-audio", "--browser-test", "-single"},
				ExecutablePath:  playwright.String("/opt/synthetic/chrome"),
				Headless:        playwright.Bool(false),
				ChromiumSandbox: playwright.Bool(true),
			},
		},
		{
			// R18: the suite's own --no-sandbox turns the sandbox off, as it did with chromedriver: Chromium reads it.
			name:        "a suite's --no-sandbox still reaches chromium",
			vars:        map[string]any{"args": []string{"--no-sandbox", "headless"}},
			wantBrowser: chromium,
			wantLaunch: playwright.BrowserTypeLaunchOptions{
				Args:            []string{"--no-sandbox", "--headless"},
				Headless:        playwright.Bool(false),
				ChromiumSandbox: playwright.Bool(true),
			},
		},
		{
			// As before: prefs never reached chrome.
			name:        "prefs are dropped on chrome",
			vars:        map[string]any{"prefs": map[string]interface{}{"intl.accept_languages": "fr"}},
			wantBrowser: chromium,
			wantLaunch:  playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false), ChromiumSandbox: playwright.Bool(true)},
		},
		{
			name:        "headless and proxy",
			vars:        map[string]any{"headless": true, "proxy": "proxy.example.test:3128", "detach": true},
			wantBrowser: chromium,
			wantLaunch: playwright.BrowserTypeLaunchOptions{
				Headless:        playwright.Bool(true),
				Proxy:           &playwright.Proxy{Server: "proxy.example.test:3128"},
				ChromiumSandbox: playwright.Bool(true),
			},
		},
		{
			// ChromiumSandbox is Chromium's alone: left unset for Firefox and WebKit.
			name:        "gecko is firefox, on Playwright's own build",
			vars:        map[string]any{"driver": "gecko", "binaryPath": "/opt/synthetic/firefox"},
			wantBrowser: firefox,
			wantLaunch:  playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(false)},
		},
		{
			name: "gecko args stay as they are; headless, proxy and prefs",
			vars: map[string]any{
				"driver": "gecko", "args": []string{"-private", `"-safe-mode"`},
				"headless": true, "proxy": "proxy.example.test:3128", "prefs": map[string]interface{}{"intl.accept_languages": "fr"},
			},
			wantBrowser: firefox,
			wantLaunch: playwright.BrowserTypeLaunchOptions{
				Args:             []string{"-private", "-safe-mode"},
				Headless:         playwright.Bool(true),
				Proxy:            &playwright.Proxy{Server: "proxy.example.test:3128"},
				FirefoxUserPrefs: map[string]any{"intl.accept_languages": "fr"},
			},
		},
		{
			name:        "webkit is new",
			vars:        map[string]any{"driver": "webkit", "args": []string{"bare"}},
			wantBrowser: webkit,
			wantLaunch:  playwright.BrowserTypeLaunchOptions{Args: []string{"bare"}, Headless: playwright.Bool(false)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx, err := setupFake(t, tt.vars)
			require.NoError(t, err)
			require.NotNil(t, getWebCtx(ctx))
			assert.Equal(t, tt.wantBrowser, pw.browserName)
			assert.Equal(t, tt.wantLaunch, pw.launchOptions)
		})
	}
}

func TestSetup_session(t *testing.T) {
	t.Run("opens a context and a page, with the timeouts, the dialog and navigation handlers set first", func(t *testing.T) {
		pw := withFake(t)
		ctx, err := setupFake(t, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{
			"Run", "Launch chromium", "NewContext", "OnDialog",
			"OnRequest", "OnRequestFailed", "OnFrameNavigated",
			"SetDefaultTimeout 30000", "SetDefaultNavigationTimeout 180000", "NewPage",
		}, pw.seen())
		webCtx := getWebCtx(ctx)
		require.NotNil(t, webCtx)
		assert.Equal(t, pw.pageList[0], webCtx.page)
		assert.Len(t, webCtx.frames, 1)
	})

	t.Run("timeout is the navigation timeout, in seconds", func(t *testing.T) {
		pw := withFake(t)
		_, err := setupFake(t, map[string]any{"timeout": 5})
		require.NoError(t, err)
		assert.Contains(t, pw.seen(), "SetDefaultNavigationTimeout 5000")
		assert.Contains(t, pw.seen(), "SetDefaultTimeout 30000", "actions keep their own 30 s")
	})

	t.Run("a zero or negative timeout keeps the 180 s default", func(t *testing.T) {
		pw := withFake(t)
		_, err := setupFake(t, map[string]any{"timeout": -1})
		require.NoError(t, err)
		assert.Contains(t, pw.seen(), "SetDefaultNavigationTimeout 180000")
	})

	t.Run("debug and logLevel leave venom's log level alone", func(t *testing.T) {
		// Changed (was: the WebDriver client set logrus' standard level, venom's own, from its log level).
		prevLevel := logrus.GetLevel()
		t.Cleanup(func() { logrus.SetLevel(prevLevel) })
		logrus.SetLevel(logrus.InfoLevel)
		withFake(t)
		_, err := setupFake(t, map[string]any{"debug": true, "logLevel": "ERROR"})
		require.NoError(t, err)
		assert.Equal(t, logrus.InfoLevel, logrus.GetLevel())
	})
}

func TestSetup_verifyTLS(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]any
		want bool
	}{
		{name: "unset keeps ignoring certificate errors", vars: nil, want: true},
		{name: "false keeps ignoring certificate errors", vars: map[string]any{"verifyTls": false}, want: true},
		{name: "true makes the browser reject them", vars: map[string]any{"verifyTls": true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			_, err := setupFake(t, tt.vars)
			require.NoError(t, err)
			require.NotNil(t, pw.contextOptions.IgnoreHttpsErrors)
			assert.Equal(t, tt.want, *pw.contextOptions.IgnoreHttpsErrors)
		})
	}
}

func TestSetup_viewport(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]any
		want playwright.BrowserNewContextOptions
	}{
		{
			name: "width and height size the viewport",
			vars: map[string]any{"width": 800, "height": 600},
			want: playwright.BrowserNewContextOptions{IgnoreHttpsErrors: playwright.Bool(true), Viewport: &playwright.Size{Width: 800, Height: 600}},
		},
		{
			name: "width alone keeps the window's size",
			vars: map[string]any{"width": 800},
			want: playwright.BrowserNewContextOptions{IgnoreHttpsErrors: playwright.Bool(true), NoViewport: playwright.Bool(true)},
		},
		{
			name: "a negative height keeps the window's size",
			vars: map[string]any{"width": 800, "height": -1},
			want: playwright.BrowserNewContextOptions{IgnoreHttpsErrors: playwright.Bool(true), NoViewport: playwright.Bool(true)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			_, err := setupFake(t, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, pw.contextOptions)
		})
	}
}

func TestSetup_inertVariables(t *testing.T) {
	// V3 (R5): a variable Playwright cannot honour is accepted and logs one warning, never an error.
	tests := []struct {
		name string
		vars map[string]any
		want []string
		// notWanted are the variables that are honoured here, so they must not warn.
		notWanted []string
	}{
		{
			name: "driver process, detach and logging variables",
			vars: map[string]any{"driverPath": "/opt/synthetic/chromedriver", "driverPort": "9515", "detach": true, "debug": true, "logLevel": "DEBUG"},
			want: []string{"driverPath", "driverPort", "detach", "debug", "logLevel"},
		},
		{
			name:      "prefs on chrome; binaryPath is honoured",
			vars:      map[string]any{"prefs": map[string]interface{}{"a": 1}, "binaryPath": "/opt/synthetic/chrome"},
			want:      []string{"prefs"},
			notWanted: []string{"binaryPath"},
		},
		{
			name:      "binaryPath on gecko; prefs are honoured",
			vars:      map[string]any{"driver": "gecko", "binaryPath": "/opt/synthetic/firefox", "prefs": map[string]interface{}{"a": 1}},
			want:      []string{"binaryPath"},
			notWanted: []string{"prefs"},
		},
		{
			name: "binaryPath and prefs on webkit",
			vars: map[string]any{"driver": "webkit", "binaryPath": "/opt/synthetic/webkit", "prefs": map[string]interface{}{"a": 1}},
			want: []string{"binaryPath", "prefs"},
		},
		{
			name:      "unset or false variables do not warn",
			vars:      map[string]any{"detach": false, "debug": false},
			notWanted: []string{"driverPath", "driverPort", "detach", "debug", "logLevel", "binaryPath", "prefs"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFake(t)
			var err error
			logged := stderrOf(t, func() { _, err = Executor{}.Setup(webVarsCtx(tt.vars), nil) })
			require.NoError(t, err)
			for _, name := range tt.want {
				assert.Equal(t, 1, strings.Count(logged, "web - "+name+" is ignored: "), "one warning for %s in:\n%s", name, logged)
			}
			for _, name := range tt.notWanted {
				assert.NotContains(t, logged, "web - "+name+" is ignored")
			}
		})
	}
}

func TestSetup_errors(t *testing.T) {
	t.Run("a driver that is not installed fails with the install command, and nothing is downloaded", func(t *testing.T) {
		// R14, on the real startPlaywright: an empty driver directory is a missing driver.
		venom.InitTestLogger(t)
		dir := t.TempDir()
		t.Setenv("PLAYWRIGHT_DRIVER_PATH", dir)
		t.Setenv("PLAYWRIGHT_CLI_PATH", "")
		ctx, err := Executor{}.Setup(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "install it with: go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium")
		assert.Contains(t, err.Error(), "please install the driver")
		assert.Nil(t, getWebCtx(ctx))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries, "Setup must not download the driver")
	})

	t.Run("the install command names the suite's browser", func(t *testing.T) {
		pw := withFake(t)
		pw.errs["Run"] = errors.New("please install the driver (v1.62.1) first")
		_, err := setupFake(t, map[string]any{"driver": "gecko"})
		require.EqualError(t, err, "unable to start Playwright (install it with: "+
			"go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps firefox): please install the driver (v1.62.1) first")
	})

	tests := []struct {
		name    string
		failing string
		wantErr string
		// wantCalls are the calls from the failing one on: what was opened is closed again.
		wantCalls []string
	}{
		{
			name:      "a browser that is not installed stops the driver",
			failing:   "Launch webkit",
			wantErr:   "unable to launch webkit (install it with: go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps webkit): synthetic failure",
			wantCalls: []string{"Launch webkit", "Stop driver"},
		},
		{
			name:      "a refused context closes the browser",
			failing:   "NewContext",
			wantErr:   "unable to open a browser context: synthetic failure",
			wantCalls: []string{"NewContext", "Close browser", "Stop driver"},
		},
		{
			// Changed (was: a failed resize left the WebDriver session and the driver running).
			name:      "a refused page closes the context",
			failing:   "NewPage",
			wantErr:   "unable to open a page: synthetic failure",
			wantCalls: []string{"NewPage", "Close context", "Close browser", "Stop driver"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			pw.errs[tt.failing] = errors.New("synthetic failure")
			ctx, err := setupFake(t, map[string]any{"driver": "webkit"})
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, getWebCtx(ctx))
			calls := pw.seen()
			for i, c := range calls {
				if c == tt.failing {
					calls = calls[i:]
					break
				}
			}
			assert.Equal(t, tt.wantCalls, calls)
		})
	}

	t.Run("a sandbox the host cannot start names the fixes, not the install command", func(t *testing.T) {
		// R20: Playwright's launch error carries Chromium's log and its own "sandboxing failed" box.
		pw := withFake(t)
		pw.errs["Launch chromium"] = errors.New("playwright: target closed\nBrowser logs:\nChromium sandboxing failed!\nTo avoid the sandboxing issue, configure your environment")
		_, err := setupFake(t, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to launch chromium: its sandbox cannot start on this host")
		assert.Contains(t, err.Error(), "pass --no-sandbox in args, point binaryPath at a system Chrome")
		assert.NotContains(t, err.Error(), "install it with")
	})

	t.Run("a failed close is added to the error", func(t *testing.T) {
		pw := withFake(t)
		pw.errs["NewPage"] = errors.New("synthetic failure")
		pw.errs["Stop driver"] = errors.New("synthetic stop failure")
		_, err := setupFake(t, nil)
		require.EqualError(t, err, "unable to open a page: synthetic failure\nsynthetic stop failure")
	})
}

func TestPlaywrightVersionMatchesGoMod(t *testing.T) {
	// installCommand installs the driver of playwrightVersion: it must be the module the executor is built with.
	goMod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(goMod), "github.com/mxschmitt/playwright-go "+playwrightVersion+"\n")
}

func TestTearDown(t *testing.T) {
	t.Run("closes the context and the browser, then stops the driver", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		require.NoError(t, Executor{}.TearDown(ctx))
		assert.Equal(t, []string{"Close context", "Close browser", "Stop driver"}, pw.seen())
	})

	t.Run("a failed close is the TearDown error, and the rest still closes", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.errs["Close browser"] = errors.New("synthetic failure")
		require.EqualError(t, Executor{}.TearDown(ctx), "synthetic failure")
		assert.Equal(t, []string{"Close context", "Close browser", "Stop driver"}, pw.seen())
	})

	t.Run("a leftover dialog is dismissed and its action finishes before the browser closes", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		clicked := newFlag()
		pw.hooks["Click css=#c first"] = pw.blockOnDialog("confirm", 1, clicked)
		_, err := webRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}}))
		require.NoError(t, err)
		pw.reset()
		require.NoError(t, Executor{}.TearDown(ctx))
		assert.True(t, clicked.isSet())
		assert.Equal(t, []string{"Dismiss confirm1", "Close context", "Close browser", "Stop driver"}, pw.seen())
	})

	t.Run("an action still blocked on a dialog ends with the context", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		clicked := newFlag()
		pw.hooks["Click css=#c first"] = pw.blockOnDialog("confirm", 2, clicked)
		_, err := webRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}}))
		require.NoError(t, err)
		pw.reset()
		require.NoError(t, Executor{}.TearDown(ctx))
		assert.True(t, clicked.isSet(), "TearDown returns once the action has returned")
		assert.Equal(t, []string{"Dismiss confirm1", "Close context", "Close browser", "Stop driver"}, pw.seen())
	})

	t.Run("a failed dismiss is added to the TearDown error", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.openDialog("alert")
		pw.errs["Dismiss alert1"] = errors.New("synthetic failure")
		require.EqualError(t, Executor{}.TearDown(ctx), "synthetic failure")
	})

	t.Run("without a web context it panics", func(t *testing.T) {
		venom.InitTestLogger(t)
		assert.Panics(t, func() { _ = Executor{}.TearDown(context.Background()) }) //nolint:errcheck // it panics before returning
	})
}

func TestGetWebCtx(t *testing.T) {
	assert.Nil(t, getWebCtx(context.Background()))
	assert.Nil(t, getWebCtx(context.WithValue(context.Background(), ContextKey, "not a web context")))
	wc := &WebContext{}
	assert.Same(t, wc, getWebCtx(context.WithValue(context.Background(), ContextKey, wc)))
}

func TestRun_actions(t *testing.T) {
	tests := []struct {
		name  string
		step  venom.TestStep
		nodes map[string][]fakeNode
		errs  map[string]error
		want  Result
		calls []string
	}{
		{
			name:  "no action only reads the page",
			step:  venom.TestStep{},
			want:  pageResult(),
			calls: pageTail(),
		},
		{
			name:  "navigate",
			step:  action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}}),
			want:  pageResult(),
			calls: withTail("Goto page1 https://example.test/next"),
		},
		{
			name: "navigate with reset deletes the cookies and opens about:blank first",
			step: action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next", "reset": true}}),
			want: pageResult(),
			calls: withTail(
				"ClearCookies",
				"Goto page1 about:blank",
				"Goto page1 https://example.test/next",
			),
		},
		{
			name:  "click with a CSS find",
			step:  action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#go")}}),
			want:  pageResult(),
			calls: settled("Click css=#go first"),
		},
		{
			name:  "click with the deprecated string find is a CSS selector",
			step:  action(map[string]interface{}{"click": map[string]interface{}{"find": "#go"}}),
			want:  pageResult(),
			calls: settled("Click css=#go first"),
		},
		{
			// A negative wait is accepted and sleeps for no time (time.Sleep of a negative duration).
			name: "click with an XPATH find, a sync timeout and a negative wait",
			step: action(map[string]interface{}{"click": map[string]interface{}{
				"find":        map[string]interface{}{"selector": "//button", "locator": "XPATH"},
				"syncTimeout": 1,
				"wait":        -1,
			}}),
			want:  pageResult(),
			calls: settled("Click xpath=//button first timeout=1000"),
		},
		{
			// An unknown key name sends nothing, no error.
			name: "fill types each text after the content, then its key",
			step: action(map[string]interface{}{"fill": []interface{}{
				map[string]interface{}{"find": findCSS("#user"), "text": "synthetic-user", "key": "ENTER"},
				map[string]interface{}{"find": findCSS("#note"), "text": "hello", "key": "NO_SUCH_KEY", "syncTimeout": 1},
				map[string]interface{}{"find": findCSS("#empty")},
			}}),
			want: pageResult(),
			calls: slices.Concat(
				[]string{"Evaluate css=#user first caretToEnd", "PressSequentially css=#user first synthetic-user", "Press css=#user first Enter"},
				settleIn("page1"),
				[]string{"Evaluate css=#note first caretToEnd timeout=1000", "PressSequentially css=#note first hello timeout=1000"},
				settleIn("page1"),
				[]string{"Evaluate css=#empty first caretToEnd", "PressSequentially css=#empty first "},
				settleIn("page1"),
				pageTail(),
			),
		},
		{
			// With exactly one match the text and value are read twice (find, then findAndRead), as before.
			name: "find one element reads its text and value, twice",
			step: action(map[string]interface{}{"find": findCSS(".item")}),
			want: Result{Find: 1, Text: "first", Value: "v1", Title: pageTitle, URL: pageURL},
			calls: withTail(
				"Count css=.item",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
			),
		},
		{
			name: "find one element via XPATH reads its text and value, twice",
			step: action(map[string]interface{}{"find": map[string]interface{}{"selector": "//div[@class='item']", "locator": "XPATH"}}),
			want: Result{Find: 1, Text: "first", Value: "v1", Title: pageTitle, URL: pageURL},
			calls: withTail(
				"Count xpath=//div[@class='item']",
				"InnerText xpath=//div[@class='item'] first",
				"Evaluate xpath=//div[@class='item'] first value",
				"InnerText xpath=//div[@class='item'] first",
				"Evaluate xpath=//div[@class='item'] first value",
			),
		},
		{
			name:  "find several elements reads the first one",
			step:  action(map[string]interface{}{"find": findCSS(".item")}),
			nodes: map[string][]fakeNode{"css=.item": {{text: "second", value: "v2"}, {text: "first", value: "v1"}}},
			want:  Result{Find: 2, Text: "second", Value: "v2", Title: pageTitle, URL: pageURL},
			calls: withTail(
				"Count css=.item",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
			),
		},
		{
			name:  "find nothing is not an error",
			step:  action(map[string]interface{}{"find": findCSS(".none")}),
			want:  pageResult(),
			calls: withTail("Count css=.none"),
		},
		{
			name: "find ignores a failed text or value read",
			step: action(map[string]interface{}{"find": findCSS(".item")}),
			errs: map[string]error{
				"InnerText css=.item first":      errors.New("synthetic failure"),
				"Evaluate css=.item first value": errors.New("synthetic failure"),
			},
			want: Result{Find: 1, Title: pageTitle, URL: pageURL},
			calls: withTail(
				"Count css=.item",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
			),
		},
		{
			// As with WebDriver's Get Element Property, whose string decoding failed on anything else.
			name:  "a value that is not a string reads as empty",
			step:  action(map[string]interface{}{"find": findCSS(".item")}),
			nodes: map[string][]fakeNode{"css=.item": {{text: "first", value: 42.0}, {text: "other", value: nil}}},
			want:  Result{Find: 2, Text: "first", Title: pageTitle, URL: pageURL},
			calls: withTail(
				"Count css=.item",
				"InnerText css=.item first",
				"Evaluate css=.item first value",
			),
		},
		{
			// D110 fixed (R12): the option is selected by its visible text; it used to be found and never clicked.
			name: "select selects the option by its text",
			step: action(map[string]interface{}{"select": map[string]interface{}{
				"find": findCSS("#colour"), "text": "Blue", "syncTimeout": 1, "wait": -1,
			}}),
			want:  pageResult(),
			calls: settled("SelectOption css=#colour first label=Blue timeout=1000"),
		},
		{
			name: "uploadFile sets each file in turn on a single-file input",
			step: action(map[string]interface{}{"uploadFile": map[string]interface{}{
				"find": findCSS("#upload"), "files": []interface{}{"fixtures/a.txt", "fixtures/b.txt"}, "syncTimeout": 1, "wait": -1,
			}}),
			nodes: map[string][]fakeNode{"css=#upload": {{}}},
			want:  pageResult(),
			calls: settled(
				"Evaluate css=#upload first multiple timeout=1000",
				"SetInputFiles css=#upload first [fixtures/a.txt] timeout=1000",
				"SetInputFiles css=#upload first [fixtures/b.txt] timeout=1000",
			),
		},
		{
			name: "uploadFile adds each file to a multiple input",
			step: action(map[string]interface{}{"uploadFile": map[string]interface{}{
				"find": findCSS("#upload"), "files": []interface{}{"fixtures/a.txt", "fixtures/b.txt"},
			}}),
			nodes: map[string][]fakeNode{"css=#upload": {{multiple: true}}},
			want:  pageResult(),
			calls: settled(
				"Evaluate css=#upload first multiple",
				"SetInputFiles css=#upload first [fixtures/a.txt]",
				"SetInputFiles css=#upload first [fixtures/a.txt fixtures/b.txt]",
			),
		},
		{
			name: "uploadFile of one file does not ask whether the input takes several",
			step: action(map[string]interface{}{"uploadFile": map[string]interface{}{
				"find": findCSS("#upload"), "files": []interface{}{"fixtures/a.txt"},
			}}),
			want:  pageResult(),
			calls: settled("SetInputFiles css=#upload first [fixtures/a.txt]"),
		},
		{
			name:  "selectFrame enters the frame of the element",
			step:  action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("iframe"), "syncTimeout": 1}}),
			nodes: map[string][]fakeNode{"css=iframe": {{frame: true}}},
			want:  pageResult(),
			calls: append([]string{"ContentFrame css=iframe first timeout=1000"}, tailIn("css=iframe")...),
		},
		{
			// At the top-level frame it stays there, as WebDriver's Switch To Parent Frame does.
			name:  "selectRootFrame at the top-level frame",
			step:  action(map[string]interface{}{"selectRootFrame": true}),
			want:  pageResult(),
			calls: pageTail(),
		},
		{
			name:  "historyAction back",
			step:  action(map[string]interface{}{"historyAction": "back"}),
			want:  pageResult(),
			calls: withTail("GoBack page1"),
		},
		{
			name:  "historyAction is case-insensitive",
			step:  action(map[string]interface{}{"historyAction": "Refresh"}),
			want:  pageResult(),
			calls: withTail("Reload page1"),
		},
		{
			name:  "historyAction forward",
			step:  action(map[string]interface{}{"historyAction": "FORWARD"}),
			want:  pageResult(),
			calls: withTail("GoForward page1"),
		},
		{
			// R8: the command is the body of a function called with the arguments.
			name:  "execute runs the script as a function body with its args",
			step:  action(map[string]interface{}{"execute": map[string]interface{}{"command": "return arguments[0];", "args": []interface{}{"a"}}}),
			want:  pageResult(),
			calls: settled("Evaluate page1 (args) => (function(){\nreturn arguments[0];\n}).apply(null, args) []string{\"a\"}"),
		},
		{
			name:  "execute without args sends an empty list, not null",
			step:  action(map[string]interface{}{"execute": map[string]interface{}{"command": "return 1;"}}),
			want:  pageResult(),
			calls: settled("Evaluate page1 (args) => (function(){\nreturn 1;\n}).apply(null, args) []string{}"),
		},
		{
			// FR-07's compatibility boundary (docs/feat/2048/plan.md): a script that returns nothing, as every existing
			// one, keeps the result it has today.
			name:  "execute of a script that returns nothing adds nothing to the result",
			step:  action(map[string]interface{}{"execute": map[string]interface{}{"command": "document.title = 'set';"}}),
			want:  pageResult(),
			calls: settled("Evaluate page1 (args) => (function(){\ndocument.title = 'set';\n}).apply(null, args) []string{}"),
		},
		{
			name:  "a negative wait returns at once",
			step:  action(map[string]interface{}{"wait": -1}),
			want:  pageResult(),
			calls: pageTail(),
		},
		{
			name: "only the first action set runs, in the order of performAction",
			step: action(map[string]interface{}{
				"navigate": map[string]interface{}{"url": "https://example.test/next"},
				"click":    map[string]interface{}{"find": findCSS("#go")},
			}),
			want:  pageResult(),
			calls: settled("Click css=#go first"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			for name, nodes := range tt.nodes {
				pw.nodes[name] = nodes
			}
			for call, err := range tt.errs {
				pw.errs[call] = err
			}
			got, err := webRun(ctx, t, tt.step)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.calls, pw.seen())
			assert.NoFileExists(t, "page.dump.html")
		})
	}
}

func TestRun_frames(t *testing.T) {
	// R10: finds, actions, execute and the page dump resolve in the current frame; selectRootFrame goes back to the
	// parent frame; a top-level navigation leaves any frame.
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.nodes["css=#outer"] = []fakeNode{{frame: true}}
	pw.nodes["css=#outer >> css=#inner"] = []fakeNode{{frame: true}}
	pw.nodes["css=#outer >> css=#inner >> css=#where"] = []fakeNode{{text: "inside"}}
	selectFrame := func(selector string) venom.TestStep {
		return action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS(selector)}})
	}
	find := action(map[string]interface{}{"find": findCSS("#where")})
	const outer, inner = "css=#outer", "css=#outer >> css=#inner"
	in := func(frame string, calls ...string) []string { return append(calls, tailIn(frame)...) }
	steps := []struct {
		step  venom.TestStep
		calls []string
	}{
		{selectFrame("#outer"), in(outer, "ContentFrame css=#outer first")},
		{selectFrame("#inner"), in(inner, "ContentFrame css=#outer >> css=#inner first")},
		{find, in(inner,
			"Count css=#outer >> css=#inner >> css=#where",
			"InnerText css=#outer >> css=#inner >> css=#where first",
			"Evaluate css=#outer >> css=#inner >> css=#where first value",
			"InnerText css=#outer >> css=#inner >> css=#where first",
			"Evaluate css=#outer >> css=#inner >> css=#where first value",
		)},
		{action(map[string]interface{}{"execute": map[string]interface{}{"command": "return 1;"}}), in(inner, append(
			[]string{"Evaluate css=#outer >> css=#inner (args) => (function(){\nreturn 1;\n}).apply(null, args) []string{}"},
			settleIn(inner)...)...,
		)},
		{action(map[string]interface{}{"selectRootFrame": true}), in(outer)},
		{find, in(outer, "Count css=#outer >> css=#where")},
		{action(map[string]interface{}{"historyAction": "back"}), withTail("GoBack page1")},
		{find, withTail("Count css=#where")},
	}
	for i, s := range steps {
		pw.reset()
		_, err := webRun(ctx, t, s.step)
		require.NoError(t, err, "step %d", i)
		assert.Equal(t, s.calls, pw.seen(), "step %d", i)
	}

	t.Run("navigate leaves the frame", func(t *testing.T) {
		_, err := webRun(ctx, t, selectFrame("#outer"))
		require.NoError(t, err)
		_, err = webRun(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}}))
		require.NoError(t, err)
		pw.reset()
		_, err = webRun(ctx, t, find)
		require.NoError(t, err)
		assert.Equal(t, withTail("Count css=#where"), pw.seen())
	})

	t.Run("the page dump is the current frame's source", func(t *testing.T) {
		_, err := webRun(ctx, t, selectFrame("#outer"))
		require.NoError(t, err)
		pw.reset()
		_, err = webRun(ctx, t, action(map[string]interface{}{"historyAction": "sideways"}))
		require.Error(t, err)
		assert.Equal(t, []string{"Content css=#outer"}, pw.seen())
	})
}

func TestRun_nextWindow(t *testing.T) {
	// D111 fixed (R12, FR-11): nextWindow switches to the next page of the context; it used to always fail.
	nextWindow := action(map[string]interface{}{"nextWindow": true})

	t.Run("switches to the next page, and the next step acts on it", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.addPage() // a window.open
		got, err := webRun(ctx, t, nextWindow)
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		page2Tail := append(settleIn("page2"), "Title page2", "URL page2")
		assert.Equal(t, append([]string{"Pages", "BringToFront page2"}, page2Tail...), pw.seen())
		pw.reset()
		_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".none")}))
		require.NoError(t, err)
		assert.Equal(t, append([]string{"Count css=.none"}, page2Tail...), pw.seen())
	})

	t.Run("the last page goes back to the first", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.addPage()
		_, err := webRun(ctx, t, nextWindow)
		require.NoError(t, err)
		pw.reset()
		_, err = webRun(ctx, t, nextWindow)
		require.NoError(t, err)
		assert.Equal(t, withTail("Pages", "BringToFront page1"), pw.seen())
	})

	t.Run("a single page stays current", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		_, err := webRun(ctx, t, nextWindow)
		require.NoError(t, err)
		assert.Equal(t, withTail("Pages", "BringToFront page1"), pw.seen())
	})

	t.Run("a closed current page is an error", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.pageList = []*fakePage{{pw: pw, name: "other"}}
		_, err := webRun(ctx, t, nextWindow)
		require.EqualError(t, err, "no such window: the current window is closed")
	})

	t.Run("a refused switch is an error", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.addPage()
		pw.errs["BringToFront page2"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, nextWindow)
		require.EqualError(t, err, "synthetic failure")
		assert.Equal(t, []string{"Pages", "BringToFront page2", "Content page1"}, pw.seen(), "the step stays on the first page")
	})
}

func TestRun_waitIsInSeconds(t *testing.T) {
	pw := withFake(t)
	ctx := openFake(t, pw)
	got, err := Executor{}.Run(ctx, action(map[string]interface{}{"wait": 1}))
	require.NoError(t, err)
	res, isPtr := got.(*Result)
	require.True(t, isPtr)
	assert.GreaterOrEqual(t, res.TimeSeconds, 1.0)
}

func TestRun_actionWaitSleeps(t *testing.T) {
	// The wait of click, select and uploadFile sleeps the requested seconds after the action; the table in
	// TestRun_actions only pins a negative wait (no sleep), which a removed sleep would still pass.
	tests := map[string]map[string]interface{}{
		"click":      {"click": map[string]interface{}{"find": findCSS("#go"), "wait": 1}},
		"select":     {"select": map[string]interface{}{"find": findCSS("#s"), "text": "Two", "wait": 1}},
		"uploadFile": {"uploadFile": map[string]interface{}{"find": findCSS("#u"), "files": []interface{}{"a.txt"}, "wait": 1}},
	}
	for name, a := range tests {
		t.Run(name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			start := time.Now()
			got, err := webRun(ctx, t, action(a))
			require.NoError(t, err)
			assert.Equal(t, pageResult(), got)
			assert.GreaterOrEqual(t, time.Since(start), time.Second)
		})
	}
}

func TestRun_syncTimeout(t *testing.T) {
	// syncTimeout is in seconds and bounds the action's wait for its element (FR-04, R15); zero or a negative value is
	// the 30 s default, the context's (no timeout on the call).
	tests := []struct {
		name string
		step venom.TestStep
		call string
	}{
		{"click", action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#late"), "syncTimeout": 3}}), "Click css=#late first timeout=3000"},
		{"click, zero", action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#late"), "syncTimeout": 0}}), "Click css=#late first"},
		{"click, negative", action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#late"), "syncTimeout": -2}}), "Click css=#late first"},
		{"select", action(map[string]interface{}{"select": map[string]interface{}{"find": findCSS("#late"), "text": "x", "syncTimeout": 2}}), "SelectOption css=#late first label=x timeout=2000"},
		{"selectFrame", action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("#late"), "syncTimeout": 2}}), "ContentFrame css=#late first timeout=2000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes["css=#late"] = []fakeNode{{frame: true}}
			_, err := webRun(ctx, t, tt.step)
			require.NoError(t, err)
			assert.Equal(t, tt.call, pw.seen()[0])
		})
	}
}

const (
	// playwrightNavigating is the error of Playwright's page read while the frame replaces its document
	// (docs/feat/2063/research.md R2).
	playwrightNavigating = "playwright: Unable to retrieve content because the page is navigating and changing the content."
	// navigatingSkip is FR-09's line for a dump skipped while the page navigates.
	navigatingSkip = "web - the page is navigating: page.dump.html is not written"
	dumpWarning    = "Error while generating the HTML file: "
)

func TestRun_screenshot(t *testing.T) {
	t.Run("writes the image, and the page dump too", func(t *testing.T) {
		// A successful step with a screenshot also writes page.dump.html (the error dump of D112).
		pw := withFake(t)
		ctx := openFake(t, pw)
		shot := filepath.Join(t.TempDir(), "shot.jpg")
		got, err := webRun(ctx, t, venom.TestStep{"screenshot": shot})
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Equal(t, slices.Concat(settleIn("page1"), []string{"Screenshot page1", "Content page1", "Title page1", "URL page1"}), pw.seen(),
			"the page is shot once it has settled")
		content, err := os.ReadFile(shot)
		require.NoError(t, err)
		assert.Equal(t, "synthetic-png", string(content), "a PNG whatever the extension, as WebDriver's")
		dump, err := os.ReadFile("page.dump.html")
		require.NoError(t, err)
		assert.Equal(t, pageSource, string(dump))
	})

	t.Run("a failed page dump fails the step", func(t *testing.T) {
		// Unlike the action error path, where a failed dump is only a warning.
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.errs["Content page1"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, venom.TestStep{"screenshot": filepath.Join(t.TempDir(), "shot.png")})
		require.EqualError(t, err, "synthetic failure")
		assert.NoFileExists(t, "page.dump.html")
	})

	t.Run("a dump refused while the page navigates fails the step too", func(t *testing.T) {
		// D120 holds for Playwright's navigating error: only the action error path turns it into the skip (#2063).
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.errs["Content page1"] = errors.New(playwrightNavigating)
		_, err := webRun(ctx, t, venom.TestStep{"screenshot": filepath.Join(t.TempDir(), "shot.png")})
		require.EqualError(t, err, playwrightNavigating)
		assert.NoFileExists(t, "page.dump.html")
	})

	t.Run("a failed screenshot fails the step without a dump", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.errs["Screenshot page1"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, venom.TestStep{"screenshot": filepath.Join(t.TempDir(), "shot.png")})
		require.EqualError(t, err, "synthetic failure")
		assert.Equal(t, append(settleIn("page1"), "Screenshot page1"), pw.seen())
		assert.NoFileExists(t, "page.dump.html")
	})
}

func TestRun_pageReadErrors(t *testing.T) {
	// The title read can fail; the URL cannot any more (Playwright keeps it, WebDriver asked the browser).
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.errs["Title page1"] = errors.New("synthetic failure")
	_, err := webRun(ctx, t, venom.TestStep{})
	require.EqualError(t, err, "synthetic failure")
	assert.NoFileExists(t, "page.dump.html", "a failed page read after the action writes no dump")
}

func TestRun_actionErrors(t *testing.T) {
	failure := errors.New("synthetic failure")
	tests := []struct {
		name  string
		step  venom.TestStep
		nodes map[string][]fakeNode
		// failing is the call that fails with failure.
		failing string
		wantErr string
		// noDump is set when the error comes before the page is touched: no call, no page.dump.html.
		noDump bool
	}{
		{
			name:    "the step does not decode",
			step:    venom.TestStep{"action": "click"},
			wantErr: "1 error(s) decoding:\n\n* 'Action' expected a map, got 'string'",
			noDump:  true,
		},
		{
			name:    "click: element not found or not actionable in time",
			step:    action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#go")}}),
			failing: "Click css=#go first",
		},
		{
			name:    "click: the load it started fails",
			step:    action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#go")}}),
			failing: "WaitForLoadState page1",
		},
		{
			name:    "click: invalid find",
			step:    action(map[string]interface{}{"click": map[string]interface{}{"find": 42}}),
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			name:    "fill: element not found",
			step:    action(map[string]interface{}{"fill": []interface{}{map[string]interface{}{"find": findCSS("#user"), "text": "x"}}}),
			failing: "Evaluate css=#user first caretToEnd",
		},
		{
			name:    "fill: invalid find",
			step:    action(map[string]interface{}{"fill": []interface{}{map[string]interface{}{"find": 42, "text": "x"}}}),
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			name:    "fill: text refused",
			step:    action(map[string]interface{}{"fill": []interface{}{map[string]interface{}{"find": findCSS("#user"), "text": "x"}}}),
			failing: "PressSequentially css=#user first x",
		},
		{
			name: "fill: key refused after the text",
			step: action(map[string]interface{}{"fill": []interface{}{
				map[string]interface{}{"find": findCSS("#user"), "text": "x", "key": "TAB"},
			}}),
			failing: "Press css=#user first Tab",
		},
		{
			name:    "find: bad locator",
			step:    action(map[string]interface{}{"find": map[string]interface{}{"selector": ".item", "locator": "ID"}}),
			wantErr: "invalid find element locator 'ID' (must be CSS or XPATH)",
		},
		{
			name:    "find: lookup failed",
			step:    action(map[string]interface{}{"find": findCSS(".item")}),
			failing: "Count css=.item",
		},
		{
			name:    "navigate refused",
			step:    action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}}),
			failing: "Goto page1 https://example.test/next",
		},
		{
			name:    "navigate: reset failed",
			step:    action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next", "reset": true}}),
			failing: "ClearCookies",
			wantErr: "unable to reset the browser: synthetic failure",
		},
		{
			name:    "navigate: reset's blank page failed",
			step:    action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next", "reset": true}}),
			failing: "Goto page1 about:blank",
			wantErr: "unable to reset the browser: synthetic failure",
		},
		{
			name:    "confirmPopup without a dialog",
			step:    action(map[string]interface{}{"confirmPopup": true}),
			wantErr: "no such alert: no dialog is open",
		},
		{
			name:    "cancelPopup without a dialog",
			step:    action(map[string]interface{}{"cancelPopup": true}),
			wantErr: "no such alert: no dialog is open",
		},
		{
			name:    "select: list or option not found",
			step:    action(map[string]interface{}{"select": map[string]interface{}{"find": findCSS("#colour"), "text": "Blue"}}),
			failing: "SelectOption css=#colour first label=Blue",
		},
		{
			name:    "select: the load it started fails",
			step:    action(map[string]interface{}{"select": map[string]interface{}{"find": findCSS("#colour"), "text": "Blue"}}),
			failing: "WaitForLoadState page1",
		},
		{
			name:    "uploadFile: the load it started fails",
			step:    action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": findCSS("#upload"), "files": []interface{}{"fixtures/a.txt"}}}),
			failing: "WaitForLoadState page1",
		},
		{
			name:    "select: invalid find",
			step:    action(map[string]interface{}{"select": map[string]interface{}{"find": 42, "text": "Blue"}}),
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			name:    "uploadFile: element not found or file refused",
			step:    action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": findCSS("#upload"), "files": []interface{}{"fixtures/a.txt"}}}),
			failing: "SetInputFiles css=#upload first [fixtures/a.txt]",
		},
		{
			name:    "uploadFile: invalid find",
			step:    action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": 42, "files": []interface{}{"fixtures/a.txt"}}}),
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			name:    "uploadFile: the multiple check failed",
			step:    action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": findCSS("#upload"), "files": []interface{}{"a", "b"}}}),
			failing: "Evaluate css=#upload first multiple",
		},
		{
			name:    "selectFrame: element not found",
			step:    action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("iframe")}}),
			failing: "ContentFrame css=iframe first",
		},
		{
			name:    "selectFrame: invalid find",
			step:    action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": 42}}),
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			name:    "selectFrame: the element is not a frame",
			step:    action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("div")}}),
			wantErr: "no such frame: the element found by map[locator:CSS selector:div] is not a frame",
		},
		{
			name:    "historyAction: unknown value",
			step:    action(map[string]interface{}{"historyAction": "sideways"}),
			wantErr: "history action 'sideways' is invalid",
		},
		{
			name:    "historyAction refused",
			step:    action(map[string]interface{}{"historyAction": "forward"}),
			failing: "GoForward page1",
		},
		{
			name:    "execute: script error",
			step:    action(map[string]interface{}{"execute": map[string]interface{}{"command": "throw 1;"}}),
			failing: "Evaluate page1 (args) => (function(){\nthrow 1;\n}).apply(null, args) []string{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			if tt.failing != "" {
				pw.errs[tt.failing] = failure
			}
			_, err := webRun(ctx, t, tt.step)
			wantErr := tt.wantErr
			if wantErr == "" {
				wantErr = failure.Error()
			}
			require.EqualError(t, err, wantErr)
			if tt.noDump {
				assert.Empty(t, pw.seen())
				assert.NoFileExists(t, "page.dump.html")
				return
			}
			// D112: a failing action writes the source of the current frame to page.dump.html in the working directory.
			dump, err := os.ReadFile("page.dump.html")
			require.NoError(t, err)
			assert.Equal(t, pageSource, string(dump))
			calls := pw.seen()
			require.NotEmpty(t, calls)
			assert.Equal(t, "Content page1", calls[len(calls)-1], "the dump is the last call: no title or url read")
		})
	}
}

func TestRun_actionErrorWithoutDump(t *testing.T) {
	// A failed dump on the action error path is only a warning: the action error is returned, no file is written.
	for _, tt := range []struct {
		name, dumpErr, logs, notLogs string
	}{
		{"any dump error is logged", "synthetic dump failure", dumpWarning + "synthetic dump failure", navigatingSkip},
		{"a dump refused while the page navigates is the skip (#2063)", playwrightNavigating, navigatingSkip, dumpWarning},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			failGoClick(pw)
			pw.errs["Content page1"] = errors.New(tt.dumpErr)
			var err error
			logged := stderrOf(t, func() { _, err = webRun(ctx, t, goClick()) })
			require.EqualError(t, err, errStepFailure.Error())
			assert.Contains(t, logged, tt.logs)
			assert.NotContains(t, logged, tt.notLogs)
			assert.NoFileExists(t, "page.dump.html")
		})
	}
}

func TestRun_dialogs(t *testing.T) {
	// R6: a dialog opened by step N stays open until a later confirmPopup or cancelPopup. Playwright does not return
	// from the action while the dialog is open; the executor returns anyway and keeps the action in flight.
	click := action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}})
	confirm := action(map[string]interface{}{"confirmPopup": true})
	cancel := action(map[string]interface{}{"cancelPopup": true})

	// open runs a click whose handler opens count dialogs, one after the other answered.
	open := func(t *testing.T, count int) (*fakePW, context.Context, *chanFlag) {
		t.Helper()
		pw := withFake(t)
		ctx := openFake(t, pw)
		clicked := newFlag()
		pw.hooks["Click css=#c first"] = pw.blockOnDialog("confirm", count, clicked)
		got, err := webRun(ctx, t, click)
		require.NoError(t, err)
		assert.Equal(t, Result{}, got, "the title and the url are empty while the dialog is open")
		assert.Equal(t, []string{"Click css=#c first"}, pw.seen(), "no load to wait for, no page read")
		assert.False(t, clicked.isSet(), "the click is still in flight")
		pw.reset()
		return pw, ctx, clicked
	}

	t.Run("confirmPopup accepts the dialog and lets the click finish", func(t *testing.T) {
		pw, ctx, clicked := open(t, 1)
		got, err := webRun(ctx, t, confirm)
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.True(t, clicked.isSet())
		assert.Equal(t, settled("Accept confirm1"), pw.seen())
	})

	t.Run("cancelPopup dismisses it", func(t *testing.T) {
		pw, ctx, clicked := open(t, 1)
		_, err := webRun(ctx, t, cancel)
		require.NoError(t, err)
		assert.True(t, clicked.isSet())
		assert.Equal(t, settled("Dismiss confirm1"), pw.seen())
	})

	t.Run("a failed answer fails the step", func(t *testing.T) {
		pw, ctx, _ := open(t, 1)
		pw.errs["Accept confirm1"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, confirm)
		require.EqualError(t, err, "synthetic failure")
	})

	t.Run("a dialog the answer lets open is pending in turn", func(t *testing.T) {
		pw, ctx, clicked := open(t, 2)
		got, err := webRun(ctx, t, confirm)
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		assert.Equal(t, []string{"Accept confirm1"}, pw.seen())
		assert.False(t, clicked.isSet())
		pw.reset()
		_, err = webRun(ctx, t, confirm)
		require.NoError(t, err)
		assert.True(t, clicked.isSet())
		assert.Equal(t, settled("Accept confirm2"), pw.seen())
	})

	t.Run("any other action dismisses the dialog and fails, as WebDriver's default prompt handling", func(t *testing.T) {
		pw, ctx, clicked := open(t, 1)
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.True(t, clicked.isSet())
		assert.Equal(t, []string{"Dismiss confirm1", "Content page1"}, pw.seen(), "then the page dump works")
		pw.reset()
		_, err = webRun(ctx, t, confirm)
		require.EqualError(t, err, "no such alert: no dialog is open")
	})

	t.Run("wait does not touch the page", func(t *testing.T) {
		pw, ctx, _ := open(t, 1)
		got, err := webRun(ctx, t, action(map[string]interface{}{"wait": -1}))
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		assert.Empty(t, pw.seen())
	})

	t.Run("selectRootFrame and nextWindow meet the dialog too", func(t *testing.T) {
		for _, step := range []venom.TestStep{
			action(map[string]interface{}{"selectRootFrame": true}),
			action(map[string]interface{}{"nextWindow": true}),
		} {
			pw, ctx, _ := open(t, 1)
			_, err := webRun(ctx, t, step)
			require.ErrorIs(t, err, errUnexpectedAlert)
			assert.Equal(t, []string{"Dismiss confirm1", "Content page1"}, pw.seen())
		}
	})

	t.Run("a screenshot meets the dialog", func(t *testing.T) {
		pw, ctx, _ := open(t, 1)
		_, err := webRun(ctx, t, venom.TestStep{"screenshot": filepath.Join(t.TempDir(), "shot.png")})
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.Equal(t, []string{"Dismiss confirm1"}, pw.seen())
	})

	t.Run("a failed dismiss is added to the error", func(t *testing.T) {
		pw, ctx, _ := open(t, 1)
		pw.errs["Dismiss confirm1"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, action(map[string]interface{}{"historyAction": "back"}))
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.ErrorContains(t, err, "synthetic failure")
	})

	t.Run("the next fill entry meets a dialog the previous one opened", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Press css=#user first Enter"] = pw.blockOnDialog("alert", 1, newFlag())
		_, err := webRun(ctx, t, action(map[string]interface{}{"fill": []interface{}{
			map[string]interface{}{"find": findCSS("#user"), "text": "x", "key": "ENTER"},
			map[string]interface{}{"find": findCSS("#note"), "text": "y"},
		}}))
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.Equal(t, []string{
			"Evaluate css=#user first caretToEnd", "PressSequentially css=#user first x", "Press css=#user first Enter",
			"Dismiss alert1", "Content page1",
		}, pw.seen())
	})

	t.Run("a dialog opened as the action finished is pending", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Click css=#c first"] = func() { pw.openDialog("alert") }
		got, err := webRun(ctx, t, click)
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		pw.reset()
		_, err = webRun(ctx, t, confirm)
		require.NoError(t, err)
		assert.Equal(t, settled("Accept alert1"), pw.seen())
	})

	t.Run("a dialog open before the step leaves the title and url empty", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.openDialog("alert")
		got, err := webRun(ctx, t, venom.TestStep{})
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		assert.Empty(t, pw.seen())
	})

	t.Run("a dialog that opens during the title read leaves it empty", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Title page1"] = pw.blockOnDialog("alert", 1, newFlag())
		got, err := webRun(ctx, t, venom.TestStep{})
		require.NoError(t, err)
		assert.Equal(t, Result{}, got)
		assert.Equal(t, append(settleIn("page1"), "Title page1"), pw.seen())
	})

	t.Run("a dialog that interrupts a read fails the step", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.hooks["Count css=.item"] = pw.blockOnDialog("alert", 1, newFlag())
		_, err := webRun(ctx, t, action(map[string]interface{}{"find": findCSS(".item")}))
		require.ErrorIs(t, err, errDialogOpened)
		assert.Equal(t, []string{"Count css=.item", "Dismiss alert1"}, pw.seen(), "the dump meets the dialog and dismisses it")
		assert.NoFileExists(t, "page.dump.html")
	})

	t.Run("reset accepts a pending dialog", func(t *testing.T) {
		pw, ctx, clicked := open(t, 1)
		_, err := webRun(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next", "reset": true}}))
		require.NoError(t, err)
		assert.True(t, clicked.isSet())
		assert.Equal(t, withTail("Accept confirm1", "ClearCookies", "Goto page1 about:blank", "Goto page1 https://example.test/next"), pw.seen())
	})

	t.Run("reset goes on when accepting fails", func(t *testing.T) {
		pw, ctx, _ := open(t, 1)
		pw.errs["Accept confirm1"] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next", "reset": true}}))
		require.NoError(t, err)
		assert.Equal(t, withTail("Accept confirm1", "ClearCookies", "Goto page1 about:blank", "Goto page1 https://example.test/next"), pw.seen())
	})

	t.Run("beyond the bound a new dialog is dismissed at once", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		logged := stderrOf(t, func() {
			for range maxPendingDialogs + 1 {
				pw.openDialog("alert")
			}
		})
		assert.Equal(t, []string{"Dismiss alert17"}, pw.seen())
		assert.Contains(t, logged, "16 dialogs already wait for an answer: dismissing a new one")
		for i := range maxPendingDialogs {
			pw.reset()
			_, err := webRun(ctx, t, confirm)
			require.NoError(t, err, "dialog %d", i+1)
		}
	})

	t.Run("a failed dismiss beyond the bound is logged", func(t *testing.T) {
		pw := withFake(t)
		openFake(t, pw)
		pw.errs["Dismiss alert17"] = errors.New("synthetic failure")
		logged := stderrOf(t, func() {
			for range maxPendingDialogs + 1 {
				pw.openDialog("alert")
			}
		})
		assert.Contains(t, logged, "dismissing the dialog failed: synthetic failure")
	})
}

func TestRun_mocks(t *testing.T) {
	// FR-06 (docs/feat/2048/research.md R1-R5, validation.md V2-V4): a step's mocks register on the context before its
	// action and answer every request that matches them, from the fake's intercept, until the test case ends.
	const api = "https://example.test/api/users"
	navigate := action(map[string]interface{}{"navigate": map[string]interface{}{"url": "https://example.test/next"}})
	click := action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#go")}})
	unset := " status=default contentType=none headers=none body=none"

	t.Run("a mock registers before the action and answers with its response", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		got, err := webRun(ctx, t, mocked(navigate, map[string]interface{}{
			"url": api, "status": 201, "headers": map[string]interface{}{"X-Total": "1", "Cache-Control": "no-store"},
			"contentType": "application/json", "body": `[{"name":"Ada"}]`,
		}))
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Equal(t, withTail("Route "+api, "Goto page1 https://example.test/next"), pw.seen())
		assert.Equal(t, "Fulfill "+api+` status=201 contentType=application/json headers=map[Cache-Control:no-store X-Total:1] body="[{\"name\":\"Ada\"}]"`,
			pw.answer(t, api))
	})

	t.Run("an unset or empty field is left to Playwright", func(t *testing.T) {
		// No status is Playwright's 200; an empty body is not sent, the empty body Playwright sends anyway.
		tests := []struct {
			name string
			mock map[string]interface{}
			want string
		}{
			{"a url alone", map[string]interface{}{"url": api}, "Fulfill " + api + unset},
			{
				"empty body, content type and headers",
				map[string]interface{}{"url": api, "body": "", "contentType": "", "headers": map[string]interface{}{}},
				"Fulfill " + api + unset,
			},
			{"a body without a status", map[string]interface{}{"url": api, "body": "ok"}, "Fulfill " + api + ` status=default contentType=none headers=none body="ok"`},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				_, err := webRun(ctx, t, mocked(venom.TestStep{}, tt.mock))
				require.NoError(t, err)
				assert.Equal(t, tt.want, pw.answer(t, api))
			})
		}
	})

	t.Run("several mocks register in order; a step with mocks alone reads the page", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		got, err := webRun(ctx, t, mocked(venom.TestStep{}, map[string]interface{}{"url": api}, map[string]interface{}{"url": "**/*.png", "status": 404}))
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Equal(t, append([]string{"Route " + api, "Route **/*.png"}, pageTail()...), pw.seen())
		assert.Equal(t, "Fulfill **/*.png status=404 contentType=none headers=none body=none", pw.answer(t, "**/*.png"))
	})

	t.Run("a mock answers until the test case ends; a later one on the same pattern takes over", func(t *testing.T) {
		// R1: registered on the context, it outlives its step; venom's TearDown and Setup around the next test case
		// open a context with no mock.
		pw := withFake(t)
		ctx := openFake(t, pw)
		for i, step := range []venom.TestStep{mocked(venom.TestStep{}, map[string]interface{}{"url": api, "body": "first"}), navigate, click} {
			_, err := webRun(ctx, t, step)
			require.NoError(t, err, "step %d", i+1)
		}
		assert.Equal(t, "Fulfill "+api+` status=default contentType=none headers=none body="first"`, pw.answer(t, api), "step 1's mock, after step 3")
		_, err := webRun(ctx, t, mocked(venom.TestStep{}, map[string]interface{}{"url": api, "body": "second"}))
		require.NoError(t, err)
		assert.Equal(t, "Fulfill "+api+` status=default contentType=none headers=none body="second"`, pw.answer(t, api))
		assert.Nil(t, pw.intercept("https://example.test/other"), "a request no mock matches reaches the network")

		require.NoError(t, Executor{}.TearDown(ctx))
		pw.reset()
		_, err = setupFake(t, nil)
		require.NoError(t, err)
		assert.False(t, slices.ContainsFunc(pw.seen(), func(c string) bool { return strings.HasPrefix(c, "Route ") }), "Setup registers no mock")
		assert.Nil(t, pw.intercept(api), "the next test case starts with no mock")
	})

	t.Run("an invalid mock fails its step before any mock registers and before the action", func(t *testing.T) {
		// R5: the message names the entry, counted from 1.
		tests := []struct {
			name    string
			mocks   []map[string]interface{}
			wantErr string
		}{
			{"no url", []map[string]interface{}{{"status": 200}}, "mock 1 of 1: url is required"},
			{
				"a status below 100, on the second entry",
				[]map[string]interface{}{{"url": api}, {"url": "https://example.test/b", "status": 42}},
				"mock 2 of 2 (https://example.test/b): status 42 is not an HTTP status (100-599)",
			},
			{"a status above 599", []map[string]interface{}{{"url": api, "status": 600}}, "mock 1 of 1 (" + api + "): status 600 is not an HTTP status (100-599)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				_, err := webRun(ctx, t, mocked(click, tt.mocks...))
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, []string{"Content page1"}, pw.seen(), "no mock registered, no click: the page dump alone")
				assert.FileExists(t, "page.dump.html", "the failure path of any action")
			})
		}
	})

	t.Run("a refused registration fails the step and names the mock; the action does not run", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.errs["Route "+api] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, mocked(navigate, map[string]interface{}{"url": api}))
		require.EqualError(t, err, "mock 1 of 1 ("+api+"): unable to register it: synthetic failure")
		assert.Equal(t, []string{"Route " + api, "Content page1"}, pw.seen())
		assert.Nil(t, pw.intercept(api))
	})

	t.Run("a mock that cannot answer logs a warning and does not panic", func(t *testing.T) {
		// R4: the request comes after the step returned; a panic would fail Playwright's whole connection.
		pw := withFake(t)
		ctx := openFake(t, pw)
		_, err := webRun(ctx, t, mocked(venom.TestStep{}, map[string]interface{}{"url": api}))
		require.NoError(t, err)
		pw.errs["Fulfill "+api+unset] = errors.New("synthetic failure")
		var answered *fakeRoute
		logged := stderrOf(t, func() { assert.NotPanics(t, func() { answered = pw.intercept(api) }) })
		require.NotNil(t, answered)
		assert.Equal(t, "Fulfill "+api+unset, answered.fulfilled)
		assert.Contains(t, logged, "web - the mock of "+api+" could not answer: synthetic failure")
	})

	t.Run("a pending dialog is dismissed and fails the step, and no mock registers", func(t *testing.T) {
		// R3: registering while a dialog is open hangs on Chromium; it is refused as any page command is.
		pw := withFake(t)
		ctx := openFake(t, pw)
		clicked := newFlag()
		pw.hooks["Click css=#c first"] = pw.blockOnDialog("confirm", 1, clicked)
		_, err := webRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#c")}}))
		require.NoError(t, err)
		pw.reset()
		_, err = webRun(ctx, t, mocked(venom.TestStep{}, map[string]interface{}{"url": api}))
		require.ErrorIs(t, err, errUnexpectedAlert)
		assert.EqualError(t, err, "mock 1 of 1 ("+api+"): unable to register it: "+errUnexpectedAlert.Error())
		assert.True(t, clicked.isSet(), "the click the dialog held has finished")
		assert.Equal(t, []string{"Dismiss confirm1", "Content page1"}, pw.seen())
		assert.Nil(t, pw.intercept(api))
	})
}

func TestCheckMocks(t *testing.T) {
	// R5: status 0 is unset (Playwright's 200); any other value is an HTTP status, 100 to 599.
	for _, status := range []int{0, 100, 200, 599} {
		assert.NoError(t, checkMocks([]Mock{{URL: "**/api", Status: status}}), "status %d", status)
	}
	for _, status := range []int{-1, 99, 600} {
		assert.Error(t, checkMocks([]Mock{{URL: "**/api", Status: status}}), "status %d", status)
	}
	assert.NoError(t, checkMocks(nil), "a step without mock")
	assert.EqualError(t, checkMocks([]Mock{{URL: "**/a"}, {Status: 404}, {URL: "**/c", Status: 1}}), "mock 2 of 3: url is required",
		"the first invalid entry is reported")

	// Braces are the glob's groups: playwright-go compiles unbalanced ones with regexp.MustCompile, which panics.
	for _, url := range []string{"**/api/{a,b}", "**/{a,{b}}", "**/{}", `**/api/\{literal`, `**/api/\}`, `**/a\`} {
		assert.NoError(t, checkMocks([]Mock{{URL: url}}), url)
	}
	for _, url := range []string{"**/api/{users", "**/api/users}", "http://app.test/api/{a,b", "**/}{", `**/{a\}`} {
		assert.Error(t, checkMocks([]Mock{{URL: url}}), url)
	}
	assert.EqualError(t, checkMocks([]Mock{{URL: "**/a"}, {URL: "**/api/{users"}}),
		"mock 2 of 2 (**/api/{users): url has an unbalanced { or }")
}

func TestExecutor_decodesMock(t *testing.T) {
	// Run decodes the step with mapstructure, whose keys match the fields case-insensitively: contentType is ContentType.
	var e Executor
	require.NoError(t, mapstructure.Decode(venom.TestStep{"mock": []interface{}{map[string]interface{}{
		"url": "**/api/users", "status": 201, "headers": map[string]interface{}{"X-Total": "1"},
		"contentType": "application/json", "body": `{"name":"Ada"}`,
	}}}, &e))
	assert.Equal(t, []Mock{{
		URL: "**/api/users", Status: 201, Headers: map[string]string{"X-Total": "1"}, ContentType: "application/json", Body: `{"name":"Ada"}`,
	}}, e.Mock)
}

func TestRun_withoutSetupPanics(t *testing.T) {
	// The engine only runs a step after a successful Setup; without one the nil web context is dereferenced.
	venom.InitTestLogger(t)
	assert.Panics(t, func() { _, _ = Executor{}.Run(context.Background(), venom.TestStep{}) }) //nolint:errcheck // it panics before returning
}

func TestReadFindElement(t *testing.T) {
	venom.InitTestLogger(t)
	tests := []struct {
		name         string
		find         interface{}
		wantSelector string
		wantLocator  string
		wantErr      string
	}{
		{name: "string (deprecated) is CSS", find: "#a", wantSelector: "#a", wantLocator: "css selector"},
		{name: "CSS", find: findCSS("#a"), wantSelector: "#a", wantLocator: "css selector"},
		{name: "XPATH", find: map[string]interface{}{"selector": "//a", "locator": "XPATH"}, wantSelector: "//a", wantLocator: "xpath"},
		{
			name:    "the locator is case-sensitive",
			find:    map[string]interface{}{"selector": "#a", "locator": "css"},
			wantErr: "invalid find element locator 'css' (must be CSS or XPATH)",
		},
		{
			// The map form has no default locator, unlike the string form.
			name:    "a missing locator is an error",
			find:    map[string]interface{}{"selector": "#a"},
			wantErr: "invalid find element locator '%!s(<nil>)' (must be CSS or XPATH)",
		},
		{
			name:    "a non-string selector",
			find:    map[string]interface{}{"selector": 42, "locator": "CSS"},
			wantErr: "invalid find element selector 42 (must be a string)",
		},
		{
			name:    "any other type",
			find:    42,
			wantErr: "invalid find element structure 42\n (must be a string or a element with selector and locator attribute)",
		},
		{
			// A YAML mapping decoded as map[interface{}]interface{} is not accepted either.
			name:    "a map with non-string keys",
			find:    map[interface{}]interface{}{"selector": "#a", "locator": "CSS"},
			wantErr: "invalid find element structure map[locator:CSS selector:#a]\n (must be a string or a element with selector and locator attribute)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, locator, err := readFindElement(context.Background(), tt.find)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, selector)
				assert.Empty(t, locator)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSelector, selector)
			assert.Equal(t, tt.wantLocator, locator)
		})
	}
}

// TestReadFindElement_deprecatedStringWarns pins the Warning venom logs for the deprecated bare-string find
// syntax ("string (deprecated) is CSS" above pins its return value only).
func TestReadFindElement_deprecatedStringWarns(t *testing.T) {
	var selector, locator string
	var findErr error
	out := stderrOf(t, func() { selector, locator, findErr = readFindElement(context.Background(), "#go") })
	require.NoError(t, findErr)
	assert.Equal(t, "#go", selector)
	assert.Equal(t, "css selector", locator)
	assert.Contains(t, out, "web - findElement : this find element syntax deprecated")
}

func TestKeys(t *testing.T) {
	assert.Len(t, Keys, 64)
	assert.Equal(t, "", Keys["ENTER"])
	assert.Equal(t, "", Keys["RETURN"], "RETURN and ENTER are different WebDriver keys")
	for alias, key := range map[string]string{"LEFT_SHIFT": "SHIFT", "LEFT_CONTROL": "CONTROL", "LEFT_ALT": "ALT", "ARROW_LEFT": "LEFT", "COMMAND": "META"} {
		assert.Equal(t, Keys[key], Keys[alias], alias)
	}
	_, found := Keys["enter"]
	assert.False(t, found, "key names are upper case only")
}

func TestPlaywrightKeys(t *testing.T) {
	// Every WebDriver key name has a Playwright key, but the four Playwright has no key for; web_playwright_test.go
	// presses each of them on a real browser.
	unmapped := map[string]bool{"NULL": true, "CANCEL": true, "HELP": true, "ZENKAKU_HANKAKU": true}
	for name := range Keys {
		_, mapped := playwrightKeys[name]
		assert.Equal(t, !unmapped[name], mapped, name)
	}
	assert.Len(t, playwrightKeys, len(Keys)-len(unmapped), "no key name that is not in Keys")

	t.Run("a key Playwright has no key for warns and is not sent", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		var err error
		logged := stderrOf(t, func() {
			_, err = webRun(ctx, t, action(map[string]interface{}{"fill": []interface{}{
				map[string]interface{}{"find": findCSS("#user"), "text": "x", "key": "HELP"},
			}}))
		})
		require.NoError(t, err)
		assert.Contains(t, logged, "web - fill: the key HELP has no Playwright equivalent, it is not sent")
		assert.Equal(t, settled("Evaluate css=#user first caretToEnd", "PressSequentially css=#user first x"), pw.seen())
	})

	t.Run("an unknown key name is silent, as before", func(t *testing.T) {
		venom.InitTestLogger(t)
		var ok bool
		logged := stderrOf(t, func() { _, ok = playwrightKey(context.Background(), "NO_SUCH_KEY") })
		assert.False(t, ok)
		assert.Empty(t, logged)
	})
}
