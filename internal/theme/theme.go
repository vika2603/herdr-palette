// Package theme resolves the colours the popup draws with.
//
// herdr does not publish its theme: the socket API has no method for it, and
// config.toml carries only the theme's name plus whatever tokens the user
// overrode. So the colours come from three places, each overriding the one
// before it: the scheme the plugin's configuration picks, the tokens the
// caller read from herdr's [theme.custom], and the plugin's own colours.
package theme

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	lipgloss2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/lipgloss"
)

// Scheme names the plugin's configuration accepts. Herd is the popup's own
// palette; Terminal draws in ANSI indexes, so it follows the terminal's
// colours instead.
const (
	SchemeHerd     = "herd"
	SchemeTerminal = "terminal"
)

// Custom is what the plugin's own configuration says about how the popup is
// drawn: the scheme, each role by the name the file gives it, each state a row
// can be in by its own name, and the two ends of a field.
type Custom struct {
	Scheme    string
	Colours   map[string]string
	Status    map[string]string
	FieldEnds []string
}

// Theme is how the popup is drawn: one colour per role, and the ends of a
// script's field.
type Theme struct {
	// Accent marks what has focus and nothing else: the mode label, the
	// stretch of rule under it and the selected row's bar.
	Accent lipgloss.TerminalColor
	// Rule is the lines above and below the list and the scrollbar's track,
	// Selected the band behind the selected row, Match the query's letters
	// inside a row, Meta the text a row carries beside its title, Faint the
	// weakest text there is — group headings, the prefix of a key — Scrollbar
	// the thumb, and Failure what cannot be undone or did not work.
	Rule      lipgloss.TerminalColor
	Selected  lipgloss.TerminalColor
	Match     lipgloss.TerminalColor
	Meta      lipgloss.TerminalColor
	Faint     lipgloss.TerminalColor
	Scrollbar lipgloss.TerminalColor
	Failure   lipgloss.TerminalColor
	// Hovered is the band behind the row the pointer is over, Menu what a
	// dropdown's options are laid on, MenuHovered the band behind the option
	// the pointer is over, and FieldHovered what a script's field the pointer
	// is over is drawn on. A scheme leaves them unset: the popup derives them
	// from its background and Selected, which is the only way they stay
	// shades of a background a scheme cannot know.
	Hovered      lipgloss.TerminalColor
	Menu         lipgloss.TerminalColor
	MenuHovered  lipgloss.TerminalColor
	FieldHovered lipgloss.TerminalColor
	// Status is the colour of the state a row is in, keyed by its name: what
	// an agent is doing, by herdr's names for it. A state with no colour of
	// its own is drawn in Meta.
	Status map[string]lipgloss.TerminalColor
	// Background is what the popup is laid on when the configuration names
	// it; otherwise Backdrop derives it from the terminal's background.
	Background color.Color
	// FieldEnds are what close a script's field on the left and the right,
	// drawn in the field's shade: a shape is its foreground, and a blank is
	// filled with it.
	FieldEnds [2]string
}

// roundEnds are the default FieldEnds, Powerline's half circles from the
// private use area: Ghostty, WezTerm and Windows Terminal draw them
// themselves, kitty ships a Nerd Font to fall back on, and other terminals
// need one installed.
var roundEnds = [2]string{"\ue0b6", "\ue0b4"}

// maxFieldEnd is the widest an end may be, in cells. A wider one would take
// the room the field's text is narrowed for.
const maxFieldEnd = 2

// resolveFieldEnds is the two ends a configuration gives. Anything else, an
// end that is not printable text on one line or one wider than maxFieldEnd,
// keeps ends.
func resolveFieldEnds(ends [2]string, configured []string) [2]string {
	if len(configured) != 2 {
		return ends
	}
	given := [2]string{configured[0], configured[1]}
	for _, end := range given {
		if lipgloss.Width(end) > maxFieldEnd || strings.ContainsFunc(end, func(r rune) bool { return !unicode.IsPrint(r) && r != ' ' }) {
			return ends
		}
	}
	return given
}

// How much a derived background darkens the terminal's. Darkening rather than
// lightening keeps the selected row's band and the rule, shades just lighter
// than a dark terminal's background, visible on it. The same share of white is
// a far larger step, so a light background is darkened less.
const (
	darkenDark  = 0.2
	darkenLight = 0.04
)

