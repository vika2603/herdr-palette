package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

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
}

func TestHerdrThemeTokensAreUsed(t *testing.T) {
	colours := Load(nil, tokens)
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
	dir := t.TempDir()
	write(t, dir, ConfigFile, "rule = \"#111111\"\nmeta = \"8\"\n")
	env := plugintest.Env(plugintest.ConfigDir(dir))

	colours := Load(env, tokens)
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
	env := plugintest.Env(plugintest.ConfigDir(t.TempDir()))

	if Load(env, nil) != Defaults() {
		t.Error("Load() changed a colour although nothing configures one")
	}
}
