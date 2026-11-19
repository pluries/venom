package web

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of the press action of spec 015 (docs/specs/015-web-recipe-actions, US3-AS3 to AS8, D-011): the key or
// chord sent for each name, and every refusal before any browser call. The helpers are those of recipe_actions_test.go.
//
// Wording these tests fix (reported in tdd-report.md): an unknown name that is more than one character, or an empty
// part, is refused with "one key or one chord" and names the key (US3-AS6 and AS8 cannot tell "Bogus" from "abc"); a
// WebDriver name with no Playwright key is refused with "no Playwright equivalent" and its name; an empty or missing key
// is refused naming the field 'key' between quotes.
//
// Wording fixed by the review of spec 015 (finding 5, a secret put in press must not leak): a refused value that looks
// like a key name (letters, digits, underscore and plus, 1 to 20 characters) is named in the message; any other value is
// never echoed, and the message says "a text of N characters" (N counts runes), in parsePress and in the refusal of a
// press whose value is not a map. A press whose key is not a string is refused by validateNewActions, naming 'key'.

const oneKeyOrChord = "one key or one chord"

// pressSent are the keys press sends: the key as a suite writes it, and the key the seam receives.
var pressSent = []struct{ key, want string }{
	{"Enter", "Enter"},
	{"Escape", "Escape"},
	{"Tab", "Tab"},
	{"ArrowDown", "ArrowDown"},
	{"F5", "F5"},
	{"Control+A", "Control+A"},
	{"Shift+ArrowLeft", "Shift+ArrowLeft"},
	{"ControlOrMeta+A", "ControlOrMeta+A"},
	{"Control+a", "Control+a"},
	{"a", "a"},
	{"A", "A"},
	{"5", "5"},
	{"ENTER", "Enter"},
	{"ESCAPE", "Escape"},
	{"TAB", "Tab"},
	{"BACK_SPACE", "Backspace"},
	{"ARROW_LEFT", "ArrowLeft"},
	{"+", "+"},
	{"Control++", "Control++"},
}

// pressRefused are keys press refuses, with what the message holds besides the key.
var pressRefused = []struct {
	name  string
	key   string
	field string // the field the message names between quotes, when it does
	parts []string
}{
	{"an unknown name", "Bogus", "", []string{"Bogus", oneKeyOrChord}},
	{"an unknown name in a chord", "Control+Bogus", "", []string{"Bogus", oneKeyOrChord}},
	{"a text", "abc", "", []string{"abc", oneKeyOrChord}},
	{"a sentence", "hello world", "", []string{"11 characters", oneKeyOrChord}},
	{"enter in lower case", "enter", "", []string{"enter", oneKeyOrChord}},
	{"a modifier in lower case", "control+a", "", []string{"control", oneKeyOrChord}},
	{"HELP, a WebDriver name without a Playwright key", "HELP", "", []string{"HELP", "no Playwright equivalent"}},
	{"NULL", "NULL", "", []string{"NULL", "no Playwright equivalent"}},
	{"CANCEL", "CANCEL", "", []string{"CANCEL", "no Playwright equivalent"}},
	{"ZENKAKU_HANKAKU", "ZENKAKU_HANKAKU", "", []string{"ZENKAKU_HANKAKU", "no Playwright equivalent"}},
	{"a WebDriver name in a chord", "Control+ENTER", "", []string{"ENTER"}},
	{"a WebDriver modifier in a chord", "CONTROL+A", "", []string{"CONTROL"}},
	{"an empty key", "", "key", []string{oneKeyOrChord}},
	{"a chord with an empty part", "Control+", "", []string{oneKeyOrChord}},
	{"a chord that starts with a plus", "+A", "", []string{oneKeyOrChord}},
}

