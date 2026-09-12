package palette

import "context"

// Choice is one thing an entry can act on: a worktree to open, a tab to move
// the pane to. Value is what the entry receives as Exec.Input.
type Choice struct {
	Value  string
	Title  string
	Detail string
	// Status is what herdr calls the state the detail describes, which gives
	// it a colour of its own, the way an agent's status has one.
	Status string
	// Search is text the query may match that the row does not show, the way
	// an entry's own search text works.
	Search string
}

// Choices is the second step of an entry whose target is one of a list herdr
// answers with, rather than a value the user types. The palette shows the
// list in its own window, under Label, and runs the entry with what was
// picked. Empty is what it says when there is nothing to pick.
//
// Stays keeps the list up afterwards, asked for again so the rows say what
// they are now. It is for a command that is a screen rather than one act —
// turning plugins on and off, where the point is to see the state and change
// more than one — and the popup closes on esc instead.
type Choices struct {
	Label string
	Empty string
	Stays bool
	List  func(context.Context, Exec) ([]Choice, error)
}

// ChoiceEntries turns what an entry can act on into the rows that pick it.
// Each row keeps the entry's id, so a handover resolves it against the command
// list and the recent order counts the command rather than the target.
func ChoiceEntries(entry Entry, choices []Choice) []Entry {
	entries := make([]Entry, 0, len(choices))
	for _, choice := range choices {
		entries = append(entries, Entry{
			ID:          entry.ID,
			Title:       choice.Title,
			Detail:      choice.Detail,
			Status:      choice.Status,
			Search:      choice.Search,
			Chosen:      choice.Value,
			AlwaysRelay: entry.AlwaysRelay,
			// An entry that asks for a value as well collects it once its
			// target is picked, so the row carries the field with it.
			Input: entry.Input,
			Run:   entry.Run,
		})
	}
	return entries
}