// Backdrop is the background the popup sets, given the terminal's: the
// configured one, or the terminal's own darkened so the popup stands apart
// from the panes it covers. herdr draws a pane background equal to the
// terminal's as the terminal's own, so a black one, which cannot be darkened,
// needs a configured background.
func (t Theme) Backdrop(terminal color.Color, dark bool) color.Color {
	if t.Background != nil {
		return t.Background
	}
	if terminal == nil {
		return nil
	}
	if dark {
		return lipgloss2.Darken(terminal, darkenDark)
	}
	return lipgloss2.Darken(terminal, darkenLight)
}

// Shades of the popup's surfaces, as shares of the way from its background to
// the selected row's band, which a field is drawn on too. A row the pointer is
// over sits halfway, so it reads as a row that could be selected without
// reading as the one that is. A field the pointer is over sits a half step
// past the band, the step the row took. The menu of a dropdown sits as far
// past the band as the band is from the background, a surface raised over the
// list in the band's own hue, and an option the pointer is over a half step
// further.
const (
	hoveredShare      = 0.5
	fieldHoveredShare = 1.5
	menuShare         = 2
	menuHoveredShare  = 2.5
)

// Surfaces are the colours of the popup's surfaces on its background.
type Surfaces struct {
	Hovered, Menu, MenuHovered, FieldHovered lipgloss.TerminalColor
}

// Surfaces are Hovered, Menu, MenuHovered and FieldHovered on the popup's
// background: each as configured, or else the shade of backdrop it derives.
// Without a background there is no such shade, and an unconfigured surface
// is nil.
func (t Theme) Surfaces(backdrop color.Color) Surfaces {
	surface := func(configured lipgloss.TerminalColor, share float64) lipgloss.TerminalColor {
		if configured != nil || backdrop == nil {
			return configured
		}
		return shade(backdrop, t.Selected, share)
	}
	return Surfaces{
		Hovered:      surface(t.Hovered, hoveredShare),
		Menu:         surface(t.Menu, menuShare),
		MenuHovered:  surface(t.MenuHovered, menuHoveredShare),
		FieldHovered: surface(t.FieldHovered, fieldHoveredShare),
	}
}

// shade is the colour share of the way from one colour to another, past the
// second for a share over one.
func shade(from color.Color, to lipgloss.TerminalColor, share float64) lipgloss.Color {
	channel := func(a, b uint32) uint32 {
		v := float64(a>>8) + (float64(b>>8)-float64(a>>8))*share
		return uint32(min(max(v, 0), 255))
	}
	r1, g1, b1, _ := from.RGBA()
	r2, g2, b2, _ := resolve(to).RGBA()
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", channel(r1, r2), channel(g1, g2), channel(b1, b2)))
}

// resolve is a theme colour as the values it was given name it. The colour
// itself would answer through the terminal's profile, which rounds it, or
// with black where the profile has no colour at all.
func resolve(c lipgloss.TerminalColor) color.Color {
	switch c := c.(type) {
	case lipgloss.AdaptiveColor:
		if lipgloss.HasDarkBackground() {
			return lipgloss2.Color(c.Dark)
		}
		return lipgloss2.Color(c.Light)
	case lipgloss.Color:
		return lipgloss2.Color(string(c))
	}
	return c
}

// Defaults is the scheme used when the configuration names none.
func Defaults() Theme { return Herd() }

// Scheme is the named scheme, or the default one for a name it does not know.
func Scheme(name string) Theme {
	if name == SchemeTerminal {
		return Terminal()
	}
	return Herd()
}

// Herd is the popup's own palette, one value for a dark terminal and one for
// a light one. The accent is violet because its hue sits apart from all four
// state colours, so what has focus never reads as a state; the greys lean the
// same way so the two belong together.
func Herd() Theme {
	muted := lipgloss.AdaptiveColor{Dark: "#8C8AA3", Light: "#66647E"}
	accent := lipgloss.AdaptiveColor{Dark: "#A48BFF", Light: "#6A4FE0"}
	return Theme{
		FieldEnds: roundEnds,
		Accent:    accent,
		Rule:      lipgloss.AdaptiveColor{Dark: "#2C2A3B", Light: "#DEDCE8"},
		Selected:  lipgloss.AdaptiveColor{Dark: "#262338", Light: "#ECE8FB"},
		Match:     accent,
		Meta:      muted,
		Faint:     lipgloss.AdaptiveColor{Dark: "#4D4D54", Light: "#9290A8"},
		Scrollbar: muted,
		Failure:   lipgloss.AdaptiveColor{Dark: "#F0506E", Light: "#C8254A"},
		Status: map[string]lipgloss.TerminalColor{
			"working": lipgloss.AdaptiveColor{Dark: "#E5B94E", Light: "#A07410"},
			"blocked": lipgloss.AdaptiveColor{Dark: "#FF6F61", Light: "#D9483B"},
			"done":    lipgloss.AdaptiveColor{Dark: "#4FD1A1", Light: "#13875E"},
			"idle":    lipgloss.AdaptiveColor{Dark: "#6E6C86", Light: "#8E8CA3"},
		},
	}
}

