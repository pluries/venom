//go:build playwright

package web

import (
	"context"
	"fmt"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Real-browser tests of spec 015 (docs/specs/015-web-recipe-actions): what the fake of the seam cannot prove, the
// matching of Chromium itself. Run with
//
//	go test -tags playwright -run 'TestPlaywright_(locator|click|fill|hostile|whitespace|frames|shadow|testID)' ./executors/web/
//
// Pages are data: URLs. The tests do not depend on timing (runResult, not webRun).

// openOn opens a headless session with vars and navigates to the page.
func openOn(t *testing.T, pageHTML string, vars map[string]any) context.Context {
	t.Helper()
	ctx := openBrowser(t, vars)
	mustRun(ctx, t, navigateTo(dataURL(pageHTML)))
	return ctx
}

func TestPlaywright_locatorKinds(t *testing.T) {
	// US1-AS1 to AS5, AS7: each kind finds what a user sees; name and exact narrow the match, which is otherwise a
	// case-insensitive substring; a find that matches nothing counts 0 at once.
	tests := []struct {
		name string
		html string
		find map[string]interface{}
		want found
	}{
		{"AS1 ROLE with a name", `<button>Add</button>`, loc("ROLE", "button", "name", "Add"), found{Find: 1, Text: "Add"}},
		{"ROLE without a name", `<button>One</button><button>Two</button>`, loc("ROLE", "button"), found{Find: 2, Text: "One"}},
		{"AS2 TEXT", `<p>Total: 3</p>`, loc("TEXT", "Total: 3"), found{Find: 1, Text: "Total: 3"}},
		{"AS2 TEXT ignores case", `<p>Total: 3</p>`, loc("TEXT", "total: 3"), found{Find: 1, Text: "Total: 3"}},
		{"AS5 TESTID", `<input data-testid="item-input" value="t0">`, loc("TESTID", "item-input"), found{Find: 1, Value: "t0"}},
		{"AS3 LABEL", `<label for="e">Email</label><input id="e" value="e0">`, loc("LABEL", "Email"), found{Find: 1, Value: "e0"}},
		{"AS4 PLACEHOLDER", `<input placeholder="Search" value="s0">`, loc("PLACEHOLDER", "Search"), found{Find: 1, Value: "s0"}},
		{"AS7 ROLE substring", `<button>Add</button><button>Add item</button>`, loc("ROLE", "button", "name", "Add"), found{Find: 2, Text: "Add"}},
		{"AS7 ROLE exact", `<button>Add</button><button>Add item</button>`, loc("ROLE", "button", "name", "Add", "exact", true), found{Find: 1, Text: "Add"}},
		{"AS7 ROLE exact on the longer name", `<button>Add</button><button>Add item</button>`, loc("ROLE", "button", "name", "Add item", "exact", true), found{Find: 1, Text: "Add item"}},
		{"AS7 TEXT substring", `<p>Total</p><p>Total: 3</p>`, loc("TEXT", "Total"), found{Find: 2, Text: "Total"}},
		{"AS7 TEXT exact", `<p>Total</p><p>Total: 3</p>`, loc("TEXT", "Total", "exact", true), found{Find: 1, Text: "Total"}},
		{"AS7 LABEL substring", labelPair, loc("LABEL", "Email"), found{Find: 2, Value: "a0"}},
		{"AS7 LABEL exact", labelPair, loc("LABEL", "Email", "exact", true), found{Find: 1, Value: "a0"}},
		{"AS7 PLACEHOLDER substring", placeholderPair, loc("PLACEHOLDER", "Search"), found{Find: 2, Value: "s1"}},
		{"AS7 PLACEHOLDER exact", placeholderPair, loc("PLACEHOLDER", "Search", "exact", true), found{Find: 1, Value: "s1"}},
		{"no match counts 0", `<button>Add</button>`, loc("ROLE", "button", "name", "Nope"), found{}},
	}
	ctx := openBrowser(t, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustRun(ctx, t, navigateTo(dataURL(tt.html)))
			assert.Equal(t, tt.want, foundOf(mustRun(ctx, t, findStep(tt.find))))
		})
	}
}

