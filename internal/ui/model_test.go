package ui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/palette"
)

func testEnv(t *testing.T) *plugin.Env {
	t.Helper()
	return plugintest.Env(plugintest.StateDir(t.TempDir()))
}

func testEntries(ran *[]string) []palette.Entry {
	record := func(id string) func(context.Context, palette.Exec) error {
		return func(context.Context, palette.Exec) error {
			*ran = append(*ran, id)
			return nil
		}
	}
	return []palette.Entry{
		{ID: "a", Title: "Split pane right", Type: "Pane", Run: record("a")},
		{ID: "b", Title: "New tab", Type: "Tab", Run: record("b")},
		{
			ID:    "c",
			Title: "Rename workspace",
			Type:  "herdr",
			Input: &palette.Input{
				Label:   "Workspace name",
				Initial: func(c *herdr.PluginInvocationContext) string { return "current" },
			},
			Run: record("c"),
		},
	}
}

func testModel(t *testing.T, recent []string, ran *[]string) model {
	t.Helper()
	m := newModel(
		context.Background(),
		nil,
		testEnv(t),
		&herdr.PluginInvocationContext{WorkspaceID: herdr.Ptr("w1")},
		testEntries(ran),
		recent,
	)
	m.width, m.height = 72, 12
	return m
}

func send(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	updated, ok := next.(model)
	if !ok {
		t.Fatalf("Update() returned %T, want model", next)
	}
	return updated, cmd
}

func typeQuery(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestTheListStartsWithTheRecentCommands(t *testing.T) {
	var ran []string
	m := testModel(t, []string{"b"}, &ran)

	if got := m.ranked[0].Entry.ID; got != "b" {
		t.Errorf("first row is %q, want the most recently run command", got)
	}
}

func TestTypingFiltersTheList(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "spr")

	if len(m.ranked) == 0 {
		t.Fatal("the query matched nothing")
	}
	if got := m.ranked[0].Entry.ID; got != "a" {
		t.Errorf("first row is %q, want the split command", got)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the selection reset to the best match", m.cursor)
	}
}

func TestEnterRunsTheSelectedCommand(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "new tab")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("running the command returned %v", cmd())
	}
	if len(ran) != 1 || ran[0] != "b" {
		t.Errorf("ran %v, want the selected command", ran)
	}
}

func TestARunCommandIsRecordedAsRecent(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "new tab")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	cmd()

	if got := palette.ReadRecent(m.env); len(got) == 0 || got[0] != "b" {
		t.Errorf("recent = %v, want the command that just ran first", got)
	}
}

func TestAnEntryThatNeedsInputOpensTheInputScreen(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "rename")

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.pending == nil {
		t.Fatal("enter ran the command instead of asking for its value")
	}
	if len(ran) != 0 {
		t.Errorf("ran %v before collecting the value", ran)
	}
	if got := m.input.Value(); got != "current" {
		t.Errorf("input starts at %q, want the current label", got)
	}
}

func TestTheInputScreenRunsOnEnterAndGoesBackOnEsc(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "rename")
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	back, _ := send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if back.pending != nil {
		t.Error("esc left the input screen open")
	}

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the input screen produced no command")
	}
	cmd()
	if len(ran) != 1 || ran[0] != "c" {
		t.Errorf("ran %v, want the rename command", ran)
	}
}

func TestAFailedCommandKeepsThePopupOpen(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	m, cmd := send(t, m, ranMsg{err: context.DeadlineExceeded})
	if cmd != nil {
		t.Error("a failed command asked the popup to close")
	}
	if m.failure == "" {
		t.Error("the failure is not shown, so the keystroke looks lost")
	}
	if !strings.Contains(m.View(), "deadline") {
		t.Error("the view does not carry the failure")
	}
}

func TestEnterWithNoMatchDoesNothing(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "zzz")

	if len(m.ranked) != 0 {
		t.Fatalf("the query matched %d entries, want none", len(m.ranked))
	}
	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("enter ran something although nothing matched")
	}
	if !strings.Contains(m.View(), "no command matches") {
		t.Error("the view does not say the query matched nothing")
	}
}

func TestTheCursorStaysInsideTheList(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	for range len(m.ranked) + 3 {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor != len(m.ranked)-1 {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, len(m.ranked)-1)
	}

	for range len(m.ranked) + 3 {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the first row", m.cursor)
	}
}

func TestEscClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEsc}); cmd == nil {
		t.Error("esc did not close the popup")
	}
}

func TestTheViewShowsTitlesAndGroups(t *testing.T) {
	var ran []string
	view := testModel(t, nil, &ran).View()

	for _, want := range []string{"Split pane right", "Pane", "New tab", "enter runs"} {
		if !strings.Contains(view, want) {
			t.Errorf("the view does not contain %q", want)
		}
	}
}

func TestSelectionActionsAreHiddenWithoutASelection(t *testing.T) {
	entries := []palette.Entry{
		{ID: "plain", Title: "New tab"},
		{ID: "clip", Title: "Clip the selection", NeedsSelection: true},
	}

	without := applicable(entries, &herdr.PluginInvocationContext{})
	if len(without) != 1 || without[0].ID != "plain" {
		t.Errorf("kept %d entries with nothing selected, want only the plain one", len(without))
	}

	with := applicable(entries, &herdr.PluginInvocationContext{SelectedText: herdr.Ptr("text")})
	if len(with) != 2 {
		t.Errorf("kept %d entries with a selection, want both", len(with))
	}
}