// Terminal follows the terminal's palette. The two shades are adaptive
// because they sit just off the terminal's own background, which the ANSI
// palette has no index for; the rest are ANSI indexes.
func Terminal() Theme {
	return Theme{
		FieldEnds: roundEnds,
		Accent:    lipgloss.Color("5"),
		Rule:      lipgloss.AdaptiveColor{Dark: "#33333F", Light: "#DCDCE6"},
		Selected:  lipgloss.AdaptiveColor{Dark: "#2C2C3A", Light: "#E6E6EE"},
		Match:     lipgloss.Color("5"),
		Meta:      lipgloss.Color("8"),
		Faint:     lipgloss.Color("8"),
		Scrollbar: lipgloss.Color("8"),
		Failure:   lipgloss.Color("1"),
		Status: map[string]lipgloss.TerminalColor{
			"working": lipgloss.Color("3"),
			// Bright rather than plain red, which Failure has: an agent
			// waiting on you is news, not a command that would not run.
			"blocked": lipgloss.Color("9"),
			"done":    lipgloss.Color("2"),
			"idle":    lipgloss.Color("8"),
		},
	}
}

// Load resolves the theme. tokens are herdr's [theme.custom] overrides, read
// with the rest of its configuration, and own what the plugin's own
// configuration said. Anything left out keeps the colour below it.
//
// The query's letters are drawn in the accent unless something names a colour
// for them, so an accent taken from herdr's theme carries over to them too.
func Load(tokens map[string]string, own Custom) Theme {
	theme := Scheme(own.Scheme)
	matched := applyHerdr(&theme, tokens)
	matched = applyPlugin(&theme, own) || matched
	theme.FieldEnds = resolveFieldEnds(theme.FieldEnds, own.FieldEnds)
	if !matched {
		theme.Match = theme.Accent
	}
	return theme
}

// applyHerdr takes the tokens herdr's theme names, where the user overrode
// them, and reports whether one of them named the match colour. The names are
// herdr's own: accent is the colour its interface marks focus with, overlay0
// its faintest line colour, surface0 the shade it lays behind a selected row,
// and overlay1 a colour that still reads against both.
func applyHerdr(theme *Theme, tokens map[string]string) bool {
	set(&theme.Accent, tokens["accent"])
	set(&theme.Rule, tokens["overlay0"])
	set(&theme.Selected, tokens["surface0"])
	return set(&theme.Match, tokens["overlay1"])
}

func applyPlugin(theme *Theme, own Custom) bool {
	set(&theme.Accent, own.Colours["accent"])
	set(&theme.Rule, own.Colours["rule"])
	set(&theme.Selected, own.Colours["selected_background"])
	matched := set(&theme.Match, own.Colours["match"])
	set(&theme.Meta, own.Colours["meta"])
	set(&theme.Faint, own.Colours["faint"])
	set(&theme.Scrollbar, own.Colours["scrollbar"])
	set(&theme.Failure, own.Colours["failure"])
	set(&theme.Hovered, own.Colours["hover_background"])
	set(&theme.Menu, own.Colours["menu_background"])
	set(&theme.MenuHovered, own.Colours["menu_hover_background"])
	set(&theme.FieldHovered, own.Colours["field_hover_background"])
	// lipgloss reads a value it cannot parse as no colour, which the terminal
	// would be told is black, so such a value leaves the background derived.
	if background := lipgloss2.Color(own.Colours["background"]); background != (lipgloss2.NoColor{}) {
		theme.Background = background
	}
	for status, colour := range own.Status {
		if colour == "" {
			continue
		}
		theme.Status[status] = lipgloss.Color(colour)
	}
	return matched
}

// set takes a colour if one was given, and reports whether it did. A value is
// either a hex colour or an ANSI index, which is what lipgloss.Color accepts.
func set(target *lipgloss.TerminalColor, value string) bool {
	if value == "" {
		return false
	}
	*target = lipgloss.Color(value)
	return true
}
