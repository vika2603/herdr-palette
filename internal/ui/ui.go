// Package ui is the popup TUI: a query line over the ranked command list.
package ui

import (
	"context"
	"encoding/json"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// Run shows the command list and runs the TUI until the popup closes. loadErr
// is what assembling the list ran into, which is worth showing next to the
// half that survived.
func Run(ctx context.Context, env *plugin.Env, list palette.List, loadErr error, colours theme.Theme) error {
	invocation := invocationContext(env)
	list.Commands = applicable(list.Commands, invocation)

	m := newModel(ctx, env, invocation, list, palette.ReadRecent(env), colours)
	if loadErr != nil {
		m.failure = loadErr.Error()
	}

	// No alternate screen: the popup is a pane herdr destroys when it closes,
	// so there is no scrollback to protect, and staying on the main screen
	// keeps the view readable to pane.read.
	//
	// Cell motion reports clicks and the wheel. herdr forwards mouse events to
	// a pane app that asks for them, so the popup gets them even while herdr's
	// own mouse capture is on.
	//
	// The renderer writes a frame on its own ticker, so the first frame waits
	// out one interval before the popup shows anything. The palette is up for
	// a keystroke or two and draws a screen of text; the fastest rate the
	// renderer accepts halves that wait and costs nothing it has to draw.
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithMouseCellMotion(), tea.WithFPS(120))
	_, err := program.Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// invocationContext is what was focused when the key opened the palette. The
// action entrypoint passes it through the popup's environment, because the
// popup's own entrypoint environment describes the popup pane instead.
func invocationContext(env *plugin.Env) *herdr.PluginInvocationContext {
	if raw := os.Getenv(palette.ContextEnv); raw != "" {
		var c herdr.PluginInvocationContext
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			return &c
		}
	}
	if c, err := env.Context(); err == nil && c != nil {
		return c
	}
	return &herdr.PluginInvocationContext{}
}

// applicable drops entries the current context cannot serve, so the list does
// not offer a selection action with nothing selected.
func applicable(entries []palette.Entry, c *herdr.PluginInvocationContext) []palette.Entry {
	hasSelection := c.SelectedText != nil && *c.SelectedText != ""
	out := make([]palette.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.NeedsSelection && !hasSelection {
			continue
		}
		out = append(out, entry)
	}
	return out
}
