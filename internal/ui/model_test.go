package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
		{ID: "b", Title: "open git jump", Type: palette.TypeCustom, Kind: palette.KindCustom, Run: record("b")},
		{
			ID:    "c",
			Title: "rename workspace",
			Type:  "Herdr",
			Input: &palette.Input{
				Label:   "Workspace name",
				Initial: func(context.Context, palette.Exec) string { return "current" },
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
		m, _ = send(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
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

func TestFrameworkBackgroundResponsesDoNotChangeTheQuery(t *testing.T) {
	previous := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(previous) })
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "split")
	selected := m.ranked[m.cursor].Entry.ID
	for _, tc := range []struct {
		colour color.Color
		dark   bool
	}{{color.White, false}, {color.Black, true}} {
		var cmd tea.Cmd
		m, cmd = send(t, m, tea.BackgroundColorMsg{Color: tc.colour})
		if lipgloss.HasDarkBackground() != tc.dark {
			t.Errorf("background response did not select dark=%v", tc.dark)
		}
		if cmd != nil || m.query.Value() != "split" || m.ranked[m.cursor].Entry.ID != selected {
			t.Error("a terminal background response changed the query or selection")
		}
	}
}

func TestThePopupSetsItsBackgroundOnceTheTerminalHasAnswered(t *testing.T) {
	previous := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(previous) })
	terminal := color.RGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}

	m := testModel(t, nil, nil)
	if got := m.View().BackgroundColor; got != nil {
		t.Errorf("background before the terminal answered = %v, want none", got)
	}
	m, _ = send(t, m, tea.BackgroundColorMsg{Color: terminal})
	if got := m.View().BackgroundColor; got == nil || got == color.Color(terminal) {
		t.Errorf("background = %v, want one set apart from the terminal's %v", got, terminal)
	}
}

func TestTheColoursFollowAConfiguredBackground(t *testing.T) {
	previous := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(previous) })
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{}, nil,
		theme.Load(nil, theme.Custom{Colours: map[string]string{"background": "#f0f0f0"}}), Toggle{})

	send(t, m, tea.BackgroundColorMsg{Color: color.Black})

	if lipgloss.HasDarkBackground() {
		t.Error("a light popup on a dark terminal is drawn in the colours for a dark one")
	}
}

func TestEnterRunsTheSelectedCommand(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "git jump")

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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
	if !strings.Contains(m.View().Content, "deadline") {
		t.Error("the view does not carry the failure")
	}
}

func TestEnterWithNoMatchDoesNothing(t *testing.T) {
	var ran []string
	m := typeQuery(t, testModel(t, nil, &ran), "zzz")

	if len(m.ranked) != 0 {
		t.Fatalf("the query matched %d entries, want none", len(m.ranked))
	}
	if _, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Error("enter ran something although nothing matched")
	}
	if !strings.Contains(m.View().Content, "no command matches") {
		t.Error("the view does not say the query matched nothing")
	}
}

// The far end of the list is one keystroke away: a step off either end goes
// round rather than stopping there.
func TestSteppingOffTheEndGoesRound(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	last := len(m.ranked) - 1

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != last {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, last)
	}

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the first row", m.cursor)
	}

	for range len(m.ranked) + 3 {
		m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
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

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the first row", m.cursor)
	}
	for range 5 {
		m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if m.cursor != last {
		t.Errorf("cursor = %d, want the last row %d", m.cursor, last)
	}
}

func TestEscClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	if _, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc}); cmd == nil {
		t.Error("esc did not close the popup")
	}
}

func TestTheToggleKeyClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	m.closer = newCloser(Toggle{Bindings: []string{"alt+space"}})

	if _, cmd := send(t, m, tea.KeyPressMsg{Code: ' ', Mod: tea.ModAlt}); cmd == nil {
		t.Error("the toggle key did not close the popup")
	}
}

