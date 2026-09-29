package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// closeModel is a list of one command that asks before it runs, a pane and a
// workspace, where closing either row records which one it was.
func closeModel(t *testing.T) (model, *[]string) {
	t.Helper()
	var closed []string
	closer := func(id string) func(context.Context, palette.Exec) error {
		return func(context.Context, palette.Exec) error {
			closed = append(closed, id)
			return nil
		}
	}
	list := palette.List{
		Commands: []palette.Entry{
			{ID: "close", Title: "close tab", Type: "Herdr", Confirm: true, Run: func(context.Context, palette.Exec) error { return nil }},
		},
		Open: []palette.Entry{
			{ID: "pane:api", Title: "go to api", Type: palette.TypePane, Goes: true, Close: closer("pane:api")},
			{ID: "workspace:docs", Title: "go to docs", Type: palette.TypeWorkspace, Goes: true, Close: closer("workspace:docs")},
		},
	}
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, list, nil, theme.Defaults(), Toggle{})
	m.setSize(96, 14)
	return m, &closed
}

// selectRow puts the selection on the row with the id.
func selectRow(t *testing.T, m *model, id string) {
	t.Helper()
	for i, r := range m.ranked {
		if r.Entry.ID == id {
			m.cursor = i
			return
		}
	}
	t.Fatalf("no row %q", id)
}

var ctrlX = tea.KeyMsg{Type: tea.KeyCtrlX}

func TestCtrlXClosesARowAfterAskingAndKeepsThePopupUp(t *testing.T) {
	m, closed := closeModel(t)
	selectRow(t, &m, "pane:api")
	if !strings.Contains(m.footer(), "⌃x close pane") {
		t.Errorf("footer = %q, want the key that closes the pane", m.footer())
	}

	m, _ = send(t, m, ctrlX)
	if len(*closed) != 0 {
		t.Fatal("the pane was closed before the question was answered")
	}
	if got := m.footer(); !strings.Contains(got, "close api?") {
		t.Errorf("footer = %q, want the question", got)
	}

	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter did not close the pane")
	}
	msg := cmd()
	if _, ok := msg.(closedMsg); !ok {
		t.Fatalf("enter answered %T, want the row closed", msg)
	}
	if len(*closed) != 1 || (*closed)[0] != "pane:api" {
		t.Errorf("closed %v, want the selected pane", *closed)
	}

	m, after := send(t, m, msg)
	if after != nil {
		t.Error("closing a row asked for something more, which would close the popup")
	}
	if m.pending || !strings.Contains(m.notice, "closed api") {
		t.Errorf("pending = %v, notice = %q, want the popup back and told what went", m.pending, m.notice)
	}
	if got := palette.ReadRecent(m.env); len(got) != 0 {
		t.Errorf("recent = %v, want closing left out of what was run", got)
	}
}

func TestAnyOtherKeyPutsTheCloseQuestionAway(t *testing.T) {
	m, closed := closeModel(t)
	selectRow(t, &m, "workspace:docs")
	if !strings.Contains(m.footer(), "⌃x close workspace") {
		t.Errorf("footer = %q, want the key that closes the workspace", m.footer())
	}
	m, _ = send(t, m, ctrlX)
	m, cmd := send(t, m, runes("x"))
	if m.confirming != nil || cmd != nil || len(*closed) != 0 {
		t.Errorf("confirming = %v, closed %v after a key that is not enter", m.confirming, *closed)
	}
}

func TestCtrlXDoesNothingOnACommand(t *testing.T) {
	m, _ := closeModel(t)
	selectRow(t, &m, "close")
	if strings.Contains(m.footer(), "⌃x") {
		t.Errorf("footer = %q offers to close a command", m.footer())
	}
	if m, _ = send(t, m, ctrlX); m.confirming != nil {
		t.Error("ctrl+x on a command asked to close it")
	}
}

// A question put away leaves nothing behind: the next command that asks runs
// as a command, which closes the popup once it is done.
func TestACommandAskedAfterACloseStillRunsAsACommand(t *testing.T) {
	m, closed := closeModel(t)
	selectRow(t, &m, "pane:api")
	m, _ = send(t, m, ctrlX)
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	selectRow(t, &m, "close")
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("the command did not run")
	}
	if msg := cmd(); !isRan(msg) {
		t.Errorf("the command answered %T, want it run as a command", msg)
	}
	if len(*closed) != 0 {
		t.Errorf("closed %v, want nothing", *closed)
	}
}

func isRan(msg tea.Msg) bool {
	_, ok := msg.(ranMsg)
	return ok
}

// The row a close took away leaves the selection on the row that took its
// place, where the next one to close is picked from.
func TestTheSelectionStaysWhereAClosedRowWas(t *testing.T) {
	m, _ := closeModel(t)
	selectRow(t, &m, "pane:api")
	at := m.cursor

	var left []palette.Entry
	for _, e := range m.open {
		if e.ID != "pane:api" {
			left = append(left, e)
		}
	}
	m, _ = send(t, m, openMsg{open: left})
	if m.cursor != min(at, len(m.ranked)-1) {
		t.Errorf("cursor = %d, want %d where the closed row was", m.cursor, at)
	}
}
