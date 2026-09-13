package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
		Toggle{},
	)
	m.setSize(72, 12)
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
		Toggle{},
	)
	m.setSize(72, 12)
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

// The far end of the list is one keystroke away: a step off either end goes
// round rather than stopping there.
func TestSteppingOffTheEndGoesRound(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	last := len(m.ranked) - 1

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != last {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, last)
	}

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the first row", m.cursor)
	}

	for range len(m.ranked) + 3 {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor < 0 || m.cursor > last {
		t.Errorf("cursor = %d, want a row of the list", m.cursor)
	}
}

// A page and a turn of the wheel stop at the ends: going round would carry the
// reader past what they were looking at.
func TestAPageStopsAtTheEnds(t *testing.T) {
	m := wheelModel(t)
	last := len(m.ranked) - 1

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the first row", m.cursor)
	}
	for range 5 {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if m.cursor != last {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, last)
	}
}

func TestEscClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEsc}); cmd == nil {
		t.Error("esc did not close the popup")
	}
}

func TestTheToggleKeyClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	m.closer = newCloser(Toggle{Binding: "alt+space"})

	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeySpace, Alt: true}); cmd == nil {
		t.Error("the toggle key did not close the popup")
	}
}

func TestAToggleChordClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	m.closer = newCloser(Toggle{Binding: "prefix+space", Prefix: "ctrl+b"})

	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if cmd != nil {
		t.Fatal("the prefix on its own closed the popup")
	}
	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeySpace}); cmd == nil {
		t.Error("the key after the prefix did not close the popup")
	}
}

func TestTheViewShowsTheRowsAsTheyAreSearched(t *testing.T) {
	var ran []string
	view := testModel(t, nil, &ran).View()

	for _, want := range []string{"herdr: split pane right", "command: open git jump", "run", "close"} {
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
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})
	m.setSize(40, 8)
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
	m := testModel(t, nil, &ran)

	// The footer draws a rule of its own between the keys, so the rows are
	// what is looked at.
	rows := strings.Split(m.View(), "\n")[headerRows : headerRows+m.rows()]
	for _, row := range rows {
		if strings.Contains(row, "┃") || strings.Contains(row, "│") {
			t.Errorf("a list that fits drew a scrollbar:\n%s", m.View())
		}
	}
}