func TestParsePress_theKeySent(t *testing.T) {
	// US3-AS3 to AS5, FR-010: a Playwright name, a one-character key and a chord of them go as they are; a WebDriver
	// name goes through playwrightKeys; the literal plus key is one key, alone or last in a chord.
	venom.InitTestLogger(t)
	for _, tt := range pressSent {
		t.Run(tt.key, func(t *testing.T) {
			got, err := parsePress(context.Background(), tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePress_everyWebDriverNameMapsLikeFill(t *testing.T) {
	// D-003: the WebDriver names press takes alone are those fill.key sends, through the same map.
	venom.InitTestLogger(t)
	for name, want := range playwrightKeys {
		t.Run(name, func(t *testing.T) {
			got, err := parsePress(context.Background(), name)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestParsePress_refusals(t *testing.T) {
	// US3-AS6 to AS8: an unknown name, a WebDriver name with no Playwright key, a text and an empty part are refused,
	// the key named; the names are case-sensitive and a chord takes Playwright names and one-character keys only.
	venom.InitTestLogger(t)
	for _, tt := range pressRefused {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePress(context.Background(), tt.key)
			require.Error(t, err)
			assert.Empty(t, got)
			for _, part := range tt.parts {
				assert.Contains(t, err.Error(), part)
			}
		})
	}
}

func TestPlaywrightKeyList(t *testing.T) {
	// D-011, F13: the table of Playwright's key names holds every value playwrightKeys sends, so a WebDriver name press
	// accepts is a name the table knows; it holds the names a recipe writes, and no name twice. The real-browser test
	// presses every entry, so a name Playwright refuses fails the build.
	list := playwrightKeyList()
	require.NotEmpty(t, list)
	for _, want := range playwrightKeys {
		assert.Contains(t, list, want, "playwrightKeys sends %s", want)
	}
	for _, want := range []string{
		"Enter", "Escape", "Tab", "Backspace", "Delete", "Home", "End", "PageUp", "PageDown", "Insert", "Space",
		"ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "F1", "F12", "KeyA", "Digit0",
		"Shift", "Control", "Alt", "Meta", "ControlOrMeta",
	} {
		assert.Contains(t, list, want)
	}
	sorted := slices.Sorted(slices.Values(list))
	assert.Equal(t, sorted, slices.Compact(slices.Clone(sorted)), "a name is in the table once")
}

func TestParsePress_everyTableNameIsAccepted(t *testing.T) {
	// D-011: each name of the table is accepted alone and as the key of a chord, and sent as written.
	venom.InitTestLogger(t)
	for _, name := range playwrightKeyList() {
		t.Run(name, func(t *testing.T) {
			got, err := parsePress(context.Background(), name)
			require.NoError(t, err)
			assert.Equal(t, name, got)
			chord, err := parsePress(context.Background(), "Shift+"+name)
			require.NoError(t, err)
			assert.Equal(t, "Shift+"+name, chord)
		})
	}
}

func pressStep(key string, withFind bool, extra ...any) venom.TestStep {
	fields := kv("key", key)
	if withFind {
		fields["find"] = findCSS("#in")
	}
	return newAction("press", extend(fields, extra))
}

func TestPress_sendsTheKey(t *testing.T) {
	// US3-AS3 to AS5: with find the element is focused and the key sent to it (Locator.Press); without find the key goes
	// to the focused element of the page (Keyboard.Press). The result holds the title and the URL and nothing else.
	for _, tt := range pressSent {
		t.Run(tt.key+" with find", func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			res := mustRun(ctx, t, pressStep(tt.key, true))
			assert.Equal(t, []string{"Press css=#in first " + tt.want}, callsWithPrefix(pw.seen(), "Press "))
			assert.Empty(t, callsWithPrefix(pw.seen(), "PressKey"))
			assert.Equal(t, pageResult(), withoutTime(res))
		})
		t.Run(tt.key+" without find", func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			res := mustRun(ctx, t, pressStep(tt.key, false))
			assert.Equal(t, []string{"PressKey page1 " + tt.want}, callsWithPrefix(pw.seen(), "PressKey"))
			assert.Empty(t, callsWithPrefix(pw.seen(), "Press "))
			assert.Equal(t, pageResult(), withoutTime(res))
		})
	}
}

func TestPress_withFindTakesTheFirstMatch(t *testing.T) {
	// US3-AS4: with several matches the key goes to the first, and nothing is counted.
	role := fakeRoleName("textbox", ptr("Search"), false)
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.nodes[role] = []fakeNode{{text: "a"}, {text: "b"}}
	step := newAction("press", kv("key", "Control+A", "find", loc("ROLE", "textbox", "name", "Search")))
	mustRun(ctx, t, step)
	assert.Equal(t, []string{"Press " + role + " first Control+A"}, callsWithPrefix(pw.seen(), "Press "))
	assert.Empty(t, callsWithPrefix(pw.seen(), "Count"))
}

func TestPress_refusals(t *testing.T) {
	// US3-AS6 to AS8: each refused key fails the step before any browser action, with and without find.
	for _, tt := range pressRefused {
		for _, withFind := range []bool{true, false} {
			name := tt.name + " without find"
			if withFind {
				name = tt.name + " with find"
			}
			t.Run(name, func(t *testing.T) {
				assertRefused(t, pressStep(tt.key, withFind), tt.field, tt.parts...)
			})
		}
	}
}

func TestPress_aMissingKeyIsRefused(t *testing.T) {
	// US3-AS8: no key at all, with and without find, fails before any browser action and says what press takes.
	tests := []struct {
		name   string
		fields map[string]interface{}
	}{
		{"an empty press", kv()},
		{"find only", kv("find", findCSS("#in"))},
		{"a nil key", kv("key", nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRefused(t, newAction("press", tt.fields), "key", oneKeyOrChord)
		})
	}
}

func TestPress_aFindThatIsRefusedStopsBeforeTheKey(t *testing.T) {
	// FR-002: press takes every locator kind, so a find that parseFind refuses fails the step before any browser action.
	assertRefused(t, newAction("press", kv("key", "Enter", "find", loc("ROLE", "Button"))), "", "selector", "button")
}

func TestPress_failureNamesTheKeyAndTheElement(t *testing.T) {
	// US3-AS1 (the same rule as hover): a key the browser fails fails the step with a message that starts with press, names
	// the key, the element when there is one, and the cause.
	tests := []struct {
		name     string
		withFind bool
		call     string
	}{
		{"with find", true, "Press css=#in first Enter"},
		{"without find", false, "PressKey page1 Enter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.errs[tt.call] = errors.New("Timeout exceeded: nothing matched")
			_, err := runResult(ctx, t, pressStep("Enter", tt.withFind))
			require.Error(t, err)
			for _, part := range []string{"press: ", "Enter", "Timeout exceeded: nothing matched"} {
				assert.Contains(t, err.Error(), part)
			}
			if tt.withFind {
				assert.Contains(t, err.Error(), describeFind(findCSS("#in")))
			}
		})
	}
}

// pressTexts are values that do not look like a key name: refused, never echoed (finding 5).
var pressTexts = []struct{ name, key string }{
	{"a template that holds a secret", "{{.password}}"},
	{"a quote and a backslash", `p\"w`},
	{"a control character", "a\x01b"},
	{"a text of 40 characters", "abcdefghijklmnopqrstuvwxyz0123456789ABCD"},
	{"a text of 21 characters", strings.Repeat("a", 21)},
	{"a text with a space", "hello world"},
	{"a text with accents", "pässwörd!"},
	{"a chord whose second part is a template", "Control+{{.password}}"},
}

// assertNotEchoed checks that the message neither holds value nor its quoted form, and says how long the text is.
func assertNotEchoed(t *testing.T, err error, value string) {
	t.Helper()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), value)
	assert.NotContains(t, err.Error(), strconv.Quote(value))
	assert.NotContains(t, err.Error(), strconv.Quote(value)[1:len(strconv.Quote(value))-1])
	assert.Contains(t, err.Error(), fmt.Sprintf("%d characters", utf8.RuneCountInString(value)))
}

func TestParsePress_aTextIsNeverEchoed(t *testing.T) {
	// FR-010, finding 5: a text put in press is refused without its value in the message, which says it is a text of N
	// characters; a secret such as {{.password}} must not reach the report, the log or the page dump.
	venom.InitTestLogger(t)
	for _, tt := range pressTexts {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePress(context.Background(), tt.key)
			assert.Empty(t, got)
			assertNotEchoed(t, err, tt.key)
			assert.Contains(t, err.Error(), oneKeyOrChord)
		})
	}
}