const (
	labelPair       = `<label for="a">Email</label><input id="a" value="a0"><label for="b">Email address</label><input id="b" value="b0">`
	placeholderPair = `<input placeholder="Search" value="s1"><input placeholder="Search all" value="s2">`
)

func TestPlaywright_clickTargetsTheFirstMatch(t *testing.T) {
	// US1-AS7: with two matches the first is the target; exact picks the other one.
	ctx := openOn(t, `<title>none</title><button onclick="document.title='first'">Add</button><button onclick="document.title='second'">Add item</button>`, nil)
	click := func(find map[string]interface{}) venom.TestStep {
		return action(map[string]interface{}{"click": map[string]interface{}{"find": find}})
	}
	assert.Equal(t, "first", mustRun(ctx, t, click(loc("ROLE", "button", "name", "Add"))).Title)
	assert.Equal(t, "second", mustRun(ctx, t, click(loc("ROLE", "button", "name", "Add item", "exact", true))).Title)
}

func TestPlaywright_fillAndClickByKind(t *testing.T) {
	// US1-AS3, AS4, AS5: fill names its input by label, placeholder or test id; click by placeholder and by role.
	ctx := openOn(t, `<title>form</title>
<label for="email">Email</label><input id="email">
<input id="search" placeholder="Search" onfocus="document.title='search focused'">
<input id="item" data-testid="item-input">
<button onclick="document.title='clicked'">Go</button>`, nil)
	fill := func(find map[string]interface{}, text string) venom.TestStep {
		return action(map[string]interface{}{"fill": []interface{}{map[string]interface{}{"find": find, "text": text}}})
	}
	valueOf := func(selector string) string { return mustRun(ctx, t, findBy(selector)).Value }

	mustRun(ctx, t, fill(loc("LABEL", "Email"), "ada@example.test"))
	assert.Equal(t, "ada@example.test", valueOf("#email"))

	got := mustRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": loc("PLACEHOLDER", "Search")}}))
	assert.Equal(t, "search focused", got.Title)
	mustRun(ctx, t, fill(loc("PLACEHOLDER", "Search"), "query"))
	assert.Equal(t, "query", valueOf("#search"))

	mustRun(ctx, t, fill(loc("TESTID", "item-input"), "milk"))
	assert.Equal(t, "milk", valueOf("#item"))

	got = mustRun(ctx, t, action(map[string]interface{}{"click": map[string]interface{}{"find": loc("ROLE", "button", "name", "Go")}}))
	assert.Equal(t, "clicked", got.Title)
}

// hostileBrowserValues are texts that would change a selector, an engine prefix or a regular expression if they were
// read as one (US1-AS12). The page holds a decoy that a /x/ read as a regular expression would also match.
var hostileBrowserValues = []string{`say "hi"`, `it's`, `back\slash`, `a]b`, `/x/`, `text=x`, `css=a`, `x"] , [data-testid="y`}

func hostilePage(values []string) string {
	var b strings.Builder
	b.WriteString("<title>hostile</title><p>exit</p><button>ex</button>")
	for i, v := range values {
		e := html.EscapeString(v)
		fmt.Fprintf(&b, `<button>%s</button><input aria-label="%s" placeholder="%s" data-testid="%s" value="in%d">`, e, e, e, e, i)
	}
	return b.String()
}

// hostileRow is a find and the element it must match.
type hostileRow struct {
	find map[string]interface{}
	want found
}