func TestAToggleChordClosesThePopup(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	m.closer = newCloser(Toggle{Bindings: []string{"prefix+space"}, Prefixes: []string{"ctrl+b"}})

	m, cmd := send(t, m, tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("the prefix on its own closed the popup")
	}
	if _, cmd := send(t, m, tea.KeyPressMsg{Code: ' ', Text: " "}); cmd == nil {
		t.Error("the key after the prefix did not close the popup")
	}
}

func TestAPasteConsumesAnArmedTogglePrefix(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)
	m.closer = newCloser(Toggle{Bindings: []string{"prefix+space"}, Prefixes: []string{"ctrl+b"}})
	m, _ = send(t, m, tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	m, _ = send(t, m, tea.PasteMsg{Content: "query"})
	if m.closer.armed || m.query.Value() != "" {
		t.Fatalf("paste after prefix: armed=%v query=%q, want the chord consumed without editing", m.closer.armed, m.query.Value())
	}
}

func TestTheViewShowsTheRowsAsTheyAreSearched(t *testing.T) {
	var ran []string
	view := testModel(t, nil, &ran).View().Content

	for _, want := range []string{"split pane right", "open git jump", "command", "run", "close"} {
		if !strings.Contains(view, want) {
			t.Errorf("the view does not contain %q", want)
		}
	}
	// Where a row comes from is a column of its own, not in front of the title.
	if rows := strings.Join(strings.Split(view, "\n")[headerRows:headerRows+3], "\n"); strings.Contains(rows, "command: open") {
		t.Errorf("the namespace is still in front of the title:\n%s", rows)
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

func wheel(button tea.MouseButton) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{Button: button}
}

func TestTheWheelMovesTheSelection(t *testing.T) {
	m := wheelModel(t)

	m, _ = send(t, m, wheel(tea.MouseWheelDown))
	if m.cursor != wheelStep {
		t.Errorf("cursor = %d after one notch down, want %d", m.cursor, wheelStep)
	}

	m, _ = send(t, m, wheel(tea.MouseWheelUp))
	if m.cursor != 0 {
		t.Errorf("cursor = %d after one notch up, want 0", m.cursor)
	}
}

func TestTheWheelScrollsTheWindow(t *testing.T) {
	m := wheelModel(t)
	for range 3 {
		m, _ = send(t, m, wheel(tea.MouseWheelDown))
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
	_, cmd := send(t, m, tea.MouseClickMsg{
		Button: tea.MouseLeft,
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
		if _, cmd := send(t, m, tea.MouseClickMsg{
			Button: tea.MouseLeft,
			Y:      y,
		}); cmd != nil {
			t.Errorf("a click on row %d ran something", y)
		}
	}
}

func TestTheScrollbarShowsTheWindow(t *testing.T) {
	m := wheelModel(t)

	view := m.View().Content
	if !strings.Contains(view, "┃") || !strings.Contains(view, "│") {
		t.Fatalf("the scrollbar is missing from a list longer than the window:\n%s", view)
	}

	lines := strings.Split(view, "\n")
	first := lines[headerRows]
	if !strings.HasSuffix(first, "┃") {
		t.Error("the thumb is not at the top while the window is")
	}

	for range 4 {
		m, _ = send(t, m, wheel(tea.MouseWheelDown))
	}
	lines = strings.Split(m.View().Content, "\n")
	last := lines[headerRows+m.rows()-1]
	if !strings.HasSuffix(last, "┃") {
		t.Errorf("the thumb does not reach the bottom on the last page:\n%s", m.View().Content)
	}
}

func TestNoScrollbarWhenEverythingFits(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	// The footer draws a rule of its own between the keys, so the rows are
	// what is looked at.
	rows := strings.Split(m.View().Content, "\n")[headerRows : headerRows+m.rows()]
	for _, row := range rows {
		if strings.Contains(row, "┃") || strings.Contains(row, "│") {
			t.Errorf("a list that fits drew a scrollbar:\n%s", m.View().Content)
		}
	}
}

func TestTheKeyColumnShowsWhatEachCommandIsBoundTo(t *testing.T) {
	entries := []palette.Entry{
		{ID: "a", Title: "New tab", Type: "herdr", Key: "prefix+c", Detail: "working"},
		{ID: "b", Title: "Split pane right", Type: "herdr", Detail: "idle"},
		{ID: "c", Title: "Manage machines", Type: "Machine Manager"},
	}
	withSpelling(t, "darwin")
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{Prefixes: []string{"ctrl+b"}})
	m.setSize(80, 12)

	view := m.View().Content
	if !strings.Contains(view, "⌃b c") {
		t.Errorf("the key is missing from the row:\n%s", view)
	}
	if m.widths.key != lipgloss.Width("⌃b c") {
		t.Errorf("key column width = %d, want the widest key", m.widths.key)
	}
	c := m.columns(m.listWidth())
	first, second := ansi.Strip(m.row(0, c)), ansi.Strip(m.row(1, c))
	if !strings.Contains(first, "New tab  ⌃b c") {
		t.Errorf("the shortcut does not follow the title inline: %q", first)
	}
	if !strings.Contains(first, "working  herdr") {
		t.Errorf("another row's long source leaves a gap between detail and source: %q", first)
	}
	if lipgloss.Width(first[:strings.Index(first, "working")+len("working")]) != lipgloss.Width(second[:strings.Index(second, "idle")+len("idle")]) {
		t.Errorf("details do not share a right edge: %q and %q", first, second)
	}
	if !strings.HasPrefix(strings.TrimLeft(second, " ▌"), "Split pane right") {
		t.Errorf("a keyless row leaves a shortcut slot: %q", second)
	}
	if lipgloss.Width(first[:strings.Index(first, "herdr")]) != lipgloss.Width(second[:strings.Index(second, "herdr")]) {
		t.Errorf("sources do not share the right edge: %q and %q", first, second)
	}
}

func TestThereIsNoKeyColumnWithoutBindings(t *testing.T) {
	var ran []string
	m := testModel(t, nil, &ran)

	if m.widths.key != 0 {
		t.Errorf("key column width = %d, want none when nothing is bound", m.widths.key)
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	view := typeQuery(t, m, "herdr.machine").View().Content

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
			Session: palette.Session{Entries: []palette.Entry{
				{ID: "pane:w1:p1", Title: "Go To shell", Type: "Agent", Kind: palette.KindAgent, Detail: "claude · idle"},
			}},
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
	m.setOpen(palette.Session{Entries: []palette.Entry{
		{ID: "pane:w1:p2", Title: "Go To nvim", Type: "Pane", Kind: palette.KindPane, Detail: "palette"},
		{ID: "pane:w1:p1", Title: "Go To shell", Type: "Agent", Kind: palette.KindAgent, Detail: "claude · working"},
	}})

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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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

	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	cmd()

	if got := palette.ReadRecent(m.env); len(got) == 0 || got[0] != "open" {
		t.Errorf("recent = %v, want the command that just ran", got)
	}
}

func TestEscLeavesTheTargetsForTheCommandList(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
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
	if m.notice != "every worktree is open already" {
		t.Errorf("notice = %q, want what the entry says about an empty list", m.notice)
	}
	// Nothing went wrong, so it is not said as a failure would be.
	if m.failure != "" {
		t.Errorf("failure = %q, want an empty list reported as a notice", m.failure)
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
	m.source.Commands[1].Input = &palette.Input{Label: "Prompt"}
	m.rank()

	m = choose(t, m, "worktree")
	_, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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
		for i, line := range strings.Split(m.View().Content, "\n") {
			if width := lipgloss.Width(line); width > cols {
				t.Errorf("at %d columns line %d is %d wide: %q", cols, i, width, line)
			}
		}
	}
}

func TestTheKeyColumnGivesWayOnANarrowPopup(t *testing.T) {
	wide, narrow := wideModel(t, 72), wideModel(t, 40)
	if got := wide.columns(wide.listWidth()).key; got != wide.widths.key {
		t.Errorf("key column = %d on a popup with room for it, want %d", got, wide.widths.key)
	}
	if got := narrow.columns(narrow.listWidth()).key; got != 0 {
		t.Errorf("key column = %d on a popup too narrow for it, want none", got)
	}
	if !strings.Contains(narrow.View().Content, "\u4e3b\u5de5\u4f5c\u533a") {
		t.Errorf("the detail lost its room to the key column:\n%s", narrow.View().Content)
	}
}

func TestTheFooterDropsKeysRatherThanOverrunning(t *testing.T) {
	narrow := wideModel(t, 24).footer()
	if strings.Contains(narrow, "esc close") {
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

	m, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("the first enter ran the command instead of asking")
	}
	if m.confirming == nil {
		t.Fatal("the command is not waiting to be confirmed")
	}
	if !strings.Contains(m.View().Content, "close pane?") {
		t.Errorf("the footer does not ask about the row:\n%s", m.View().Content)
	}

	m, cmd = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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
	m, _ := send(t, confirmModel(t, &ran), tea.KeyPressMsg{Code: tea.KeyEnter})

	m, cmd := send(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
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

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyExtended, Text: "ab"})
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.choosing == nil {
		t.Fatal("backspace left the targets although the query had something to delete")
	}
	if m.query.Value() != "a" {
		t.Errorf("query = %q, want the field to have taken the backspace", m.query.Value())
	}

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.choosing != nil {
		t.Error("backspace on an empty query did not leave the targets")
	}
}

// A row has to share its width with the detail beside it and with the key
// column; the footer shares with neither, so what the row had to cut is
// readable there.
func TestTheFooterNamesTheSelectedRow(t *testing.T) {
	// The widths are chosen against the keys as macOS spells them.
	withSpelling(t, "darwin")
	m := wideModel(t, 72)
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyUp})

	rows := strings.Join(strings.Split(m.View().Content, "\n")[headerRows:headerRows+m.rows()], "\n")
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

// mixedModel is a palette of commands, a command that opens a scope, and rows
// of every kind of place, which is what the scopes have to tell apart.
func mixedModel(t *testing.T) model {
	t.Helper()
	var ran []string
	commands := append(testEntries(&ran),
		palette.Entry{ID: "switch", Title: "switch workspace", Type: "Herdr", Scope: palette.ScopeWorkspaces},
		palette.Entry{ID: "plugin:notes/open", Title: "open notes", Type: "notes", Kind: palette.KindPlugin},
	)
	m := newModel(
		context.Background(),
		testEnv(t),
		&herdr.PluginInvocationContext{},
		palette.List{
			Commands: commands,
			Session: palette.Session{Entries: []palette.Entry{
				{ID: "pane:p1", Title: "Go To shell", Type: "Agent", Kind: palette.KindAgent, Detail: "claude · working"},
				{ID: "pane:p2", Title: "Go To git jump", Type: "Pane", Kind: palette.KindPane, Detail: "palette"},
				{ID: "tab:t2", Title: "Go To 2", Type: "Tab", Kind: palette.KindTab},
				{ID: "workspace:w2", Title: "Go To docs", Type: "Workspace", Kind: palette.KindWorkspace},
			}},
		},
		nil,
		theme.Defaults(),
		Toggle{},
	)
	m.setSize(72, 12)
	return m
}

func tab(t *testing.T, m model) model {
	t.Helper()
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m
}

// The command list holds the panes but not the tabs and workspaces they sit
// in, which would mostly be the same place again.
func TestTheCommandListLeavesTabsAndWorkspacesOut(t *testing.T) {
	m := mixedModel(t)
	for _, id := range ids(m.ranked) {
		if id == "tab:t2" || id == "workspace:w2" {
			t.Errorf("%s is in the command list", id)
		}
	}
	if !slices.Contains(ids(m.ranked), "pane:p2") {
		t.Errorf("rows = %v, want the panes among the commands", ids(m.ranked))
	}
}

// The start of a scope's name completes to the scope, which lists its rows
// and nothing else, and what is typed next filters those.
func TestTabCompletesTheStartOfAScope(t *testing.T) {
	m := tab(t, typeQuery(t, mixedModel(t), "Pan"))
	if m.scope != palette.ScopePanes || m.query.Value() != "" {
		t.Fatalf("scope = %q with query %q, want the panes and an empty query", m.scope.Name, m.query.Value())
	}
	if got, want := ids(m.ranked), []string{"pane:p1", "pane:p2"}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v: an agent's pane is a pane too", got, want)
	}
	if !strings.Contains(m.header(), "PANES") {
		t.Errorf("header = %q, want the scope named", m.header())
	}

	m = typeQuery(t, m, "shell")
	if got := ids(m.ranked); !slices.Equal(got, []string{"pane:p1"}) {
		t.Errorf("rows = %v, want the pane the query names", got)
	}
}