// wheelModel has more commands than the window fits, which is when the wheel
// and the scrollbar do anything.
func wheelModel(t *testing.T) model {
	t.Helper()
	entries := make([]palette.Entry, 0, 10)
	for i := range 10 {
		entries = append(entries, palette.Entry{
			ID:    string(rune('a' + i)),
			Title: "Command " + string(rune('a'+i)),
			Type:  "herdr",
			Run:   func(context.Context, palette.Exec) error { return nil },
		})
	}
	m := newModel(context.Background(), nil, testEnv(t), &herdr.PluginInvocationContext{}, entries, nil)
	m.width, m.height = 40, 8
	return m
}

func wheel(button tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{Action: tea.MouseActionPress, Button: button}
}

func TestTheWheelMovesTheSelection(t *testing.T) {
	m := wheelModel(t)

	m, _ = send(t, m, wheel(tea.MouseButtonWheelDown))
	if m.cursor != wheelStep {
		t.Errorf("cursor = %d after one notch down, want %d", m.cursor, wheelStep)
	}

	m, _ = send(t, m, wheel(tea.MouseButtonWheelUp))
	if m.cursor != 0 {
		t.Errorf("cursor = %d after one notch up, want 0", m.cursor)
	}
}

func TestTheWheelScrollsTheWindow(t *testing.T) {
	m := wheelModel(t)
	for range 3 {
		m, _ = send(t, m, wheel(tea.MouseButtonWheelDown))
	}
	if m.cursor != len(m.ranked)-1 {
		t.Fatalf("cursor = %d, want the last row", m.cursor)
	}
	if m.offset == 0 {
		t.Error("the window did not follow the selection past the last visible row")
	}
}

func TestAClickRunsTheRowItLandsOn(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	// The second row: the query line and the rule come first.
	_, cmd := send(t, m, tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Y:      headerRows + 1,
	})
	if cmd == nil {
		t.Fatal("the click ran nothing")
	}
	cmd()
	if want := m.ranked[1].Entry.ID; len(ran) != 1 || ran[0] != want {
		t.Errorf("ran %v, want the clicked row %q", ran, want)
	}
}

func TestAClickOutsideTheListDoesNothing(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	for _, y := range []int{0, headerRows - 1, headerRows + len(m.ranked)} {
		if _, cmd := send(t, m, tea.MouseMsg{
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
			Y:      y,
		}); cmd != nil {
			t.Errorf("a click on row %d ran something", y)
		}
	}
}

func TestTheInputScreenIgnoresTheMouse(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "rename")
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	before := m.cursor
	m, cmd := send(t, m, wheel(tea.MouseButtonWheelDown))
	if cmd != nil || m.cursor != before {
		t.Error("the wheel moved the list behind the input screen")
	}
}

func TestTheScrollbarShowsTheWindow(t *testing.T) {
	m := wheelModel(t)

	view := m.View()
	if !strings.Contains(view, "┃") || !strings.Contains(view, "│") {
		t.Fatalf("the scrollbar is missing from a list longer than the window:\n%s", view)
	}

	lines := strings.Split(view, "\n")
	first := lines[headerRows]
	if !strings.HasSuffix(first, "┃") {
		t.Error("the thumb is not at the top while the window is")
	}

	for range 4 {
		m, _ = send(t, m, wheel(tea.MouseButtonWheelDown))
	}
	lines = strings.Split(m.View(), "\n")
	last := lines[headerRows+m.rows()-1]
	if !strings.HasSuffix(last, "┃") {
		t.Errorf("the thumb does not reach the bottom on the last page:\n%s", m.View())
	}
}

func TestNoScrollbarWhenEverythingFits(t *testing.T) {
	var ran []string
	view := testModel(t, nil, &ran).View()

	if strings.Contains(view, "┃") || strings.Contains(view, "│") {
		t.Errorf("a list that fits drew a scrollbar:\n%s", view)
	}
}

func TestTheKeyColumnShowsWhatEachCommandIsBoundTo(t *testing.T) {
	entries := []palette.Entry{
		{ID: "a", Title: "New tab", Type: "herdr", Key: "prefix+c"},
		{ID: "b", Title: "Split pane right", Type: "herdr"},
	}
	m := newModel(context.Background(), nil, testEnv(t), &herdr.PluginInvocationContext{}, entries, nil)
	m.width, m.height = 60, 12

	view := m.View()
	if !strings.Contains(view, "prefix+c") {
		t.Errorf("the key is missing from the row:\n%s", view)
	}
	if m.keyWidth != len("prefix+c") {
		t.Errorf("key column width = %d, want the widest key", m.keyWidth)
	}
}

func TestThereIsNoKeyColumnWithoutBindings(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	if m.keyWidth != 0 {
		t.Errorf("key column width = %d, want none when nothing is bound", m.keyWidth)
	}
}

func TestAnEntryThatOpensAPopupIsRelayed(t *testing.T) {
	var ran []string
	entries := []palette.Entry{{
		ID:         "plugin:herdr.machine-manager/open",
		Title:      "Manage machines",
		Type:       palette.TypePlugin,
		OpensPopup: true,
		Run: func(context.Context, palette.Exec) error {
			ran = append(ran, "direct")
			return nil
		},
	}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	m := newModel(context.Background(), env.Client(), env, &herdr.PluginInvocationContext{}, entries, nil)
	m.width, m.height = 60, 12

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("running the entry returned %v", cmd())
	}
	if len(ran) != 0 {
		t.Error("the entry ran inside the popup, where herdr would refuse its own popup")
	}
	if server.Calls()[0].Method != herdr.MethodPluginActionInvoke {
		t.Errorf("called %q, want the entry to be handed over", server.Calls()[0].Method)
	}
}
