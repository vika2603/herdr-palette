package ui

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Toggle is every key bound to the toggle action, as config.toml spells them,
// and the prefix keys a chord starts with, the primary one first. herdr hands
// every key to a popup while one is up, before it looks at its own bindings,
// so a second press of a toggle key never reaches the action: it arrives
// inside the popup, which closes itself when it recognises the key.
type Toggle struct {
	Bindings []string
	Prefixes []string
}

// closer is the toggle bindings as the popup's terminal reports them, with
// how far into a chord the keys so far have come. A binding no terminal can
// deliver to the popup is left out.
type closer struct {
	// direct close the popup on their own.
	direct []string
	// prefixes start a chord, and chord close the popup after one of them.
	// Both are empty when no chord binding can be delivered.
	prefixes []string
	chord    []string
	// armed is set from a prefix until the key that follows it.
	armed bool
}

func newCloser(toggle Toggle) closer {
	var c closer
	for _, binding := range toggle.Bindings {
		binding = strings.TrimSpace(binding)
		if rest, found := strings.CutPrefix(binding, "prefix+"); found {
			if key, ok := keyName(rest); ok {
				c.chord = append(c.chord, key)
			}
			continue
		}
		if key, ok := keyName(binding); ok {
			c.direct = append(c.direct, key)
		}
	}
	if len(c.chord) > 0 {
		for _, prefix := range toggle.Prefixes {
			if key, ok := keyName(prefix); ok {
				c.prefixes = append(c.prefixes, key)
			}
		}
	}
	if len(c.prefixes) == 0 {
		c.chord = nil
	}
	return c
}

// press takes a keystroke. closes says the popup should go, and taken that the
// key belonged to the chord rather than to the list: a prefix itself, or
// whatever followed it, the way herdr's own prefix mode takes the key after
// the prefix.
func (c *closer) press(msg tea.KeyPressMsg) (closes, taken bool) {
	name := msg.String()
	legacy, _ := keyName(msg.Keystroke())
	matches := func(keys []string) bool {
		return slices.Contains(keys, name) || legacy != "" && slices.Contains(keys, legacy)
	}
	if c.armed {
		c.armed = false
		return matches(c.chord), true
	}
	if matches(c.direct) {
		return true, false
	}
	if matches(c.prefixes) {
		c.armed = true
		return false, true
	}
	return false, false
}

// namedCharacters are the spellings herdr accepts for a character key.
var namedCharacters = map[string]rune{
	"space":        ' ',
	"minus":        '-',
	"comma":        ',',
	"period":       '.',
	"slash":        '/',
	"backslash":    '\\',
	"quote":        '\'',
	"double_quote": '"',
	"double-quote": '"',
	"semicolon":    ';',
	"colon":        ':',
	"percent":      '%',
	"ampersand":    '&',
	"backtick":     '`',
	"plus":         '+',
}

// keyName is the name bubbletea gives the key a binding spells after herdr's
// legacy terminal encoding: alt is an escape in front, ctrl with a character
// is a control character with no room for shift, and cmd or super has no
// encoding at all. A binding that cannot arrive is reported as not ok.
func keyName(spelling string) (string, bool) {
	var ctrl, alt, shift bool
	var name string
	for _, part := range strings.Split(spelling, "+") {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "":
			return "", false
		case "ctrl", "control":
			ctrl = true
		case "shift":
			shift = true
		case "alt", "option", "meta":
			alt = true
		case "cmd", "command", "super":
			return "", false
		default:
			if name != "" {
				return "", false
			}
			name = part
		}
	}
	if name == "" {
		return "", false
	}

	lower := strings.ToLower(name)
	if ch, ok := namedCharacters[lower]; ok {
		return characterName(ch, ctrl, alt, shift), true
	}
	if runes := []rune(name); len(runes) == 1 {
		return characterName(runes[0], ctrl, alt, shift), true
	}

	prefix := ""
	if alt {
		prefix = "alt+"
	}
	switch lower {
	case "up", "down", "left", "right":
		// xterm's modifier parameter carries all three, and bubbletea names
		// them in this order.
		mods := ""
		if ctrl {
			mods += "ctrl+"
		}
		if shift {
			mods += "shift+"
		}
		return prefix + mods + lower, true
	case "enter", "return":
		// A legacy terminal sends the bare character whatever else was held,
		// and the same goes for esc, tab and backspace.
		return prefix + "enter", true
	case "esc", "escape":
		return prefix + "esc", true
	case "tab":
		if shift {
			return prefix + "shift+tab", true
		}
		return prefix + "tab", true
	case "backspace", "bs":
		return prefix + "backspace", true
	}
	if number, found := strings.CutPrefix(lower, "f"); found && !ctrl && !alt && !shift {
		if n, err := strconv.Atoi(number); err == nil && n >= 1 && n <= 12 {
			return "f" + strconv.Itoa(n), true
		}
	}
	return "", false
}

// characterName is what bubbletea calls a character key after herdr's legacy
// encoding of it.
func characterName(ch rune, ctrl, alt, shift bool) string {
	mod := tea.KeyMod(0)
	if alt {
		mod |= tea.ModAlt
	}
	if ctrl {
		if code, ok := controlCode(ch); ok {
			key := tea.Key{Mod: mod}
			switch code {
			case 0:
				key.Code, key.Mod = tea.KeySpace, mod|tea.ModCtrl
			case 9:
				key.Code = tea.KeyTab
			case 13:
				key.Code = tea.KeyEnter
			case 27:
				key.Code = tea.KeyEsc
			case 28, 29, 30, 31:
				key.Code, key.Mod = rune(code+64), mod|tea.ModCtrl
			default:
				key.Code, key.Mod = rune(code+96), mod|tea.ModCtrl
			}
			return key.String()
		}
		// herdr sends the character itself for a ctrl combination with no
		// control character, so the modifier is lost on the way.
	} else if shift && ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	key := tea.Key{Code: ch, Mod: mod}
	if !alt {
		key.Text = string(ch)
	}
	if ch == ' ' {
		key.Code = tea.KeySpace
	}
	return key.String()
}

// controlCode is the control character a legacy terminal sends for ctrl held
// with the character, following herdr's table.
func controlCode(ch rune) (byte, bool) {
	if ch >= 'a' && ch <= 'z' {
		ch -= 'a' - 'A'
	}
	switch {
	case ch >= 'A' && ch <= 'Z':
		return byte(ch) - 64, true
	case ch == ' ' || ch == '@' || ch == '2':
		return 0, true
	case ch == '[' || ch == '3':
		return 27, true
	case ch == '\\' || ch == '4':
		return 28, true
	case ch == ']' || ch == '5':
		return 29, true
	case ch == '^' || ch == '6':
		return 30, true
	case ch == '_' || ch == '/' || ch == '7' || ch == '-':
		return 31, true
	}
	return 0, false
}