// hostileRows are the finds of value v, number i of the page, by kind.
func hostileRows(i int, v string, exact bool) map[string]hostileRow {
	in := fmt.Sprintf("in%d", i)
	rows := map[string]hostileRow{
		"ROLE":        {loc("ROLE", "button", "name", v, "exact", exact), found{Find: 1, Text: v}},
		"TEXT":        {loc("TEXT", v, "exact", exact), found{Find: 1, Text: v}},
		"LABEL":       {loc("LABEL", v, "exact", exact), found{Find: 1, Value: in}},
		"PLACEHOLDER": {loc("PLACEHOLDER", v, "exact", exact), found{Find: 1, Value: in}},
	}
	if !exact {
		rows["TESTID"] = hostileRow{loc("TESTID", v), found{Find: 1, Value: in}}
	}
	return rows
}

func TestPlaywright_hostileValuesAreMatchedAsText(t *testing.T) {
	// US1-AS12: a value with quotes, a backslash, a bracket, /x/ or the look of an engine prefix is matched as that
	// literal text by every new kind, with and without exact; the test id is matched exactly.
	ctx := openOn(t, hostilePage(hostileBrowserValues), nil)
	for i, v := range hostileBrowserValues {
		for _, exact := range []bool{false, true} {
			for kind, row := range hostileRows(i, v, exact) {
				t.Run(fmt.Sprintf("%s %q exact=%t", kind, v, exact), func(t *testing.T) {
					assert.Equal(t, row.want, foundOf(mustRun(ctx, t, findStep(row.find))))
				})
			}
		}
	}
}

func TestPlaywright_whitespace(t *testing.T) {
	// US1-AS12, D-014: TEXT, LABEL and a ROLE name are matched trimmed and with whitespace normalised, even with exact;
	// a TESTID value is matched exactly. PLACEHOLDER is not normalised: Playwright matches the attribute text as it is
	// (observed in Chromium; D-014 expects it normalised, a point for the spec owner).
	page := `<title>spaces</title>
<button>a   b</button>
<label for="l">c   d</label><input id="l" value="lv">
<p>e   f</p>
<input placeholder=" g  h " value="pv"><input placeholder="g h" value="qv">
<span data-testid=" i  j ">T1</span><span data-testid="l1&#10;l2">T2</span>`
	tests := []struct {
		name string
		find map[string]interface{}
		want found
	}{
		{"TEXT trimmed", loc("TEXT", " e f "), found{Find: 1, Text: "e f"}},
		{"TEXT normalised, exact", loc("TEXT", "e    f", "exact", true), found{Find: 1, Text: "e f"}},
		{"ROLE name trimmed", loc("ROLE", "button", "name", "  a b "), found{Find: 1, Text: "a b"}},
		{"ROLE name normalised, exact", loc("ROLE", "button", "name", "a      b", "exact", true), found{Find: 1, Text: "a b"}},
		{"LABEL trimmed", loc("LABEL", " c d "), found{Find: 1, Value: "lv"}},
		{"LABEL normalised, exact", loc("LABEL", "c     d", "exact", true), found{Find: 1, Value: "lv"}},
		{"TESTID exact, spaces kept", loc("TESTID", " i  j "), found{Find: 1, Text: "T1"}},
		{"TESTID is not trimmed", loc("TESTID", "i  j"), found{}},
		{"TESTID is not normalised", loc("TESTID", " i j "), found{}},
		{"TESTID with a newline", loc("TESTID", "l1\nl2"), found{Find: 1, Text: "T2"}},
		{"PLACEHOLDER as it is", loc("PLACEHOLDER", " g  h "), found{Find: 1, Value: "pv"}},
		{"PLACEHOLDER as it is, other attribute", loc("PLACEHOLDER", "g h"), found{Find: 1, Value: "qv"}},
		{"PLACEHOLDER is not trimmed or normalised", loc("PLACEHOLDER", "  g h"), found{}},
	}
	ctx := openOn(t, page, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, foundOf(mustRun(ctx, t, findStep(tt.find))))
		})
	}
}

