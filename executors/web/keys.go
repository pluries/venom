package web

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ovh/venom"
)

// Keys map returning key code by its name (WebDriver key codes; kept for library users, fill uses playwrightKeys)
var Keys = map[string]string{
	"NULL":            "\uE000",
	"CANCEL":          "\uE001",
	"HELP":            "\uE002",
	"BACK_SPACE":      "\uE003",
	"TAB":             "\uE004",
	"CLEAR":           "\uE005",
	"RETURN":          "\uE006",
	"ENTER":           "\uE007",
	"SHIFT":           "\uE008",
	"LEFT_SHIFT":      "\uE008",
	"CONTROL":         "\uE009",
	"LEFT_CONTROL":    "\uE009",
	"ALT":             "\uE00A",
	"LEFT_ALT":        "\uE00A",
	"PAUSE":           "\uE00B",
	"ESCAPE":          "\uE00C",
	"SPACE":           "\uE00D",
	"PAGE_UP":         "\uE00E",
	"PAGE_DOWN":       "\uE00F",
	"END":             "\uE010",
	"HOME":            "\uE011",
	"LEFT":            "\uE012",
	"ARROW_LEFT":      "\uE012",
	"UP":              "\uE013",
	"ARROW_UP":        "\uE013",
	"RIGHT":           "\uE014",
	"ARROW_RIGHT":     "\uE014",
	"DOWN":            "\uE015",
	"ARROW_DOWN":      "\uE015",
	"INSERT":          "\uE016",
	"DELETE":          "\uE017",
	"SEMICOLON":       "\uE018",
	"EQUALS":          "\uE019",
	"NUMPAD0":         "\uE01A",
	"NUMPAD1":         "\uE01B",
	"NUMPAD2":         "\uE01C",
	"NUMPAD3":         "\uE01D",
	"NUMPAD4":         "\uE01E",
	"NUMPAD5":         "\uE01F",
	"NUMPAD6":         "\uE020",
	"NUMPAD7":         "\uE021",
	"NUMPAD8":         "\uE022",
	"NUMPAD9":         "\uE023",
	"MULTIPLY":        "\uE024",
	"ADD":             "\uE025",
	"SEPARATOR":       "\uE026",
	"SUBTRACT":        "\uE027",
	"DECIMAL":         "\uE028",
	"DIVIDE":          "\uE029",
	"F1":              "\uE031",
	"F2":              "\uE032",
	"F3":              "\uE033",
	"F4":              "\uE034",
	"F5":              "\uE035",
	"F6":              "\uE036",
	"F7":              "\uE037",
	"F8":              "\uE038",
	"F9":              "\uE039",
	"F10":             "\uE03A",
	"F11":             "\uE03B",
	"F12":             "\uE03C",
	"META":            "\uE03D",
	"COMMAND":         "\uE03D",
	"ZENKAKU_HANKAKU": "\uE040",
}

// playwrightKeys names each key of Keys for Playwright's Press (R9). Playwright's layout models the numpad with NumLock
// off (Numpad5 is the Clear key, Numpad7 is Home), so the numpad digits and the decimal point map onto the keys that
// type the same character. NULL, CANCEL, HELP and ZENKAKU_HANKAKU have no Playwright key.
var playwrightKeys = playwrightKeyNames()

func playwrightKeyNames() map[string]string {
	names := map[string]string{
		"BACK_SPACE":   "Backspace",
		"TAB":          "Tab",
		"CLEAR":        "Numpad5",
		"RETURN":       "Enter",
		"ENTER":        "Enter",
		"SHIFT":        "Shift",
		"LEFT_SHIFT":   "Shift",
		"CONTROL":      "Control",
		"LEFT_CONTROL": "Control",
		"ALT":          "Alt",
		"LEFT_ALT":     "Alt",
		"PAUSE":        "Pause",
		"ESCAPE":       "Escape",
		"SPACE":        "Space",
		"PAGE_UP":      "PageUp",
		"PAGE_DOWN":    "PageDown",
		"END":          "End",
		"HOME":         "Home",
		"LEFT":         "ArrowLeft",
		"ARROW_LEFT":   "ArrowLeft",
		"UP":           "ArrowUp",
		"ARROW_UP":     "ArrowUp",
		"RIGHT":        "ArrowRight",
		"ARROW_RIGHT":  "ArrowRight",
		"DOWN":         "ArrowDown",
		"ARROW_DOWN":   "ArrowDown",
		"INSERT":       "Insert",
		"DELETE":       "Delete",
		"SEMICOLON":    "Semicolon",
		"EQUALS":       "Equal",
		"NUMPAD0":      "0",
		"NUMPAD1":      "1",
		"NUMPAD2":      "2",
		"NUMPAD3":      "3",
		"NUMPAD4":      "4",
		"NUMPAD5":      "5",
		"NUMPAD6":      "6",
		"NUMPAD7":      "7",
		"NUMPAD8":      "8",
		"NUMPAD9":      "9",
		"MULTIPLY":     "NumpadMultiply",
		"ADD":          "NumpadAdd",
		"SEPARATOR":    "Comma",
		"SUBTRACT":     "NumpadSubtract",
		"DECIMAL":      "Period",
		"DIVIDE":       "NumpadDivide",
		"META":         "Meta",
		"COMMAND":      "Meta",
	}
	for i := 1; i <= 12; i++ { // F1 to F12 have the same name in both
		names["F"+strconv.Itoa(i)] = "F" + strconv.Itoa(i)
	}
	return names
}

