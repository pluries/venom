package web

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of the actions of spec 015 (docs/specs/015-web-recipe-actions, US2 and US3): waitFor, hover and
// dragAndDrop, and what they share with press (press_test.go): the validation before any browser call, the order of
// the actions, the wait for a navigation in flight. They run on the fake of the seam; Chromium's own behaviour is
// proven by web_recipe_actions_playwright_test.go. Each test names its scenarios (USn-ASm).
//
// Wording these tests fix, which the plan leaves open (reported in tdd-report.md):
//   - a failure of waitFor, hover, dragAndDrop or press starts with the action's name and ": ", and holds describeFind of
//     every element it names (dragAndDrop: from and to), the cause, and for waitFor the state and the timeout as "<n>s";
//   - a refusal names the field at fault between quotes (single, double or back), and lists the allowed values or keys.

// kv is a map of the field and value pairs.
func kv(pairs ...any) map[string]interface{} {
	m := map[string]interface{}{}
	for i := 0; i+1 < len(pairs); i += 2 {
		name, ok := pairs[i].(string)
		if !ok {
			panic(fmt.Sprintf("kv: the name at %d is a %T, not a string", i, pairs[i]))
		}
		m[name] = pairs[i+1]
	}
	return m
}

// newAction is a step that sets one new action, as a suite writes it.
func newAction(name string, fields map[string]interface{}) venom.TestStep {
	return action(map[string]interface{}{name: fields})
}

// extend adds the pairs to fields.
func extend(fields map[string]interface{}, extra []any) map[string]interface{} {
	for k, v := range kv(extra...) {
		fields[k] = v
	}
	return fields
}

func withoutTime(r Result) Result {
	r.TimeSeconds = 0
	return r
}

// newActionForm is a valid step of one new action on the fake, with the one call it sends and the finds a failure names.
type newActionForm struct {
	name  string
	step  func(extra ...any) venom.TestStep
	call  string // as the fake logs it, without its timeout
	named []interface{}
}

var newActionForms = []newActionForm{
	{"waitFor", func(extra ...any) venom.TestStep {
		return newAction("waitFor", extend(kv("find", findCSS(".t")), extra))
	}, "WaitFor css=.t first state=visible", []interface{}{findCSS(".t")}},
	{"hover", func(extra ...any) venom.TestStep {
		return newAction("hover", extend(kv("find", findCSS(".t")), extra))
	}, "Hover css=.t first", []interface{}{findCSS(".t")}},
	{"dragAndDrop", func(extra ...any) venom.TestStep {
		return newAction("dragAndDrop", extend(kv("from", findCSS(".a"), "to", findCSS(".b")), extra))
	}, "DragTo css=.a first -> css=.b first", []interface{}{findCSS(".a"), findCSS(".b")}},
	{"press", func(extra ...any) venom.TestStep {
		return newAction("press", extend(kv("key", "Enter", "find", findCSS(".t")), extra))
	}, "Press css=.t first Enter", []interface{}{findCSS(".t")}},
}

// pressWithoutFind is press with no element: the key goes to the page.
var pressWithoutFind = newActionForm{"press without find", func(extra ...any) venom.TestStep {
	return newAction("press", extend(kv("key", "Enter"), extra))
}, "PressKey page1 Enter", nil}

// newActionCalls keeps the calls of the four new actions.
func newActionCalls(calls []string) []string {
	var out []string
	for _, prefix := range []string{"WaitFor ", "Hover ", "DragTo ", "Press ", "PressKey "} {
		out = append(out, callsWithPrefix(calls, prefix)...)
	}
	return out
}

// assertNoBrowserAction checks that the step reached no action of the browser: the only call it may have made is the
// read of the page for the failure dump.
func assertNoBrowserAction(t *testing.T, pw *fakePW) {
	t.Helper()
	for _, c := range pw.seen() {
		assert.Equal(t, "Content page1", c, "a refusal comes before any browser action")
	}
}