func TestPlaywright_framesAndNewKinds(t *testing.T) {
	// US1-AS13: after selectFrame a TEXT or ROLE locator reads the frame, and selectRootFrame goes back; selectFrame
	// takes a new kind, and fails with "not a frame" on an element that is none.
	ctx := openOn(t, `<title>frames</title><p>Total: 3 outside</p><button>Go outside</button>
<iframe data-testid="the-frame" srcdoc="<p>Total: 3 inside</p><button>Go</button>"></iframe>`, nil)
	selectFrame := action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": loc("TESTID", "the-frame")}})
	steps := []struct {
		name string
		step venom.TestStep
		want found
	}{
		{"the page's text", findStep(loc("TEXT", "Total: 3")), found{Find: 1, Text: "Total: 3 outside"}},
		{"enter the frame by test id", selectFrame, found{}},
		{"the frame's text", findStep(loc("TEXT", "Total: 3")), found{Find: 1, Text: "Total: 3 inside"}},
		{"the frame's button", findStep(loc("ROLE", "button", "name", "Go")), found{Find: 1, Text: "Go"}},
		{"leave the frame", action(map[string]interface{}{"selectRootFrame": true}), found{}},
		{"the page's button", findStep(loc("ROLE", "button", "name", "Go")), found{Find: 1, Text: "Go outside"}},
	}
	for _, s := range steps {
		assert.Equal(t, s.want, foundOf(mustRun(ctx, t, s.step)), s.name)
	}

	_, err := runResult(ctx, t, action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": loc("TEXT", "Total: 3")}}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such frame")
}

func TestPlaywright_shadowRoots(t *testing.T) {
	// US1-AS14: TEXT and ROLE reach into an open shadow root and not into a closed one.
	ctx := openOn(t, `<title>shadow</title><div id="open"></div><div id="closed"></div><script>
document.getElementById('open').attachShadow({mode: 'open'}).innerHTML = '<button>openbtn</button><p>opentext</p>';
document.getElementById('closed').attachShadow({mode: 'closed'}).innerHTML = '<button>closedbtn</button><p>closedtext</p>';
</script>`, nil)
	tests := []struct {
		name string
		find map[string]interface{}
		want found
	}{
		{"TEXT in the open root", loc("TEXT", "opentext"), found{Find: 1, Text: "opentext"}},
		{"ROLE in the open root", loc("ROLE", "button", "name", "openbtn"), found{Find: 1, Text: "openbtn"}},
		{"TEXT in the closed root", loc("TEXT", "closedtext"), found{}},
		{"ROLE in the closed root", loc("ROLE", "button", "name", "closedbtn"), found{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, foundOf(mustRun(ctx, t, findStep(tt.find))))
		})
	}
}

func TestPlaywright_testIDAttributePerSession(t *testing.T) {
	// US1-AS6, AS15: two sessions of one process, one reading data-test and one data-testid, interleaved on the same
	// page: each finds its own element.
	page := `<title>ids</title><span data-test="item-input">by data-test</span><span data-testid="item-input">by data-testid</span>`
	withAttr := openOn(t, page, map[string]any{"testIdAttribute": "data-test"})
	without := openOn(t, page, nil)
	step := findStep(loc("TESTID", "item-input"))
	for range 2 {
		assert.Equal(t, found{Find: 1, Text: "by data-test"}, foundOf(mustRun(withAttr, t, step)))
		assert.Equal(t, found{Find: 1, Text: "by data-testid"}, foundOf(mustRun(without, t, step)))
	}
}

func TestPlaywright_testIDAttributeNames(t *testing.T) {
	// US1-AS12, D-009: an attribute name with a . or a : is escaped, so the selector stays valid CSS and reads that
	// attribute.
	page := `<title>names</title><div data-a.b="x">dot</div><div data-te:st="x">colon</div>`
	for attr, text := range map[string]string{"data-a.b": "dot", "data-te:st": "colon"} {
		t.Run(attr, func(t *testing.T) {
			ctx := openOn(t, page, map[string]any{"testIdAttribute": attr})
			assert.Equal(t, found{Find: 1, Text: text}, foundOf(mustRun(ctx, t, findStep(loc("TESTID", "x")))))
		})
	}
}

