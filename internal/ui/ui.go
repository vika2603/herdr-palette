// Package ui is the popup TUI: a query line over the ranked command list.
package ui

import (
	"context"
	"encoding/json"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/catalog"
	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// Run loads the command list and runs the TUI until the popup closes.
func Run(ctx context.Context, env *plugin.Env) error {
	client := env.Client()

	invocation := invocationContext(env)
	entries, loadErr := palette.Load(ctx, client, env.PluginID, catalog.Entries(), keys.Load(env.BinPath))
	entries = applicable(entries, invocation)

	m := newModel(ctx, client, env, invocation, entries, palette.ReadRecent(env), theme.Load(env))
	if loadErr != nil {
		// The catalog is still usable, so the popup opens and says which half
		// of the list is missing.
		m.failure = "plugin actions unavailable: " + loadErr.Error()
	}

	// No alternate screen: the popup is a pane herdr destroys when it closes,
	// so there is no scrollback to protect, and staying on the main screen
	// keeps the view readable to pane.read.
	//
	// Cell motion reports clicks and the wheel. herdr forwards mouse events to
	// a pane app that asks for them, so the popup gets them even while herdr's
	// own mouse capture is on.
	program := tea.NewProgram(m, tea.WithContext(ctx), tea.WithMouseCellMotion())
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