func TestTheKeyColumnShowsWhatEachCommandIsBoundTo(t *testing.T) {
	entries := []palette.Entry{
		{ID: "a", Title: "New tab", Type: "herdr", Key: "prefix+c"},
		{ID: "b", Title: "Split pane right", Type: "herdr"},
	}
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})
	m.setSize(60, 12)

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

	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})
	m.setSize(60, 12)

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

	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})
	m.setSize(60, 12)

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
		Toggle{},
	)
	m.setSize(72, 12)

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
				{ID: "pane:w1:p1", Title: "go to shell", Type: "Agent", Goes: true, Detail: "claude · idle"},
			},
		},
		nil,
		theme.Defaults(),
		Toggle{},
	)
	m.setSize(72, 12)

	m = typeQuery(t, m, "go to")
	if len(m.ranked) != 1 {
		t.Fatalf("the query matched %d rows, want the agent", len(m.ranked))
	}
	m.setOpen([]palette.Entry{
		{ID: "pane:w1:p2", Title: "go to nvim", Type: "Pane", Goes: true, Detail: "palette"},
		{ID: "pane:w1:p1", Title: "go to shell", Type: "Agent", Goes: true, Detail: "claude · working"},
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

// chooserModel is a palette of one command, which picks its target from a
// list. picked records what running it received.
func chooserModel(t *testing.T, picked *string, list func(context.Context, palette.Exec) ([]palette.Choice, error)) model {
	t.Helper()
	entries := []palette.Entry{
		{ID: "a", Title: "split pane right", Type: "Herdr", Run: func(context.Context, palette.Exec) error { return nil }},
		{
			ID:    "open",
			Title: "open worktree workspace",
			Type:  "Herdr",
			Choices: &palette.Choices{
				Label: "Worktree to open",
				Empty: "every worktree is open already",
				List:  list,
			},
			Run: func(_ context.Context, e palette.Exec) error {
				*picked = e.Chosen
				return nil
			},
		},
	}

	m := newModel(
		context.Background(),
		testEnv(t),
		&herdr.PluginInvocationContext{WorkspaceID: new("w1")},
		palette.List{Commands: entries},
		nil,
		theme.Defaults(),
		Toggle{},
	)
	m.setSize(72, 12)
	return m
}

func worktreeChoices(context.Context, palette.Exec) ([]palette.Choice, error) {
	return []palette.Choice{
		{Value: "/trees/spike", Title: "spike"},
		{Value: "/trees/fix", Title: "fix"},
	}, nil
}

// choose selects the command whose targets are listed and takes the list it
// asks herdr for, which is what the popup does with the message.
func choose(t *testing.T, m model, query string) model {
	t.Helper()
	m = typeQuery(t, m, query)

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	msg, ok := cmd().(choicesMsg)
	if !ok {
		t.Fatalf("enter returned %T, want the entry's targets", cmd())
	}
	m, _ = send(t, m, msg)
	return m
}

func TestAnEntryWithTargetsListsThemInThePalette(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")

	if m.choosing == nil {
		t.Fatal("the palette still shows the command list")
	}
	if len(m.ranked) != 2 {
		t.Fatalf("shows %d rows, want the two worktrees", len(m.ranked))
	}
	if m.query.Placeholder != "Worktree to open" {
		t.Errorf("placeholder = %q, want what the list is collecting", m.query.Placeholder)
	}
	if picked != "" {
		t.Errorf("the command ran with %q before a target was picked", picked)
	}
}

func TestPickingATargetRunsTheCommandWithIt(t *testing.T) {
	var picked string
	m := typeQuery(t, choose(t, chooserModel(t, &picked, worktreeChoices), "worktree"), "fix")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("running the command returned %v", cmd())
	}
	if picked != "/trees/fix" {
		t.Errorf("ran with %q, want the target that was picked", picked)
	}
}

// The recent order counts the command, not the target it was run on.
func TestAPickedTargetRecordsTheCommandAsRecent(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	cmd()

	if got := palette.ReadRecent(m.env); len(got) == 0 || got[0] != "open" {
		t.Errorf("recent = %v, want the command that just ran", got)
	}
}

func TestEscLeavesTheTargetsForTheCommandList(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.choosing != nil {
		t.Fatal("the targets are still up")
	}
	if m.query.Value() != "worktree" {
		t.Errorf("query = %q, want the one the command list was filtered by", m.query.Value())
	}
	if m.query.Placeholder != searchPlaceholder {
		t.Errorf("placeholder = %q, want the command list's own", m.query.Placeholder)
	}
	if len(m.ranked) == 0 || m.ranked[0].Entry.ID != "open" {
		t.Error("the command list did not come back")
	}
}

func TestAnEntryWithNothingToActOnSaysSo(t *testing.T) {
	var picked string
	none := func(context.Context, palette.Exec) ([]palette.Choice, error) {
		return nil, nil
	}
	m := choose(t, chooserModel(t, &picked, none), "worktree")

	if m.choosing != nil {
		t.Fatal("an empty list is up, which has nothing to pick")
	}
	if m.failure != "every worktree is open already" {
		t.Errorf("failure = %q, want what the entry says about an empty list", m.failure)
	}
}

// An entry that asks for a value as well is handed over once its target is
// picked: the field is a popup, which cannot open while the palette's is up.
func TestPickingATargetForAnEntryThatAlsoAsksForAValue(t *testing.T) {
	var picked string
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	m := chooserModel(t, &picked, worktreeChoices)
	m.env = env
	m.commands[1].Input = &palette.Input{Label: "Prompt"}
	m.collect()
	m.rank()

	m = choose(t, m, "worktree")
	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("handing the entry over returned %v", cmd())
	}
	if picked != "" {
		t.Errorf("the command ran with %q before its value was collected", picked)
	}

	var pending palette.Pending
	if err := env.ReadStateJSON(palette.PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.Chosen != "/trees/fix" || pending.Prompt == nil {
		t.Errorf("pending = %+v, want the picked target and the field to collect the value", pending)
	}
}

// A list that stays is a screen the command is used from: running a row leaves
// it up, with the rows saying what they are now.
func TestAListThatStaysIsAskedForAgainAfterARowRuns(t *testing.T) {
	var picked string
	state := map[string]string{"a": "enabled", "b": "disabled"}
	list := func(context.Context, palette.Exec) ([]palette.Choice, error) {
		return []palette.Choice{
			{Value: "a", Title: "Auto Title", Detail: state["a"]},
			{Value: "b", Title: "Machine Manager", Detail: state["b"]},
		}, nil
	}

	m := chooserModel(t, &picked, list)
	m.commands[1].Choices.Stays = true
	m.commands[1].Run = func(_ context.Context, e palette.Exec) error {
		state[e.Chosen] = "disabled"
		picked = e.Chosen
		return nil
	}
	m.collect()
	m.rank()

	m = choose(t, m, "worktree")
	if m.cursor != 0 || m.ranked[0].Detail != "enabled" {
		t.Fatalf("the list starts at %+v, want the first row as it is now", m.ranked[0])
	}

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	if ran, ok := msg.(ranMsg); !ok || ran.err != nil {
		t.Fatalf("running the row returned %v", msg)
	}
	if picked != "a" {
		t.Fatalf("ran with %q, want the row that was selected", picked)
	}

	// The popup asks for the list again instead of quitting.
	next, cmd := send(t, m, ranMsg{})
	if cmd == nil {
		t.Fatal("the list was not asked for again")
	}
	again, ok := cmd().(choicesMsg)
	if !ok {
		t.Fatalf("asking again returned %T, want the rebuilt list", cmd())
	}

	next, _ = send(t, next, again)
	if next.choosing == nil {
		t.Fatal("the popup left the list after running a row")
	}
	if next.ranked[0].Detail != "disabled" {
		t.Errorf("the row still says %q, want what it is now", next.ranked[0].Detail)
	}
	if next.cursor != 0 {
		t.Errorf("cursor = %d, want the row that was just run", next.cursor)
	}
}

// On a screen of states, tab is what turning one over reads as; the command
// list has nothing for it to do.
func TestTabTurnsARowOverOnlyOnAListThatStays(t *testing.T) {
	var picked string
	m := chooserModel(t, &picked, worktreeChoices)

	if _, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyTab}); cmd != nil {
		t.Error("tab did something in the command list")
	}

	m.commands[1].Choices.Stays = true
	m.collect()
	m.rank()
	m = choose(t, m, "worktree")

	_, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("tab did not run the selected row")
	}
	if msg, ok := cmd().(ranMsg); !ok || msg.err != nil {
		t.Fatalf("running the row returned %v", cmd())
	}
	if picked != "/trees/fix" {
		t.Errorf("ran with %q, want the row that was selected", picked)
	}
}