// countOf is how many elements the CSS selector matches now: find counts without waiting.
func countOf(ctx context.Context, t *testing.T, selector string) int {
	t.Helper()
	return mustRun(ctx, t, findBy(selector)).Find
}

func TestPlaywright_waitForAnElementThatAppears(t *testing.T) {
	// US2-AS1: an element inserted after 3 s is not there when the step starts; the default state and timeout wait for
	// it, and it is visible when the step passes.
	ctx := openOn(t, `<title>late</title><script>setTimeout(() => {
const p = document.createElement('p'); p.id = 'late'; p.textContent = 'Ready'; document.body.appendChild(p);
}, 3000)</script>`, nil)
	require.Equal(t, 0, countOf(ctx, t, "#late"))
	mustRun(ctx, t, newAction("waitFor", kv("find", findCSS("#late"))))
	assert.Equal(t, found{Find: 1, Text: "Ready"}, foundOf(mustRun(ctx, t, findBy("#late"))))
}

func TestPlaywright_waitForFailsAfterItsTimeout(t *testing.T) {
	// US2-AS2: the same element with syncTimeout 1 is not there in time: the step fails naming the element, the state and
	// the timeout, and writes the page dump.
	ctx := openOn(t, `<title>late</title><script>setTimeout(() => {
const p = document.createElement('p'); p.id = 'late'; document.body.appendChild(p);
}, 3000)</script>`, nil)
	_, err := runResult(ctx, t, newAction("waitFor", kv("find", findCSS("#late"), "syncTimeout", 1)))
	require.Error(t, err)
	for _, part := range []string{"waitFor: ", "#late", "visible", "1s"} {
		assert.Contains(t, err.Error(), part)
	}
	assert.FileExists(t, "page.dump.html")
}

func TestPlaywright_waitForTheFourStates(t *testing.T) {
	// US2-AS3: hidden passes when the element is hidden or gone, detached when it is gone; attached means present
	// whether or not it is visible, visible does not pass for a hidden element. One element is removed and one hidden
	// after 2 s; each is there when the step starts.
	ctx := openOn(t, `<title>states</title><p id="gone">x</p><p id="hid">y</p><script>setTimeout(() => {
document.getElementById('gone').remove(); document.getElementById('hid').style.display = 'none';
}, 2000)</script>`, nil)
	wait := func(selector, state string, extra ...any) (Result, error) {
		return runResult(ctx, t, newAction("waitFor", extend(kv("find", findCSS(selector), "state", state), extra)))
	}
	require.Equal(t, 1, countOf(ctx, t, "#gone"))
	require.Equal(t, 1, countOf(ctx, t, "#hid"))

	_, err := wait("#gone", "detached")
	require.NoError(t, err)
	assert.Equal(t, 0, countOf(ctx, t, "#gone"))
	_, err = wait("#hid", "hidden")
	require.NoError(t, err)
	assert.Equal(t, 1, countOf(ctx, t, "#hid"), "hidden is not detached")
	_, err = wait("#gone", "hidden")
	require.NoError(t, err, "an element that is gone is hidden")
	_, err = wait("#hid", "attached")
	require.NoError(t, err, "a hidden element is attached")
	_, err = wait("#hid", "visible", "syncTimeout", 1)
	require.Error(t, err, "a hidden element is not visible")
	_, err = wait("#hid", "detached", "syncTimeout", 1)
	require.Error(t, err, "a hidden element is not detached")
}

