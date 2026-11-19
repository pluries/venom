package web

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

// The locator kinds a find element names besides cssLocator and xpathLocator: the values of its locator key.
const (
	roleLocator        = "ROLE"
	textLocator        = "TEXT"
	labelLocator       = "LABEL"
	placeholderLocator = "PLACEHOLDER"
	testIDLocator      = "TESTID"
)

const (
	defaultTestIDAttr = "data-testid"
	keySelector       = "selector"
	keyName           = "name"
	keyExact          = "exact"
)

// locatorKeys are the keys a find element of each new kind takes, besides locator.
var locatorKeys = map[string][]string{
	roleLocator:        {keySelector, keyName, keyExact},
	textLocator:        {keySelector, keyExact},
	labelLocator:       {keySelector, keyExact},
	placeholderLocator: {keySelector, keyExact},
	testIDLocator:      {keySelector},
}

// locatorSpec is a find element once read: its kind (cssLocator, xpathLocator or one of the constants above), its
// selector (the role name for ROLE, the text, the test id), and for ROLE the optional accessible name; exact narrows
// the match of name, text, label and placeholder. An empty name is an unset name.
type locatorSpec struct {
	kind     string
	selector string
	name     string
	exact    bool
}

// parseFind reads a find element of any of the seven kinds.
func parseFind(ctx context.Context, findElement interface{}) (locatorSpec, error) {
	switch find := findElement.(type) {
	case string:
		venom.Warning(ctx, "web - findElement : this find element syntax deprecated and will be not supported in next version")
		return locatorSpec{kind: cssLocator, selector: find}, nil
	case map[string]interface{}:
		return parseFindMap(find)
	}
	return locatorSpec{}, fmt.Errorf("invalid find element structure %v\n (must be a string or a element with selector and locator attribute)", findElement)
}

func parseFindMap(find map[string]interface{}) (locatorSpec, error) {
	selector, ok := find[keySelector].(string)
	if !ok {
		return locatorSpec{}, fmt.Errorf("invalid find element selector %v (must be a string)", find[keySelector])
	}
	locator, isString := find["locator"].(string)
	switch {
	case !isString:
	case locator == "CSS":
		return locatorSpec{kind: cssLocator, selector: selector}, nil
	case locator == "XPATH":
		return locatorSpec{kind: xpathLocator, selector: selector}, nil
	case locatorKeys[locator] != nil:
		return parseNewKind(locator, selector, find)
	}
	return locatorSpec{}, fmt.Errorf("invalid find element locator '%s' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)", find["locator"])
}

// parseNewKind reads the keys of a ROLE, TEXT, LABEL, PLACEHOLDER or TESTID find element.
func parseNewKind(kind, selector string, find map[string]interface{}) (locatorSpec, error) {
	if err := checkKeys(kind, find); err != nil {
		return locatorSpec{}, err
	}
	spec := locatorSpec{kind: kind, selector: selector}
	if err := checkSelector(kind, selector); err != nil {
		return locatorSpec{}, err
	}
	if v, present := find[keyName]; present {
		name, ok := v.(string)
		if !ok {
			return locatorSpec{}, fmt.Errorf("invalid find element name %v (must be a string)", v)
		}
		spec.name = name
	}
	if v, present := find[keyExact]; present {
		exact, ok := v.(bool)
		if !ok {
			return locatorSpec{}, fmt.Errorf("invalid find element exact %v (must be a boolean)", v)
		}
		spec.exact = exact
	}
	return spec, nil
}

// checkKeys refuses a key the kind does not take, naming it and what the kind does take.
func checkKeys(kind string, find map[string]interface{}) error {
	allowed := locatorKeys[kind]
	for _, key := range slices.Sorted(maps.Keys(find)) {
		if key == "locator" || slices.Contains(allowed, key) {
			continue
		}
		switch key {
		case keyName:
			return fmt.Errorf("invalid find element name: only a ROLE locator takes a name, not %s", kind)
		case keyExact:
			return fmt.Errorf("invalid find element exact: only a ROLE, TEXT, LABEL or PLACEHOLDER locator takes exact, not %s", kind)
		}
		return fmt.Errorf("invalid find element key '%s' for a %s locator (must be locator, %s)", key, kind, strings.Join(allowed, ", "))
	}
	return nil
}

// checkSelector refuses a selector the kind cannot match: a word that is not an ARIA role, an empty text, a test id
// with a NUL.
func checkSelector(kind, selector string) error {
	switch {
	case kind == roleLocator:
		if !slices.Contains(ariaRoleNames, selector) {
			return fmt.Errorf("invalid find element selector '%s' (must be one of the ARIA roles: %s)", selector, strings.Join(ariaRoleNames, ", "))
		}
	case selector == "":
		return fmt.Errorf("invalid find element selector '' (must not be empty for a %s locator)", kind)
	case kind == testIDLocator && strings.ContainsRune(selector, 0):
		return fmt.Errorf("invalid find element selector %q (a %s test id cannot hold a NUL character)", selector, kind)
	}
	return nil
}

// resolveLocator is every element spec matches in f; testIDAttr is the attribute of a TESTID locator, empty meaning
// data-testid.
func resolveLocator(f frame, spec locatorSpec, testIDAttr string) element {
	switch spec.kind {
	case roleLocator:
		var name *string
		if spec.name != "" {
			name = &spec.name
		}
		return f.getByRole(playwright.AriaRole(spec.selector), name, spec.exact)
	case textLocator:
		return f.getByText(spec.selector, spec.exact)
	case labelLocator:
		return f.getByLabel(spec.selector, spec.exact)
	case placeholderLocator:
		return f.getByPlaceholder(spec.selector, spec.exact)
	case testIDLocator:
		if testIDAttr == "" {
			testIDAttr = defaultTestIDAttr
		}
		return f.locator("css=" + testIDSelector(testIDAttr, spec.selector))
	case xpathLocator:
		return f.locator("xpath=" + spec.selector)
	}
	return f.locator("css=" + spec.selector)
}

// testIDSelector is the CSS attribute selector `[attr="value"]` that matches value exactly: attr and value are
// escaped, so neither can end the selector.
func testIDSelector(attr, value string) string {
	return "[" + cssIdent(attr) + `="` + cssString(value) + `"]`
}

// cssIdent escapes every character a CSS identifier cannot hold as itself.
func cssIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !isIdentChar(r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isIdentChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-'
}

// cssString escapes s for the inside of a double-quoted CSS string: a control character becomes its hex escape and the
// space that ends it.
func cssString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			b.WriteString("\\" + strconv.FormatInt(int64(r), 16) + " ")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// describeFind names an element in a message.
func describeFind(findElement interface{}) string {
	find, ok := findElement.(map[string]interface{})
	if !ok {
		return fmt.Sprint(findElement)
	}
	kind := fmt.Sprint(find["locator"])
	text := fmt.Sprintf("%v %q", find["locator"], find[keySelector])
	if name, ok := find[keyName].(string); ok && name != "" && kind == roleLocator {
		text += fmt.Sprintf(" %s %q", keyName, name)
	}
	if exact, ok := find[keyExact].(bool); ok && exact && slices.Contains(locatorKeys[kind], keyExact) {
		text += " " + keyExact
	}
	return text
}