// playwrightKey is the Playwright name of a key of Keys. ok is false for a name fill does not send: an unknown one, as
// before, or a WebDriver key Playwright has no key for, which logs a warning.
func playwrightKey(ctx context.Context, name string) (key string, ok bool) {
	if key, ok = playwrightKeys[name]; ok {
		return key, true
	}
	if _, known := Keys[name]; known {
		venom.Warning(ctx, "web - fill: the key %s has no Playwright equivalent, it is not sent", name)
	}
	return "", false
}

// playwrightKeyList is the table of the key names Playwright's keyboard accepts (D-011), the names parsePress takes
// alone or as the parts of a chord: every value of playwrightKeys and the names only Playwright has, each once.
func playwrightKeyList() []string {
	names := slices.Collect(maps.Values(playwrightKeys))
	names = append(names,
		"Backquote", "Minus", "Backslash", "BracketLeft", "BracketRight", "Quote", "Slash",
		"ControlOrMeta", "NumpadDecimal", "NumpadEnter",
	)
	for i := 0; i <= 9; i++ {
		d := strconv.Itoa(i)
		names = append(names, d, "Digit"+d, "Numpad"+d)
	}
	for c := 'A'; c <= 'Z'; c++ {
		names = append(names, "Key"+string(c))
	}
	slices.Sort(names)
	return slices.Compact(names)
}

var playwrightKeyNameSet = playwrightKeyList()

// parsePress is the Playwright key or chord a press action sends for key; it refuses an empty key, a text, and a name
// Playwright does not know, naming it.
func parsePress(_ context.Context, key string) (string, error) {
	parts := chordParts(key)
	if len(parts) == 1 {
		return singleKey(key, parts[0])
	}
	for _, part := range parts {
		if !isPlaywrightKey(part) {
			return "", pressRefusal(key, fmt.Sprintf("the part %s is not a Playwright key name or a one-character key, and a chord takes only those", describeValue(part)))
		}
	}
	return key, nil
}

// chordParts splits key on "+"; a "+" that ends the key after another "+", or is the whole key, is the plus key.
func chordParts(key string) []string {
	if key == "+" {
		return []string{key}
	}
	if head, ok := strings.CutSuffix(key, "++"); ok {
		return append(strings.Split(head, "+"), "+")
	}
	return strings.Split(key, "+")
}

// isPlaywrightKey is a name of the table or a one-character printable key.
func isPlaywrightKey(part string) bool {
	if slices.Contains(playwrightKeyNameSet, part) {
		return true
	}
	r, size := utf8.DecodeRuneInString(part)
	return size > 0 && size == len(part) && unicode.IsPrint(r)
}

// singleKey is the key a press of one part sends: a Playwright key as written, a WebDriver name through playwrightKeys.
func singleKey(key, part string) (string, error) {
	if isPlaywrightKey(part) {
		return part, nil
	}
	if mapped, ok := playwrightKeys[part]; ok {
		return mapped, nil
	}
	if _, known := Keys[part]; known {
		return "", pressRefusal(key, describeValue(part)+" has no Playwright equivalent")
	}
	return "", pressRefusal(key, "not a key name")
}

// keyNameLike matches a value that reads as a key name: a refusal may name it. Any other value may be a secret put in
// press by mistake ({{.password}}), so a refusal gives its length only.
var keyNameLike = regexp.MustCompile(`^[A-Za-z0-9_+]{1,20}$`)

// describeValue is s quoted when it reads as a key name (or is empty), else only how long it is.
func describeValue(s string) string {
	if s == "" || keyNameLike.MatchString(s) {
		return strconv.Quote(s)
	}
	return fmt.Sprintf("(a text of %d characters)", utf8.RuneCountInString(s))
}

func pressRefusal(key, reason string) error {
	return fmt.Errorf("press: invalid 'key' %s: %s; press takes one key or one chord "+
		"(a Playwright name such as Enter or a one-character key, alone or joined by + as in Control+A)", describeValue(key), reason)
}
