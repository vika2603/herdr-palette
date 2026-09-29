package ui

import "testing"

func TestABindingIsSpelledTheWayTheKeyColumnDrawsIt(t *testing.T) {
	for _, tc := range []struct {
		binding, prefix, lead, key string
	}{
		{"prefix+v", "ctrl+b", "⌃b ", "v"},
		{"prefix+shift+x", "ctrl+b", "⌃b ", "X"},
		{"prefix+x", "ctrl+a", "⌃a ", "x"},
		{"ctrl+shift+p", "ctrl+b", "⌃", "P"},
		{"alt+space", "ctrl+b", "⌥", "space"},
		{"prefix+shift+1..9", "ctrl+b", "⌃b ⇧", "1..9"},
		{"prefix+alt+1..9", "ctrl+b", "⌃b ⌥", "1..9"},
		{"prefix+enter", "ctrl+b", "⌃b ", "⏎"},
		{"prefix+?", "ctrl+b", "⌃b ", "?"},
		{"ctrl++", "ctrl+b", "⌃", "+"},
		{"f5", "ctrl+b", "", "f5"},
		// Which key starts the chord is what is unknown, so it is not guessed.
		{"prefix+v", "", "prefix+", "v"},
		{"", "ctrl+b", "", ""},
	} {
		lead, key := keycap(tc.binding, tc.prefix)
		if lead != tc.lead || key != tc.key {
			t.Errorf("keycap(%q, %q) = %q, %q, want %q, %q", tc.binding, tc.prefix, lead, key, tc.lead, tc.key)
		}
	}
}