// assertNamesField checks that the message names the field between quotes, whichever quotes.
func assertNamesField(t *testing.T, err error, field string) {
	t.Helper()
	re := regexp.MustCompile("['\"`]" + regexp.QuoteMeta(field) + "['\"`]")
	assert.Regexp(t, re, err.Error(), "the message must name the field %q between quotes", field)
}

// assertRefused runs step on a new fake and checks the refusal: the field named between quotes (when not empty), each
// part in the message, and no browser action.
func assertRefused(t *testing.T, step venom.TestStep, field string, parts ...string) {
	t.Helper()
	pw := withFake(t)
	ctx := openFake(t, pw)
	_, err := runResult(ctx, t, step)
	require.Error(t, err)
	if field != "" {
		assertNamesField(t, err, field)
	}
	for _, part := range parts {
		assert.Contains(t, err.Error(), part)
	}
	assertNoBrowserAction(t, pw)
}

func TestWaitFor_stateAndTimeoutReachTheSeam(t *testing.T) {
	// US2-AS1, AS3: the state is the one the step names, visible when it names none; the step passes and its result
	// holds the title and URL and nothing else.
	tests := []struct {
		name  string
		extra []any
		want  string
	}{
		{"no state means visible", nil, "WaitFor css=.late first state=visible"},
		{"visible", []any{"state", "visible"}, "WaitFor css=.late first state=visible"},
		{"hidden", []any{"state", "hidden"}, "WaitFor css=.late first state=hidden"},
		{"attached", []any{"state", "attached"}, "WaitFor css=.late first state=attached"},
		{"detached", []any{"state", "detached"}, "WaitFor css=.late first state=detached"},
		{"a timeout and a state", []any{"state", "hidden", "syncTimeout", 2}, "WaitFor css=.late first state=hidden timeout=2000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			res := mustRun(ctx, t, newAction("waitFor", extend(kv("find", findCSS(".late")), tt.extra)))
			assert.Equal(t, []string{tt.want}, callsWithPrefix(pw.seen(), "WaitFor "))
			assert.Equal(t, pageResult(), withoutTime(res))
		})
	}
}

func TestNewActions_syncTimeoutReachesTheSeam(t *testing.T) {
	// US2-AS6 and the same rule for hover, dragAndDrop and press with find: whole seconds go to the seam as
	// milliseconds; 0, a negative value and no value send no timeout, so the context's 30 s applies. The value passed
	// is checked, never a real wait.
	tests := []struct {
		name   string
		extra  []any
		suffix string
	}{
		{"5 s", []any{"syncTimeout", 5}, " timeout=5000"},
		{"1 s", []any{"syncTimeout", 1}, " timeout=1000"},
		{"0 means the default", []any{"syncTimeout", 0}, ""},
		{"a negative value means the default", []any{"syncTimeout", -3}, ""},
		{"no value means the default", nil, ""},
	}
	for _, form := range newActionForms {
		for _, tt := range tests {
			t.Run(form.name+"/"+tt.name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				mustRun(ctx, t, form.step(tt.extra...))
				assert.Equal(t, []string{form.call + tt.suffix}, newActionCalls(pw.seen()))
			})
		}
	}
}

func TestWaitFor_failureNamesTheElementTheStateAndTheTimeout(t *testing.T) {
	// US2-AS2: a state that is not reached fails the step with a message that names the element, the state and the
	// timeout (the default 30 s when the step sets none, 0 or a negative value), and the page dump is written as for any
	// failed step.
	tests := []struct {
		name    string
		extra   []any
		state   string
		timeout string
	}{
		{"the default state and timeout", nil, "visible", "30s"},
		{"hidden in 2 s", []any{"state", "hidden", "syncTimeout", 2}, "hidden", "2s"},
		{"attached with a negative timeout", []any{"state", "attached", "syncTimeout", -1}, "attached", "30s"},
		{"detached with 0", []any{"state", "detached", "syncTimeout", 0}, "detached", "30s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			find := findCSS(".never")
			pw.errs["WaitFor css=.never first state="+tt.state] = errors.New("Timeout exceeded: nothing matched")
			_, err := runResult(ctx, t, newAction("waitFor", extend(kv("find", find), tt.extra)))
			require.Error(t, err)
			for _, part := range []string{"waitFor: ", ".never", describeFind(find), tt.state, tt.timeout, "Timeout exceeded: nothing matched"} {
				assert.Contains(t, err.Error(), part)
			}
			assert.Contains(t, pw.seen(), "Content page1", "the page is read for the dump")
			dump, readErr := os.ReadFile("page.dump.html")
			require.NoError(t, readErr)
			assert.Equal(t, pageSource, string(dump))
		})
	}
}