func TestTheFooterSaysTabTurnsARowOver(t *testing.T) {
	var picked string
	m := chooserModel(t, &picked, worktreeChoices)
	m.commands[1].Choices.Stays = true
	m.collect()
	m.rank()

	if strings.Contains(m.footer(), "toggle") {
		t.Error("the command list offers a toggle")
	}
	if got := choose(t, m, "worktree").footer(); !strings.Contains(got, "toggle ⇥") {
		t.Errorf("footer = %q, want the key that turns a row over", got)
	}
}

// wideModel is a list whose rows carry wide characters, which cost two columns
// each and are what a row measured in runes gets wrong.
func wideModel(t *testing.T, cols int) model {
	t.Helper()
	entries := []palette.Entry{
		{ID: "a", Title: "split pane right", Type: "Herdr", Key: "prefix+shift+p"},
		{ID: "b", Title: "\u6253\u5f00\u5f53\u524d\u9879\u76ee\u7684\u6784\u5efa\u65e5\u5fd7\u7a97\u683c\u5e76\u8ddf\u968f\u8f93\u51fa", Type: "Pane", Detail: "\u5de5\u4f5c\u533a / ~/Workspace/x"},
		{ID: "c", Title: "\u7f16\u8f91\u5668", Type: "Pane", Detail: "\u4e3b\u5de5\u4f5c\u533a / ~/Workspace/herdr-palette"},
	}
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})

	m.setSize(cols, 10)
	return m
}

