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
	"github.com/vika2603/herdr-palette/internal/theme"
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
		{ID: "a", Title: "split pane right", Type: "Herdr", Run: record("a")},
		{ID: "b", Title: "open git jump", Type: palette.TypeCustom, Run: record("b")},
		{
			ID:    "c",
			Title: "rename workspace",
			Type:  "Herdr",
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
		testEnv(t),
		&herdr.PluginInvocationContext{WorkspaceID: new("w1")},
		palette.List{Commands: testEntries(ran)},
		recent,
		theme.Defaults(),
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

// saw reports whether the server was asked for this method. The popup also
// opens a subscription to follow the session, so the call under test is not
// the only one.
func saw(t *testing.T, server *plugintest.Server, method string) bool {
	t.Helper()
	for _, call := range server.Calls() {
		if call.Method == method {
			return true
		}
	}
	return false
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
	m := typeQuery(t, testModel(t, nil, &ran), "git jump")

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
	m := typeQuery(t, testModel(t, nil, &ran), "git jump")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	cmd()

	if got := palette.ReadRecent(m.env); len(got) == 0 || got[0] != "b" {
		t.Errorf("recent = %v, want the command that just ran first", got)
	}
}

func TestAnEntryThatNeedsAValueIsHandedOverToTheField(t *testing.T) {
	var ran []string
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	m := newModel(
		context.Background(),
		env,
		&herdr.PluginInvocationContext{},
		palette.List{Commands: testEntries(&ran)},
		nil,
		theme.Defaults(),
	)
	m.width, m.height = 72, 12
	m = typeQuery(t, m, "rename")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("handing the entry over returned %v", cmd())
	}
	if len(ran) != 0 {
		t.Errorf("ran %v before its value was collected", ran)
	}

	var pending palette.Pending
	if err := env.ReadStateJSON(palette.PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.Prompt == nil || pending.Prompt.Initial != "current" {
		t.Errorf("pending = %+v, want the field to start from the current label", pending)
	}
	if !saw(t, server, herdr.MethodPluginActionInvoke) {
		t.Error("the entry was not handed over")
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

func TestTheViewShowsTheRowsAsTheyAreSearched(t *testing.T) {
	var ran []string
	view := testModel(t, nil, &ran).View()

	for _, want := range []string{"herdr: split pane right", "command: open git jump", "enter runs"} {
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

	with := applicable(entries, &herdr.PluginInvocationContext{SelectedText: new("text")})
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
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults())
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

	// The third row: the query line and the rule come first, and the rows
	// before it belong to commands that ask for a value.
	_, cmd := send(t, m, tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
		Y:      headerRows + 2,
	})
	if cmd == nil {
		t.Fatal("the click ran nothing")
	}
	cmd()
	if want := m.ranked[2].Entry.ID; len(ran) != 1 || ran[0] != want {
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
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults())
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

func TestAPluginActionIsRelayedWithoutRunning(t *testing.T) {
	var ran []string
	entries := []palette.Entry{{
		ID:          "plugin:herdr.machine-manager/open",
		Title:       "Manage machines",
		Type:        "Machine Manager",
		AlwaysRelay: true,
		Run: func(context.Context, palette.Exec) error {
			ran = append(ran, "direct")
			return nil
		},
	}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults())
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
	if !saw(t, server, herdr.MethodPluginActionInvoke) {
		t.Error("the entry was not handed over")
	}
}

// herdr refuses a second popup with ui_busy, which is what tells the palette
// to hand the command over instead of reporting a failure.
func TestAUIBusyRefusalIsRelayed(t *testing.T) {
	entries := []palette.Entry{{
		ID:    "config:prefix+f",
		Title: "Open git jump",
		Type:  palette.TypeCustom,
		Run: func(context.Context, palette.Exec) error {
			return &herdr.Error{Code: herdr.ErrCodeUIBusy, Message: "a popup pane is already open"}
		},
	}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults())
	m.width, m.height = 60, 12

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("a refused command reported %v instead of being handed over", cmd())
	}
	if !saw(t, server, herdr.MethodPluginActionInvoke) {
		t.Error("the command was not handed over")
	}
}

func TestARowMatchedOnTextItDoesNotShowShowsThatText(t *testing.T) {
	var ran []string
	entries := []palette.Entry{{
		ID:     "e",
		Title:  "manage machines",
		Type:   "Machine Manager",
		Search: "herdr.machine-manager",
		Run:    func(context.Context, palette.Exec) error { ran = append(ran, "e"); return nil },
	}}
	m := newModel(
		context.Background(),
		testEnv(t),
		&herdr.PluginInvocationContext{},
		palette.List{Commands: entries},
		nil,
		theme.Defaults(),
	)
	m.width, m.height = 72, 12

	view := typeQuery(t, m, "herdr.machine").View()

	if !strings.Contains(view, "herdr.machine") {
		t.Errorf("the row does not say what the query matched:\n%s", view)
	}
}

// An agent's status changes under the popup, so the rows that show it are
// rebuilt. The selection belongs to the user and stays where it was.
func TestRebuildingWhatIsOpenKeepsTheSelection(t *testing.T) {
	var ran []string
	m := newModel(
		context.Background(),
		testEnv(t),
		&herdr.PluginInvocationContext{},
		palette.List{
			Commands: testEntries(&ran),
			Open: []palette.Entry{
				{ID: "pane:w1:p1", Title: "go to shell", Type: "Agent", Detail: "claude · idle"},
			},
		},
		nil,
		theme.Defaults(),
	)
	m.width, m.height = 72, 12

	m = typeQuery(t, m, "go to")
	if len(m.ranked) != 1 {
		t.Fatalf("the query matched %d rows, want the agent", len(m.ranked))
	}
	m.setOpen([]palette.Entry{
		{ID: "pane:w1:p2", Title: "go to nvim", Type: "Pane", Detail: "palette"},
		{ID: "pane:w1:p1", Title: "go to shell", Type: "Agent", Detail: "claude · working"},
	})

	if len(m.ranked) != 2 {
		t.Fatalf("the rebuilt list has %d rows, want both panes", len(m.ranked))
	}
	if got := m.ranked[m.cursor].Entry.ID; got != "pane:w1:p1" {
		t.Errorf("the selection moved to %q, want the row it was on", got)
	}
	if got := m.ranked[m.cursor].Detail; got != "claude · working" {
		t.Errorf("detail = %q, want the status the agent has now", got)
	}
}