func TestWaitFor_refusals(t *testing.T) {
	// US2-AS4: an unknown state, a missing find, a mistyped key and a find that is itself refused fail the step before
	// any browser action; the message names the field and lists the allowed values.
	states := []string{"visible", "hidden", "attached", "detached"}
	tests := []struct {
		name   string
		fields map[string]interface{}
		field  string
		parts  []string
	}{
		{"an unknown state", kv("find", findCSS(".t"), "state", "ready"), "state", append([]string{"ready"}, states...)},
		{"a state in the wrong case", kv("find", findCSS(".t"), "state", "Visible"), "state", append([]string{"Visible"}, states...)},
		{"no find", kv("state", "hidden"), "find", []string{"waitFor"}},
		{"an empty waitFor", kv(), "find", []string{"waitFor"}},
		{"a nil find", kv("find", nil), "find", []string{"waitFor"}},
		{"a mistyped key", kv("find", findCSS(".t"), "syncTimout", 3), "syncTimout", []string{"find", "state", "syncTimeout"}},
		{"a find that is refused", kv("find", loc("ROLE", "Button")), "", []string{"selector", "button"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRefused(t, newAction("waitFor", tt.fields), tt.field, tt.parts...)
		})
	}
}

func TestWaitFor_waitsOnTheFirstMatch(t *testing.T) {
	// US2-AS7: with two matches the wait is on the first one (a first() locator: no strict-mode error), and no count is
	// made.
	role := fakeRoleName("button", ptr("Add"), false)
	for _, state := range []string{"visible", "hidden", "attached", "detached"} {
		t.Run(state, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes[role] = []fakeNode{{text: "Add"}, {text: "Add item"}}
			mustRun(ctx, t, newAction("waitFor", kv("find", loc("ROLE", "button", "name", "Add"), "state", state)))
			assert.Equal(t, []string{"WaitFor " + role + " first state=" + state}, callsWithPrefix(pw.seen(), "WaitFor "))
			assert.Empty(t, callsWithPrefix(pw.seen(), "Count"))
		})
	}
}

func TestFind_stillCountsWithoutWaiting(t *testing.T) {
	// US2-AS5 (characterization, already satisfied: find is unchanged): a find on an element that is not there counts 0
	// at once and sends no wait, alone and as the first action of a step that also sets a waitFor.
	tests := []struct {
		name string
		step venom.TestStep
	}{
		{"find alone", findStep(findCSS(".gone"))},
		{"find beside waitFor", action(map[string]interface{}{"find": findCSS(".gone"), "waitFor": kv("find", findCSS(".gone"))})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			res := mustRun(ctx, t, tt.step)
			assert.Equal(t, 0, res.Find)
			assert.Equal(t, []string{"Count css=.gone"}, callsWithPrefix(pw.seen(), "Count"))
			assert.Empty(t, callsWithPrefix(pw.seen(), "WaitFor "))
		})
	}
}

func TestHover_takesTheFirstMatch(t *testing.T) {
	// US3-AS1: hover moves onto the first of several matches, whatever the locator kind.
	tests := []struct {
		name    string
		find    interface{}
		element string
	}{
		{"CSS", findCSS(".menu"), "css=.menu"},
		{"ROLE with two matches", loc("ROLE", "button", "name", "Menu"), fakeRoleName("button", ptr("Menu"), false)},
		{"TEXT with two matches", loc("TEXT", "Menu"), fakeTextName("getByText", "Menu", false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes[tt.element] = []fakeNode{{text: "Menu"}, {text: "Menu 2"}}
			mustRun(ctx, t, newAction("hover", kv("find", tt.find)))
			assert.Equal(t, []string{"Hover " + tt.element + " first"}, callsWithPrefix(pw.seen(), "Hover"))
			assert.Empty(t, callsWithPrefix(pw.seen(), "Count"))
		})
	}
}