// A line wider than the popup wraps, which pushes every line below it down one
// and puts a click on the wrong row.
func TestNoLineOutgrowsThePopup(t *testing.T) {
	for _, cols := range []int{6, 8, 16, 24, 40, 72} {
		m := wideModel(t, cols)
		for i, line := range strings.Split(m.View(), "\n") {
			if width := lipgloss.Width(line); width > cols {
				t.Errorf("at %d columns line %d is %d wide: %q", cols, i, width, line)
			}
		}
	}
}

func TestTheKeyColumnGivesWayOnANarrowPopup(t *testing.T) {
	wide, narrow := wideModel(t, 72), wideModel(t, 40)
	if wide.keyColumn() != wide.keyWidth {
		t.Errorf("key column = %d on a popup with room for it, want %d", wide.keyColumn(), wide.keyWidth)
	}
	if narrow.keyColumn() != 0 {
		t.Errorf("key column = %d on a popup too narrow for it, want none", narrow.keyColumn())
	}
	if !strings.Contains(narrow.View(), "\u4e3b\u5de5\u4f5c\u533a") {
		t.Errorf("the detail lost its room to the key column:\n%s", narrow.View())
	}
}

func TestTheFooterDropsKeysRatherThanOverrunning(t *testing.T) {
	narrow := wideModel(t, 24).footer()
	if strings.Contains(narrow, "close esc") {
		t.Errorf("a footer too narrow for both halves kept the keys: %q", narrow)
	}
	if !strings.Contains(narrow, "run") {
		t.Errorf("the footer dropped the key it had room for: %q", narrow)
	}
}

// The line where the rows would be already says the query matched nothing, so
// the line under the list does not say it again.
func TestAnEmptyListLeavesTheFooterToTheKeys(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "nothing by this name")
	if len(m.ranked) != 0 {
		t.Fatalf("the query matched %d rows, want none", len(m.ranked))
	}
	if !strings.Contains(m.footer(), "run") {
		t.Errorf("the footer lost its keys: %q", m.footer())
	}
	if strings.Contains(m.footer(), "command") {
		t.Errorf("the footer repeats what the empty line says: %q", m.footer())
	}
}

// confirmModel is a palette of one command that cannot be undone. ran records
// whether it actually went through.
func confirmModel(t *testing.T, ran *[]string) model {
	t.Helper()
	entries := []palette.Entry{{
		ID:      "close",
		Title:   "close pane",
		Type:    "Herdr",
		Confirm: true,
		Run: func(context.Context, palette.Exec) error {
			*ran = append(*ran, "close")
			return nil
		},
	}}
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})

	m.setSize(72, 12)
	return m
}

// A row that closes somebody's work asks before it runs, so a keystroke meant
// for the row above does not take it with it.
func TestARowThatCannotBeUndoneAsksFirst(t *testing.T) {
	var ran []string
	m := confirmModel(t, &ran)

	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("the first enter ran the command instead of asking")
	}
	if m.confirming == nil {
		t.Fatal("the command is not waiting to be confirmed")
	}
	if !strings.Contains(m.View(), "close pane?") {
		t.Errorf("the footer does not ask about the row:\n%s", m.View())
	}

	m, cmd = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("the second enter did not run the command")
	}
	cmd()
	if len(ran) != 1 {
		t.Errorf("the command ran %d times, want once", len(ran))
	}
	if m.confirming != nil {
		t.Error("the question is still up after it was answered")
	}
}

func TestAnyOtherKeyPutsTheQuestionAway(t *testing.T) {
	var ran []string
	m, _ := send(t, confirmModel(t, &ran), tea.KeyMsg{Type: tea.KeyEnter})

	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil || len(ran) != 0 {
		t.Error("a key that is not enter ran the command")
	}
	if m.confirming != nil {
		t.Error("the question is still up")
	}
	if m.query.Value() != "" {
		t.Errorf("query = %q, want the answering keystroke not to reach the field", m.query.Value())
	}
}

// An empty query has nothing left to delete, so backspace undoes the step that
// opened the targets instead.
func TestBackspaceLeavesTheTargetsWhenTheQueryIsEmpty(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")
	if m.choosing == nil {
		t.Fatal("the targets are not up")
	}

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")})
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.choosing == nil {
		t.Fatal("backspace left the targets although the query had something to delete")
	}
	if m.query.Value() != "a" {
		t.Errorf("query = %q, want the field to have taken the backspace", m.query.Value())
	}

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.choosing != nil {
		t.Error("backspace on an empty query did not leave the targets")
	}
}

