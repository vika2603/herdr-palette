package keys

import (
	"os"
	"path/filepath"
	"testing"
)

func loadTestdata(t *testing.T) Config {
	t.Helper()
	raw, err := os.ReadFile("testdata/default-config.toml")
	if err != nil {
		t.Fatalf("reading the default config: %v", err)
	}
	cfg := newConfig(parseDefaults(string(raw)))
	apply(&cfg, "testdata/config.toml")
	return cfg
}

func TestThePrefixIsReadWithTheBindings(t *testing.T) {
	cfg := loadTestdata(t)

	if cfg.Prefix != "ctrl+b" {
		t.Errorf("prefix = %q, want ctrl+b", cfg.Prefix)
	}
	if got, ok := cfg.Action["prefix"]; ok {
		t.Errorf("prefix = %q was listed as an action binding", got)
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\nprefix = \"ctrl+a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	apply(&cfg, path)
	if cfg.Prefix != "ctrl+a" {
		t.Errorf("prefix = %q, want the value from config.toml", cfg.Prefix)
	}
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

	if len(cfg.Custom) != 3 {
		t.Fatalf("collected %d custom commands, want the three that are not plugin actions", len(cfg.Custom))
	}
	first := cfg.Custom[0]
	if first.Key != "prefix+f" || first.Type != TypePopup || first.Command != "zsh jump.sh" {
		t.Errorf("first custom command = %+v", first)
	}
	if first.Width.Percent != 70 || first.Height.Percent != 60 {
		t.Errorf("popup size = %v x %v, want the configured percentages", first.Width, first.Height)
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

// herdr accepts a popup size as a cell count as well as a percentage.
func TestPopupSizeInCells(t *testing.T) {
	cfg := loadTestdata(t)

	var cells Custom
	for _, custom := range cfg.Custom {
		if custom.Key == "prefix+alt+t" {
			cells = custom
		}
	}
	if cells.Width.Percent != 0 || cells.Width.Cells != 80 {
		t.Errorf("width = %v, want 80 cells", cells.Width)
	}
	if cells.Height.Cells != 24 {
		t.Errorf("height = %v, want 24 cells", cells.Height)
	}
}

func TestThemeTokensAreRead(t *testing.T) {
	cfg := loadTestdata(t)

	if got := cfg.Theme["overlay0"]; got != "#414868" {
		t.Errorf("overlay0 = %q, want the token from [theme.custom]", got)
	}
}
