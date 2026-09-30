package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// A binding is matched against what bubbletea reports once herdr has encoded
// the key for the popup's terminal, so each case pairs a spelling with the
// key that encoding parses back to.
func TestABindingIsNamedAsThePopupReceivesIt(t *testing.T) {
	cases := []struct {
		spelling string
		key      tea.Key
	}{
		{"alt+space", tea.Key{Code: tea.KeySpace, Mod: tea.ModAlt}},
		{"space", tea.Key{Code: tea.KeySpace, Text: " "}},
		{"ctrl+p", tea.Key{Code: 'p', Mod: tea.ModCtrl}},
		// A control character has no room for shift, so herdr sends the
		// same byte for both.
		{"ctrl+shift+p", tea.Key{Code: 'p', Mod: tea.ModCtrl}},
		{"alt+ctrl+p", tea.Key{Code: 'p', Mod: tea.ModAlt | tea.ModCtrl}},
		{"shift+p", tea.Key{Code: 'p', Text: "P", Mod: tea.ModShift}},
		{"P", tea.Key{Code: 'p', Text: "P", Mod: tea.ModShift}},
		{"alt+p", tea.Key{Code: 'p', Mod: tea.ModAlt}},
		{"ctrl+minus", tea.Key{Code: '_', Mod: tea.ModCtrl}},
		{"ctrl+space", tea.Key{Code: tea.KeySpace, Mod: tea.ModCtrl}},
		{"alt+comma", tea.Key{Code: ',', Mod: tea.ModAlt}},
		{"f5", tea.Key{Code: tea.KeyF5}},
		{"alt+up", tea.Key{Code: tea.KeyUp, Mod: tea.ModAlt}},
		{"ctrl+shift+down", tea.Key{Code: tea.KeyDown, Mod: tea.ModCtrl | tea.ModShift}},
		{"shift+tab", tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}},
		{"alt+enter", tea.Key{Code: tea.KeyEnter, Mod: tea.ModAlt}},
		{"ctrl+enter", tea.Key{Code: tea.KeyEnter}},
		{"alt+backspace", tea.Key{Code: tea.KeyBackspace, Mod: tea.ModAlt}},
	}
	for _, c := range cases {
		got, ok := keyName(c.spelling)
		if !ok {
			t.Errorf("%q: not recognised", c.spelling)
			continue
		}
		if want := c.key.String(); got != want {
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
	space := tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	prefix := tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}

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
	if closes, taken := c.press(tea.KeyPressMsg{Code: 'x', Text: "x"}); closes || !taken {
		t.Error("another key after the prefix was not taken, or closed the popup")
	}
	if closes, taken := c.press(space); closes || taken {
		t.Error("the chord stayed armed past the key that followed the prefix")
	}
}

func TestEnhancedKeyStillMatchesTheHerdrBinding(t *testing.T) {
	for _, tc := range []struct {
		binding string
		key     tea.KeyPressMsg
	}{
		{"ctrl+shift+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl | tea.ModShift}},
		{"ctrl+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}},
	} {
		c := newCloser(Toggle{Binding: tc.binding})
		if closes, _ := c.press(tc.key); !closes {
			t.Errorf("%q did not close for enhanced key %q", tc.binding, tc.key.String())
		}
	}
}

func TestNothingBoundClosesNothing(t *testing.T) {
	for _, toggle := range []Toggle{{}, {Binding: "prefix+space"}, {Binding: "cmd+p"}} {
		c := newCloser(toggle)
		for _, msg := range []tea.KeyPressMsg{{Code: tea.KeySpace, Text: " "}, {Code: 'b', Mod: tea.ModCtrl}, {Code: 'p', Text: "p"}} {
			if closes, taken := c.press(msg); closes || taken {
				t.Errorf("%+v: %q closed the popup or was taken", toggle, msg.String())
			}
		}
	}
}