// A row has to share its width with the detail beside it and with the key
// column; the footer shares with neither, so what the row had to cut is
// readable there.
func TestTheFooterNamesTheSelectedRow(t *testing.T) {
	m := wideModel(t, 72)
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyUp})

	rows := strings.Join(strings.Split(m.View(), "\n")[headerRows:headerRows+m.rows()], "\n")
	if strings.Contains(rows, "herdr-palette") {
		t.Fatalf("the row was not cut, so the footer has nothing to add:\n%s", rows)
	}
	if !strings.Contains(m.footer(), "herdr-palette") {
		t.Errorf("the footer does not carry what the row had to cut: %q", m.footer())
	}

	if !strings.Contains(m.footer(), "run") {
		t.Errorf("the footer lost its keys to a long name: %q", m.footer())
	}
	if footer := wideModel(t, 40).footer(); !strings.Contains(footer, "run") {
		t.Errorf("a narrow footer lost its keys to the name: %q", footer)
	}
}

// mixedModel is a palette of commands and rows that go to what is open, which
// is what the prefix has to tell apart.
func mixedModel(t *testing.T) model {
	t.Helper()
	var ran []string
	m := newModel(
		context.Background(),
		testEnv(t),
		&herdr.PluginInvocationContext{},
		palette.List{
			Commands: testEntries(&ran),
			Open: []palette.Entry{
				{ID: "pane:p1", Title: "go to shell", Type: "Agent", Goes: true, Detail: "claude · working"},
				{ID: "pane:p2", Title: "go to git jump", Type: "Pane", Goes: true, Detail: "palette"},
			},
		},
		nil,
		theme.Defaults(),
		Toggle{},
	)
	m.setSize(72, 12)
	return m
}

// The prefix narrows the list to what it goes to without a step of its own, so
// reaching a pane by name costs one character rather than a second keystroke.
func TestThePrefixNarrowsTheListToWhatIsOpen(t *testing.T) {
	m := mixedModel(t)

	m = typeQuery(t, m, GoesPrefix)
	if len(m.ranked) != 2 {
		t.Fatalf("the prefix matched %d rows, want the two that go somewhere", len(m.ranked))
	}
	for _, ranked := range m.ranked {
		if !ranked.Entry.Goes {
			t.Errorf("%q runs a command, which the prefix leaves out", ranked.Entry.Name())
		}
	}

	// What follows the prefix filters those rows the way it filters any others.
	m = typeQuery(t, m, "shell")
	if len(m.ranked) != 1 || m.ranked[0].Entry.ID != "pane:p1" {
		t.Errorf("%q matched %d rows, want the agent's pane", GoesPrefix+"shell", len(m.ranked))
	}
}

