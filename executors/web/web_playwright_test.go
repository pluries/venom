//go:build playwright

package web

import (
	"archive/zip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Real-browser tests: what a fake of the seam cannot prove (validation.md V4, V5, V7-V9; research.md R6-R10, R14;
// for mocks, execute's value and the trace, docs/feat/2048/validation.md V1, V5-V8).
// They need Playwright's driver and browsers with their OS packages:
//
//	go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium
//	go test -tags playwright ./executors/web/... -race -v
//
// Pages are data: URLs, an httptest server or mocked URLs on venom.invalid; no network. Firefox and WebKit cases skip
// when not installed.

// dataURL is a data: URL serving html; PathEscape keeps a '#' or a '?' in the page from ending the URL.
func dataURL(html string) string {
	return "data:text/html," + url.PathEscape(html)
}

// openBrowser runs Setup on the real Playwright with vars (headless unless vars say otherwise) and TearDown at the
// end of the test; the test runs in a temporary directory (page dumps).
func openBrowser(t *testing.T, vars map[string]any) context.Context {
	t.Helper()
	venom.InitTestLogger(t)
	t.Chdir(t.TempDir())
	all := map[string]any{"headless": true}
	for k, v := range vars {
		all[k] = v
	}
	ctx, err := Executor{}.Setup(webVarsCtx(all), nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, Executor{}.TearDown(ctx)) })
	return ctx
}

// steps runs each step and fails the test on the first error; it returns the last result.
func steps(ctx context.Context, t *testing.T, all ...venom.TestStep) Result {
	t.Helper()
	var last Result
	for i, s := range all {
		got, err := webRun(ctx, t, s)
		require.NoError(t, err, "step %d", i)
		last = got
	}
	return last
}

func navigateTo(u string) venom.TestStep {
	return action(map[string]interface{}{"navigate": map[string]interface{}{"url": u}})
}

func clickOn(selector string) venom.TestStep {
	return action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS(selector)}})
}

func findBy(selector string) venom.TestStep {
	return action(map[string]interface{}{"find": findCSS(selector)})
}

func executeJS(command string, args ...interface{}) venom.TestStep {
	return action(map[string]interface{}{"execute": map[string]interface{}{"command": command, "args": args}})
}

func TestPlaywright_browsers(t *testing.T) {
	// V4 (FR-02): each driver value connects to the matching engine.
	page := dataURL(`<title>ua</title><p id="ua"></p><script>document.getElementById('ua').textContent = navigator.userAgent</script>`)
	for _, tt := range []struct{ driver, userAgent string }{
		{"chrome", "Chrome/"},
		{"gecko", "Firefox/"},
		{"webkit", "AppleWebKit/"},
	} {
		t.Run(tt.driver, func(t *testing.T) {
			venom.InitTestLogger(t)
			ctx, err := Executor{}.Setup(webVarsCtx(map[string]any{"driver": tt.driver, "headless": true}), nil)
			if err != nil && tt.driver != "chrome" && strings.Contains(err.Error(), "install it with") {
				t.Skipf("%s is not installed: %v", tt.driver, err)
			}
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, Executor{}.TearDown(ctx)) })
			t.Chdir(t.TempDir())
			got := steps(ctx, t, navigateTo(page), findBy("#ua"))
			assert.Equal(t, 1, got.Find)
			assert.Contains(t, got.Text, tt.userAgent)
			assert.Equal(t, "ua", got.Title)
		})
	}
}

