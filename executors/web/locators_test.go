package web

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of the locators of spec 015 (docs/specs/015-web-recipe-actions, US1): the find element of seven kinds
// read by parseFind and resolved by resolveLocator, on the fake of the seam. The matching itself is Chromium's, proven
// by web_recipe_actions_playwright_test.go. Each test names its scenarios (US1-ASn).

// unknownLocatorMessage is D-013's text for a locator that is none of the seven kinds.
const unknownLocatorMessage = "invalid find element locator '%s' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)"

// refusedFinds are find elements a step must refuse before any browser action, each with the fragments its message
// holds: the field at fault and the allowed values (US1-AS10, AS11, AS12). The role names are checked against the
// binding's list in roles_test.go; here "button", "alert" and "tree" stand for the list.
var refusedFinds = []struct {
	name  string
	find  interface{}
	parts []string
}{
	{"ROLE without a selector", map[string]interface{}{"locator": "ROLE"}, []string{"selector"}},
	{"TEXT with an empty selector", loc("TEXT", ""), []string{"selector"}},
	{"TESTID with a selector that is not a string", map[string]interface{}{"locator": "TESTID", "selector": 42}, []string{"selector"}},
	{"name on TEXT", loc("TEXT", "Total", "name", "x"), []string{"name", "ROLE"}},
	{"name on LABEL", loc("LABEL", "Email", "name", "x"), []string{"name", "ROLE"}},
	{"name on PLACEHOLDER", loc("PLACEHOLDER", "Search", "name", "x"), []string{"name", "ROLE"}},
	{"name on TESTID", loc("TESTID", "item-input", "name", "x"), []string{"name", "ROLE"}},
	{"exact on TESTID", loc("TESTID", "item-input", "exact", true), []string{"exact", "ROLE", "TEXT", "LABEL", "PLACEHOLDER"}},
	{"a name that is not a string", loc("ROLE", "button", "name", 42), []string{"name"}},
	{"an exact that is not a bool", loc("TEXT", "Total", "exact", "yes"), []string{"exact"}},
	{"an unknown key beside ROLE", loc("ROLE", "button", "bogus", 1), []string{"bogus", "selector", "name", "exact"}},
	{"an unknown key beside TESTID", loc("TESTID", "item-input", "bogus", 1), []string{"bogus", "selector"}},
	{"a NUL in a TESTID value", loc("TESTID", "a\x00b"), []string{"NUL"}},
	{"a role in the wrong case", loc("ROLE", "Button"), []string{"selector", "button", "alert", "tree"}},
	{"a word that is not a role", loc("ROLE", "banana"), []string{"selector", "button", "alert", "tree"}},
	{"an unknown locator", loc("ID", "#a"), []string{"ID", "CSS", "XPATH", "ROLE", "TEXT", "LABEL", "PLACEHOLDER", "TESTID"}},
}

