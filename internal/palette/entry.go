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
	// Chosen is the target picked from the entry's list, empty for an entry
	// that has none. It is separate from Input so an entry can ask for both.
	Chosen string
	Env    *plugin.Env
}

// Input describes the second step of an entry that cannot run on its own,
// such as a rename. Initial fills the field so a rename starts from the
// current label. It runs while the palette is up, with the client, so it can
// ask herdr for what the invocation context does not carry.
type Input struct {
	Label   string
	Initial func(context.Context, Exec) string
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
	// runs the entry with it, and a handover writes it down beside the input.
	Chosen string
	Run    func(context.Context, Exec) error
	// Close closes what a row that goes somewhere goes to, so it can be
	// closed from the list without going there first. Nil on every other row.
	Close func(context.Context, Exec) error
	// NeedsSelection keeps the entry out of the list when no pane text is
	// selected, the way a plugin action declaring the selection context is
	// only meaningful with one.
	NeedsSelection bool
	// Kind is what the row is, which decides the scopes that list it.
	Kind Kind
	// Here marks the row for where the palette was opened from. It is listed
	// so it can be closed without leaving it, but going there goes nowhere,
	// so it ranks after the rows its score ties with and gains nothing from
	// having been run recently.
	Here bool
	// Scope is the scope choosing the entry opens, in place of running
	// anything. The zero Scope for every other entry.
	Scope Scope
	// Confirm marks an entry that cannot be undone: the palette asks before
	// running it, so neither a keystroke meant for the row above nor a click
	// on a row that moved closes somebody's work.
	Confirm bool
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
	// Pane is the pane the row is about, whose screen the popup previews
	// beside the list. Empty for a row that is not about one pane.
	Pane string
	// Description is what the entry does as its source puts it: the text a
	// plugin gave its action, or the command line a configured command runs.
	// The preview shows it on the entry's card.
	Description string
}

// Namespace is where the entry comes from, drawn in front of the title and
// ending in ": ", or empty when there is nothing to say.
func (e Entry) Namespace() string {
	if e.Type == "" {
		return ""
	}
	return strings.ToLower(e.Type) + ": "
}

// Goes reports whether the row focuses something already open rather than
// running a command.
func (e Entry) Goes() bool { return places.Has(e.Kind) }

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
func (e Entry) Initial(ctx context.Context, x Exec) string {
	if e.Input == nil || e.Input.Initial == nil {
		return ""
	}
	return e.Input.Initial(ctx, x)
}

// TypeCustom is the namespace of the commands configured under
// [[keys.command]]. A plugin action shows under the plugin's own name
// instead, and the rows that go somewhere under what they go to.
const TypeCustom = "Command"

// ContextEnv is the environment variable the action entrypoint uses to pass
// its invocation context to the popup pane it opens.
const ContextEnv = "HERDR_PALETTE_CONTEXT"
