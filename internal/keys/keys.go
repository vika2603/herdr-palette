// Package keys reads what the palette cannot get from the socket API: which
// key each command is bound to, and the commands the user defined under
// [[keys.command]].
//
// herdr exposes neither over the API, so this reads the same two sources herdr
// does — the defaults it prints with --default-config, and config.toml.
package keys

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/vika2603/herdr-client/plugin/manifest"
)

// Config is what herdr's configuration says about commands, their keys, and
// the theme tokens the user overrode.
type Config struct {
	// Prefix is the key that starts a prefix chord, as [keys] spells it.
	Prefix string
	// Action maps a built-in action name, such as new_workspace, to its key.
	Action map[string]string
	// Plugin maps a plugin action, as PluginBinding spells it, to its key.
	Plugin map[string]string
	// Custom are the shell, pane and popup commands from [[keys.command]].
	Custom []Custom
	// Theme is the [theme.custom] table, token name to colour.
	Theme map[string]string
}

// Custom is one [[keys.command]] entry other than a plugin action. The sizes
// are decoded by the manifest package, which accepts both spellings herdr
// does: a cell count and a percentage string.
type Custom struct {
	Key         string             `toml:"key"`
	Description string             `toml:"description"`
	Type        string             `toml:"type"`
	Command     string             `toml:"command"`
	Width       manifest.PopupSize `toml:"width"`
	Height      manifest.PopupSize `toml:"height"`
}

// PluginBinding is how a plugin action is named in [[keys.command]], and the
// key under which its binding is stored.
func PluginBinding(pluginID, actionID string) string {
	return pluginID + "." + actionID
}

// Command types herdr accepts in [[keys.command]].
const (
	TypeShell        = "shell"
	TypePane         = "pane"
	TypePopup        = "popup"
	TypePluginAction = "plugin_action"
)

// Load reads the defaults from the herdr binary and lays config.toml over
// them. Anything unreadable only costs the key column, so failures are
// answered with what could be read.
func Load(herdrBin string) Config {
	cfg := newConfig(defaults(herdrBin))
	apply(&cfg, ConfigPath())
	return cfg
}

// Commands reads only what the user configured, skipping the defaults that
// only the key column needs. It saves the exec entrypoint a subprocess.
func Commands() Config {
	cfg := newConfig(nil)
	apply(&cfg, ConfigPath())
	return cfg
}

func newConfig(actions map[string]string) Config {
	if actions == nil {
		actions = map[string]string{}
	}
	cfg := Config{Action: actions, Plugin: map[string]string{}, Theme: map[string]string{}}
	// The prefix shares the [keys] table with the action bindings, so the
	// defaults carry it as one; it is a key of its own rather than an action's.
	cfg.Prefix = actions[prefixKey]
	delete(actions, prefixKey)
	return cfg
}

// ConfigPath is the file herdr reads, honouring the override it documents.
func ConfigPath() string {
	if path := os.Getenv("HERDR_CONFIG_PATH"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "herdr", "config.toml")
}

// binding matches a key assignment, commented out or not.
var binding = regexp.MustCompile(`^#?\s*([a-z_]+)\s*=\s*"([^"]*)"`)

// defaults parses the [keys] section of `herdr --default-config`, where every
// built-in action appears as a commented-out assignment.
//
// The prose in that section also contains lines shaped like assignments, such
// as the `type = "popup"` that documents custom commands, and they are
// collected too. Lookups are by a fixed set of action names, so a value under
// a name that is not an action is never read.
func defaults(herdrBin string) map[string]string {
	if herdrBin == "" {
		herdrBin = "herdr"
	}
	out, err := exec.Command(herdrBin, "--default-config").Output()
	if err != nil {
		return nil
	}
	return parseDefaults(string(out))
}

func parseDefaults(config string) map[string]string {
	actions := map[string]string{}
	inKeys := false
	for line := range strings.SplitSeq(config, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		switch {
		case trimmed == "[keys]":
			inKeys = true
			continue
		case !inKeys:
			continue
		case strings.HasPrefix(trimmed, "["):
			// Any other section ends [keys], including the [[keys.command]]
			// example and [keys.indexed].
			return actions
		}
		if m := binding.FindStringSubmatch(trimmed); m != nil && m[2] != "" {
			actions[m[1]] = m[2]
		}
	}
	return actions
}

// userConfig is the part of config.toml this package reads. Under [keys],
// action bindings are plain strings and commands are one array of tables, so
// the values stay undecoded until their shape is known.
type userConfig struct {
	Keys  map[string]toml.Primitive `toml:"keys"`
	Theme struct {
		Custom map[string]string `toml:"custom"`
	} `toml:"theme"`
}

func apply(cfg *Config, path string) {
	if path == "" {
		return
	}
	var parsed userConfig
	meta, err := toml.DecodeFile(path, &parsed)
	if err != nil {
		return
	}
	cfg.Theme = parsed.Theme.Custom

	// Keys the configuration assigns explicitly, and the actions that were
	// assigned one. A default binding whose key is taken elsewhere is dropped
	// below rather than shown for two commands.
	taken := map[string]bool{}
	configured := map[string]bool{}

	for name, value := range parsed.Keys {
		if name == commandsKey {
			continue
		}
		var text string
		// A value that is not a key assignment, such as [keys.indexed], is not
		// an action binding.
		if err := meta.PrimitiveDecode(value, &text); err != nil {
			continue
		}
		if name == prefixKey {
			if text != "" {
				cfg.Prefix = text
			}
			continue
		}
		if text == "" {
			// An explicit empty value unbinds the action.
			delete(cfg.Action, name)
			configured[name] = true
			continue
		}
		cfg.Action[name] = text
		configured[name] = true
		taken[text] = true
	}

	var commands []Custom
	if primitive, ok := parsed.Keys[commandsKey]; ok {
		_ = meta.PrimitiveDecode(primitive, &commands)
	}
	for _, custom := range commands {
		if custom.Key != "" {
			taken[custom.Key] = true
		}
		if custom.Type == TypePluginAction {
			// The action itself already reaches the palette through
			// plugin.action.list; only its key is news.
			cfg.Plugin[custom.Command] = custom.Key
			continue
		}
		if custom.Command != "" {
			cfg.Custom = append(cfg.Custom, custom)
		}
	}

	for name, key := range cfg.Action {
		if !configured[name] && taken[key] {
			delete(cfg.Action, name)
		}
	}
}

// commandsKey is the [[keys.command]] array of tables and prefixKey the prefix
// assignment, both of which share the [keys] table with the action bindings.
const (
	commandsKey = "command"
	prefixKey   = "prefix"
)