func TestParsePress_aKeyNameIsNamed(t *testing.T) {
	// Finding 5: a refused value that looks like a key name (letters, digits, underscore, plus; 1 to 20 characters) is
	// named, so the author sees the typo; 21 characters is already a text.
	venom.InitTestLogger(t)
	for _, key := range []string{"Bogus", "Control+Bogus", "enter", "control+a", "Not_A_Key", strings.Repeat("a", 20)} {
		t.Run(key, func(t *testing.T) {
			_, err := parsePress(context.Background(), key)
			require.Error(t, err)
			assert.Contains(t, err.Error(), key)
			assert.NotContains(t, err.Error(), "characters")
		})
	}
}

func TestPress_aTextIsNeverEchoedByTheStep(t *testing.T) {
	// Finding 5, through Run: the step's error, with and without find, does not hold the text either.
	for _, tt := range pressTexts {
		for _, withFind := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, find %t", tt.name, withFind), func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				_, err := runResult(ctx, t, pressStep(tt.key, withFind))
				assertNotEchoed(t, err, tt.key)
				assertNoBrowserAction(t, pw)
			})
		}
	}
}

func TestValidateNewActions_aPressTextIsNeverEchoed(t *testing.T) {
	// Finding 5: press: <text> (not a map) is refused by validateNewActions without the text, saying how long it is, and
	// still names the action and the field it takes; a key that is not a string is refused naming 'key', without its value.
	t.Run("a press that is a text", func(t *testing.T) {
		const secret = "{{.password}}"
		err := validateNewActions(action(map[string]interface{}{"press": secret}))
		assertNotEchoed(t, err, secret)
		assert.Contains(t, err.Error(), "press")
		assert.Contains(t, err.Error(), "key")
	})
	for name, key := range map[string]interface{}{
		"a list":    []interface{}{"{{.password}}"},
		"a mapping": map[string]interface{}{"secret": "{{.password}}"},
	} {
		t.Run("a key that is "+name, func(t *testing.T) {
			err := validateNewActions(newAction("press", kv("key", key)))
			require.Error(t, err, "a key that is not a string is refused before the decode, which would echo it")
			assertNamesField(t, err, "key")
			assert.NotContains(t, err.Error(), "{{.password}}")
		})
	}
}