func TestPlaywright_waitForWithTwoMatches(t *testing.T) {
	// US2-AS7: two matching elements appear after 1 s; the step waits on the first and does not fail with a strict-mode
	// error, whatever the state and the locator kind, and both are there when it passes. (The detached half is
	// TestPlaywright_waitForDetachedWithTwoMatches.)
	page := `<title>two</title><script>setTimeout(() => {
for (let i = 0; i < 2; i++) { const p = document.createElement('p'); p.className = 'm'; p.textContent = 'same'; document.body.appendChild(p); }
}, 1000)</script>`
	for name, find := range map[string]interface{}{"CSS": findCSS(".m"), "TEXT": loc("TEXT", "same")} {
		for _, state := range []string{"visible", "attached"} {
			t.Run(name+" "+state, func(t *testing.T) {
				ctx := openOn(t, page, nil)
				require.Equal(t, 0, countOf(ctx, t, ".m"))
				mustRun(ctx, t, newAction("waitFor", kv("find", find, "state", state, "syncTimeout", 5)))
				assert.Equal(t, 2, countOf(ctx, t, ".m"))
			})
		}
	}
}

// The hidden case with two matches is TestPlaywright_waitForHiddenWithTwoMatches (D-023, answered): an element hidden
// with CSS keeps matching the locator, so the wait ends when the first one is hidden.
func TestPlaywright_waitForDetachedWithTwoMatches(t *testing.T) {
	// US2-AS7, D-017: with two matches, detached follows the first match as resolved at each check, so the wait ends
	// only when none is left. The first is removed after 1 s and the second after 3 s.
	page := `<title>two</title><p class="m">a</p><p class="m">b</p><script>
const els = document.querySelectorAll('.m');
setTimeout(() => els[0].remove(), 1000);
setTimeout(() => els[1].remove(), 3000);
</script>`
	wait := func(ctx context.Context, timeout int) (Result, error) {
		return runResult(ctx, t, newAction("waitFor", kv("find", findCSS(".m"), "state", "detached", "syncTimeout", timeout)))
	}
	t.Run("waits for the second", func(t *testing.T) {
		ctx := openOn(t, page, nil)
		require.Equal(t, 2, countOf(ctx, t, ".m"))
		start := time.Now()
		_, err := wait(ctx, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, time.Since(start), 2*time.Second, "the step must outlast the first element")
		assert.Equal(t, 0, countOf(ctx, t, ".m"))
	})
	t.Run("times out while the second is there", func(t *testing.T) {
		ctx := openOn(t, page, nil)
		_, err := wait(ctx, 2)
		require.Error(t, err)
		for _, part := range []string{"waitFor: ", ".m", "detached", "2s"} {
			assert.Contains(t, err.Error(), part)
		}
	})
}

func TestPlaywright_waitForHiddenWithTwoMatches(t *testing.T) {
	// US2-AS7, D-023: with two matches, hidden follows the first match, which stays the same element while it exists
	// (display:none keeps it matching), so the wait ends when the first is hidden, while the second is still shown.
	ctx := openOn(t, `<title>two</title><p class="m">a</p><p class="m">b</p><script>
const els = document.querySelectorAll('.m');
setTimeout(() => { els[0].style.display = 'none'; }, 1000);
setTimeout(() => { els[1].style.display = 'none'; }, 3000);
</script>`, nil)
	require.Equal(t, 2, countOf(ctx, t, ".m"))
	start := time.Now()
	mustRun(ctx, t, newAction("waitFor", kv("find", findCSS(".m"), "state", "hidden", "syncTimeout", 10)))
	assert.Less(t, time.Since(start), 2500*time.Millisecond, "the step ends when the first is hidden")
	assert.Equal(t, 2, countOf(ctx, t, ".m"), "hidden is not detached")
	assert.Equal(t, 1, countOf(ctx, t, ".m:visible"), "the second is still shown when the step ends")
}