func TestParseFind_newKinds(t *testing.T) {
	// US1-AS1 to AS5, AS7: each new kind is read into its spec, with name and exact where the kind takes them.
	tests := []struct {
		name string
		find interface{}
		want locatorSpec
	}{
		{"ROLE with a name", loc("ROLE", "button", "name", "Add"), locatorSpec{kind: roleLocator, selector: "button", name: "Add"}},
		{"ROLE without a name", loc("ROLE", "button"), locatorSpec{kind: roleLocator, selector: "button"}},
		{"ROLE with an empty name counts as unset", loc("ROLE", "button", "name", ""), locatorSpec{kind: roleLocator, selector: "button"}},
		{"ROLE exact", loc("ROLE", "button", "name", "Add", "exact", true), locatorSpec{kind: roleLocator, selector: "button", name: "Add", exact: true}},
		{"TEXT", loc("TEXT", "Total: 3"), locatorSpec{kind: textLocator, selector: "Total: 3"}},
		{"TEXT exact", loc("TEXT", "Total: 3", "exact", true), locatorSpec{kind: textLocator, selector: "Total: 3", exact: true}},
		{"LABEL", loc("LABEL", "Email"), locatorSpec{kind: labelLocator, selector: "Email"}},
		{"LABEL exact", loc("LABEL", "Email", "exact", true), locatorSpec{kind: labelLocator, selector: "Email", exact: true}},
		{"PLACEHOLDER", loc("PLACEHOLDER", "Search"), locatorSpec{kind: placeholderLocator, selector: "Search"}},
		{"PLACEHOLDER exact", loc("PLACEHOLDER", "Search", "exact", true), locatorSpec{kind: placeholderLocator, selector: "Search", exact: true}},
		{"TESTID", loc("TESTID", "item-input"), locatorSpec{kind: testIDLocator, selector: "item-input"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFind(context.Background(), tt.find)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFind_cssAndXpathAreReadAsToday(t *testing.T) {
	// US1-AS9: extra keys beside CSS and XPATH (name, exact, an unknown key) are ignored and nothing is refused; the
	// bare string is still CSS.
	venom.InitTestLogger(t)
	tests := []struct {
		name string
		find interface{}
		want locatorSpec
	}{
		{"string", "#a", locatorSpec{kind: cssLocator, selector: "#a"}},
		{"CSS", loc("CSS", "#a"), locatorSpec{kind: cssLocator, selector: "#a"}},
		{"CSS with extra keys", loc("CSS", "#a", "name", "x", "exact", true, "bogus", 1), locatorSpec{kind: cssLocator, selector: "#a"}},
		{"XPATH", loc("XPATH", "//a"), locatorSpec{kind: xpathLocator, selector: "//a"}},
		{"XPATH with extra keys", loc("XPATH", "//a", "name", "x", "exact", "yes", "bogus", 1), locatorSpec{kind: xpathLocator, selector: "//a"}},
		{"CSS with an empty selector", loc("CSS", ""), locatorSpec{kind: cssLocator}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFind(context.Background(), tt.find)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFind_refusals(t *testing.T) {
	// US1-AS10, AS11, AS12: a malformed find of a new kind is refused, naming the field and the allowed values.
	venom.InitTestLogger(t)
	for _, tt := range refusedFinds {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFind(context.Background(), tt.find)
			require.Error(t, err)
			for _, part := range tt.parts {
				assert.Contains(t, err.Error(), part)
			}
			assert.Equal(t, locatorSpec{}, got)
		})
	}
}

func TestParseFind_unknownLocatorMessage(t *testing.T) {
	// US1-AS10, D-013: the message lists the seven kinds; the locator is case-sensitive and the map form has no default.
	venom.InitTestLogger(t)
	tests := []struct {
		name string
		find map[string]interface{}
		got  interface{}
	}{
		{"an unknown kind", loc("ID", "#a"), "ID"},
		{"lower case css", loc("css", "#a"), "css"},
		{"lower case role", loc("role", "button"), "role"},
		{"a missing locator", map[string]interface{}{"selector": "#a"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFind(context.Background(), tt.find)
			require.EqualError(t, err, fmt.Sprintf(unknownLocatorMessage, tt.got))
		})
	}
}

func TestRun_refusedFindsMakeNoBrowserCall(t *testing.T) {
	// US1-AS10, AS11, AS12: the step fails with the message of the refusal, and the only call the browser receives is
	// the read of the page for the failure dump.
	for _, tt := range refusedFinds {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			_, err := runResult(ctx, t, findStep(tt.find))
			require.Error(t, err)
			for _, part := range tt.parts {
				assert.Contains(t, err.Error(), part)
			}
			assert.Equal(t, []string{"Content page1"}, pw.seen())
		})
	}
}

func TestResolveLocator_theCallOfEachKind(t *testing.T) {
	// US1-AS1 to AS5, AS7: the seam receives the call of the kind, with the role, the name or the text and exact; a
	// TESTID is the CSS attribute selector of the attribute, data-testid when none is given (US1-AS6).
	tests := []struct {
		name string
		spec locatorSpec
		attr string
		want string
	}{
		{"ROLE with a name", locatorSpec{kind: roleLocator, selector: "button", name: "Add"}, "", fakeRoleName("button", ptr("Add"), false)},
		{"ROLE exact", locatorSpec{kind: roleLocator, selector: "button", name: "Add", exact: true}, "", fakeRoleName("button", ptr("Add"), true)},
		{"ROLE without a name sends no name", locatorSpec{kind: roleLocator, selector: "button"}, "", fakeRoleName("button", nil, false)},
		{"TEXT", locatorSpec{kind: textLocator, selector: "Total: 3"}, "", fakeTextName("getByText", "Total: 3", false)},
		{"TEXT exact", locatorSpec{kind: textLocator, selector: "Total: 3", exact: true}, "", fakeTextName("getByText", "Total: 3", true)},
		{"LABEL", locatorSpec{kind: labelLocator, selector: "Email"}, "", fakeTextName("getByLabel", "Email", false)},
		{"LABEL exact", locatorSpec{kind: labelLocator, selector: "Email", exact: true}, "", fakeTextName("getByLabel", "Email", true)},
		{"PLACEHOLDER", locatorSpec{kind: placeholderLocator, selector: "Search"}, "", fakeTextName("getByPlaceholder", "Search", false)},
		{"PLACEHOLDER exact", locatorSpec{kind: placeholderLocator, selector: "Search", exact: true}, "", fakeTextName("getByPlaceholder", "Search", true)},
		{"TESTID, default attribute", locatorSpec{kind: testIDLocator, selector: "item-input"}, "", `css=[data-testid="item-input"]`},
		{"TESTID, the attribute of the suite", locatorSpec{kind: testIDLocator, selector: "item-input"}, "data-test", `css=[data-test="item-input"]`},
		{"CSS", locatorSpec{kind: cssLocator, selector: ".item"}, "", "css=.item"},
		{"XPATH", locatorSpec{kind: xpathLocator, selector: "//div"}, "", "xpath=//div"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			got := resolveLocator(&fakeFrame{pw: pw, name: "page1"}, tt.spec, tt.attr)
			elt, ok := got.(*fakeElement)
			require.Truef(t, ok, "want a *fakeElement, got %T", got)
			assert.Equal(t, tt.want, elt.name)
		})
	}
}

// hostileValues are texts no new locator may read as a selector, an engine prefix or a regular expression, and texts
// with surrounding and repeated whitespace (US1-AS12).
var hostileValues = []string{
	`say "hi"`, `it's`, `back\slash`, `a]b`, `/x/`, `text=x`, `css=a`, `internal:text="x"`, `x"] , [data-testid="y`,
	" a  b ", "a\tb", "line1\nline2",
}

func TestResolveLocator_valuesReachTheSeamAsGiven(t *testing.T) {
	// US1-AS12: the four kinds that use the binding's GetBy* calls pass the value as a plain Go string, untouched: no
	// selector is built from it, whatever it holds. The whitespace is Playwright's to read.
	for _, v := range hostileValues {
		for _, exact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q exact=%t", v, exact), func(t *testing.T) {
				pw := withFake(t)
				f := &fakeFrame{pw: pw, name: "page1"}
				want := map[string]string{
					roleLocator:        fakeRoleName("button", ptr(v), exact),
					textLocator:        fakeTextName("getByText", v, exact),
					labelLocator:       fakeTextName("getByLabel", v, exact),
					placeholderLocator: fakeTextName("getByPlaceholder", v, exact),
				}
				for kind, name := range want {
					spec := locatorSpec{kind: kind, selector: v, exact: exact}
					if kind == roleLocator {
						spec = locatorSpec{kind: kind, selector: "button", name: v, exact: exact}
					}
					elt, ok := resolveLocator(f, spec, "").(*fakeElement)
					require.True(t, ok, kind)
					assert.Equal(t, name, elt.name, kind)
				}
			})
		}
	}
}

