package theme

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// tokens are what herdr's [theme.custom] would supply.
var tokens = map[string]string{
	"overlay0": "#414868",
	"overlay1": "#7aa2f7",
	"surface0": "#24283b",
}

func TestEverySchemeIsComplete(t *testing.T) {
	for _, colours := range []Theme{Herd(), Terminal()} {
		complete(t, colours)
	}
}

func complete(t *testing.T, colours Theme) {
	t.Helper()

	for name, colour := range map[string]lipgloss.TerminalColor{
		"accent": colours.Accent, "rule": colours.Rule, "selected": colours.Selected,
		"match": colours.Match, "meta": colours.Meta, "faint": colours.Faint,
		"scrollbar": colours.Scrollbar, "failure": colours.Failure,
	} {
		if colour == nil {
			t.Errorf("%s has no default colour", name)
		}
	}
	for _, status := range []string{"working", "blocked", "done", "idle"} {
		if colours.Status[status] == nil {
			t.Errorf("%s has no default colour", status)
		}
	}
}

func TestHerdrThemeTokensAreUsed(t *testing.T) {
	colours := Load(tokens, Custom{})
	if colours.Rule != lipgloss.Color("#414868") {
		t.Errorf("rule = %v, want herdr's overlay0", colours.Rule)
	}
	if colours.Selected != lipgloss.Color("#24283b") {
		t.Errorf("selected = %v, want herdr's surface0", colours.Selected)
	}
	if colours.Match != lipgloss.Color("#7aa2f7") {
		t.Errorf("match = %v, want herdr's overlay1", colours.Match)
	}
	if colours.Meta != Defaults().Meta {
		t.Error("a colour herdr's theme says nothing about lost its default")
	}
}

func TestThePluginConfigurationWins(t *testing.T) {
	own := Custom{Colours: map[string]string{"rule": "#111111", "meta": "8"}}

	colours := Load(tokens, own)

	if colours.Rule != lipgloss.Color("#111111") {
		t.Errorf("rule = %v, want the plugin's own configuration", colours.Rule)
	}
	if colours.Meta != lipgloss.Color("8") {
		t.Errorf("meta = %v, want an ANSI index to be accepted", colours.Meta)
	}
	if colours.Match != lipgloss.Color("#7aa2f7") {
		t.Errorf("match = %v, want herdr's token where the plugin says nothing", colours.Match)
	}
}

func TestNoConfigurationAtAll(t *testing.T) {
	if !reflect.DeepEqual(Load(nil, Custom{}), Defaults()) {
		t.Error("Load() changed a colour although nothing configures one")
	}
}

func TestAStatusColourCanBeReplaced(t *testing.T) {
	colours := Load(nil, Custom{Status: map[string]string{"working": "#fab387"}})

	if colours.Status["working"] != lipgloss.Color("#fab387") {
		t.Errorf("working = %v, want the configured colour", colours.Status["working"])
	}
	if colours.Status["blocked"] != Defaults().Status["blocked"] {
		t.Error("a status nothing configures lost its colour")
	}
}

func TestTheMatchFollowsAnAccentHerdrSupplies(t *testing.T) {
	colours := Load(map[string]string{"accent": "#f5c2e7"}, Custom{})

	if colours.Accent != lipgloss.Color("#f5c2e7") {
		t.Errorf("accent = %v, want herdr's accent", colours.Accent)
	}
	if colours.Match != colours.Accent {
		t.Errorf("match = %v, want the accent when nothing names a colour for it", colours.Match)
	}
}

func TestAMatchColourOfItsOwnOutlivesTheAccent(t *testing.T) {
	colours := Load(nil, Custom{Colours: map[string]string{"accent": "#111111", "match": "#222222"}})

	if colours.Accent != lipgloss.Color("#111111") || colours.Match != lipgloss.Color("#222222") {
		t.Errorf("accent = %v and match = %v, want each the one configured", colours.Accent, colours.Match)
	}
}

func TestTheTerminalSchemeDrawsInTheTerminalsColours(t *testing.T) {
	colours := Load(nil, Custom{Scheme: SchemeTerminal})

	if colours.Accent != lipgloss.Color("5") || colours.Meta != lipgloss.Color("8") {
		t.Errorf("accent = %v and meta = %v, want ANSI indexes", colours.Accent, colours.Meta)
	}
	if !reflect.DeepEqual(Load(nil, Custom{Scheme: "no such scheme"}), Defaults()) {
		t.Error("a scheme the palette does not know did not fall back to the default")
	}
}

func TestTheBackgroundIsDerivedFromTheTerminals(t *testing.T) {
	colours := Load(nil, Custom{})
	for _, tc := range []struct {
		terminal color.Color
		dark     bool
	}{
		{color.RGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}, true},
		{color.White, false},
	} {
		got := colours.Backdrop(tc.terminal, tc.dark)
		if got == nil || brightness(got) >= brightness(tc.terminal) {
			t.Errorf("backdrop for %v = %v, want a darker shade of it", tc.terminal, got)
		}
	}
	if got := colours.Backdrop(nil, true); got != nil {
		t.Errorf("backdrop with no terminal background = %v, want none", got)
	}
}

func TestAConfiguredBackgroundReplacesTheDerivedOne(t *testing.T) {
	colours := Load(nil, Custom{Colours: map[string]string{"background": "#f0f0f0"}})
	r, g, b, _ := colours.Backdrop(color.Black, true).RGBA()
	if r>>8 != 0xf0 || g>>8 != 0xf0 || b>>8 != 0xf0 {
		t.Errorf("backdrop = %v, want the configured colour", colours.Backdrop(color.Black, true))
	}

	colours = Load(nil, Custom{Colours: map[string]string{"background": "blue"}})
	if colours.Background != nil {
		t.Errorf("background = %v, want an unreadable value to leave it derived", colours.Background)
	}
}

func brightness(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	return r + g + b
}
