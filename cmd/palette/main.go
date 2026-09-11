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
	"github.com/vika2603/herdr-palette/internal/prompt"
	"github.com/vika2603/herdr-palette/internal/settings"
	"github.com/vika2603/herdr-palette/internal/theme"
	"github.com/vika2603/herdr-palette/internal/ui"
)

// Entrypoint ids herdr-plugin.toml declares.
const (
	panePalette = "palette"
	paneRun     = palette.RunEntrypoint
	paneInput   = palette.InputEntrypoint
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
	p.Pane(paneInput, onInput)
	return p
}

// onOpen opens the palette popup. Placement and size come from the manifest,
// so the call names only the plugin and the entrypoint.
func onOpen(ctx context.Context, env *plugin.Env) error {
	params := herdr.PluginPaneOpenParams{
		PluginID:   env.PluginID,
		Entrypoint: panePalette,
		Focus:      new(true),
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
//
// Only the configured commands are read from herdr's configuration: the key
// column is the one thing this path never renders.
func onExec(ctx context.Context, env *plugin.Env) error {
	pending, ok := palette.ReadPending(env)
	if !ok {
		return nil
	}
	if pending.Prompt != nil {
		return palette.OpenPrompt(ctx, env.Client(), env, pending)
	}

	// An unreachable action list still leaves the catalog and the configured
	// commands, and the handed-over entry may well be one of them.
	list, _ := entries(ctx, env, keys.Commands(), settings.Load(env).Commands)
	return palette.RunPending(ctx, env.Client(), list.All(), pending)
}

// onInput collects the value an entry is missing and hands the entry back to
// the exec entrypoint, which runs it once this popup is gone.
func onInput(ctx context.Context, env *plugin.Env) error {
	pending, ok := palette.ReadPrompt()
	if !ok {
		return nil
	}

	own := settings.Load(env)
	value, ok, err := prompt.Ask(os.Stdin, os.Stdout, prompt.Field{
		Title:   pending.Prompt.Title,
		Label:   pending.Prompt.Label,
		Initial: pending.Prompt.Initial,
	}, theme.Load(keys.Commands().Theme, own.Theme))
	if err != nil || !ok {
		return err
	}
	return palette.RelayValue(ctx, env.Client(), env, pending, value)
}

// entries assembles the command list both entrypoints work from.
func entries(ctx context.Context, env *plugin.Env, cfg keys.Config, own []settings.Command) (palette.List, error) {
	return palette.Load(ctx, env.Client(), env.PluginID, catalog.Entries(), cfg, own)
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

// onPalette runs the TUI until the popup closes. ui.Run already answers a
// closed popup with nil: it is a normal exit, not a failed plugin command.
func onPalette(ctx context.Context, env *plugin.Env) error {
	cfg := keys.Load(env.BinPath)
	own := settings.Load(env)
	list, loadErr := entries(ctx, env, cfg, own.Commands)
	return ui.Run(ctx, env, list, loadErr, theme.Load(cfg.Theme, own.Theme))
}