// A query that shares a word with a command still leaves the commands out
// while the prefix is there, and brings them back the moment it goes.
func TestDeletingThePrefixPutsTheCommandsBack(t *testing.T) {
	m := typeQuery(t, mixedModel(t), GoesPrefix+"git jump")
	if len(m.ranked) != 1 || m.ranked[0].Entry.ID != "pane:p2" {
		t.Fatalf("%q matched %d rows, want the pane rather than the command", GoesPrefix+"git jump", len(m.ranked))
	}

	for range len(GoesPrefix + "git jump") {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = typeQuery(t, m, "git jump")
	if len(m.ranked) == 0 || m.ranked[0].Entry.ID != "b" {
		t.Errorf("the command is not back without the prefix, %d rows matched", len(m.ranked))
	}
}

func TestNothingOpenMatchingSaysSo(t *testing.T) {
	m := typeQuery(t, mixedModel(t), GoesPrefix+"nothing by this name")
	if len(m.ranked) != 0 {
		t.Fatalf("the query matched %d rows, want none", len(m.ranked))
	}
	if !strings.Contains(m.View(), "nothing open matches") {
		t.Errorf("the view does not say what was searched:\n%s", m.View())
	}
}

// The entrypoint bound to its own key opens the popup already narrowed, which
// is the same query typed before the first frame.
func TestStartingNarrowedToWhatIsOpen(t *testing.T) {
	m := mixedModel(t)
	m.start(GoesPrefix)

	if m.query.Value() != GoesPrefix {
		t.Errorf("query = %q, want the prefix", m.query.Value())
	}
	if len(m.ranked) != 2 {
		t.Errorf("the popup opened on %d rows, want the two that go somewhere", len(m.ranked))
	}
}

// A query longer than the popup scrolls inside the field. A field that keeps
// drawing all of it wraps the line, which pushes every row below it down and
// puts a click on the wrong row.
func TestALongQueryStaysOnOneLine(t *testing.T) {
	var ran []string
	for _, cols := range []int{24, 40, 72} {
		m := testModel(t, nil, &ran)
		m.setSize(cols, 12)
		m = typeQuery(t, m, strings.Repeat("abcdefgh ", 12))

		for i, line := range strings.Split(m.View(), "\n") {
			if width := lipgloss.Width(line); width > cols {
				t.Errorf("at %d columns line %d is %d wide: %q", cols, i, width, line)
			}
		}
	}
}

// A second keystroke while the first is still out on the socket would run the
// command twice, which for anything that creates something leaves two of it.
func TestACommandAlreadyRunningIsNotRunAgain(t *testing.T) {
	runs := 0
	entries := []palette.Entry{{
		ID: "slow", Title: "new tab", Type: "Herdr",
		Run: func(context.Context, palette.Exec) error {
			runs++
			return nil
		},
	}}
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})

	m.setSize(72, 12)

	var cmds []tea.Cmd
	for range 3 {
		var cmd tea.Cmd
		m, cmd = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if cmd != nil {
			cmd()
		}
	}

	if runs != 1 {
		t.Errorf("three keystrokes ran the command %d times, want once", runs)
	}
	if !strings.Contains(m.View(), "working") {
		t.Errorf("the footer does not say the keystroke landed:\n%s", m.View())
	}
}

// A click answers the question the way enter does, so the mouse and the
// keyboard do not disagree about what a second press means.
func TestAClickOnTheWaitingRowConfirmsIt(t *testing.T) {
	var ran []string
	m, _ := send(t, confirmModel(t, &ran), tea.KeyMsg{Type: tea.KeyEnter})

	m, cmd := send(t, m, tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: headerRows,
	})
	if cmd == nil {
		t.Fatal("the click did not run the command it was asked about")
	}
	cmd()
	if len(ran) != 1 {
		t.Errorf("the command ran %d times, want once", len(ran))
	}
	if m.confirming != nil {
		t.Error("the question is still up after it was answered")
	}
}

func TestAClickOffTheRowsPutsTheQuestionAway(t *testing.T) {
	var ran []string
	m, _ := send(t, confirmModel(t, &ran), tea.KeyMsg{Type: tea.KeyEnter})

	m, cmd := send(t, m, tea.MouseMsg{
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: 0,
	})
	if cmd != nil || len(ran) != 0 {
		t.Error("a click off the rows ran the command")
	}
	if m.confirming != nil {
		t.Error("the question is still up")
	}
}

// confirmable is a row of the session that cannot be undone, which is what a
// rebuild can take out from under a question.
func confirmable(id, title string) palette.Entry {
	return palette.Entry{
		ID: id, Title: title, Type: "Pane", Goes: true, Confirm: true,
		Run: func(context.Context, palette.Exec) error { return nil },
	}
}

func questionModel(t *testing.T, open []palette.Entry) model {
	t.Helper()
	var ran []string
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: testEntries(&ran), Open: open}, nil, theme.Defaults(), Toggle{})

	m.setSize(72, 12)
	return m
}

// The session keeps the list current while a question is up: a row the palette
// still offers after a rebuild is one it can still act on.
func TestARebuildKeepsAQuestionWhoseRowSurvives(t *testing.T) {
	m := questionModel(t, []palette.Entry{confirmable("pane:p1", "go to shell")})
	m = typeQuery(t, m, "shell")

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirming == nil {
		t.Fatal("the row did not ask before running")
	}

	// The second pane matches the query too, so the rows it is ranked into say
	// whether the rebuild reached the list at all.
	m.setOpen([]palette.Entry{
		confirmable("pane:p1", "go to shell"),
		{ID: "pane:p2", Title: "go to another shell", Type: "Pane", Goes: true},
	})
	if m.confirming == nil {
		t.Error("the question went away although the row it names is still on show")
	}
	if got := m.ranked[m.cursor].Entry.ID; got != "pane:p1" {
		t.Errorf("the selection is on %q, want the row being asked about", got)
	}
	if len(m.ranked) != 2 {
		t.Errorf("the list has %d rows, want the pane the rebuild added: %v", len(m.ranked), ids(m.ranked))
	}
}