func TestPlaywright_missingBrowser(t *testing.T) {
	// R14: a browser that is not installed fails Setup with the install command, and nothing is downloaded.
	venom.InitTestLogger(t)
	missing := filepath.Join(t.TempDir(), "no-browsers")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", missing)
	_, err := Executor{}.Setup(webVarsCtx(map[string]any{"headless": true}), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to launch chromium (install it with: "+installCommand(chromium)+")")
	assert.NoDirExists(t, missing)
}

func TestPlaywright_actionability(t *testing.T) {
	// V5 (FR-04, R15): a click on a covered button waits, without syncTimeout, until the cover lifts.
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>covered</title>
<button id="b" onclick="document.title='clicked'">b</button>
<div id="cover" style="position:fixed;inset:0;background:white"></div>
<script>setTimeout(() => document.getElementById('cover').remove(), 500)</script>`)
	steps(ctx, t, navigateTo(page))
	start := time.Now()
	got := steps(ctx, t, clickOn("#b"))
	assert.Equal(t, "clicked", got.Title)
	assert.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond, "the click waited for the cover")

	t.Run("syncTimeout bounds the wait", func(t *testing.T) {
		steps(ctx, t, navigateTo(dataURL(`<title>never</title><button id="b" disabled>b</button>`)))
		start := time.Now()
		_, err := webRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#b"), "syncTimeout": 1}}))
		require.Error(t, err)
		assert.Less(t, time.Since(start), 10*time.Second)
		assert.FileExists(t, "page.dump.html")
	})

	t.Run("find does not wait", func(t *testing.T) {
		// V6 on a real browser: find counts at once.
		start := time.Now()
		got := steps(ctx, t, findBy("#none"))
		assert.Equal(t, 0, got.Find)
		assert.Less(t, time.Since(start), 5*time.Second)
	})
}

func TestPlaywright_shadowDOM(t *testing.T) {
	// V7 (FR-05): a CSS selector reaches into an open shadow root; an XPath does not.
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>shadow</title><div id="host"></div><script>
const root = document.getElementById('host').attachShadow({mode: 'open'});
root.innerHTML = '<p id="inner">inside</p><button id="sb" onclick="document.title=\'pressed\'">b</button>';
</script>`)
	got := steps(ctx, t, navigateTo(page), findBy("#inner"))
	assert.Equal(t, 1, got.Find)
	assert.Equal(t, "inside", got.Text)
	got = steps(ctx, t, clickOn("#sb"))
	assert.Equal(t, "pressed", got.Title)
	got = steps(ctx, t, action(map[string]interface{}{"find": map[string]interface{}{"selector": "//p[@id='inner']", "locator": "XPATH"}}))
	assert.Equal(t, 0, got.Find)
}

func TestPlaywright_dialogs(t *testing.T) {
	// V8 (R6): step 1 returns with the dialog open, title and url empty; step 2 answers it and the page reacts.
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>popups</title><button id="c" onclick="document.title = confirm('sure') ? 'accepted' : 'dismissed'">c</button>`)
	steps(ctx, t, navigateTo(page))

	got := steps(ctx, t, clickOn("#c"))
	assert.Empty(t, got.Title)
	assert.Empty(t, got.URL)
	got = steps(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
	assert.Equal(t, "accepted", got.Title)

	steps(ctx, t, clickOn("#c"))
	got = steps(ctx, t, action(map[string]interface{}{"cancelPopup": true}))
	assert.Equal(t, "dismissed", got.Title)

	t.Run("another action dismisses the dialog and fails", func(t *testing.T) {
		steps(ctx, t, executeJS("document.title = 'unanswered'"), clickOn("#c"))
		_, err := webRun(ctx, t, findBy("#c"))
		require.ErrorIs(t, err, errUnexpectedAlert)
		got := steps(ctx, t, findBy("#c"))
		assert.Equal(t, "dismissed", got.Title)
	})

	t.Run("an alert opened by a script, then a navigation with reset", func(t *testing.T) {
		got := steps(ctx, t, executeJS("alert('hello')"))
		assert.Empty(t, got.Title)
		got = steps(ctx, t, action(map[string]interface{}{"navigate": map[string]interface{}{"url": dataURL("<title>after</title>"), "reset": true}}))
		assert.Equal(t, "after", got.Title)
	})

	t.Run("confirmPopup without a dialog", func(t *testing.T) {
		_, err := webRun(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
		require.ErrorIs(t, err, errNoAlert)
	})
}

func TestPlaywright_dialogLeftOpenAtTearDown(t *testing.T) {
	// TearDown dismisses a leftover dialog and closes everything: it returns, and without an error.
	venom.InitTestLogger(t)
	t.Chdir(t.TempDir())
	ctx, err := Executor{}.Setup(webVarsCtx(map[string]any{"headless": true}), nil)
	require.NoError(t, err)
	steps(ctx, t, navigateTo(dataURL(`<button id="c" onclick="confirm('left open')">c</button>`)), clickOn("#c"))
	done := make(chan error, 1)
	go func() { done <- Executor{}.TearDown(ctx) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("TearDown hangs with a dialog left open")
	}
}

func TestPlaywright_verifyTLS(t *testing.T) {
	// #2049: the test server's certificate is self-signed, so the browser trusts it only while certificate errors are
	// ignored, which is the default.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<title>secure</title>`)) //nolint:errcheck // test server
	}))
	t.Cleanup(srv.Close)

	t.Run("default ignores the invalid certificate", func(t *testing.T) {
		ctx := openBrowser(t, nil)
		got := steps(ctx, t, navigateTo(srv.URL))
		assert.Equal(t, "secure", got.Title)
	})
	t.Run("verifyTls rejects it", func(t *testing.T) {
		ctx := openBrowser(t, map[string]any{"verifyTls": true})
		_, err := webRun(ctx, t, navigateTo(srv.URL))
		require.ErrorContains(t, err, "net::ERR_CERT_AUTHORITY_INVALID")
	})
}