func TestPlaywright_waitForAMistypedSelector(t *testing.T) {
	// US2-AS8: an element that never existed is hidden and detached at once, as Playwright says (the trap the README
	// writes down); waiting for it to be visible or attached fails.
	ctx := openOn(t, `<title>typo</title><p id="real">x</p>`, nil)
	for _, state := range []string{"hidden", "detached"} {
		t.Run(state+" passes", func(t *testing.T) {
			mustRun(ctx, t, newAction("waitFor", kv("find", findCSS("#raelpar"), "state", state, "syncTimeout", 1)))
		})
	}
	for _, state := range []string{"visible", "attached"} {
		t.Run(state+" fails", func(t *testing.T) {
			_, err := runResult(ctx, t, newAction("waitFor", kv("find", findCSS("#raelpar"), "state", state, "syncTimeout", 1)))
			require.Error(t, err)
		})
	}
}

const hoverPage = `<title>hover</title><style>.panel{display:none}.panel.open{display:block}</style>
<button id="t1" onmouseover="document.getElementById('p1').className='panel open'">Menu</button>
<ul id="p1" class="panel"><li>One</li></ul>
<button id="t2" onmouseover="document.getElementById('p2').className='panel open'">Menu two</button>
<ul id="p2" class="panel"><li>Two</li></ul>`

func TestPlaywright_hoverOpensAMenu(t *testing.T) {
	// US3-AS1: a hover on the trigger adds the class "open" to its list; with two matching triggers the first is hovered.
	ctx := openOn(t, hoverPage, nil)
	require.Equal(t, 0, countOf(ctx, t, "#p1.open"))
	mustRun(ctx, t, newAction("hover", kv("find", loc("ROLE", "button", "name", "Menu"))))
	assert.Equal(t, 1, countOf(ctx, t, "#p1.open"))
	assert.Equal(t, 0, countOf(ctx, t, "#p2.open"), "the second of the two matches is not hovered")

	mustRun(ctx, t, newAction("hover", kv("find", findCSS("#t2"))))
	assert.Equal(t, 1, countOf(ctx, t, "#p2.open"))
}

