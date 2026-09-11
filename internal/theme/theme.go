// Package theme resolves the colours the popup draws with.
//
// herdr does not publish its theme: the socket API has no method for it, and
// config.toml carries only the theme's name plus whatever tokens the user
// overrode. So the colours come from three places, each overriding the one
// before it: built-in defaults, the tokens the caller read from herdr's
// [theme.custom], and the plugin's own configuration.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Custom is what the plugin's own configuration says about colours: each role
// by the name the file gives it, and the agent statuses by herdr's names.
type Custom struct {
	Colours map[string]string
	Status  map[string]string
}

// Theme is one colour per role the popup draws.
type Theme struct {
	// Rule is the lines above and below the list and the scrollbar's track,
	// Selected the band behind the selected row, Match the query's letters
	// inside a title, Meta the key column and the namespace, Scrollbar the
	// thumb, and Failure the error line.
	Rule      lipgloss.TerminalColor
	Selected  lipgloss.TerminalColor
	Match     lipgloss.TerminalColor
	Meta      lipgloss.TerminalColor
	Scrollbar lipgloss.TerminalColor
	Failure   lipgloss.TerminalColor
	// Status is the colour of what an agent is doing, keyed by the status
	// herdr reports. A status with no colour of its own is drawn in Meta.
	Status map[string]lipgloss.TerminalColor
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
		Status: map[string]lipgloss.TerminalColor{
			"working": lipgloss.Color("3"),
			"blocked": lipgloss.Color("1"),
			"done":    lipgloss.Color("2"),
			"idle":    lipgloss.Color("8"),
		},
	}
}

// Load resolves the theme. tokens are herdr's [theme.custom] overrides, read
// with the rest of its configuration, and own what the plugin's own
// configuration said. Anything left out keeps the colour below it.
func Load(tokens map[string]string, own Custom) Theme {
	theme := Defaults()
	applyHerdr(&theme, tokens)
	applyPlugin(&theme, own)
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

func applyPlugin(theme *Theme, own Custom) {
	set(&theme.Rule, own.Colours["rule"])
	set(&theme.Selected, own.Colours["selected_background"])
	set(&theme.Match, own.Colours["match"])
	set(&theme.Meta, own.Colours["meta"])
	set(&theme.Scrollbar, own.Colours["scrollbar"])
	set(&theme.Failure, own.Colours["failure"])
	for status, colour := range own.Status {
		if colour == "" {
			continue
		}
		theme.Status[status] = lipgloss.Color(colour)
	}
}

// set takes a colour if one was given. A value is either a hex colour or an
// ANSI index, which is what lipgloss.Color accepts.
func set(target *lipgloss.TerminalColor, value string) {
	if value == "" {
		return
	}
	*target = lipgloss.Color(value)
}
