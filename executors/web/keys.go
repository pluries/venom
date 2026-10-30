package web

import (
	"context"
	"strconv"

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