func TestDragAndDrop_takesTheFirstMatchOfEach(t *testing.T) {
	// US3-AS2: from and to are each the first of their matches, in the current frame, whatever the kind.
	item := fakeRoleName("listitem", ptr("Item"), false)
	zone := fakeTextName("getByText", "Zone", false)
	pw := withFake(t)
	ctx := openFake(t, pw)
	pw.nodes[item] = []fakeNode{{text: "Item"}, {text: "Item 2"}}
	pw.nodes[zone] = []fakeNode{{text: "Zone"}, {text: "Zone 2"}}
	mustRun(ctx, t, newAction("dragAndDrop", kv("from", loc("ROLE", "listitem", "name", "Item"), "to", loc("TEXT", "Zone"))))
	assert.Equal(t, []string{"DragTo " + item + " first -> " + zone + " first"}, callsWithPrefix(pw.seen(), "DragTo"))
	assert.Empty(t, callsWithPrefix(pw.seen(), "Count"))
}

func TestNewActions_failureNamesTheActionAndTheElements(t *testing.T) {
	// US3-AS1, AS2: a call the browser fails (an element that never appears, in practice) fails the step with a message
	// that starts with the action, names every element it works on (dragAndDrop: from and to) and gives the cause.
	for _, form := range newActionForms {
		t.Run(form.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.errs[form.call] = errors.New("Timeout exceeded: nothing matched")
			_, err := runResult(ctx, t, form.step())
			require.Error(t, err)
			assert.Contains(t, err.Error(), form.name+": ")
			assert.Contains(t, err.Error(), "Timeout exceeded: nothing matched")
			for _, named := range form.named {
				assert.Contains(t, err.Error(), describeFind(named))
			}
		})
	}
}

// malformed are new actions the step refuses before any browser action, with the field at fault and the allowed values.
var malformed = []struct {
	name   string
	action string
	fields map[string]interface{}
	field  string
	parts  []string
}{
	{"hover without find", "hover", kv(), "find", []string{"hover"}},
	{"hover with a nil find", "hover", kv("find", nil), "find", []string{"hover"}},
	{"hover with a mistyped key", "hover", kv("find", findCSS(".t"), "syncTimout", 1), "syncTimout", []string{"find", "syncTimeout"}},
	{"dragAndDrop without from", "dragAndDrop", kv("to", findCSS(".b")), "from", []string{"dragAndDrop"}},
	{"dragAndDrop without to", "dragAndDrop", kv("from", findCSS(".a")), "to", []string{"dragAndDrop"}},
	{"dragAndDrop without either", "dragAndDrop", kv(), "from", []string{"dragAndDrop"}},
	{"dragAndDrop with a mistyped key", "dragAndDrop", kv("from", findCSS(".a"), "to", findCSS(".b"), "bogus", 1), "bogus", []string{"from", "to", "syncTimeout"}},
	{"press with a mistyped key", "press", kv("key", "Enter", "bogus", 1), "bogus", []string{"key", "find", "syncTimeout"}},
	{"press with a text", "press", kv("key", "abc"), "", []string{"one key or one chord"}},
	{"waitFor with an unknown state", "waitFor", kv("find", findCSS(".t"), "state", "ready"), "state", []string{"ready", "visible", "hidden", "attached", "detached"}},
	{"waitFor without find", "waitFor", kv("state", "hidden"), "find", []string{"waitFor"}},
	{"waitFor with a mistyped key", "waitFor", kv("find", findCSS(".t"), "syncTimout", 1), "syncTimout", []string{"find", "state", "syncTimeout"}},
}