// heldFor is a hook that does not return until the test ends or d has passed: a page whose main thread is stuck, bounded
// so that a call without a limit ends the test instead of hanging it.
func heldFor(pw *fakePW, d time.Duration) func() {
	return func() {
		select {
		case <-pw.closed:
		case <-time.After(d):
		}
	}
}

func TestPress_withoutFindIsBoundedByTheReadLimit(t *testing.T) {
	// FR-010, US3-AS3, R17: Keyboard.Press has no Playwright timeout, so a page whose main thread is stuck would hold the
	// step for ever under ownTimeout. It fails with errNoAnswer once readTimeout (shortened here) has passed.
	shortLimits(t)
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.hooks["PressKey page1 Enter"] = heldFor(pw, 3*time.Second)
	_, err := runResult(ctx, t, pressStep("Enter", false))
	require.ErrorIs(t, err, errNoAnswer)
	assert.ErrorContains(t, err, "within 0.05s")
}

func TestPress_withoutFindKeepsTheSyncTimeoutBeforeTheReadLimit(t *testing.T) {
	// FR-010, R17: as elementRead does, the limit of press without find is syncTimeout plus readTimeout, so the step
	// that waits for its syncTimeout is not cut at readTimeout. readTimeout is 50 ms, syncTimeout 1 s: the limit is 1.05 s
	// (the one real wait of the file) and the page, held for 4 s, never answers within it.
	shortLimits(t)
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.hooks["PressKey page1 Enter"] = heldFor(pw, 4*time.Second)
	start := time.Now()
	_, err := runResult(ctx, t, pressStep("Enter", false, "syncTimeout", 1))
	require.ErrorIs(t, err, errNoAnswer)
	assert.ErrorContains(t, err, "within 1.05s")
	assert.GreaterOrEqual(t, time.Since(start), time.Second, "the syncTimeout is waited before the read limit")
}

func TestPress_aNumericKeyIsRefusedWithTheHintToQuoteIt(t *testing.T) {
	// FR-010 (characterization, already satisfied): key: 5 is a YAML int, not a text; it is refused before any browser
	// action, naming 'key' and telling the author to write a digit in quotes. The value is never echoed (a number may be
	// a secret): 7357 stands for it, 5 being in the hint itself.
	const hint = "write a digit in quotes"
	for _, withFind := range []bool{true, false} {
		t.Run(fmt.Sprintf("5, find %t", withFind), func(t *testing.T) {
			fields := kv("key", 5)
			if withFind {
				fields["find"] = findCSS("#in")
			}
			assertRefused(t, newAction("press", fields), "key", hint)
		})
	}
	t.Run("the value is not echoed", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		_, err := runResult(ctx, t, newAction("press", kv("key", 7357)))
		require.Error(t, err)
		assertNamesField(t, err, "key")
		assert.NotContains(t, err.Error(), "7357")
	})
}

func TestPress_aDigitInQuotesIsSent(t *testing.T) {
	// FR-010 (characterization, already satisfied): key: "5", a string, is accepted and the seam receives the key 5.
	for _, tt := range []struct{ name, call string }{
		{"with find", "Press css=#in first 5"},
		{"without find", "PressKey page1 5"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			mustRun(ctx, t, pressStep("5", tt.name == "with find"))
			assert.Contains(t, pw.seen(), tt.call)
		})
	}
}