func TestPlaywright_selectAndNextWindow(t *testing.T) {
	// V9 (FR-13, FR-11; R12): select selects the option by its text; nextWindow acts on the new tab. Chromium does not
	// let window.open load a data: URL, so the pages come from a test server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		page := `<title>main</title><select id="s"><option value="one">One</option><option value="two"> Two </option></select>
<button id="o" onclick="window.open('/second')">o</button>`
		if r.URL.Path == "/second" {
			page = `<title>second</title><button id="b" onclick="document.title='second clicked'">b</button>`
		}
		_, _ = w.Write([]byte(page)) //nolint:errcheck // test server
	}))
	t.Cleanup(srv.Close)
	ctx := openBrowser(t, nil)
	steps(ctx, t, navigateTo(srv.URL), action(map[string]interface{}{"select": map[string]interface{}{"find": findCSS("#s"), "text": "Two"}}))
	got := steps(ctx, t, findBy("#s"))
	assert.Equal(t, "two", got.Value)

	got = steps(ctx, t, clickOn("#o"))
	assert.Equal(t, "main", got.Title, "the new tab opens beside the current one")
	got = steps(ctx, t, action(map[string]interface{}{"nextWindow": true}))
	assert.Equal(t, "second", got.Title)
	assert.Equal(t, srv.URL+"/second", got.URL)
	got = steps(ctx, t, clickOn("#b"))
	assert.Equal(t, "second clicked", got.Title)
	got = steps(ctx, t, action(map[string]interface{}{"nextWindow": true}))
	assert.Equal(t, "main", got.Title, "after the last tab, the first")

	t.Run("an option that does not exist fails", func(t *testing.T) {
		_, err := webRun(ctx, t, action(map[string]interface{}{"select": map[string]interface{}{"find": findCSS("#s"), "text": "Three", "syncTimeout": 1}}))
		require.Error(t, err)
	})
}

func TestPlaywright_readText(t *testing.T) {
	// R7: text is the rendered text (hidden text excluded), value the DOM property ("" where there is none).
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>read</title><p id="p">  shown <span style="display:none">hidden</span> text  </p>
<input id="i" value="typed"><li id="li" value="3">item</li>`)
	got := steps(ctx, t, navigateTo(page), findBy("#p"))
	assert.Equal(t, "shown text", got.Text)
	assert.Empty(t, got.Value)
	got = steps(ctx, t, findBy("#i"))
	assert.Equal(t, "typed", got.Value)
	got = steps(ctx, t, findBy("#li"))
	assert.Equal(t, "item", got.Text)
	assert.Empty(t, got.Value, "a number value reads as empty, as under WebDriver")
}

func TestPlaywright_execute(t *testing.T) {
	// R8: the command is a function body: return and arguments work; a DOM node return value is no error.
	ctx := openBrowser(t, nil)
	steps(ctx, t, navigateTo(dataURL("<title>execute</title>")))
	got := steps(ctx, t, executeJS("document.title = 'no args'"))
	assert.Equal(t, "no args", got.Title)
	got = steps(ctx, t, executeJS("document.title = arguments[0] + '-' + arguments[1] // a trailing comment", "alpha", "beta"))
	assert.Equal(t, "alpha-beta", got.Title)
	steps(ctx, t, executeJS("return document.body"), executeJS("return window"))
	_, err := webRun(ctx, t, executeJS("throw new Error('synthetic')"))
	require.ErrorContains(t, err, "synthetic")
}