func TestNewActions_malformedAreRefused(t *testing.T) {
	// US3-AS2, D-012: a step with no find, from or to, or a key the action does not take (a typo mapstructure would
	// ignore), is refused before any browser action, naming the field and the allowed keys.
	for _, m := range malformed {
		t.Run(m.name, func(t *testing.T) {
			assertRefused(t, newAction(m.action, m.fields), m.field, m.parts...)
		})
	}
}

func TestRun_malformedNewActionBesideAnExistingOneIsIgnored(t *testing.T) {
	// US3-AS10, F17 (characterization, already satisfied): the same malformed action that is refused alone is ignored
	// when an earlier action of the order is set: the existing action runs and the new one is not looked at.
	for _, m := range malformed {
		t.Run(m.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			step := action(map[string]interface{}{"click": kv("find", findCSS(".go")), m.action: m.fields})
			mustRun(ctx, t, step)
			assert.Equal(t, []string{"Click css=.go first"}, callsWithPrefix(pw.seen(), "Click"))
			assert.Empty(t, newActionCalls(pw.seen()))
		})
	}
}

func TestRun_aValueThatIsNotAMapIsRefused(t *testing.T) {
	// US3-AS10 (F17): hover: true, press: Enter and the like are refused before any browser action; the message names
	// the action and the fields it takes, not only that a map was expected.
	tests := []struct {
		action string
		value  interface{}
		parts  []string
	}{
		{"hover", true, []string{"hover", "find"}},
		{"waitFor", "soon", []string{"waitFor", "find"}},
		{"dragAndDrop", 3, []string{"dragAndDrop", "from", "to"}},
		{"press", "Enter", []string{"press", "key"}},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			assertRefused(t, action(map[string]interface{}{tt.action: tt.value}), "", tt.parts...)
		})
	}
}

func TestRun_aMistypedSyncTimeoutKeepsMapstructuresError(t *testing.T) {
	// US3-AS10, F17 (characterization, already satisfied): a syncTimeout that is not a number fails at decoding, as it
	// does for the existing actions, whatever the new action.
	for _, form := range newActionForms {
		t.Run(form.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			_, err := runResult(ctx, t, form.step("syncTimeout", "soon"))
			require.Error(t, err)
			assert.Regexp(t, "(?i)synctimeout", err.Error())
			assert.Empty(t, newActionCalls(pw.seen()))
		})
	}
}

func TestRun_anExistingActionBeatsANewOne(t *testing.T) {
	// US3-AS10 (characterization, already satisfied): a step that sets an existing action and a valid new one runs the
	// existing one and ignores the new one, without error. wait is -1: it takes no time.
	fieldsOf := map[string]map[string]interface{}{
		"waitFor":     kv("find", findCSS(".t")),
		"hover":       kv("find", findCSS(".t")),
		"dragAndDrop": kv("from", findCSS(".a"), "to", findCSS(".b")),
		"press":       kv("key", "Enter", "find", findCSS(".t")),
	}
	existing := []struct {
		name   string
		fields map[string]interface{}
		want   string // the call of the existing action; empty for wait, which makes none
	}{
		{"click", map[string]interface{}{"click": kv("find", findCSS(".go"))}, "Click css=.go first"},
		{"find", map[string]interface{}{"find": findCSS(".go")}, "Count css=.go"},
		{"wait", map[string]interface{}{"wait": -1}, ""},
	}
	for _, ex := range existing {
		for name, fields := range fieldsOf {
			t.Run(ex.name+" and "+name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				step := map[string]interface{}{name: fields}
				for k, v := range ex.fields {
					step[k] = v
				}
				mustRun(ctx, t, action(step))
				assert.Empty(t, newActionCalls(pw.seen()))
				if ex.want != "" {
					assert.Contains(t, pw.seen(), ex.want)
				}
			})
		}
	}
}

