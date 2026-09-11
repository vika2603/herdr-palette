// Package settings reads the palette's own configuration file, the one in the
// plugin's config directory: the size of the popup and the colours it is drawn
// in. Neither is available over herdr's API.
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

// Window is the size of the palette's own popup. herdr centres a popup in the
// pane area and offers no say over where it sits, so the height is also what
// decides how high up it starts: a taller one begins closer to the top.
type Window struct {
	Width  manifest.PopupSize `toml:"width"`
	Height manifest.PopupSize `toml:"height"`
}

// Settings is what the file says.
type Settings struct {
	// Window is the size the palette asks for, empty where the file says
	// nothing and the manifest's own size stands.
	Window Window
	// Theme are the colours the file overrides.
	Theme theme.Custom
}

// file is the whole document, decoded in one pass.
type file struct {
	Window Window `toml:"window"`

	Rule               string            `toml:"rule"`
	SelectedBackground string            `toml:"selected_background"`
	Match              string            `toml:"match"`
	Meta               string            `toml:"meta"`
	Scrollbar          string            `toml:"scrollbar"`
	Failure            string            `toml:"failure"`
	Status             map[string]string `toml:"status"`
}

// Load reads the file. An unreadable or missing one leaves the palette with
// its own size and colours, which is what no configuration at all looks like.
func Load(env *plugin.Env) Settings {
	if env == nil {
		return Settings{}
	}

	var parsed file
	if _, err := toml.DecodeFile(env.ConfigPath(File), &parsed); err != nil {
		return Settings{}
	}

	return Settings{
		Window: parsed.Window,
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
