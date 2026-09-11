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

func TestACommandRunsInTheBackgroundUnlessItAsksForAWindow(t *testing.T) {
	own := configured(t, `
[[command]]
title = "Sync dotfiles"
run = "zsh sync.sh"

[[command]]
title = "Watch tests"
run = "just watch"
window = "tab"

[[command]]
title = "Open lazygit"
run = "lazygit"
window = "popup"
width = "80%"
`)

	if len(own.Commands) != 3 {
		t.Fatalf("read %d commands, want the three that are configured", len(own.Commands))
	}
	if own.Commands[0].Window != "" {
		t.Errorf("window = %q, want a command that says nothing to run in the background", own.Commands[0].Window)
	}
	if own.Commands[1].Window != WindowTab {
		t.Errorf("window = %q, want the tab it asked for", own.Commands[1].Window)
	}
	if own.Commands[2].Width.Percent != 80 {
		t.Errorf("width = %v, want the configured percentage", own.Commands[2].Width)
	}
}

func TestACommandWithNothingToRunIsDropped(t *testing.T) {
	own := configured(t, "[[command]]\ntitle = \"Nothing\"\n")

	if len(own.Commands) != 0 {
		t.Errorf("read %+v, want an entry with nothing to run left out", own.Commands)
	}
}

func TestACommandIsNamedAfterWhatItRuns(t *testing.T) {
	own := configured(t, "[[command]]\nrun = \"zsh sync.sh\"\n")

	if len(own.Commands) != 1 || own.Commands[0].Title != "zsh sync.sh" {
		t.Errorf("read %+v, want the command line as the title", own.Commands)
	}
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

	if len(own.Commands) != 0 {
		t.Errorf("read %+v with no file to read", own.Commands)
	}
}