func TestPlaywright_fill(t *testing.T) {
	// R9: fill types after the content, as Send Keys did (Playwright's own focus would put the caret first), and a
	// key such as ENTER is pressed.
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>fill</title><form onsubmit="document.title = 'submitted ' + document.getElementById('q').value; return false">
<input id="q" value="abc"><textarea id="t">abc</textarea><div id="e" contenteditable="true">abc</div></form>`)
	fillStep := func(selector, text string, key ...string) venom.TestStep {
		f := map[string]interface{}{"find": findCSS(selector), "text": text}
		if len(key) > 0 {
			f["key"] = key[0]
		}
		return action(map[string]interface{}{"fill": []interface{}{f}})
	}
	steps(ctx, t, navigateTo(page), fillStep("#t", "XY"), fillStep("#e", "XY"))
	got := steps(ctx, t, findBy("#t"))
	assert.Equal(t, "abcXY", got.Value)
	got = steps(ctx, t, findBy("#e"))
	assert.Equal(t, "abcXY", got.Text)
	got = steps(ctx, t, fillStep("#q", "XY", "ENTER"))
	assert.Equal(t, "submitted abcXY", got.Title)
	got = steps(ctx, t, fillStep("#q", "Z"))
	assert.Equal(t, "submitted abcXY", got.Title)
	got = steps(ctx, t, findBy("#q"))
	assert.Equal(t, "abcXYZ", got.Value, "a field that has the focus keeps its caret")

	t.Run("every mapped key is a key Playwright knows", func(t *testing.T) {
		steps(ctx, t, navigateTo(dataURL(`<input id="k">`)))
		for name := range playwrightKeys {
			_, err := webRun(ctx, t, fillStep("#k", "", name))
			require.NoError(t, err, name)
		}
	})
}

func TestPlaywright_navigationSettles(t *testing.T) {
	// A click that navigates returns once the new page has loaded, as a WebDriver click did: Playwright's click
	// returns when the navigation starts, and the title of a page still streaming reads "".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path != "/slow" {
			_, _ = w.Write([]byte(`<title>start</title><a id="l" href="/slow">go</a>`)) //nolint:errcheck // test server
			return
		}
		_, _ = w.Write([]byte("<html><head>")) //nolint:errcheck // test server
		assert.NoError(t, http.NewResponseController(w).Flush())
		time.Sleep(400 * time.Millisecond)
		_, _ = w.Write([]byte("<title>slow</title></head><body>slow</body></html>")) //nolint:errcheck // test server
	}))
	t.Cleanup(srv.Close)
	ctx := openBrowser(t, nil)
	got := steps(ctx, t, navigateTo(srv.URL), clickOn("#l"))
	assert.Equal(t, "slow", got.Title)
	assert.Equal(t, srv.URL+"/slow", got.URL)
	got = steps(ctx, t, action(map[string]interface{}{"historyAction": "back"}))
	assert.Equal(t, "start", got.Title)
}

func TestPlaywright_frames(t *testing.T) {
	// R10: selectFrame enters an iframe, selectRootFrame goes back to the parent.
	ctx := openBrowser(t, nil)
	page := dataURL(`<title>frames</title><p id="where">root</p><iframe id="f" srcdoc="<p id='where'>inside</p>"></iframe>`)
	steps(ctx, t, navigateTo(page), action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("#f")}}))
	got := steps(ctx, t, findBy("#where"))
	assert.Equal(t, "inside", got.Text)
	assert.Equal(t, "frames", got.Title, "the title stays the page's")
	got = steps(ctx, t, action(map[string]interface{}{"selectRootFrame": true}),
		action(map[string]interface{}{"find": map[string]interface{}{"selector": "//p[@id='where']", "locator": "XPATH"}}))
	assert.Equal(t, "root", got.Text)
	_, err := webRun(ctx, t, action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("#where")}}))
	require.ErrorContains(t, err, "is not a frame")
}

func TestPlaywright_uploadAndScreenshot(t *testing.T) {
	ctx := openBrowser(t, map[string]any{"width": 640, "height": 480})
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("synthetic a"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("synthetic b"), 0o600))
	page := dataURL(`<title>upload</title><input type="file" id="u"><input type="file" id="m" multiple>
<p id="size"></p><script>document.getElementById('size').textContent = innerWidth + 'x' + innerHeight</script>`)
	got := steps(ctx, t, navigateTo(page), findBy("#size"))
	assert.Equal(t, "640x480", got.Text, "width and height size the viewport")

	steps(ctx, t, action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": findCSS("#u"), "files": []interface{}{a, b}}}))
	got = steps(ctx, t, findBy("#u"))
	assert.Contains(t, got.Value, "fakepath")
	assert.True(t, strings.HasSuffix(got.Value, "b.txt"), "a single-file input keeps the last file: %s", got.Value)

	steps(ctx, t,
		action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": findCSS("#m"), "files": []interface{}{a, b}}}),
		executeJS("document.title = document.getElementById('m').files.length + ' files'"))
	got = steps(ctx, t, findBy("#m"))
	assert.Equal(t, "2 files", got.Title, "a multiple input keeps every file")

	shot := filepath.Join(dir, "shot.jpg")
	_, err := webRun(ctx, t, venom.TestStep{"screenshot": shot})
	require.NoError(t, err)
	png, err := os.ReadFile(shot)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(png), "\x89PNG"), "a PNG whatever the extension")
	assert.FileExists(t, "page.dump.html")
}

// slowServer serves /start, /timer (a button whose handler navigates from a zero-delay timer), /hang, which never
// answers while its client is there, and /slow, which sends
// nothing for 800 ms: a navigation to it stays requested, not committed, meanwhile.
func slowServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/slow":
			time.Sleep(800 * time.Millisecond)
			_, _ = w.Write([]byte("<title>slow</title>arrived")) //nolint:errcheck // test server
		case "/hang":
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Minute):
			}
		case "/timer":
			_, _ = w.Write([]byte(`<title>timer</title><button id="b" onclick="setTimeout(() => location.href = '/slow', 0)">go</button>`)) //nolint:errcheck // test server
		default:
			_, _ = w.Write([]byte("<title>start</title>start")) //nolint:errcheck // test server
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPlaywright_pendingNavigations(t *testing.T) {
	// R16: chromedriver waited for a navigation a command started however it started; so must the step. Measured on
	// develop with this server: both cases read "slow".
	srv := slowServer(t)

	t.Run("a script's navigation is awaited, and the next navigate is not interrupted by it", func(t *testing.T) {
		ctx := openBrowser(t, nil)
		got := steps(ctx, t, navigateTo(srv.URL+"/start"), executeJS("location.href = '/slow'"))
		assert.Equal(t, "slow", got.Title)
		assert.Equal(t, srv.URL+"/slow", got.URL)
		got = steps(ctx, t, navigateTo(srv.URL+"/start"))
		assert.Equal(t, "start", got.Title)
	})

	t.Run("a navigation a click's zero-delay timer starts is awaited", func(t *testing.T) {
		ctx := openBrowser(t, nil)
		got := steps(ctx, t, navigateTo(srv.URL+"/timer"), clickOn("#b"))
		assert.Equal(t, "slow", got.Title)
		assert.Equal(t, srv.URL+"/slow", got.URL)
	})

	t.Run("a navigation that never commits fails its step within web.timeout, and a navigate ends it", func(t *testing.T) {
		// Playwright holds an evaluation sent while the frame navigates: a timer flush sent then would wait for the
		// read limit (60 s), not web.timeout.
		ctx := openBrowser(t, map[string]any{"timeout": 2})
		steps(ctx, t, navigateTo(srv.URL+"/start"))
		start := time.Now()
		_, err := webRun(ctx, t, executeJS("location.href = '/hang'"))
		require.EqualError(t, err, "the page did not commit its navigation within web.timeout (2s)")
		assert.Less(t, time.Since(start), 15*time.Second)
		// R19: the next action waits for the navigation before its first call, which Playwright would hold 60 s.
		start = time.Now()
		_, err = webRun(ctx, t, action(map[string]interface{}{"find": findCSS("body")}))
		require.EqualError(t, err, "the page did not commit its navigation within web.timeout (2s)")
		assert.Less(t, time.Since(start), 15*time.Second)
		got := steps(ctx, t, navigateTo(srv.URL+"/start"))
		assert.Equal(t, "start", got.Title)
	})
}

func TestPlaywright_scriptLimit(t *testing.T) {
	// R17: a script that never settles fails its step within the script limit, and TearDown still closes everything.
	prev := scriptTimeout
	scriptTimeout = time.Second
	t.Cleanup(func() { scriptTimeout = prev })
	ctx := openBrowser(t, nil)
	steps(ctx, t, navigateTo(dataURL("<title>t</title>t")))

	start := time.Now()
	_, err := webRun(ctx, t, executeJS("return new Promise(() => {});"))
	require.ErrorIs(t, err, errNoAnswer)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestPlaywright_sandboxHint(t *testing.T) {
	// R20: on a host that restricts unprivileged user namespaces (native Ubuntu 23.10+), Chromium prints this line and
	// exits. This host cannot reproduce it, so a stand-in browser prints the same line through a real Playwright launch.
	fake := filepath.Join(t.TempDir(), "chrome")
	script := "#!/bin/sh\necho '[FATAL:zygote_host_impl_linux.cc(128)] No usable sandbox!' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o700)) //nolint:gosec // the stand-in browser must be executable
	venom.InitTestLogger(t)
	t.Chdir(t.TempDir())
	_, err := Executor{}.Setup(webVarsCtx(map[string]any{"headless": true, "binaryPath": fake}), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to launch chromium: its sandbox cannot start on this host")
	assert.NotContains(t, err.Error(), "install it with")
}

// appPage is a document for a host that never resolves (venom.invalid, RFC 6761): its script fetches the API and
// writes the first user's name into #user.
const appPage = `<title>mocked</title><div id="user"></div><script>
fetch('/api/users').then(r => r.json()).then(users => { document.getElementById('user').textContent = users[0].name })
</script>`

// jsRenderedUser returns #user's text once appPage's script has written it: the mocked API may answer after the load
// the navigate step waits for.
const jsRenderedUser = `const el = document.getElementById('user');
if (el.textContent) return el.textContent;
return new Promise(resolve => new MutationObserver(() => resolve(el.textContent)).observe(el, {childList: true}));`

// usersMock answers the API with body, status 201, JSON and an X-Total header.
func usersMock(body string) map[string]interface{} {
	return map[string]interface{}{
		"url": "**/api/users", "status": 201, "contentType": "application/json",
		"headers": map[string]interface{}{"X-Total": "1"}, "body": body,
	}
}

func TestPlaywright_mock(t *testing.T) {
	// docs/feat/2048/validation.md V1 (FR-06, R2): a mocked document and API stand in for the whole application, with
	// no server and no network; R1: the mocks answer until the test case ends, the newest first.
	const app = "http://venom.invalid/app"
	ctx := openBrowser(t, nil)
	steps(ctx, t, mocked(venom.TestStep{},
		map[string]interface{}{"url": "**/app", "contentType": "text/html", "body": appPage},
		usersMock(`[{"name":"Ada"}]`)))

	got := steps(ctx, t, navigateTo(app))
	assert.Equal(t, "mocked", got.Title)
	assert.Equal(t, app, got.URL)
	got = steps(ctx, t, executeJS(jsRenderedUser))
	assert.Equal(t, "Ada", got.Value)
	got = steps(ctx, t, findBy("#user"))
	assert.Equal(t, 1, got.Find)
	assert.Equal(t, "Ada", got.Text)

	got = steps(ctx, t, executeJS("return fetch('/api/users').then(r => ({status: r.status, total: r.headers.get('x-total')}))"))
	assert.Equal(t, `{"status":201,"total":"1"}`, got.Value)
	assert.Equal(t, map[string]any{"status": 201.0, "total": "1"}, got.ValueJSON, "status, content type and headers are the mock's")

	t.Run("the mocks keep answering in the later steps of the test case", func(t *testing.T) {
		got := steps(ctx, t, executeJS("return fetch('/api/users').then(r => r.text())"))
		assert.Equal(t, `[{"name":"Ada"}]`, got.Value)
		got = steps(ctx, t, navigateTo(app), executeJS(jsRenderedUser))
		assert.Equal(t, "Ada", got.Value)
	})

	t.Run("a later mock whose pattern also matches takes over", func(t *testing.T) {
		got := steps(ctx, t, mocked(navigateTo(app), usersMock(`[{"name":"Grace"}]`)), executeJS(jsRenderedUser))
		assert.Equal(t, "Grace", got.Value)
		assert.Equal(t, "mocked", got.Title, "the document's mock still answers")
	})

	t.Run("a url whose braces do not balance fails its step, and the test case goes on", func(t *testing.T) {
		// playwright-go compiles the glob with regexp.MustCompile: registered, it would panic the whole run.
		_, err := webRun(ctx, t, mocked(venom.TestStep{}, map[string]interface{}{"url": "**/api/{users", "body": "[]"}))
		require.EqualError(t, err, "mock 1 of 1 (**/api/{users): url has an unbalanced { or }")
		got := steps(ctx, t, navigateTo(app))
		assert.Equal(t, "mocked", got.Title)
	})

	t.Run("the next test case starts with no mock", func(t *testing.T) {
		ctx := openBrowser(t, nil)
		_, err := webRun(ctx, t, navigateTo(app))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "net::ERR_NAME_NOT_RESOLVED at "+app, "without its mock, venom.invalid does not resolve")
	})
}

func TestPlaywright_failedLoad(t *testing.T) {
	// #2063 (docs/feat/2063/research.md R1-R3): the page read after a load that failed before any response meets
	// Chromium replacing the document with its error page, and Playwright refuses it: FR-09's skip, no dump.
	const unreachable = "http://venom.invalid/unreachable"
	ctx := openBrowser(t, nil)
	var err error
	logged := stderrOf(t, func() { _, err = webRun(ctx, t, navigateTo(unreachable)) })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "net::ERR_NAME_NOT_RESOLVED at "+unreachable)
	assert.Contains(t, logged, navigatingSkip)
	assert.NotContains(t, logged, dumpWarning)
	assert.NoFileExists(t, "page.dump.html")
}

func TestPlaywright_executeValue(t *testing.T) {
	// docs/feat/2048/validation.md V5, V6 (FR-07; docs/feat/2048/research.md R6, R7): result.value is what the script
	// returned as text, result.valuejson as a typed value; nothing and null leave both empty, as before FR-07.
	ctx := openBrowser(t, nil)
	steps(ctx, t, navigateTo(dataURL("<title>value</title>")))
	for _, tt := range []struct {
		name, command, value string
		typed                any
	}{
		{"nothing", "document.title = 'value'", "", nil},
		{"null", "return null", "", nil},
		{"a number", "return 42", "42", 42.0},
		{"a boolean", "return true", "true", true},
		{"a string", "return 's'", "s", "s"},
		{"a list", "return [1, 'a']", `[1,"a"]`, []any{1.0, "a"}},
		{"an object", "return {n: 42, ok: true, user: {name: 'Ada'}}", `{"n":42,"ok":true,"user":{"name":"Ada"}}`,
			map[string]any{"n": 42.0, "ok": true, "user": map[string]any{"name": "Ada"}}},
		{"NaN", "return NaN", "NaN", "NaN"},
		{"a date", "return new Date(0)", "1970-01-01T00:00:00Z", "1970-01-01T00:00:00Z"},
		{"a URL", "return new URL('https://a.test/x?y=1')", "https://a.test/x?y=1", "https://a.test/x?y=1"},
		{"a DOM node", "return window", "ref: <Window>", "ref: <Window>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := steps(ctx, t, executeJS(tt.command))
			assert.Equal(t, tt.value, got.Value)
			assert.Equal(t, tt.typed, got.ValueJSON)
			assert.Equal(t, "value", got.Title)
		})
	}

	t.Run("a field of a returned object is what a suite assertion reads", func(t *testing.T) {
		got := steps(ctx, t, executeJS("return {n: 42, list: [1, 2], user: {name: 'Ada'}}"))
		// venom flattens the step result as it does for any step (process_teststep.go): a list's elements are named
		// after the list, from 0.
		vars := venom.GetExecutorResult(&got)
		assert.Equal(t, "Ada", vars["result.valuejson.user.name"])
		assert.Equal(t, 42.0, vars["result.valuejson.n"])
		assert.Equal(t, 1.0, vars["result.valuejson.list.list0"])
		assert.Equal(t, 2.0, vars["result.valuejson.list.list1"])
		assert.Equal(t, 2, vars["result.valuejson.list.__Len__"])
		assert.Equal(t, `{"list":[1,2],"n":42,"user":{"name":"Ada"}}`, vars["result.value"])
		assert.Equal(t, 8, vars["result.__Len__"], "every field of Result, valuejson included (plan.md § Deviations)")
	})

	t.Run("a value that contains itself leaves both empty, with a warning, and the step passes", func(t *testing.T) {
		var got Result
		var err error
		logged := stderrOf(t, func() { got, err = webRun(ctx, t, executeJS("const a = {}; a.self = a; return a;")) })
		require.NoError(t, err)
		assert.Empty(t, got.Value)
		assert.Nil(t, got.ValueJSON)
		assert.Contains(t, logged, "the script's return value contains itself: result.value and result.valuejson are left empty")
	})

	t.Run("a URL Go cannot parse leaves both empty, with a warning, and the step passes", func(t *testing.T) {
		// The page accepts %of in a fragment, Go's url.Parse does not, and playwright-go drops its error.
		var got Result
		var err error
		logged := stderrOf(t, func() { got, err = webRun(ctx, t, executeJS("return {link: new URL('https://a.test/x#50%off')}")) })
		require.NoError(t, err)
		assert.Empty(t, got.Value)
		assert.Nil(t, got.ValueJSON)
		assert.Contains(t, logged, "could not decode: a nil *url.URL: result.value and result.valuejson are left empty")
	})
}

// zipEntry is the content of the entry name in the zip file at path; the test fails when there is none.
func zipEntry(t *testing.T, path, name string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer func() { assert.NoError(t, r.Close()) }()
	f, err := r.Open(name)
	require.NoError(t, err, "%s holds no %s", path, name)
	defer func() { assert.NoError(t, f.Close()) }()
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(b)
}

func TestPlaywright_trace(t *testing.T) {
	// docs/feat/2048/validation.md V7, V8 (FR-08; docs/feat/2048/research.md R8, R9): with web.trace, the failing step
	// leaves its chunk of the trace next to the page dump; a passing step leaves nothing.
	page := dataURL(`<title>traced</title><button id="b" onclick="document.title = 'clicked'">b</button>
<button id="c" onclick="document.title = confirm('sure') ? 'accepted' : 'dismissed'">c</button>`)
	missing := action(map[string]interface{}{"click": map[string]interface{}{"find": findCSS("#missing"), "syncTimeout": 1}})

	t.Run("a failing step leaves page.trace.zip, which holds that step's action, beside page.dump.html", func(t *testing.T) {
		ctx := openBrowser(t, map[string]any{"trace": true})
		got := steps(ctx, t, navigateTo(page), clickOn("#b"))
		assert.Equal(t, "clicked", got.Title)
		assert.NoFileExists(t, "page.trace.zip")
		_, err := webRun(ctx, t, missing)
		require.Error(t, err)
		assert.FileExists(t, "page.dump.html")
		info, err := os.Stat("page.trace.zip")
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "the driver's own write leaves 0644 under umask 022")
		trace := zipEntry(t, "page.trace.zip", "trace.trace")
		assert.Contains(t, trace, "#missing")
		assert.NotContains(t, trace, "css=#b", "the passing steps' chunks were discarded")
	})

	t.Run("only passing steps leave no trace", func(t *testing.T) {
		ctx := openBrowser(t, map[string]any{"trace": true})
		steps(ctx, t, navigateTo(page), clickOn("#b"), findBy("#b"))
		assert.NoFileExists(t, "page.trace.zip")
		assert.NotNil(t, getWebCtx(ctx).tracing)
	})

	t.Run("a click that leaves a confirm open passes, and the next step answers it", func(t *testing.T) {
		// R8: the chunk ends while the dialog is open, without dismissing it.
		ctx := openBrowser(t, map[string]any{"trace": true})
		steps(ctx, t, navigateTo(page))
		start := time.Now()
		got := steps(ctx, t, clickOn("#c"))
		assert.Empty(t, got.Title)
		assert.Less(t, time.Since(start), 10*time.Second)
		got = steps(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
		assert.Equal(t, "accepted", got.Title)
		require.NotNil(t, getWebCtx(ctx).tracing, "tracing is still on")
		_, err := webRun(ctx, t, missing)
		require.Error(t, err)
		assert.Contains(t, zipEntry(t, "page.trace.zip", "trace.trace"), "#missing")
	})
}
