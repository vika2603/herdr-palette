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
)

// Config is what the configuration says about commands and their keys.
type Config struct {
	// Action maps a built-in action name, such as new_workspace, to its key.
	Action map[string]string
	// Plugin maps "<plugin id>.<action id>" to its key.
	Plugin map[string]string
	// Custom are the shell, pane and popup commands from [[keys.command]].
	Custom []Custom
}

// Custom is one [[keys.command]] entry other than a plugin action.
type Custom struct {
	Key         string
	Description string
	Type        string
	Command     string
	Width       string
	Height      string
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
	cfg := Config{Action: defaults(herdrBin), Plugin: map[string]string{}}
	if cfg.Action == nil {
		cfg.Action = map[string]string{}
	}
	apply(&cfg, ConfigPath())
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
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		switch {
		case trimmed == "[keys]":
			inKeys = true
			continue
		case !inKeys:
			continue
		case strings.HasPrefix(trimmed, "[[keys.command]]"), strings.HasPrefix(trimmed, "[keys.indexed]"):
			return actions
		case strings.HasPrefix(trimmed, "["):
			return actions
		}
		if m := binding.FindStringSubmatch(trimmed); m != nil && m[2] != "" {
			actions[m[1]] = m[2]
		}
	}
	return actions
}

// userConfig is the part of config.toml this package reads. Action bindings
// are plain string values under [keys]; commands are its one array of tables.
type userConfig struct {
	Keys map[string]any `toml:"keys"`
}

func apply(cfg *Config, path string) {
	if path == "" {
		return
	}
	var parsed userConfig
	if _, err := toml.DecodeFile(path, &parsed); err != nil {
		return
	}

	// Keys the configuration assigns explicitly, and the actions that were
	// assigned one. A default binding whose key is taken elsewhere is dropped
	// below rather than shown for two commands.
	taken := map[string]bool{}
	configured := map[string]bool{}

	for name, value := range parsed.Keys {
		text, ok := value.(string)
		if !ok || name == "prefix" {
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

	commands, _ := parsed.Keys["command"].([]map[string]any)
	for _, entry := range commands {
		custom := Custom{
			Key:         text(entry, "key"),
			Description: text(entry, "description"),
			Type:        text(entry, "type"),
			Command:     text(entry, "command"),
			Width:       text(entry, "width"),
			Height:      text(entry, "height"),
		}
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

func text(entry map[string]any, field string) string {
	value, _ := entry[field].(string)
	return value
}
