// Command palette is the herdr command palette plugin. One binary serves both
// manifest entrypoints: the action bound to a key, which opens the popup, and
// the popup pane that runs the TUI. See docs/design.md.
package main

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/catalog"
	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/ui"
)

// Entrypoint ids herdr-plugin.toml declares.
const (
	panePalette = "palette"
	paneRun     = palette.RunEntrypoint
	actionOpen  = "open"
	actionExec  = palette.ExecAction
)

func main() {
	ctx, stop := plugin.ShutdownContext(context.Background())
	code := newPlugin().Run(ctx)
	stop()
	os.Exit(code)
}

func newPlugin() *plugin.Plugin {
	p := plugin.New()
	p.Action(actionOpen, onOpen)
	p.Action(actionExec, onExec)
	p.Pane(panePalette, onPalette)
	p.Pane(paneRun, onRun)
	return p
}

// onOpen opens the palette popup. Placement and size come from the manifest,
// so the call names only the plugin and the entrypoint.
func onOpen(ctx context.Context, env *plugin.Env) error {
	params := herdr.PluginPaneOpenParams{
		PluginID:   env.PluginID,
		Entrypoint: panePalette,
		Focus:      herdr.Ptr(true),
	}
	// The commands act on what was focused when the key was pressed. The
	// popup's own entrypoint environment describes the popup pane, so the
	// invocation context is handed over explicitly.
	if len(env.ContextJSON) > 0 {
		params.Env = map[string]string{palette.ContextEnv: string(env.ContextJSON)}
	}
	_, err := env.Client().PluginPaneOpen(ctx, params)
	return err
}

// onExec runs what the popup handed over on its way out. It runs outside the
// popup, so a command that opens one of its own is no longer refused.
func onExec(ctx context.Context, env *plugin.Env) error {
	client := env.Client()
	entries, err := palette.Load(ctx, client, env.PluginID, catalog.Entries(), keys.Load(env.BinPath))
	if err != nil {
		// The catalog and the configured commands survive an unreachable
		// action list, and the handed-over entry may well be one of them.
		_ = err
	}
	return palette.RunPending(ctx, client, env, entries)
}

// onRun runs the configured command this pane was opened for. The pane closes
// when the command exits, the way herdr's own pane and popup commands behave.
// A command that exits non-zero is the command's business, not a failed plugin
// entrypoint.
func onRun(ctx context.Context, env *plugin.Env) error {
	command := os.Getenv(palette.RunEnv)
	if command == "" {
		return errors.New("the pane was opened without a command to run")
	}

	cmd := exec.CommandContext(ctx, palette.Shell(), "-c", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()

	var exit *exec.ExitError
	if errors.As(err, &exit) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// onPalette runs the TUI until the popup closes. A closed popup is a normal
// exit, not a failed plugin command.
func onPalette(ctx context.Context, env *plugin.Env) error {
	err := ui.Run(ctx, env)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
