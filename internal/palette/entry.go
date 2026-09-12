// Package palette assembles the searchable command list and ranks it against
// what the user types. Entries come from two sources: the hand-written herdr
// catalog, and every action the other installed plugins registered.
package palette

import (
	"context"
	"strings"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

// Exec carries what an entry needs to run: the socket client, the context
// herdr passed to the action, the text the palette collected for an entry that
// asks for input, and the entrypoint's own environment, which is how an entry
// that keeps something of its own reaches the plugin's state directory.
type Exec struct {
	Client *herdr.Client
	Ctx    *herdr.PluginInvocationContext
	Input  string
	Env    *plugin.Env
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
	// Type is the last column: the group a herdr command belongs to, or what
	// an entry is when it does not come from herdr.
	Type string
	// Binding is the herdr action this entry mirrors, such as new_workspace,
	// used to look up the key that does the same thing. Empty when herdr has
	// no such action.
	Binding string
	// Key is what the configuration binds the entry to, filled in at load
	// time and shown next to the title.
	Key   string
	Input *Input
	// Choices is the list an entry picks its target from, shown in the
	// palette's own window once the entry is chosen. What was picked reaches
	// Run as Exec.Input, the way a collected value does.
	Choices *Choices
	// Chosen is the value a row picked from that list carries: the palette
	// runs the entry with it, and a handover writes it down as the input.
	Chosen string
	Run    func(context.Context, Exec) error
	// NeedsSelection keeps the entry out of the list when no pane text is
	// selected, the way a plugin action declaring the selection context is
	// only meaningful with one.
	NeedsSelection bool
	// AlwaysRelay marks an entry whose refusal never reaches the palette, so
	// it is handed to an entrypoint outside the popup without trying first.
	// Everything else is tried, and relayed only if herdr answers ui_busy.
	AlwaysRelay bool
	// Search is text the query may match that the row does not show, such as
	// the plugin an action came from. A row matched through it shows it, so
	// the match is visible.
	Search string
	// Detail is shown after the title, dimmed, and is searched with the row:
	// what an agent is doing, or the workspace a pane sits in.
	Detail string
	// Status is what herdr calls the state the detail describes, which gives
	// it a colour of its own. Empty for a detail that is not a state.
	Status string
}

// Namespace is where the entry comes from, drawn in front of the title and
// ending in ": ", or empty when there is nothing to say.
func (e Entry) Namespace() string {
	if e.Type == "" {
		return ""
	}
	return strings.ToLower(e.Type) + ": "
}

// Name is how a row reads: the namespace in front of the title, as in
// "herdr: split pane right". It is both what the row renders and what a query
// matches, so the letters highlighted in a row are the ones that were
// searched.
//
// A command's title is lowercase, the way an editor's command list reads. The
// rows that go somewhere keep the name the workspace, tab or pane carries,
// which is a name rather than a command.
func (e Entry) Name() string {
	return e.Namespace() + e.Title
}

// Initial is the text the input field starts with, empty when the entry takes
// no input or defines no initial value.
func (e Entry) Initial(ctx *herdr.PluginInvocationContext) string {
	if e.Input == nil || e.Input.Initial == nil {
		return ""
	}
	return e.Input.Initial(ctx)
}

// TypeCustom is the namespace of the commands configured under
// [[keys.command]]. A plugin action shows under the plugin's own name
// instead, and the rows that go somewhere under what they go to.
const TypeCustom = "Command"

// ContextEnv is the environment variable the action entrypoint uses to pass
// its invocation context to the popup pane it opens.
const ContextEnv = "HERDR_PALETTE_CONTEXT"
