// Package settings reads the palette's own configuration file, the one in the
// plugin's config directory: the size of the popup, the colours it is drawn
// in, and the directories containing palette scripts.
package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

// Settings is the resolved palette configuration.
type Settings struct {
	// Window is the size the palette asks for, empty where the file says
	// nothing and the manifest's own size stands.
	Window Window
	// Theme are the colours the file overrides.
	Theme theme.Custom
	// ScriptDirs are the resolved directories searched for palette scripts.
	ScriptDirs []string
}

// file is the whole document, decoded in one pass.
type file struct {
	Window     Window   `toml:"window"`
	ScriptDirs []string `toml:"script_dirs"`

	Scheme             string            `toml:"scheme"`
	Accent             string            `toml:"accent"`
	Rule               string            `toml:"rule"`
	SelectedBackground string            `toml:"selected_background"`
	Match              string            `toml:"match"`
	Meta               string            `toml:"meta"`
	Faint              string            `toml:"faint"`
	Scrollbar          string            `toml:"scrollbar"`
	Failure            string            `toml:"failure"`
	Status             map[string]string `toml:"status"`
}

// Load reads the file. An unreadable or missing one leaves the palette with
// its own size and colours and the default scripts directory.
func Load(env *plugin.Env) Settings {
	if env == nil {
		return Settings{}
	}

	defaultDirs := defaultScriptDirs(env)
	var parsed file
	meta, err := toml.DecodeFile(env.ConfigPath(File), &parsed)
	if err != nil {
		return Settings{ScriptDirs: defaultDirs}
	}
	dirs := defaultDirs
	if meta.IsDefined("script_dirs") {
		dirs = resolveScriptDirs(env, parsed.ScriptDirs)
	}

	return Settings{
		Window:     parsed.Window,
		ScriptDirs: dirs,
		Theme: theme.Custom{
			Scheme: parsed.Scheme,
			Colours: map[string]string{
				"accent":              parsed.Accent,
				"rule":                parsed.Rule,
				"selected_background": parsed.SelectedBackground,
				"match":               parsed.Match,
				"meta":                parsed.Meta,
				"faint":               parsed.Faint,
				"scrollbar":           parsed.Scrollbar,
				"failure":             parsed.Failure,
			},
			Status: parsed.Status,
		},
	}
}

func defaultScriptDirs(env *plugin.Env) []string {
	if path := env.ConfigPath("scripts"); path != "" {
		return []string{path}
	}
	return nil
}

func resolveScriptDirs(env *plugin.Env, configured []string) []string {
	dirs := make([]string, 0, len(configured))
	for _, path := range configured {
		if path == "" {
			continue
		}
		if resolved := resolveScriptDir(env, path); resolved != "" {
			dirs = append(dirs, resolved)
		}
	}
	return dirs
}

func resolveScriptDir(env *plugin.Env, configured string) string {
	if strings.HasPrefix(configured, "~/") || runtime.GOOS == "windows" && strings.HasPrefix(configured, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		return filepath.Join(home, configured[2:])
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	base := env.ConfigPath()
	if base == "" {
		return ""
	}
	return filepath.Join(base, configured)
}