func TestTestIDSelector(t *testing.T) {
	// US1-AS12, D-009, D-014: the selector matches the value exactly (spaces included) and the value is a CSS string:
	// a quote, a backslash or a control character is escaped, a ] or a / is not special inside it. The attribute name
	// is escaped as a CSS identifier, so a . or a : in it stays part of the name.
	tests := []struct {
		name        string
		attr, value string
		want        string
	}{
		{"plain", "data-testid", "item-input", `[data-testid="item-input"]`},
		{"double quote", "data-testid", `a"b`, `[data-testid="a\"b"]`},
		{"single quote", "data-testid", `a'b`, `[data-testid="a'b"]`},
		{"backslash", "data-testid", `a\b`, `[data-testid="a\\b"]`},
		{"closing bracket", "data-testid", `a]b`, `[data-testid="a]b"]`},
		{"slashes", "data-testid", `/x/`, `[data-testid="/x/"]`},
		{"an injected selector", "data-testid", `x"] , [data-testid="y`, `[data-testid="x\"] , [data-testid=\"y"]`},
		{"leading, trailing and repeated spaces are kept", "data-testid", " a  b ", `[data-testid=" a  b "]`},
		{"newline", "data-testid", "a\nb", `[data-testid="a\a b"]`},
		{"tab", "data-testid", "a\tb", `[data-testid="a\9 b"]`},
		{"unit separator", "data-testid", "a\x1fb", `[data-testid="a\1f b"]`},
		{"another attribute", "data-test", "item-input", `[data-test="item-input"]`},
		{"a dot in the attribute", "data-a.b", "v", `[data-a\.b="v"]`},
		{"a colon in the attribute", "xlink:href", "v", `[xlink\:href="v"]`},
		{"an underscore start", "_qa", "v", `[_qa="v"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, testIDSelector(tt.attr, tt.value))
		})
	}
}

// elementFields are the actions that take an element, as a suite writes them, with the first call each sends to
// the element it names.
var elementFields = []struct {
	name  string
	step  func(find interface{}) venom.TestStep
	first func(element string) string
}{
	{"click", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"click": map[string]interface{}{"find": f}})
	}, func(e string) string { return "Click " + e + " first" }},
	{"fill", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"fill": []interface{}{map[string]interface{}{"find": f, "text": "x"}}})
	}, func(e string) string { return "Evaluate " + e + " first caretToEnd" }},
	{"find", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"find": f})
	}, func(e string) string { return "Count " + e }},
	{"select", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"select": map[string]interface{}{"find": f, "text": "x"}})
	}, func(e string) string { return "SelectOption " + e + " first label=x" }},
	{"uploadFile", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"uploadFile": map[string]interface{}{"find": f, "files": []interface{}{"a.txt"}}})
	}, func(e string) string { return "SetInputFiles " + e + " first [a.txt]" }},
	{"selectFrame", func(f interface{}) venom.TestStep {
		return action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": f}})
	}, func(e string) string { return "ContentFrame " + e + " first" }},
}

// newKinds are the five new locator kinds, as a suite writes them, with the name of the element each resolves to on
// the fake.
var newKinds = []struct {
	name    string
	find    map[string]interface{}
	element string
}{
	{"ROLE", loc("ROLE", "button", "name", "Add"), fakeRoleName("button", ptr("Add"), false)},
	{"TEXT", loc("TEXT", "Total: 3"), fakeTextName("getByText", "Total: 3", false)},
	{"LABEL", loc("LABEL", "Email"), fakeTextName("getByLabel", "Email", false)},
	{"PLACEHOLDER", loc("PLACEHOLDER", "Search"), fakeTextName("getByPlaceholder", "Search", false)},
	{"TESTID", loc("TESTID", "item-input"), `css=[data-testid="item-input"]`},
}

func TestRun_everyFieldTakesEveryKind(t *testing.T) {
	// FR-002, US1-AS3, AS4, AS7: click, fill, find, select, uploadFile and selectFrame each resolve every new kind, and
	// act on the first match.
	for _, kind := range newKinds {
		for _, field := range elementFields {
			t.Run(field.name+"/"+kind.name, func(t *testing.T) {
				pw := withFake(t)
				ctx := openFake(t, pw)
				pw.nodes[kind.element] = []fakeNode{{text: "x", frame: true}}
				mustRun(ctx, t, field.step(kind.find))
				require.NotEmpty(t, pw.seen())
				assert.Equal(t, field.first(kind.element), pw.seen()[0])
			})
		}
	}
}

func TestRun_findByKind(t *testing.T) {
	// US1-AS1, AS2, AS5: a find by role, text or test id counts the matches and reads the first.
	tests := []struct {
		name    string
		find    map[string]interface{}
		element string
		want    found
	}{
		{"ROLE", loc("ROLE", "button", "name", "Add"), fakeRoleName("button", ptr("Add"), false), found{Find: 1, Text: "Add"}},
		{"TEXT", loc("TEXT", "Total: 3"), fakeTextName("getByText", "Total: 3", false), found{Find: 1, Text: "Total: 3"}},
		{"TESTID", loc("TESTID", "item-input"), `css=[data-testid="item-input"]`, found{Find: 1, Text: "", Value: "v"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes[tt.element] = []fakeNode{{text: tt.want.Text, value: tt.want.Value}}
			assert.Equal(t, tt.want, foundOf(mustRun(ctx, t, findStep(tt.find))))
			assert.Equal(t, []string{"Count " + tt.element}, callsWithPrefix(pw.seen(), "Count"))
		})
	}
}

func TestRun_cssAndXpathIgnoreExtraKeys(t *testing.T) {
	// US1-AS9 (characterization, green before the change): a CSS or XPATH find with extra keys behaves as it does today.
	tests := []struct {
		name string
		find map[string]interface{}
	}{
		{"CSS", loc("CSS", ".item", "name", "x", "exact", true, "bogus", 1)},
		{"XPATH", loc("XPATH", "//div[@class='item']", "name", "x", "exact", "yes", "bogus", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			assert.Equal(t, found{Find: 1, Text: "first", Value: "v1"}, foundOf(mustRun(ctx, t, findStep(tt.find))))
		})
	}
}

func TestRun_selectFrameThenNewKinds(t *testing.T) {
	// US1-AS13: after selectFrame, a TEXT or ROLE locator resolves in the frame, not in the parent page.
	tests := []struct {
		name    string
		find    map[string]interface{}
		element string
	}{
		{"TEXT", loc("TEXT", "Total: 3"), fakeTextName("getByText", "Total: 3", false)},
		{"ROLE", loc("ROLE", "button", "name", "Go"), fakeRoleName("button", ptr("Go"), false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes["css=#outer"] = []fakeNode{{frame: true}}
			pw.nodes["css=#outer >> "+tt.element] = []fakeNode{{text: "inside"}}
			pw.nodes[tt.element] = []fakeNode{{text: "outside"}}
			mustRun(ctx, t, action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": findCSS("#outer")}}))
			pw.reset()
			got := mustRun(ctx, t, findStep(tt.find))
			assert.Equal(t, "inside", got.Text)
			assert.Equal(t, []string{"Count css=#outer >> " + tt.element}, callsWithPrefix(pw.seen(), "Count"))
		})
	}
}

func TestRun_selectFrameOnAnElementThatIsNotAFrame(t *testing.T) {
	// US1-AS13: a LABEL, TEXT, PLACEHOLDER or ROLE locator that resolves to an element that is not a frame fails with
	// the existing "not a frame" message. (A locator that matches nothing times out instead.)
	for _, kind := range newKinds[:4] {
		t.Run(kind.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.nodes[kind.element] = []fakeNode{{text: "not a frame"}}
			_, err := runResult(ctx, t, action(map[string]interface{}{"selectFrame": map[string]interface{}{"find": kind.find}}))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no such frame")
			assert.Contains(t, err.Error(), "is not a frame")
		})
	}
}

func TestRun_testIDAttributeIsPerSession(t *testing.T) {
	// US1-AS15, AS6: two suites of one process, one with web.testIdAttribute and one without, each read their own
	// attribute, however the steps interleave.
	pw := withFake(t)
	t.Chdir(t.TempDir())
	withAttr, err := setupFake(t, map[string]any{"testIdAttribute": "data-test"})
	require.NoError(t, err)
	without, err := setupFake(t, nil)
	require.NoError(t, err)
	pw.reset()
	step := findStep(loc("TESTID", "item-input"))
	for _, ctx := range []context.Context{withAttr, without, withAttr, without} {
		mustRun(ctx, t, step)
	}
	assert.Equal(t, []string{
		`Count css=[data-test="item-input"]`,
		`Count css=[data-testid="item-input"]`,
		`Count css=[data-test="item-input"]`,
		`Count css=[data-testid="item-input"]`,
	}, callsWithPrefix(pw.seen(), "Count"))
}

func TestRun_aSessionWithoutAnAttributeReadsDataTestid(t *testing.T) {
	// US1-AS6: a WebContext that Setup did not fill in (an empty attribute) reads data-testid.
	pw := withFake(t)
	ctx := openFake(t, pw)
	getWebCtx(ctx).testIDAttr = ""
	mustRun(ctx, t, findStep(loc("TESTID", "item-input")))
	assert.Equal(t, []string{`Count css=[data-testid="item-input"]`}, callsWithPrefix(pw.seen(), "Count"))
}

func TestRun_testIDValueIsEscapedForTheSessionAttribute(t *testing.T) {
	// US1-AS12, D-009: a hostile value and an attribute with a . and a : reach the seam as one escaped CSS selector.
	pw := withFake(t)
	ctx := openFake(t, pw)
	getWebCtx(ctx).testIDAttr = "data-a.b:c"
	mustRun(ctx, t, findStep(loc("TESTID", `x"] , [data-testid="y`)))
	assert.Equal(t, []string{`Count css=[data-a\.b\:c="x\"] , [data-testid=\"y"]`}, callsWithPrefix(pw.seen(), "Count"))
}

func TestDescribeFind_namesTheKindTheSelectorTheNameAndExact(t *testing.T) {
	// US2-AS2, US3-AS1, FR-006: a message names an element by its kind and selector, and for ROLE its name, and where the
	// kind takes it the exact flag, as the fragment ` exact` closing the text. An empty or absent name adds nothing;
	// CSS and XPATH ignore the extra keys, as parseFind does; a bare string is shown as written.
	tests := []struct {
		name string
		find interface{}
		want string
	}{
		{"a bare CSS string", "#a", "#a"},
		{"CSS", loc("CSS", "#a"), `CSS "#a"`},
		{"CSS with extra keys", loc("CSS", "#a", "name", "x", "exact", true), `CSS "#a"`},
		{"XPATH", loc("XPATH", "//a"), `XPATH "//a"`},
		{"ROLE without a name", loc("ROLE", "button"), `ROLE "button"`},
		{"ROLE with an empty name", loc("ROLE", "button", "name", ""), `ROLE "button"`},
		{"ROLE with a name", loc("ROLE", "button", "name", "Add"), `ROLE "button" name "Add"`},
		{"ROLE with a name, exact", loc("ROLE", "button", "name", "Add", "exact", true), `ROLE "button" name "Add" exact`},
		{"ROLE with a name, exact false", loc("ROLE", "button", "name", "Add", "exact", false), `ROLE "button" name "Add"`},
		{"ROLE with a name that holds a quote", loc("ROLE", "button", "name", `say "hi"`), `ROLE "button" name "say \"hi\""`},
		{"TEXT", loc("TEXT", "Total: 3"), `TEXT "Total: 3"`},
		{"TEXT exact", loc("TEXT", "Total: 3", "exact", true), `TEXT "Total: 3" exact`},
		{"LABEL", loc("LABEL", "Email"), `LABEL "Email"`},
		{"LABEL exact", loc("LABEL", "Email", "exact", true), `LABEL "Email" exact`},
		{"PLACEHOLDER", loc("PLACEHOLDER", "Search"), `PLACEHOLDER "Search"`},
		{"PLACEHOLDER exact", loc("PLACEHOLDER", "Search", "exact", true), `PLACEHOLDER "Search" exact`},
		{"TESTID", loc("TESTID", "item-input"), `TESTID "item-input"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, describeFind(tt.find))
		})
	}
}

func TestRun_roleWithAnEmptyNameSendsNoNameFilter(t *testing.T) {
	// FR-006 (characterization, already satisfied): name: "" on ROLE is no name filter; the seam receives getByRole with
	// a nil name, not an empty one, through a step.
	pw := withFake(t)
	ctx := openFake(t, pw)
	mustRun(ctx, t, newAction("hover", kv("find", loc("ROLE", "button", "name", ""))))
	assert.Equal(t, []string{"Hover " + fakeRoleName("button", nil, false) + " first"}, callsWithPrefix(pw.seen(), "Hover"))
}
