package keys

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func loadTestdata(t *testing.T) Config {
	t.Helper()
	cfg := loadDefaults(t)
	apply(&cfg, "testdata/config.toml")
	return cfg
}

func loadDefaults(t *testing.T) Config {
	t.Helper()
	raw, err := os.ReadFile("testdata/default-config.toml")
	if err != nil {
		t.Fatalf("reading the default config: %v", err)
	}
	return newConfig(parseDefaults(string(raw)))
}

func TestThePrefixIsReadWithTheBindings(t *testing.T) {
	cfg := loadTestdata(t)

	if !slices.Equal(cfg.Prefixes, []string{"ctrl+b"}) {
		t.Errorf("prefixes = %q, want ctrl+b", cfg.Prefixes)
	}
	if got, ok := cfg.Action["prefix"]; ok {
		t.Errorf("prefix = %q was listed as an action binding", got)
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[keys]\nprefix = \"ctrl+a\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	apply(&cfg, path)
	if !slices.Equal(cfg.Prefixes, []string{"ctrl+a"}) {
		t.Errorf("prefixes = %q, want the value from config.toml", cfg.Prefixes)
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

	if got := cfg.Plugin["herdr.machine-manager.open"]; !slices.Equal(got, []string{"prefix+shift+s"}) {
		t.Errorf("machine manager binding = %q, want prefix+shift+s", got)
	}
	for _, custom := range cfg.Custom {
		if custom.Type == TypePluginAction {
			t.Error("a plugin action was also listed as a custom command, which would show it twice")
		}
	}
}

// applyConfig lays a config.toml with the given contents over the defaults.
func applyConfig(t *testing.T, contents string) Config {
	t.Helper()
	cfg := loadDefaults(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	apply(&cfg, path)
	return cfg
}

// herdr accepts an array wherever it accepts a key. An array in one command
// must not cost the commands after it.
func TestBindingsMayBeArrays(t *testing.T) {
	cfg := applyConfig(t, `
[keys]
new_tab = ["prefix+t", "alt+t"]
next_tab = []

[[keys.command]]
key = ["prefix+x", "alt+x"]
type = "shell"
command = "first"

[[keys.command]]
key = "alt+j"
type = "plugin_action"
command = "herdr.palette.toggle"

[[keys.command]]
key = ["alt+k"]
type = "plugin_action"
command = "herdr.palette.toggle"
`)

	if got := cfg.Action["new_tab"]; got != "prefix+t" {
		t.Errorf("new_tab = %q, want the first key of the array", got)
	}
	if got, ok := cfg.Action["next_tab"]; ok {
		t.Errorf("next_tab = %q, want an empty array to unbind it", got)
	}
	if got, ok := cfg.Action["close_pane"]; ok {
		t.Errorf("close_pane = %q, want its default dropped for the second key of a command", got)
	}
	if len(cfg.Custom) != 1 || cfg.Custom[0].Key != "prefix+x" || cfg.Custom[0].Command != "first" {
		t.Errorf("custom commands = %+v, want the one bound by an array", cfg.Custom)
	}
	if got := cfg.Plugin["herdr.palette.toggle"]; !slices.Equal(got, []string{"alt+j", "alt+k"}) {
		t.Errorf("toggle bindings = %q, want the keys of both entries", got)
	}
}

func TestACommandHerdrWouldRejectLeavesTheOthers(t *testing.T) {
	cfg := applyConfig(t, `
[[keys.command]]
key = 5
type = "shell"
command = "broken"

[[keys.command]]
key = "alt+j"
type = "shell"
command = "kept"
`)

	if len(cfg.Custom) != 1 || cfg.Custom[0].Command != "kept" {
		t.Errorf("custom commands = %+v, want only the well-formed one", cfg.Custom)
	}
}

func TestPrefixesMergeAsHerdrMergesThem(t *testing.T) {
	cfg := applyConfig(t, "[keys]\nprefix = [\"ctrl+space\", \"ctrl+s\"]\nextra_prefixes = \"f12\"\n")
	if want := []string{"ctrl+space", "ctrl+s", "f12"}; !slices.Equal(cfg.Prefixes, want) {
		t.Errorf("prefixes = %q, want %q", cfg.Prefixes, want)
	}
	if _, ok := cfg.Action["extra_prefixes"]; ok {
		t.Error("extra_prefixes was listed as an action binding")
	}

	// herdr rejects a prefix with no key and keeps the one it had.
	cfg = applyConfig(t, "[keys]\nprefix = []\n")
	if !slices.Equal(cfg.Prefixes, []string{"ctrl+b"}) {
		t.Errorf("prefixes = %q, want the default kept", cfg.Prefixes)
	}
}

func TestFullscreenBindsZoom(t *testing.T) {
	cfg := applyConfig(t, "[keys]\nfullscreen = \"prefix+f\"\n")
	if got := cfg.Action["zoom"]; got != "prefix+f" {
		t.Errorf("zoom = %q, want the binding given under its older name", got)
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

// The palette reads the file herdr reads, found the way herdr finds it.
func TestTheConfigIsWhereHerdrLooks(t *testing.T) {
	for _, c := range []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{"override", "windows", map[string]string{"HERDR_CONFIG_PATH": `D:\herdr.toml`, "APPDATA": `C:\roaming`}, `D:\herdr.toml`},
		{"xdg first", "windows", map[string]string{"XDG_CONFIG_HOME": "/xdg", "APPDATA": `C:\roaming`}, filepath.Join("/xdg", "herdr", "config.toml")},
		{"appdata", "windows", map[string]string{"APPDATA": `C:\roaming`, "HOME": "/home/vika"}, filepath.Join(`C:\roaming`, "herdr", "config.toml")},
		{"profile", "windows", map[string]string{"USERPROFILE": `C:\Users\vika`}, filepath.Join(`C:\Users\vika`, "AppData", "Roaming", "herdr", "config.toml")},
		{"home on windows", "windows", map[string]string{"HOME": "/home/vika"}, filepath.Join("/home/vika", ".config", "herdr", "config.toml")},
		{"home", "darwin", map[string]string{"HOME": "/Users/vika", "APPDATA": `C:\roaming`}, filepath.Join("/Users/vika", ".config", "herdr", "config.toml")},
		{"xdg", "linux", map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/vika"}, filepath.Join("/xdg", "herdr", "config.toml")},
		{"nothing", "linux", nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := configPath(c.goos, func(name string) string { return c.env[name] }); got != c.want {
				t.Errorf("configPath = %q, want %q", got, c.want)
			}
		})
	}
}
