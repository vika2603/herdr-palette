package palette

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/keys"
)

func editFile(t *testing.T, path string) herdr.PluginPaneOpenParams {
	t.Helper()
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	exec := Exec{Client: env.Client(), Ctx: &herdr.PluginInvocationContext{}, Env: env}
	if err := EditFile(context.Background(), exec, "herdr config", path); err != nil {
		t.Fatalf("EditFile() = %v", err)
	}

	var params herdr.PluginPaneOpenParams
	decode(t, server.Calls()[0].Params, &params)
	return params
}

// The editor is the one the environment herdr passed the plugin names, which
// the shell that runs the command line resolves.
func TestEditFileOpensThePathInTheEnvironmentsEditor(t *testing.T) {
	params := editFile(t, "/home/vika/.config/herdr/config.toml")

	line := params.Env[RunEnv]
	if !strings.HasPrefix(line, "${VISUAL:-${EDITOR:-vi}} ") {
		t.Errorf("command = %q, want the editor the environment names", line)
	}
	if !strings.HasSuffix(line, "'/home/vika/.config/herdr/config.toml'") {
		t.Errorf("command = %q, want the file to edit", line)
	}
	if params.Placement == nil || *params.Placement != herdr.PluginPanePlacementPopup {
		t.Errorf("placement = %v, want a popup of the plugin's own", params.Placement)
	}
	if params.Width == nil || params.Width.Percent == 0 {
		t.Error("the editor opens at herdr's default popup size, which is smaller than a file")
	}
}

// The path is the one thing about the command line this plugin composes.
func TestEditFileQuotesThePath(t *testing.T) {
	line := editFile(t, "/tmp/it's here/config.toml").Env[RunEnv]

	if !strings.HasSuffix(line, `'/tmp/it'\''s here/config.toml'`) {
		t.Errorf("command = %q, want the path quoted for the shell that runs it", line)
	}
}

func TestEditFileWithNoPath(t *testing.T) {
	server := plugintest.NewServer(t)
	env := server.Env(plugintest.StateDir(t.TempDir()))

	exec := Exec{Client: env.Client(), Ctx: &herdr.PluginInvocationContext{}, Env: env}
	if err := EditFile(context.Background(), exec, "herdr config", ""); err == nil {
		t.Fatal("EditFile() opened an editor on nothing")
	}
	if len(server.Calls()) != 0 {
		t.Error("a pane was opened although there is no file to edit")
	}
}

func focusedContext() *herdr.PluginInvocationContext {
	return &herdr.PluginInvocationContext{
		WorkspaceID:    new("w1"),
		TabID:          new("w1:t2"),
		FocusedPaneID:  new("w1:p3"),
		FocusedPaneCwd: new("/tmp"),
	}
}

// herdr runs a configured command with the workspace, tab and pane the key
// was pressed in, and command lines are written against them. Run from the
// palette's popup, whose own environment describes the popup instead, a
// command that reads them would act on nothing.
func TestAShellCommandSeesWhereTheKeyWasPressed(t *testing.T) {
	out := filepath.Join(t.TempDir(), "seen")
	command := `printf '%s %s %s %s' "$HERDR_ACTIVE_WORKSPACE_ID" "$HERDR_ACTIVE_TAB_ID" "$HERDR_ACTIVE_PANE_ID" "$HERDR_ACTIVE_PANE_CWD" > ` + shellQuote(out)
	if runtime.GOOS == "windows" {
		// The quoted path is what goes wrong if cmd.exe is handed the line
		// quoted as an argument.
		command = `echo %HERDR_ACTIVE_WORKSPACE_ID% %HERDR_ACTIVE_TAB_ID% %HERDR_ACTIVE_PANE_ID% %HERDR_ACTIVE_PANE_CWD%> "` + out + `"`
	}
	entries := customEntries("herdr.palette", []keys.Custom{{Type: keys.TypeShell, Command: command}})
	if err := entries[0].Run(context.Background(), Exec{Ctx: focusedContext()}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	// The command runs detached, so it is waited for by what it writes.
	deadline := time.Now().Add(5 * time.Second)
	var seen []byte
	for time.Now().Before(deadline) {
		if seen, _ = os.ReadFile(out); len(seen) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	// cmd.exe's echo ends the line, which printf does not.
	if got, want := strings.TrimSpace(string(seen)), "w1 w1:t2 w1:p3 /tmp"; got != want {
		t.Errorf("the command saw %q, want %q", got, want)
	}
}

func TestACommandInAWindowSeesWhereTheKeyWasPressed(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	entries := customEntries("herdr.palette", []keys.Custom{{Type: keys.TypePopup, Command: "lazygit"}})

	if err := entries[0].Run(context.Background(), Exec{Client: env.Client(), Ctx: focusedContext(), Env: env}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	var params herdr.PluginPaneOpenParams
	decode(t, server.Calls()[0].Params, &params)
	for name, want := range map[string]string{
		"HERDR_ACTIVE_WORKSPACE_ID": "w1",
		"HERDR_ACTIVE_TAB_ID":       "w1:t2",
		"HERDR_ACTIVE_PANE_ID":      "w1:p3",
		"HERDR_ACTIVE_PANE_CWD":     "/tmp",
		RunEnv:                      "lazygit",
	} {
		if got := params.Env[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
