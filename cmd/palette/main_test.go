package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/settings"
)

// The manifest and the registered entrypoints have to agree: herdr runs the
// binary with the id from the manifest, and an id with no handler fails at
// the keypress.
func TestManifestMatchesTheRegisteredEntrypoints(t *testing.T) {
	plugintest.CheckManifest(t, "../../herdr-plugin.toml", newPlugin())
}

func writeScript(t *testing.T, dir string) string {
	t.Helper()
	name, body := "context.sh", "#!/bin/sh\n# @palette.title Script context\nprintf '%s' \"$HERDR_ACTIVE_PANE_ID\" > observed\n"
	if runtime.GOOS == "windows" {
		name, body = "context.cmd", "REM @palette.title Script context\r\n@echo %HERDR_ACTIVE_PANE_ID%>observed\r\n"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEntriesIncludeScriptsAlongsideExistingCommands(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir)
	server := plugintest.NewServer(t)
	list, err := entries(context.Background(), server.Env(), keys.Config{
		Custom: []keys.Custom{{Key: "ctrl+f", Command: "configured", Type: keys.TypeShell}},
	}, settings.Settings{ScriptDirs: []string{dir}})
	// An unavailable API must leave both local command sources usable.
	if err == nil {
		t.Fatal("expected the unavailable API to be reported")
	}
	ids := make(map[string]bool)
	for _, entry := range list.Rows(palette.ScopeCommands) {
		ids[entry.ID] = true
	}
	if !ids["script:"+path] || !ids["config:ctrl+f"] {
		t.Fatalf("commands = %v, want the script and configured command", ids)
	}
}

func TestExecReloadsTheConfiguredScriptDirectory(t *testing.T) {
	configDir, scriptsDir, cwd := t.TempDir(), t.TempDir(), t.TempDir()
	path := writeScript(t, scriptsDir)
	if err := os.WriteFile(filepath.Join(configDir, settings.File), []byte("script_dirs = ["+strconv.Quote(scriptsDir)+"]"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := plugintest.NewServer(t)
	env := server.Env(plugintest.ConfigDir(configDir), plugintest.StateDir(t.TempDir()))
	if err := env.WriteStateJSON(palette.PendingFile, palette.Pending{
		EntryID: "script:" + path,
		Context: &herdr.PluginInvocationContext{FocusedPaneID: new("source-pane"), FocusedPaneCwd: &cwd},
	}); err != nil {
		t.Fatal(err)
	}
	if err := onExec(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(filepath.Join(cwd, "observed"))
		if strings.TrimSpace(string(data)) == "source-pane" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("script output = %q, want the captured pane in the captured directory", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunPaneExecutesTheScriptFile(t *testing.T) {
	path, cwd := writeScript(t, t.TempDir()), t.TempDir()
	t.Chdir(cwd)
	t.Setenv(palette.RunEnv, "")
	t.Setenv(palette.ScriptEnv, path)
	t.Setenv("HERDR_ACTIVE_PANE_ID", "source-pane")
	if err := onRun(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cwd, "observed"))
	if err != nil || strings.TrimSpace(string(data)) != "source-pane" {
		t.Fatalf("script output = %q, %v", data, err)
	}
}

func TestRunPanePassesTheArgumentsOn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows scripts read their arguments from the environment only")
	}
	dir, cwd := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "args.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s|' \"$#\" \"$1\" \"$2\" > observed\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	t.Setenv(palette.RunEnv, "")
	t.Setenv(palette.ScriptEnv, path)
	t.Setenv(palette.ArgsEnv, `["two words",""]`)
	if err := onRun(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cwd, "observed"))
	if err != nil || string(data) != "2|two words||" {
		t.Fatalf("script output = %q, %v, want both arguments in place", data, err)
	}
}
