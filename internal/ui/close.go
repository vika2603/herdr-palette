package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// closedMsg carries the outcome of closing a row, and what the row was called
// so the line under the list can say it went.
type closedMsg struct {
	epoch int
	name  string
	err   error
}

// canClose reports whether the row goes to something that can be closed from
// the list. Not among an entry's targets, where the rows are what a command
// acts on rather than what is open, and not while a command is still out.
func (m model) canClose() bool {
	return m.choosing == nil && !m.pending && m.cursor < len(m.ranked) && m.ranked[m.cursor].Entry.Close != nil
}

// startClose asks before closing the selected row, the way a command that
// cannot be undone asks. The question is the row's own, so a rebuild that
// takes the selection off the row puts it away.
func (m model) startClose() (tea.Model, tea.Cmd) {
	row := m.ranked[m.cursor].Entry
	m.confirming = &palette.Entry{
		ID:      row.ID,
		Title:   "close " + strings.TrimPrefix(row.Title, palette.GoTo),
		Type:    row.Type,
		Confirm: true,
		Run:     row.Close,
	}
	m.closing = true
	m.failure, m.notice = "", ""
	return m, nil
}

// answer runs the row that was waiting to be confirmed.
func (m model) answer(entry palette.Entry) tea.Cmd {
	if m.closing {
		return m.close(entry)
	}
	return m.run(entry)
}

// close closes what the row goes to and keeps the popup up: the list is where
// the next one to close is picked from, and the session's own events take the
// row away. It is not a command run, so the recent order is left alone.
func (m model) close(entry palette.Entry) tea.Cmd {
	epoch, name := m.epoch, strings.TrimPrefix(entry.Title, "close ")
	return func() tea.Msg {
		err := entry.Run(m.ctx, palette.Exec{Client: m.env.Client(), Ctx: m.invocation, Env: m.env})
		return closedMsg{epoch: epoch, name: name, err: err}
	}
}

// closeHint names what the key closes, since esc closes the popup and the two
// would otherwise read the same.
func closeHint(entry palette.Entry) hint {
	switch entry.Type {
	case palette.TypeWorkspace:
		return hint{"⌃x", "close workspace"}
	case palette.TypeTab:
		return hint{"⌃x", "close tab"}
	}
	return hint{"⌃x", "close pane"}
}
