package settings

import (
	"os"
	"path/filepath"
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
	own := configured(t, "rule = \"#111111\"\n\n[status]\nworking = \"#fab387\"\n")

	if own.Theme.Colours["rule"] != "#111111" {
		t.Errorf("rule = %q, want the configured colour", own.Theme.Colours["rule"])
	}
	if own.Theme.Status["working"] != "#fab387" {
		t.Errorf("working = %q, want the configured colour", own.Theme.Status["working"])
	}
}

func TestNoConfigurationFile(t *testing.T) {
	own := Load(plugintest.Env(plugintest.ConfigDir(t.TempDir())))

	if own.Window != (Window{}) {
		t.Errorf("read %+v with no file to read", own.Window)
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
