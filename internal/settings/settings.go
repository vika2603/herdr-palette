// Package settings reads the palette's own configuration file, the one in the
// plugin's config directory: the commands the user added to the list, and the
// colours they chose for it.
//
// It is where a command that no key is worth goes. herdr runs a
// [[keys.command]] entry from a key and nothing else, so a script reached
// through the palette would otherwise need a binding it never uses — and a
// command written here runs in the background unless it asks for a window.
package settings

import (
	"github.com/BurntSushi/toml"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/manifest"

	"github.com/vika2603/herdr-palette/internal/theme"
)

// File is the plugin's configuration, in the directory
// `herdr plugin config-dir herdr.palette` prints.
const File = "config.toml"

// Windows a command can ask for. Without one it runs in the background, where
// nothing is shown and its output goes nowhere.
const (
	// WindowTab runs the command in a tab of its own without leaving the one
	// you are in, which is the window you can come back to: the palette lists
	// its pane, so "go to" reaches it later.
	WindowTab   = "tab"
	WindowPopup = "popup"
	WindowPane  = "pane"
)

// Command is one [[command]] entry: what the row says, what it runs, and the
// window it runs in, if any.
type Command struct {
	Title  string             `toml:"title"`
	Run    string             `toml:"run"`
	Window string             `toml:"window"`
	Width  manifest.PopupSize `toml:"width"`
	Height manifest.PopupSize `toml:"height"`
}

// Window is the size of the palette's own popup. herdr centres a popup in the
// pane area and offers no say over where it sits, so the height is also what
// decides how high up it starts: a taller one begins closer to the top.
type Window struct {
	Width  manifest.PopupSize `toml:"width"`
	Height manifest.PopupSize `toml:"height"`
}

// Settings is what the file says.
type Settings struct {
	Commands []Command
	// Window is the size the palette asks for, empty where the file says
	// nothing and the manifest's own size stands.
	Window Window
	// Theme are the colours the file overrides.
	Theme theme.Custom
}

// file is the whole document, decoded in one pass.
type file struct {
	Command []Command `toml:"command"`
	Window  Window    `toml:"window"`

	Rule               string            `toml:"rule"`
	SelectedBackground string            `toml:"selected_background"`
	Match              string            `toml:"match"`
	Meta               string            `toml:"meta"`
	Scrollbar          string            `toml:"scrollbar"`
	Failure            string            `toml:"failure"`
	Status             map[string]string `toml:"status"`
}

// Load reads the file. An unreadable or missing one leaves the palette with
// its own colours and no commands of the user's own, which is what no
// configuration at all looks like.
func Load(env *plugin.Env) Settings {
	if env == nil {
		return Settings{}
	}

	var parsed file
	if _, err := toml.DecodeFile(env.ConfigPath(File), &parsed); err != nil {
		return Settings{}
	}

	return Settings{
		Commands: commands(parsed.Command),
		Window:   parsed.Window,
		Theme: theme.Custom{
			Colours: map[string]string{
				"rule":                parsed.Rule,
				"selected_background": parsed.SelectedBackground,
				"match":               parsed.Match,
				"meta":                parsed.Meta,
				"scrollbar":           parsed.Scrollbar,
				"failure":             parsed.Failure,
			},
			Status: parsed.Status,
		},
	}
}

// commands drops the entries that cannot be run and names the ones that left
// out their title after what they run.
func commands(configured []Command) []Command {
	out := make([]Command, 0, len(configured))
	for _, command := range configured {
		if command.Run == "" {
			continue
		}
		if command.Title == "" {
			command.Title = command.Run
		}
		out = append(out, command)
	}
	return out
}
