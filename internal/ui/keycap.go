package ui

import (
	"runtime"
	"strings"
	"unicode"
)

// keyStyle is how a key is spelled on the platform the palette runs on. macOS
// draws modifiers and a few keys as the symbols its keyboards carry; Linux and
// Windows programs spell them out, as Ctrl+B or Shift+Enter, and a symbol few
// of their keyboards print reads as noise there.
type keyStyle struct {
	symbols bool
	// super is what the key macOS calls command is named on the platform.
	super string
}

func keyStyleFor(goos string) keyStyle {
	switch goos {
	case "darwin":
		return keyStyle{symbols: true}
	case "windows":
		return keyStyle{super: "Win"}
	}
	return keyStyle{super: "Super"}
}

// spelling is how every key the popup draws is spelled.
var spelling = keyStyleFor(runtime.GOOS)

// keySymbols are the keys macOS draws as the symbol printed on them. The rest
// keep their names, which read at least as well as a symbol few keyboards
// carry.
var keySymbols = map[string]string{
	"enter":     "⏎",
	"return":    "⏎",
	"tab":       "⇥",
	"backspace": "⌫",
}

// keycap spells a binding the way the key column draws it, the prefix chord
// as the prefix key itself: "prefix+shift+x" under a ctrl+b prefix reads
// "⌃b X" on macOS, where shift on a letter is its capital, and "Ctrl+B Shift+X"
// elsewhere. lead is what comes before the key, which the column draws fainter
// than the key: the key is what tells two bindings apart.
//
// prefix is the prefix key as [keys] spells it. Without one the chord is left
// as written, since which key starts it is exactly what is unknown.
func (s keyStyle) keycap(binding, prefix string) (lead, key string) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return "", ""
	}
	if rest, found := strings.CutPrefix(binding, "prefix+"); found {
		if prefix == "" {
			lead = "prefix+"
		} else {
			lead = s.name(prefix) + " "
		}
		binding = rest
	}

	modifiers, name := splitBinding(binding)
	shift := false
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "ctrl", "control":
			lead += s.pick("⌃", "Ctrl+")
		case "alt", "option", "meta":
			lead += s.pick("⌥", "Alt+")
		case "super", "cmd", "command":
			lead += s.pick("⌘", s.super+"+")
		case "shift":
			shift = true
		default:
			lead += modifier + "+"
		}
	}

	key = s.spellKey(name)
	if shift {
		if runes := []rune(key); s.symbols && len(runes) == 1 && unicode.IsLetter(runes[0]) {
			key = string(unicode.ToUpper(runes[0]))
		} else {
			lead += s.pick("⇧", "Shift+")
		}
	}
	return lead, key
}

// name is a binding spelled whole, as the line under the list names a key.
func (s keyStyle) name(binding string) string {
	lead, key := s.keycap(binding, "")
	return lead + key
}

func (s keyStyle) pick(symbol, word string) string {
	if s.symbols {
		return symbol
	}
	return word
}

// spellKey is the key itself. Spelled out, a letter is its capital, as it is
// printed on the key, and a named key starts with one, as in Enter or Space.
func (s keyStyle) spellKey(name string) string {
	if s.symbols {
		if symbol, ok := keySymbols[strings.ToLower(name)]; ok {
			return symbol
		}
		return name
	}
	runes := []rune(name)
	if len(runes) == 0 {
		return name
	}
	return string(unicode.ToUpper(runes[0])) + string(runes[1:])
}

// splitBinding separates the modifiers from the key. A binding that ends in
// "++" names the plus key itself.
func splitBinding(binding string) ([]string, string) {
	if binding == "+" {
		return nil, "+"
	}
	if rest, found := strings.CutSuffix(binding, "++"); found {
		return strings.Split(rest, "+"), "+"
	}
	parts := strings.Split(binding, "+")
	return parts[:len(parts)-1], parts[len(parts)-1]
}