func ids(ranked []palette.Ranked) []string {
	out := make([]string, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.Entry.ID)
	}
	return out
}

// A pane that closes under the question takes the question with it, rather
// than leaving it standing over whatever row the selection landed on.
func TestAQuestionGoesWithTheRowItNames(t *testing.T) {
	m := questionModel(t, []palette.Entry{confirmable("pane:p1", "go to shell")})
	m = typeQuery(t, m, "shell")

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirming == nil {
		t.Fatal("the row did not ask before running")
	}

	m.setOpen(nil)
	if m.confirming != nil {
		t.Errorf("the question stands over %q, whose row has gone", m.confirming.ID)
	}
}

// An answer from a screen the user has walked out of is not acted on: it would
// put the targets back up, or close the popup on the way out of them.
func TestAnAnswerFromAScreenAlreadyLeftIsDropped(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")
	if m.choosing == nil {
		t.Fatal("the targets are not up")
	}
	entry := m.choosing.entry
	stale := m.epoch

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.choosing != nil || m.pending {
		t.Fatalf("esc left choosing=%v pending=%v, want the command list", m.choosing != nil, m.pending)
	}

	// The list the popup asked for before esc answers now.
	m, _ = send(t, m, choicesMsg{
		epoch: stale, entry: entry, query: "worktree",
		choices: []palette.Choice{{Value: "w1", Title: "main"}},
	})
	if m.choosing != nil {
		t.Error("an answer from the list already left put the targets back up")
	}

	// So does a command that was running when esc was pressed.
	if _, cmd := send(t, m, ranMsg{epoch: stale}); cmd != nil {
		t.Error("a command that finished after esc closed the popup")
	}
}

// Every line the popup draws is cut to it, the line that stands where the rows
// would be and the one a question takes over included — both used to be built
// at whatever width their text ran to.
func TestNoLineOutgrowsThePopupOnAnyScreen(t *testing.T) {
	var ran []string
	for _, cols := range []int{6, 10, 16, 20, 24, 40, 72} {
		empty := typeQuery(t, sized(t, testEntries(&ran), cols), "nothing by this name")
		goes := typeQuery(t, sized(t, testEntries(&ran), cols), GoesPrefix+"nothing by this name")

		asking := sized(t, []palette.Entry{{
			ID: "close", Title: "close workspace", Type: "Herdr", Confirm: true,
			Run: func(context.Context, palette.Exec) error { return nil },
		}}, cols)
		asking, _ = send(t, asking, tea.KeyMsg{Type: tea.KeyEnter})
		if asking.confirming == nil {
			t.Fatalf("at %d columns the row did not ask before running", cols)
		}

		for what, m := range map[string]model{"empty": empty, "@ empty": goes, "asking": asking} {
			for i, line := range strings.Split(m.View(), "\n") {
				if width := lipgloss.Width(line); width > cols {
					t.Errorf("%s at %d columns: line %d is %d wide: %q", what, cols, i, width, line)
				}
			}
		}
	}
}

func sized(t *testing.T, entries []palette.Entry, cols int) model {
	t.Helper()
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})

	m.setSize(cols, 12)
	return m
}

