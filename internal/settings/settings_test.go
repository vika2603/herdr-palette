package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func configured(t *testing.T, content string) Settings {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte(content), 0o600); err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}
	return Load(plugintest.Env(plugintest.ConfigDir(dir)))
}

func TestTheColoursReachTheTheme(t *testing.T) {
	own := configured(t, "rule = \"#111111\"\nbackground = \"#181825\"\n\n[status]\nworking = \"#fab387\"\n")

	if own.Theme.Colours["rule"] != "#111111" {
		t.Errorf("rule = %q, want the configured colour", own.Theme.Colours["rule"])
	}
	if own.Theme.Colours["background"] != "#181825" {
		t.Errorf("background = %q, want the configured colour", own.Theme.Colours["background"])
	}
	if own.Theme.Status["working"] != "#fab387" {
		t.Errorf("working = %q, want the configured colour", own.Theme.Status["working"])
	}
}

func TestNoConfigurationFile(t *testing.T) {
	dir := t.TempDir()
	own := Load(plugintest.Env(plugintest.ConfigDir(dir)))

	if own.Window != (Window{}) {
		t.Errorf("read %+v with no file to read", own.Window)
	}
	if want := []string{filepath.Join(dir, "scripts")}; !slices.Equal(own.ScriptDirs, want) {
		t.Errorf("script directories = %q, want %q", own.ScriptDirs, want)
	}
}

func TestValidConfigurationWithoutScriptDirsUsesDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte("accent = \"#f5c2e7\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	own := Load(plugintest.Env(plugintest.ConfigDir(dir)))
	if want := []string{filepath.Join(dir, "scripts")}; !slices.Equal(own.ScriptDirs, want) {
		t.Errorf("script directories = %q, want %q", own.ScriptDirs, want)
	}
}

func TestInvalidConfigurationKeepsDefaultScriptDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte("script_dirs = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	own := Load(plugintest.Env(plugintest.ConfigDir(dir)))
	if want := []string{filepath.Join(dir, "scripts")}; !slices.Equal(own.ScriptDirs, want) {
		t.Errorf("script directories = %q, want %q", own.ScriptDirs, want)
	}
}

func TestNilEnvironmentHasNoScriptDirectories(t *testing.T) {
	if got := Load(nil).ScriptDirs; len(got) != 0 {
		t.Errorf("script directories = %q, want empty", got)
	}
	if got := Load(plugintest.Env()).ScriptDirs; len(got) != 0 {
		t.Errorf("script directories without config path = %q, want empty", got)
	}
}

func TestConfiguredScriptDirectories(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	absolute := filepath.Join(t.TempDir(), "tools")
	cases := []struct {
		name       string
		configured []string
		want       []string
	}{
		{"explicit empty disables scripts", []string{}, []string{}},
		{"multiple paths keep order and skip empty", []string{"tools/local", "", absolute + string(filepath.Separator) + ".." + string(filepath.Separator) + "tools", "~/more"}, []string{filepath.Join(dir, "tools", "local"), absolute, filepath.Join(home, "more")}},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases, struct {
			name       string
			configured []string
			want       []string
		}{
			"windows tilde home", []string{`~\tools`}, []string{filepath.Join(home, "tools")},
		})
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			quoted := make([]string, len(tt.configured))
			for i, path := range tt.configured {
				quoted[i] = strconv.Quote(path)
			}
			content := "script_dirs = [" + strings.Join(quoted, ", ") + "]\n"
			if err := os.WriteFile(filepath.Join(dir, File), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			got := Load(plugintest.Env(plugintest.ConfigDir(dir))).ScriptDirs
			if !slices.Equal(got, tt.want) {
				t.Errorf("script directories = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfiguredScriptDirectoriesKeepWindowAndTheme(t *testing.T) {
	own := configured(t, "script_dirs = [\"tools\"]\naccent = \"#f5c2e7\"\n[window]\nwidth = \"70%\"\n")
	if own.Window.Width.Percent != 70 || own.Theme.Colours["accent"] != "#f5c2e7" {
		t.Errorf("window or theme changed: %+v", own)
	}
}

func TestTheWindowSizeIsRead(t *testing.T) {
	own := configured(t, "[window]\nwidth = \"70%\"\nheight = 30\n")

	if own.Window.Width.Percent != 70 {
		t.Errorf("width = %v, want the configured percentage", own.Window.Width)
	}
	if own.Window.Height.Cells != 30 {
		t.Errorf("height = %v, want the configured cell count", own.Window.Height)
	}
}

func TestTheSchemeAndAccentAreRead(t *testing.T) {
	own := configured(t, "scheme = \"terminal\"\naccent = \"#f5c2e7\"\nfaint = \"8\"\n")

	if own.Theme.Scheme != "terminal" {
		t.Errorf("scheme = %q, want the configured one", own.Theme.Scheme)
	}
	if own.Theme.Colours["accent"] != "#f5c2e7" || own.Theme.Colours["faint"] != "8" {
		t.Errorf("colours = %v, want the accent and faint colours read", own.Theme.Colours)
	}
}
