package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A binding is matched against what bubbletea reports once herdr has encoded
// the key for the popup's terminal, so each case pairs a spelling with the
// key that encoding parses back to.
func TestABindingIsNamedAsThePopupReceivesIt(t *testing.T) {
	cases := []struct {
		spelling string
		key      tea.Key
	}{
		{"alt+space", tea.Key{Type: tea.KeySpace, Alt: true}},
		{"space", tea.Key{Type: tea.KeySpace}},
		{"ctrl+p", tea.Key{Type: tea.KeyCtrlP}},
		// A control character has no room for shift, so herdr sends the
		// same byte for both.
		{"ctrl+shift+p", tea.Key{Type: tea.KeyCtrlP}},
		{"alt+ctrl+p", tea.Key{Type: tea.KeyCtrlP, Alt: true}},
		{"shift+p", tea.Key{Type: tea.KeyRunes, Runes: []rune{'P'}}},
		{"P", tea.Key{Type: tea.KeyRunes, Runes: []rune{'P'}}},
		{"alt+p", tea.Key{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true}},
		{"ctrl+minus", tea.Key{Type: tea.KeyCtrlUnderscore}},
		{"ctrl+space", tea.Key{Type: tea.KeyCtrlAt}},
		{"alt+comma", tea.Key{Type: tea.KeyRunes, Runes: []rune{','}, Alt: true}},
		{"f5", tea.Key{Type: tea.KeyF5}},
		{"alt+up", tea.Key{Type: tea.KeyUp, Alt: true}},
		{"ctrl+shift+down", tea.Key{Type: tea.KeyCtrlShiftDown}},
		{"shift+tab", tea.Key{Type: tea.KeyShiftTab}},
		{"alt+enter", tea.Key{Type: tea.KeyEnter, Alt: true}},
		{"ctrl+enter", tea.Key{Type: tea.KeyEnter}},
		{"alt+backspace", tea.Key{Type: tea.KeyBackspace, Alt: true}},
	}
	for _, c := range cases {
		got, ok := keyName(c.spelling)
		if !ok {
			t.Errorf("%q: not recognised", c.spelling)
			continue
		}
		if want := tea.KeyMsg(c.key).String(); got != want {
			t.Errorf("%q = %q, want %q", c.spelling, got, want)
		}
	}
}

func TestABindingNoTerminalCanDeliverIsLeftOut(t *testing.T) {
	for _, spelling := range []string{"", "cmd+p", "super+space", "ctrl+f5", "ctrl+", "a+b", "home"} {
		if got, ok := keyName(spelling); ok {
			t.Errorf("%q = %q, want it left out", spelling, got)
		}
	}
}

func TestAChordArmsOnThePrefixAndClosesOnTheKey(t *testing.T) {
	c := newCloser(Toggle{Binding: "prefix+space", Prefix: "ctrl+b"})
	space := tea.KeyMsg{Type: tea.KeySpace}
	prefix := tea.KeyMsg{Type: tea.KeyCtrlB}

	if closes, taken := c.press(space); closes || taken {
		t.Error("the key on its own closed the popup or was taken, before the prefix")
	}
	if closes, taken := c.press(prefix); closes || !taken {
		t.Error("the prefix was not taken, or closed the popup by itself")
	}
	if closes, _ := c.press(space); !closes {
		t.Error("the key after the prefix did not close the popup")
	}

	// Any other key after the prefix is taken with it and disarms the chord.
	c.press(prefix)
	if closes, taken := c.press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}); closes || !taken {
		t.Error("another key after the prefix was not taken, or closed the popup")
	}
	if closes, taken := c.press(space); closes || taken {
		t.Error("the chord stayed armed past the key that followed the prefix")
	}
}

func TestNothingBoundClosesNothing(t *testing.T) {
	for _, toggle := range []Toggle{{}, {Binding: "prefix+space"}, {Binding: "cmd+p"}} {
		c := newCloser(toggle)
		for _, msg := range []tea.KeyMsg{{Type: tea.KeySpace}, {Type: tea.KeyCtrlB}, {Type: tea.KeyRunes, Runes: []rune{'p'}}} {
			if closes, taken := c.press(msg); closes || taken {
				t.Errorf("%+v: %q closed the popup or was taken", toggle, msg.String())
			}
		}
	}
}