// A list longer than the window is what tells a row that was drawn from one
// that only the arithmetic would put there: the rule under the list and the
// line below it sit inside the popup too, and reading them as rows runs a
// command that is not on screen.
func TestAClickBelowTheDrawnRowsDoesNothing(t *testing.T) {
	var ran []string
	entries := make([]palette.Entry, 0, 50)
	for i := range 50 {
		id := fmt.Sprintf("e%02d", i)
		entries = append(entries, palette.Entry{
			ID: id, Title: "command " + id, Type: "Herdr",
			Run: func(context.Context, palette.Exec) error {
				ran = append(ran, id)
				return nil
			},
		})
	}
	m := sized(t, entries, 72)
	m.setSize(72, 16)
	if len(m.ranked) <= m.rows() {
		t.Fatalf("the list is %d rows and the window %d: it has to be longer", len(m.ranked), m.rows())
	}

	last := headerRows + m.rows() - 1
	if _, ok := m.rowAt(last); !ok {
		t.Errorf("the last drawn row is not read as one")
	}
	for _, y := range []int{last + 1, last + 2, last + 3} {
		if index, ok := m.rowAt(y); ok {
			t.Errorf("y=%d is read as row %d, which was never drawn", y, index)
		}
		if _, cmd := send(t, m, tea.MouseMsg{
			Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, Y: y,
		}); cmd != nil {
			t.Errorf("a click at y=%d ran something", y)
		}
	}
	if len(ran) != 0 {
		t.Errorf("clicks below the rows ran %v", ran)
	}
}

// The epoch has to survive a staying list asking for its rows again, or the
// command it just ran would answer into a screen that never sees it.
func TestAStayingListSurvivesItsOwnRebuild(t *testing.T) {
	var picked string
	state := map[string]string{"a": "enabled", "b": "enabled"}
	list := func(context.Context, palette.Exec) ([]palette.Choice, error) {
		return []palette.Choice{
			{Value: "a", Title: "Auto Title", Detail: state["a"]},
			{Value: "b", Title: "Machine Manager", Detail: state["b"]},
		}, nil
	}
	m := chooserModel(t, &picked, list)
	m.commands[1].Choices.Stays = true
	m.commands[1].Run = func(_ context.Context, e palette.Exec) error {
		state[e.Chosen] = "disabled"
		return nil
	}
	m.collect()
	m.rank()
	m = choose(t, m, "worktree")
	if !m.staying() {
		t.Fatal("the list does not stay up")
	}
	before := m.epoch

	// Running a row asks the list for its rows again, on the same screen.
	m, cmd := send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = send(t, m, cmd())
	if m.epoch != before {
		t.Fatalf("epoch moved to %d while the screen stayed, want %d", m.epoch, before)
	}
	if cmd == nil {
		t.Fatal("running a row did not ask the staying list for its rows again")
	}
	m, _ = send(t, m, cmd())
	if m.choosing == nil {
		t.Error("the staying list closed on the answer it asked for")
	}

	// Leaving it does move the epoch, so what it asked for lands nowhere.
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.epoch == before {
		t.Error("leaving the list did not move the epoch")
	}
	if m.choosing != nil || m.pending {
		t.Errorf("esc left choosing=%v pending=%v", m.choosing != nil, m.pending)
	}
}

// A command that fails after its screen was left still says so: it went out
// and did not work, wherever the reader is now.
func TestAFailureFromAScreenAlreadyLeftIsStillReported(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")
	stale := m.epoch

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = send(t, m, ranMsg{epoch: stale, err: errors.New("worktree busy")})

	if m.failure != "worktree busy" {
		t.Errorf("failure = %q, want the reason the command gave", m.failure)
	}
	if m.pending {
		t.Error("the popup is still waiting on a command that has answered")
	}
}

// The question mark is what makes the line a question rather than a label, so
// the namespace — which the selected row above already carries — gives way
// before the mark does.
func TestTheQuestionKeepsItsMarkOnANarrowPopup(t *testing.T) {
	entries := []palette.Entry{{
		ID: "close", Title: "close workspace", Type: "Herdr", Confirm: true,
		Run: func(context.Context, palette.Exec) error { return nil },
	}}

	wide := sized(t, entries, 72)
	wide, _ = send(t, wide, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(wide.footer(), "herdr: close workspace?") {
		t.Errorf("a popup with room for it dropped the namespace: %q", wide.footer())
	}

	narrow := sized(t, entries, 30)
	narrow, _ = send(t, narrow, tea.KeyMsg{Type: tea.KeyEnter})
	footer := narrow.footer()
	if !strings.Contains(footer, "close workspace?") {
		t.Errorf("the question lost its mark rather than its namespace: %q", footer)
	}
	if strings.Contains(footer, "herdr:") {
		t.Errorf("the namespace survived on a popup with no room for it: %q", footer)
	}
}
