// Command palette is the herdr command palette plugin. One binary serves both
// manifest entrypoints: the action bound to a key, which opens the popup, and
// the popup pane that runs the TUI. See docs/design.md.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/ui"
)

// Entrypoint ids herdr-plugin.toml declares.
const (
	panePalette = "palette"
	actionOpen  = "open"
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
	p.Pane(panePalette, onPalette)
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

// onPalette runs the TUI until the popup closes. A closed popup is a normal
// exit, not a failed plugin command.
func onPalette(ctx context.Context, env *plugin.Env) error {
	err := ui.Run(ctx, env)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