func TestRun_newActionsKeepTheirOrder(t *testing.T) {
	// US3-AS10, FR-011: two new actions in one step run in the order waitFor, hover, dragAndDrop, press; the later one
	// is ignored.
	set := map[string]map[string]interface{}{
		"waitFor":     kv("find", findCSS(".t")),
		"hover":       kv("find", findCSS(".t")),
		"dragAndDrop": kv("from", findCSS(".a"), "to", findCSS(".b")),
		"press":       kv("key", "Enter", "find", findCSS(".t")),
	}
	tests := []struct {
		first, second string
		want          string
	}{
		{"waitFor", "hover", "WaitFor css=.t first state=visible"},
		{"waitFor", "press", "WaitFor css=.t first state=visible"},
		{"hover", "dragAndDrop", "Hover css=.t first"},
		{"hover", "press", "Hover css=.t first"},
		{"dragAndDrop", "press", "DragTo css=.a first -> css=.b first"},
	}
	for _, tt := range tests {
		t.Run(tt.first+" before "+tt.second, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			mustRun(ctx, t, action(map[string]interface{}{tt.second: set[tt.second], tt.first: set[tt.first]}))
			assert.Equal(t, []string{tt.want}, newActionCalls(pw.seen()))
		})
	}
}

func TestAction_callsFrame_newActions(t *testing.T) {
	// US3-AS9, FR-011, R19: each new field starts with a call to the current frame, press without find included, so the
	// step waits for a navigation in flight; beside another action the one performAction runs decides.
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{"waitFor", Action{WaitFor: &WaitFor{}}, true},
		{"hover", Action{Hover: &Hover{}}, true},
		{"dragAndDrop", Action{DragAndDrop: &DragAndDrop{}}, true},
		{"press with find", Action{Press: &Press{Key: "Enter", Find: ".t"}}, true},
		{"press without find", Action{Press: &Press{Key: "Enter"}}, true},
		{"click runs before hover", Action{Click: &Click{}, Hover: &Hover{}}, true},
		{"execute runs before press", Action{Execute: &Execute{}, Press: &Press{}}, true},
		{"navigate runs before waitFor", Action{Navigate: &Navigate{}, WaitFor: &WaitFor{}}, false},
		{"wait runs before waitFor", Action{Wait: 1, WaitFor: &WaitFor{}}, false},
		{"confirmPopup runs before hover", Action{ConfirmPopup: true, Hover: &Hover{}}, false},
		{"selectRootFrame runs before press", Action{SelectRootFrame: true, Press: &Press{}}, false},
		{"nextWindow runs before dragAndDrop", Action{NextWindow: true, DragAndDrop: &DragAndDrop{}}, false},
		{"historyAction runs before press", Action{HistoryAction: "back", Press: &Press{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.callsFrame())
		})
	}
}

func TestAction_reachesFrame(t *testing.T) {
	// US3-AS9: the helper of callsFrame is true for each new field alone and for none of the existing actions.
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{"waitFor", Action{WaitFor: &WaitFor{}}, true},
		{"hover", Action{Hover: &Hover{}}, true},
		{"dragAndDrop", Action{DragAndDrop: &DragAndDrop{}}, true},
		{"press without find", Action{Press: &Press{}}, true},
		{"click", Action{Click: &Click{}}, false},
		{"execute", Action{Execute: &Execute{}}, false},
		{"no action", Action{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.reachesFrame())
		})
	}
}

func TestRun_newActionsWaitForANavigationInFlight(t *testing.T) {
	// US3-AS9: after a step that started a navigation (a click on a link), each new action is sent once the navigation
	// committed, never to the navigating frame, as the existing element actions are (R19).
	for _, form := range append(slices.Clone(newActionForms), pressWithoutFind) {
		t.Run(form.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.requestNavigation(pw.mainFrame(), "late")
			later(func() { pw.commitNavigation(pw.mainFrame()) })

			mustRun(ctx, t, form.step())

			seen := pw.seen()
			commit, call := slices.Index(seen, "commit page1"), slices.Index(seen, form.call)
			require.NotEqual(t, -1, commit, "no commit in %v", seen)
			require.NotEqual(t, -1, call, "no %q in %v", form.call, seen)
			assert.Less(t, commit, call, "the action is sent once the navigation committed: %v", seen)
			for _, c := range seen {
				assert.NotContains(t, c, " held", "a call was sent to the navigating frame: %v", seen)
			}
		})
	}
}