// The headings of the command list tell commands from places, which a scope
// holding one kind of row has no use for.
func TestAScopeIsNotGrouped(t *testing.T) {
	m := sessionModel(t, testEnv(t), []string{"pane:working"}, 72, 16)
	m.enter(palette.ScopeAgents)
	if got := headings(m); len(got) != 0 {
		t.Errorf("headings = %v, want none in a scope", got)
	}
	// What needs you still comes first, then where the palette went last.
	if got, want := ids(m.ranked), []string{"pane:blocked", "pane:done", "pane:working"}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

func TestEachScopeListsItsKind(t *testing.T) {
	for query, want := range map[string][]string{
		"agent":     {"pane:p1"},
		"tabs":      {"tab:t2"},
		"workspace": {"workspace:w2"},
		"plug":      {"plugin:notes/open"},
		"herdr":     {"c", "a", "switch"},
		"commands":  {"b"},
	} {
		m := tab(t, typeQuery(t, mixedModel(t), query))
		if got := ids(m.ranked); !slices.Equal(got, want) {
			t.Errorf("%q<tab> lists %v, want %v", query, got, want)
		}
	}
}

// Tab only completes a word that starts a scope's name and is long enough not
// to be the start of every other name searched for.
func TestTabLeavesAQueryThatNamesNoScope(t *testing.T) {
	for _, query := range []string{"p", "pane x", "shell"} {
		m := tab(t, typeQuery(t, mixedModel(t), query))
		if m.narrowed() || m.query.Value() != query {
			t.Errorf("%q<tab> went to %q with query %q, want the query left as typed", query, m.scope.Name, m.query.Value())
		}
	}
}

// The footer says what tab does before it is pressed, since the same key also
// answers a waiting agent.
func TestTheFooterNamesTheScopeTabCompletesTo(t *testing.T) {
	withSpelling(t, "darwin")
	m := typeQuery(t, mixedModel(t), "work")
	if !strings.Contains(m.footer(), "⇥ workspaces") {
		t.Errorf("footer = %q, want the scope tab goes to", m.footer())
	}
}

func TestAnEntryWithAScopeOpensIt(t *testing.T) {
	m := typeQuery(t, mixedModel(t), "switch workspace")
	if !strings.Contains(m.footer(), "open") {
		t.Errorf("footer = %q, want enter to say it opens the scope", m.footer())
	}
	m, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || m.scope != palette.ScopeWorkspaces {
		t.Fatalf("scope = %q, want the workspaces opened without running anything", m.scope.Name)
	}
	if got := ids(m.ranked); !slices.Equal(got, []string{"workspace:w2"}) {
		t.Errorf("rows = %v, want the workspaces", got)
	}
}

// A scope is a step inside the palette: esc and backspace on an empty query
// both go back to the command list rather than closing the popup.
func TestLeavingAScopePutsTheCommandsBack(t *testing.T) {
	for _, code := range []rune{tea.KeyEsc, tea.KeyBackspace} {
		key := tea.KeyPressMsg{Code: code}
		m := tab(t, typeQuery(t, mixedModel(t), "pan"))
		m, cmd := send(t, m, key)
		if cmd != nil {
			t.Errorf("%s closed the popup from a scope", key)
		}
		if m.narrowed() || m.query.Placeholder != searchPlaceholder {
			t.Errorf("%s left the scope at %q", key, m.scope.Name)
		}
		if !slices.Contains(ids(m.ranked), "a") {
			t.Errorf("%s did not put the commands back: %v", key, ids(m.ranked))
		}
	}

	// With something typed, backspace belongs to the query.
	m := typeQuery(t, tab(t, typeQuery(t, mixedModel(t), "pan")), "sh")
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.scope != palette.ScopePanes || m.query.Value() != "s" {
		t.Errorf("scope = %q with query %q, want the scope kept and a letter deleted", m.scope.Name, m.query.Value())
	}
}

func TestNothingInAScopeMatchingSaysSo(t *testing.T) {
	m := typeQuery(t, tab(t, typeQuery(t, mixedModel(t), "pan")), "nothing by this name")
	if len(m.ranked) != 0 {
		t.Fatalf("the query matched %d rows, want none", len(m.ranked))
	}
	if !strings.Contains(m.View().Content, "no pane matches") {
		t.Errorf("the view does not say what was searched:\n%s", m.View().Content)
	}
}

// A scope of places leaves out the column saying what each row is, which the
// label says for all of them; a plugin's name is still worth its column.
func TestAScopeOfPlacesHasNoSourceColumn(t *testing.T) {
	m := tab(t, typeQuery(t, mixedModel(t), "pan"))
	if got := ansi.Strip(m.row(0, m.columns(m.listWidth()))); strings.Contains(got, "agent") {
		t.Errorf("row = %q, want no source column in a scope of panes", got)
	}
	m = tab(t, typeQuery(t, mixedModel(t), "plug"))
	if got := ansi.Strip(m.row(0, m.columns(m.listWidth()))); !strings.Contains(got, "notes") {
		t.Errorf("row = %q, want the plugin's name", got)
	}
}

// An empty scope says why there is nothing in it: nothing of the kind is
// open, or no plugin put an action in the list.
func TestAnEmptyScopeSaysWhy(t *testing.T) {
	var ran []string
	for scope, want := range map[palette.Scope]string{
		palette.ScopeAgents:     "no agent is open",
		palette.ScopePlugins:    "no plugin is installed",
		palette.ScopeWorkspaces: "no workspace is open",
	} {
		m := testModel(t, nil, &ran)
		m.enter(scope)
		if !strings.Contains(m.View().Content, want) {
			t.Errorf("an empty %s scope does not say %q:\n%s", scope.Name, want, m.View().Content)
		}
	}

	m := sized(t, testEntries(&ran)[:1], 72)
	m.enter(palette.ScopeCommands)
	if !strings.Contains(m.View().Content, "no command is configured") {
		t.Errorf("an empty commands scope does not say none is configured:\n%s", m.View().Content)
	}
}

// The popup a key opened in a scope is that scope's: esc closes it, and
// backspace still reaches the commands.
func TestOpeningInAScope(t *testing.T) {
	m := mixedModel(t)
	m.openIn(palette.ScopePanes)
	if len(m.ranked) != 2 {
		t.Fatalf("the popup opened on %d rows, want the two panes", len(m.ranked))
	}
	if _, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc}); cmd == nil {
		t.Error("esc did not close a popup opened in a scope")
	}

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.narrowed() {
		t.Fatal("backspace did not leave the scope the popup opened in")
	}
	if _, cmd := send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc}); cmd == nil {
		t.Error("esc from the command list did not close the popup")
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

		for i, line := range strings.Split(m.View().Content, "\n") {
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
		m, cmd = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
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
	if !strings.Contains(m.View().Content, "working") {
		t.Errorf("the footer does not say the keystroke landed:\n%s", m.View().Content)
	}
}

// A click answers the question the way enter does, so the mouse and the
// keyboard do not disagree about what a second press means.
func TestAClickOnTheWaitingRowConfirmsIt(t *testing.T) {
	var ran []string
	m, _ := send(t, confirmModel(t, &ran), tea.KeyPressMsg{Code: tea.KeyEnter})

	m, cmd := send(t, m, tea.MouseClickMsg{
		Button: tea.MouseLeft, Y: headerRows,
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
	m, _ := send(t, confirmModel(t, &ran), tea.KeyPressMsg{Code: tea.KeyEnter})

	m, cmd := send(t, m, tea.MouseClickMsg{
		Button: tea.MouseLeft, Y: 0,
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
		ID: id, Title: title, Type: "Pane", Kind: palette.KindPane, Confirm: true,
		Run: func(context.Context, palette.Exec) error { return nil },
	}
}

func questionModel(t *testing.T, open []palette.Entry) model {
	t.Helper()
	var ran []string
	m := newModel(
		context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: testEntries(&ran), Session: palette.Session{Entries: open}}, nil, theme.Defaults(), Toggle{})

	m.setSize(72, 12)
	return m
}

// The session keeps the list current while a question is up: a row the palette
// still offers after a rebuild is one it can still act on.
func TestARebuildKeepsAQuestionWhoseRowSurvives(t *testing.T) {
	m := questionModel(t, []palette.Entry{confirmable("pane:p1", "Go To shell")})
	m = typeQuery(t, m, "shell")

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirming == nil {
		t.Fatal("the row did not ask before running")
	}

	// The second pane matches the query too, so the rows it is ranked into say
	// whether the rebuild reached the list at all.
	m.setOpen(palette.Session{Entries: []palette.Entry{
		confirmable("pane:p1", "Go To shell"),
		{ID: "pane:p2", Title: "Go To another shell", Type: "Pane", Kind: palette.KindPane},
	}})
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
	m := questionModel(t, []palette.Entry{confirmable("pane:p1", "Go To shell")})
	m = typeQuery(t, m, "shell")

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirming == nil {
		t.Fatal("the row did not ask before running")
	}

	m.setOpen(palette.Session{})
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

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
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
		scoped := sized(t, testEntries(&ran), cols)
		scoped.enter(palette.ScopeWorkspaces)
		scoped = typeQuery(t, scoped, "nothing by this name")

		asking := sized(t, []palette.Entry{{
			ID: "close", Title: "close workspace", Type: "Herdr", Confirm: true,
			Run: func(context.Context, palette.Exec) error { return nil },
		}}, cols)
		asking, _ = send(t, asking, tea.KeyPressMsg{Code: tea.KeyEnter})
		if asking.confirming == nil {
			t.Fatalf("at %d columns the row did not ask before running", cols)
		}

		for what, m := range map[string]model{"empty": empty, "scope empty": scoped, "asking": asking} {
			for i, line := range strings.Split(m.View().Content, "\n") {
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
	if _, ok := m.rowAt(0, last); !ok {
		t.Errorf("the last drawn row is not read as one")
	}
	for _, y := range []int{last + 1, last + 2, last + 3} {
		if index, ok := m.rowAt(0, y); ok {
			t.Errorf("y=%d is read as row %d, which was never drawn", y, index)
		}
		if _, cmd := send(t, m, tea.MouseClickMsg{
			Button: tea.MouseLeft, Y: y,
		}); cmd != nil {
			t.Errorf("a click at y=%d ran something", y)
		}
	}
	if len(ran) != 0 {
		t.Errorf("clicks below the rows ran %v", ran)
	}
}

// A command that fails after its screen was left still says so: it went out
// and did not work, wherever the reader is now.
func TestAFailureFromAScreenAlreadyLeftIsStillReported(t *testing.T) {
	var picked string
	m := choose(t, chooserModel(t, &picked, worktreeChoices), "worktree")
	stale := m.epoch

	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
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
	wide, _ = send(t, wide, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(wide.footer(), "herdr: close workspace?") {
		t.Errorf("a popup with room for it dropped the namespace: %q", wide.footer())
	}

	narrow := sized(t, entries, 30)
	narrow, _ = send(t, narrow, tea.KeyPressMsg{Code: tea.KeyEnter})
	footer := narrow.footer()
	if !strings.Contains(footer, "close workspace?") {
		t.Errorf("the question lost its mark rather than its namespace: %q", footer)
	}
	if strings.Contains(footer, "herdr:") {
		t.Errorf("the namespace survived on a popup with no room for it: %q", footer)
	}
}
