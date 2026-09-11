package keys

import (
	"os"
	"testing"
)

func loadTestdata(t *testing.T) Config {
	t.Helper()
	raw, err := os.ReadFile("testdata/default-config.toml")
	if err != nil {
		t.Fatalf("reading the default config: %v", err)
	}
	cfg := Config{Action: parseDefaults(string(raw)), Plugin: map[string]string{}}
	apply(&cfg, "testdata/config.toml")
	return cfg
}

func TestDefaultsCoverTheBuiltInActions(t *testing.T) {
	cfg := loadTestdata(t)

	for name, want := range map[string]string{
		"new_tab":        "prefix+c",
		"close_pane":     "prefix+x",
		"zoom":           "prefix+z",
		"rename_pane":    "prefix+shift+p",
		"split_vertical": "prefix+v",
	} {
		if got := cfg.Action[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestActionsWithoutADefaultAreAbsent(t *testing.T) {
	cfg := loadTestdata(t)

	if got, ok := cfg.Action["open_worktree"]; ok {
		t.Errorf("open_worktree = %q, want an action that ships unbound to be absent", got)
	}
}

func TestUserConfigOverridesAndUnbinds(t *testing.T) {
	cfg := loadTestdata(t)

	if got := cfg.Action["new_workspace"]; got != "prefix+alt+n" {
		t.Errorf("new_workspace = %q, want the value from config.toml", got)
	}
	if got, ok := cfg.Action["previous_tab"]; ok {
		t.Errorf("previous_tab = %q, want an empty value to unbind it", got)
	}
}

func TestCustomCommandsAreCollected(t *testing.T) {
	cfg := loadTestdata(t)

	if len(cfg.Custom) != 2 {
		t.Fatalf("collected %d custom commands, want the two that are not plugin actions", len(cfg.Custom))
	}
	first := cfg.Custom[0]
	if first.Key != "prefix+f" || first.Type != TypePopup || first.Command != "zsh jump.sh" {
		t.Errorf("first custom command = %+v", first)
	}
	if first.Width != "70%" || first.Height != "60%" {
		t.Errorf("popup size = %q x %q, want the configured size", first.Width, first.Height)
	}
}

func TestPluginActionBindingsAreKeptApart(t *testing.T) {
	cfg := loadTestdata(t)

	if got := cfg.Plugin["herdr.machine-manager.open"]; got != "prefix+shift+s" {
		t.Errorf("machine manager binding = %q, want prefix+shift+s", got)
	}
	for _, custom := range cfg.Custom {
		if custom.Type == TypePluginAction {
			t.Error("a plugin action was also listed as a custom command, which would show it twice")
		}
	}
}
