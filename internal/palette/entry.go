// Package palette assembles the searchable command list and ranks it against
// what the user types. Entries come from two sources: the hand-written herdr
// catalog, and every action the other installed plugins registered.
package palette

import (
	"context"

	"github.com/vika2603/herdr-client/herdr"
)

// Exec carries what an entry needs to run: the socket client, the context
// herdr passed to the action, and the text the palette collected for an entry
// that asks for input.
type Exec struct {
	Client *herdr.Client
	Ctx    *herdr.PluginInvocationContext
	Input  string
}

// Input describes the second step of an entry that cannot run on its own,
// such as a rename. Initial fills the field so a rename starts from the
// current label.
type Input struct {
	Label   string
	Initial func(*herdr.PluginInvocationContext) string
}

// Entry is one row of the palette.
type Entry struct {
	// ID is stable across runs: it keys the recent-command order.
	ID    string
	Title string
	// Detail names where the entry comes from, the herdr group or the plugin.
	Detail string
	Input  *Input
	Run    func(context.Context, Exec) error
	// NeedsSelection keeps the entry out of the list when no pane text is
	// selected, the way a plugin action declaring the selection context is
	// only meaningful with one.
	NeedsSelection bool
}

// Initial is the text the input field starts with, empty when the entry takes
// no input or defines no initial value.
func (e Entry) Initial(ctx *herdr.PluginInvocationContext) string {
	if e.Input == nil || e.Input.Initial == nil {
		return ""
	}
	return e.Input.Initial(ctx)
}

// ContextEnv is the environment variable the action entrypoint uses to pass
// its invocation context to the popup pane it opens.
const ContextEnv = "HERDR_PALETTE_CONTEXT"
