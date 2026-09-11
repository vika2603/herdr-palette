// Package theme resolves the colours the popup draws with.
//
// herdr does not publish its theme: the socket API has no method for it, and
// config.toml carries only the theme's name plus whatever tokens the user
// overrode. So the colours come from three places, each overriding the one
// before it: built-in defaults, the tokens the caller read from herdr's
// [theme.custom], and the plugin's own configuration.
package theme

import (
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	"github.com/vika2603/herdr-client/plugin"
)

// ConfigFile is the plugin's own configuration, in its config directory.
const ConfigFile = "config.toml"

// Theme is one colour per role the popup draws.
type Theme struct {
	// Rule is the lines above and below the list and the scrollbar's track,
	// Selected the band behind the selected row, Match the query's letters
	// inside a title, Meta the key and type columns, Scrollbar the thumb, and
	// Failure the error line.
	Rule      lipgloss.TerminalColor
	Selected  lipgloss.TerminalColor
	Match     lipgloss.TerminalColor
	Meta      lipgloss.TerminalColor
	Scrollbar lipgloss.TerminalColor
	Failure   lipgloss.TerminalColor
}

// Defaults are used for every colour nothing else supplies. The two shades are
// adaptive because they sit just off the terminal's own background, which the
// ANSI palette has no index for; the rest are ANSI indexes, so they follow the
// terminal's colours.
func Defaults() Theme {
	return Theme{
		Rule:      lipgloss.AdaptiveColor{Dark: "#33333F", Light: "#DCDCE6"},
		Selected:  lipgloss.AdaptiveColor{Dark: "#2C2C3A", Light: "#E6E6EE"},
		Match:     lipgloss.Color("4"),
		Meta:      lipgloss.Color("8"),
		Scrollbar: lipgloss.Color("8"),
		Failure:   lipgloss.Color("1"),
	}
}

// Load resolves the theme. tokens are herdr's [theme.custom] overrides, read
// with the rest of its configuration. Anything unreadable leaves the defaults
// in place.
func Load(env *plugin.Env, tokens map[string]string) Theme {
	theme := Defaults()
	applyHerdr(&theme, tokens)
	if env != nil {
		applyPlugin(&theme, env.ConfigPath(ConfigFile))
	}
	return theme
}

// applyHerdr takes the tokens herdr's theme names, where the user overrode
// them. The names are herdr's own: overlay0 is its faintest line colour,
// surface0 the shade it lays behind a selected row, and overlay1 a colour that
// still reads against both.
func applyHerdr(theme *Theme, tokens map[string]string) {
	set(&theme.Rule, tokens["overlay0"])
	set(&theme.Selected, tokens["surface0"])
	set(&theme.Match, tokens["overlay1"])
}

// pluginConfig is the plugin's own configuration file.
type pluginConfig struct {
	Rule               string `toml:"rule"`
	SelectedBackground string `toml:"selected_background"`
	Match              string `toml:"match"`
	Meta               string `toml:"meta"`
	Scrollbar          string `toml:"scrollbar"`
	Failure            string `toml:"failure"`
}

func applyPlugin(theme *Theme, path string) {
	if path == "" {
		return
	}
	var parsed pluginConfig
	if _, err := toml.DecodeFile(path, &parsed); err != nil {
		return
	}
	set(&theme.Rule, parsed.Rule)
	set(&theme.Selected, parsed.SelectedBackground)
	set(&theme.Match, parsed.Match)
	set(&theme.Meta, parsed.Meta)
	set(&theme.Scrollbar, parsed.Scrollbar)
	set(&theme.Failure, parsed.Failure)
}

// set takes a colour if one was given. A value is either a hex colour or an
// ANSI index, which is what lipgloss.Color accepts.
func set(target *lipgloss.TerminalColor, value string) {
	if value == "" {
		return
	}
	*target = lipgloss.Color(value)
}