func TestPlaywright_hoverOnAnElementThatIsNotThere(t *testing.T) {
	// US3-AS1: a hover on an element that does not exist fails after syncTimeout and names the element.
	ctx := openOn(t, hoverPage, nil)
	_, err := runResult(ctx, t, newAction("hover", kv("find", findCSS("#nowhere"), "syncTimeout", 1)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hover: ")
	assert.Contains(t, err.Error(), "#nowhere")
}

const dragPage = `<title>drag</title>
<ul id="src"><li draggable="true" id="item">Item</li><li draggable="true" id="other">Other</li></ul>
<div id="z1" class="zone" style="width:300px;height:80px;border:1px solid"></div>
<div id="z2" class="zone" style="width:300px;height:80px;border:1px solid"></div>
<script>
for (const zone of document.querySelectorAll('.zone')) {
  zone.addEventListener('dragover', e => e.preventDefault());
  zone.addEventListener('drop', e => { e.preventDefault(); zone.appendChild(document.getElementById(e.dataTransfer.getData('text'))); });
}
for (const el of document.querySelectorAll('[draggable]')) {
  el.addEventListener('dragstart', e => e.dataTransfer.setData('text', el.id));
}
</script>`

func TestPlaywright_dragMovesAnItem(t *testing.T) {
	// US3-AS2: a drag of an item onto a zone moves it there; with several matches for either, the first is used
	// (HTML5 drag and drop in Chromium, assumed by the spec and proven here).
	ctx := openOn(t, dragPage, nil)
	require.Equal(t, 0, countOf(ctx, t, "#z1 #item"))
	mustRun(ctx, t, newAction("dragAndDrop", kv("from", findCSS("#item"), "to", findCSS("#z2"))))
	assert.Equal(t, 1, countOf(ctx, t, "#z2 #item"))

	mustRun(ctx, t, newAction("dragAndDrop", kv("from", findCSS("#src [draggable]"), "to", findCSS(".zone"))))
	assert.Equal(t, 1, countOf(ctx, t, "#z1 #other"), "the first remaining item went to the first zone")
	assert.Equal(t, 1, countOf(ctx, t, "#z2 #item"))
}

func TestPlaywright_dragFromAnElementThatIsNotThere(t *testing.T) {
	// US3-AS2: a drag of an element that does not exist fails after syncTimeout and names it.
	ctx := openOn(t, dragPage, nil)
	_, err := runResult(ctx, t, newAction("dragAndDrop", kv("from", findCSS("#nowhere"), "to", findCSS("#z1"), "syncTimeout", 1)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dragAndDrop: ")
	assert.Contains(t, err.Error(), "#nowhere")
}

const keysPage = `<title>keys</title>
<input id="in" value="hello" onselect="document.getElementById('sel').textContent = this.value.substring(this.selectionStart, this.selectionEnd)">
<button id="other">other</button><p id="log"></p><p id="sel"></p>
<script>
const keys = [];
document.addEventListener('keydown', e => { keys.push(e.target.id + ':' + e.key); document.getElementById('log').textContent = keys.join(','); });
</script>`

// logOf is the keys the page received, as "<target id>:<key>" joined by commas.
func logOf(ctx context.Context, t *testing.T) string {
	t.Helper()
	return mustRun(ctx, t, findBy("#log")).Text
}

func TestPlaywright_pressReachesTheFocusedElement(t *testing.T) {
	// US3-AS3: without find the key goes to the focused element of the page, and to the body when nothing is focused.
	ctx := openOn(t, keysPage, nil)
	mustRun(ctx, t, pressStep("Enter", false))
	assert.Equal(t, ":Enter", logOf(ctx, t), "nothing focused: the body")

	mustRun(ctx, t, clickOn("#in"))
	mustRun(ctx, t, pressStep("Escape", false))
	assert.Equal(t, ":Enter,in:Escape", logOf(ctx, t), "the focused input")
}

func TestPlaywright_pressWithFindFocusesTheElement(t *testing.T) {
	// US3-AS4: with find the element is focused and the chord sent to it: Control+A selects the text of the input while
	// the focus was on a button.
	ctx := openOn(t, keysPage, nil)
	mustRun(ctx, t, clickOn("#other"))
	mustRun(ctx, t, pressStep("Control+A", true, "syncTimeout", 5))
	assert.Equal(t, "hello", mustRun(ctx, t, findBy("#sel")).Text)
	assert.Contains(t, logOf(ctx, t), "in:Control", "the key went to the input, not to the button")
}

func TestPlaywright_pressWebDriverNames(t *testing.T) {
	// US3-AS5: ENTER, ESCAPE, TAB and BACK_SPACE reach the page as Enter, Escape, Tab and Backspace; Tab moves the focus
	// off the input, so the Backspace goes to the button.
	ctx := openOn(t, keysPage, nil)
	mustRun(ctx, t, clickOn("#in"))
	for _, name := range []string{"ENTER", "ESCAPE", "TAB", "BACK_SPACE"} {
		mustRun(ctx, t, pressStep(name, false))
	}
	assert.Equal(t, "in:Enter,in:Escape,in:Tab,other:Backspace", logOf(ctx, t))
}

func TestPlaywright_everyKeyOfTheTableIsAcceptedByPlaywright(t *testing.T) {
	// D-011, F13: Playwright accepts every name of the table, pressed as a step; so a name the table holds that Playwright
	// refuses fails the build. The control: Playwright does refuse a name it does not know, so this test can fail.
	ctx := openOn(t, `<title>table</title><p>nothing to type into</p>`, nil)
	require.Error(t, getWebCtx(ctx).page.pressKey("NoSuchKey"))
	for _, name := range playwrightKeyList() {
		t.Run(name, func(t *testing.T) {
			mustRun(ctx, t, pressStep(name, false))
		})
	}
}
