package ui

import (
	"strings"
	"testing"
)

// withSpelling draws the popup's keys as the platform goos would, for the
// test, whatever platform the test runs on.
func withSpelling(t *testing.T, goos string) {
	t.Helper()
	saved := spelling
	spelling = keyStyleFor(goos)
	t.Cleanup(func() { spelling = saved })
}

func TestABindingIsSpelledWithTheSymbolsOfAMac(t *testing.T) {
	mac := keyStyleFor("darwin")
	for _, tc := range []struct {
		binding, prefix, lead, key string
	}{
		{"prefix+v", "ctrl+b", "⌃b ", "v"},
		{"prefix+shift+x", "ctrl+b", "⌃b ", "X"},
		{"prefix+x", "ctrl+a", "⌃a ", "x"},
		{"ctrl+shift+p", "ctrl+b", "⌃", "P"},
		{"alt+space", "ctrl+b", "⌥", "space"},
		{"super+k", "ctrl+b", "⌘", "k"},
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
		lead, key := mac.keycap(tc.binding, tc.prefix)
		if lead != tc.lead || key != tc.key {
			t.Errorf("keycap(%q, %q) = %q, %q, want %q, %q", tc.binding, tc.prefix, lead, key, tc.lead, tc.key)
		}
	}
}

// Linux and Windows spell a key out, the way their programs write one, and
// shift is a word of its own rather than a capital.
func TestABindingIsSpelledOutOnLinuxAndWindows(t *testing.T) {
	linux, windows := keyStyleFor("linux"), keyStyleFor("windows")
	for _, tc := range []struct {
		binding, prefix, lead, key string
	}{
		{"prefix+v", "ctrl+b", "Ctrl+B ", "V"},
		{"prefix+shift+x", "ctrl+b", "Ctrl+B Shift+", "X"},
		{"ctrl+shift+p", "ctrl+b", "Ctrl+Shift+", "P"},
		{"alt+space", "ctrl+b", "Alt+", "Space"},
		{"prefix+alt+1..9", "ctrl+b", "Ctrl+B Alt+", "1..9"},
		{"prefix+enter", "ctrl+b", "Ctrl+B ", "Enter"},
		{"prefix+?", "ctrl+b", "Ctrl+B ", "?"},
		{"ctrl++", "ctrl+b", "Ctrl+", "+"},
		{"f5", "ctrl+b", "", "F5"},
		{"super+k", "ctrl+b", "Super+", "K"},
		{"prefix+v", "", "prefix+", "V"},
	} {
		lead, key := linux.keycap(tc.binding, tc.prefix)
		if lead != tc.lead || key != tc.key {
			t.Errorf("keycap(%q, %q) = %q, %q, want %q, %q", tc.binding, tc.prefix, lead, key, tc.lead, tc.key)
		}
	}
	if got := windows.name("super+k"); got != "Win+K" {
		t.Errorf("super+k on Windows = %q, want the key named as Windows names it", got)
	}
}

// The keys the line under the list names follow the same spelling as the key
// column.
func TestTheFooterSpellsItsKeysForThePlatform(t *testing.T) {
	for goos, want := range map[string][]string{
		"darwin": {"⏎ run", "esc close"},
		"linux":  {"Enter run", "Esc close"},
	} {
		t.Run(goos, func(t *testing.T) {
			withSpelling(t, goos)
			m, _ := closeModel(t)
			selectRow(t, &m, "close")
			for _, hint := range want {
				if footer := m.footer(); !strings.Contains(footer, hint) {
					t.Errorf("footer = %q, want %q", footer, hint)
				}
			}
		})
	}
}