func TestRun_anExistingFindThatIsAZeroValueStillBeatsAMalformedNewAction(t *testing.T) {
	// US3-AS10, regression note of the review: validateNewActions and performAction must agree on whether find is set.
	// performAction runs find whenever the decoded value is non-nil, so a step with find: 0 or find: false is a find step
	// whose structure is invalid: it reports the find error, not the refusal of the malformed hover beside it; find: ""
	// is a find on an empty CSS selector, which runs and passes, and the hover is not looked at.
	for _, find := range []interface{}{0, false} {
		t.Run(fmt.Sprintf("find: %v", find), func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			_, err := runResult(ctx, t, action(map[string]interface{}{"find": find, "hover": kv()}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid find element structure")
			assert.NotContains(t, err.Error(), "hover")
			assert.Empty(t, newActionCalls(pw.seen()))
		})
	}
	t.Run(`find: ""`, func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		_, err := runResult(ctx, t, action(map[string]interface{}{"find": "", "hover": kv()}))
		require.NoError(t, err, "the empty find runs; the malformed hover beside it is not looked at")
		assert.NotEmpty(t, callsWithPrefix(pw.seen(), "Count"), "find ran")
		assert.Empty(t, newActionCalls(pw.seen()))
	})
}

func TestNewActions_failureOfARoleElementNamesItsNameAndExact(t *testing.T) {
	// US2-AS2, US3-AS1, FR-006: the failure of waitFor, hover, dragAndDrop (from and to) and press with find names a ROLE
	// element by its name and, when set, its exact flag, as describeFind writes them, not only by kind and selector.
	add, remove := fakeRoleName("button", ptr("Add"), false), fakeRoleName("button", ptr("Remove"), true)
	roleAdd := loc("ROLE", "button", "name", "Add")
	roleRemove := loc("ROLE", "button", "name", "Remove", "exact", true)
	tests := []struct {
		name  string
		step  venom.TestStep
		call  string
		wants []string
	}{
		{"waitFor", newAction("waitFor", kv("find", roleAdd)), "WaitFor " + add + " first state=visible", []string{`name "Add"`}},
		{"waitFor exact", newAction("waitFor", kv("find", roleRemove)), "WaitFor " + remove + " first state=visible", []string{`name "Remove" exact`}},
		{"hover", newAction("hover", kv("find", roleAdd)), "Hover " + add + " first", []string{`name "Add"`}},
		{"hover exact", newAction("hover", kv("find", roleRemove)), "Hover " + remove + " first", []string{`name "Remove" exact`}},
		{"dragAndDrop from and to", newAction("dragAndDrop", kv("from", roleAdd, "to", roleRemove)), "DragTo " + add + " first -> " + remove + " first", []string{`name "Add"`, `name "Remove" exact`}},
		{"press with find", newAction("press", kv("key", "Enter", "find", roleAdd)), "Press " + add + " first Enter", []string{`name "Add"`}},
		{"press with find exact", newAction("press", kv("key", "Enter", "find", roleRemove)), "Press " + remove + " first Enter", []string{`name "Remove" exact`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.errs[tt.call] = errors.New("Timeout exceeded: nothing matched")
			_, err := runResult(ctx, t, tt.step)
			require.Error(t, err)
			for _, want := range tt.wants {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestWaitFor_anEmptyStateMeansVisible(t *testing.T) {
	// US2-AS2, FR-006 (characterization, already satisfied): state: "" is the default state, visible; the step passes and
	// the seam receives visible. An unknown state stays refused (TestWaitFor_refusals).
	pw := withFake(t)
	ctx := openFake(t, pw)
	mustRun(ctx, t, newAction("waitFor", kv("find", findCSS(".late"), "state", "")))
	assert.Equal(t, []string{"WaitFor css=.late first state=visible"}, callsWithPrefix(pw.seen(), "WaitFor "))
}
