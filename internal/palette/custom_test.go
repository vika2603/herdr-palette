package palette

import (
	"context"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
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
