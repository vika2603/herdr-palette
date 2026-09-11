package theme

import (
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

func TestDefaultsAreComplete(t *testing.T) {
	colours := Defaults()

	for name, colour := range map[string]lipgloss.TerminalColor{
		"rule": colours.Rule, "selected": colours.Selected, "match": colours.Match,
		"meta": colours.Meta, "scrollbar": colours.Scrollbar, "failure": colours.Failure,
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
