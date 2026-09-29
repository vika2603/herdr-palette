package ui

import (
	"strings"
	"unicode"
)

// keycap spells a binding the way the key column draws it: the prefix chord
// as the prefix key itself, modifiers as their symbols, and shift on a letter
// as the capital, so "prefix+shift+x" under a ctrl+b prefix reads "⌃b X". lead
// is what comes before the key, which the column draws fainter than the key:
// the key is what tells two bindings apart.
//
// prefix is the prefix key as [keys] spells it. Without one the chord is left
// as written, since which key starts it is exactly what is unknown.
func keycap(binding, prefix string) (lead, key string) {
	binding = strings.TrimSpace(binding)
	if binding == "" {
		return "", ""
	}
	if rest, found := strings.CutPrefix(binding, "prefix+"); found {
		if prefix == "" {
			lead = "prefix+"
		} else {
			prefixLead, prefixKey := keycap(prefix, "")
			lead = prefixLead + prefixKey + " "
		}
		binding = rest
	}

	modifiers, name := splitBinding(binding)
	shift := false
	for _, modifier := range modifiers {
		switch strings.ToLower(modifier) {
		case "ctrl", "control":
			lead += "⌃"
		case "alt", "option", "meta":
			lead += "⌥"
		case "super", "cmd", "command":
			lead += "⌘"
		case "shift":
			shift = true
		default:
			lead += modifier + "+"
		}
	}

	key = spellKey(name)
	if shift {
		if runes := []rune(key); len(runes) == 1 && unicode.IsLetter(runes[0]) {
			key = string(unicode.ToUpper(runes[0]))
		} else {
			lead += "⇧"
		}
	}
	return lead, key
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

// keySymbols are the keys drawn as the symbol printed on them. The rest keep
// their names, which read at least as well as a symbol few keyboards carry.
var keySymbols = map[string]string{
	"enter":     "⏎",
	"return":    "⏎",
	"tab":       "⇥",
	"backspace": "⌫",
}

func spellKey(name string) string {
	if symbol, ok := keySymbols[strings.ToLower(name)]; ok {
		return symbol
	}
	return name
}
