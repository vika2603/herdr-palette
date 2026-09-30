package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// sessionList is a session with an agent waiting, one finished, one working
// and a workspace, beside two commands.
func sessionList() palette.List {
	return palette.List{
		Commands: []palette.Entry{
			{ID: "split", Title: "split pane right", Type: "Herdr", Key: "prefix+v"},
			{ID: "jump", Title: "open git jump", Type: palette.TypeCustom},
		},
		Session: palette.Session{Entries: []palette.Entry{
			{ID: "pane:done", Title: "Go To api", Type: "Agent", Kind: palette.KindAgent, Detail: "done · claude", Status: "done", Pane: "p-done"},
			{ID: "pane:working", Title: "Go To billing", Type: "Agent", Kind: palette.KindAgent, Detail: "working · claude", Status: "working", Pane: "p-working"},
			{ID: "pane:blocked", Title: "Go To frontend", Type: "Agent", Kind: palette.KindAgent, Detail: "blocked · codex", Status: "blocked", Pane: "p-blocked"},
			{ID: "workspace:docs", Title: "Go To docs", Type: "Workspace", Kind: palette.KindWorkspace},
		}, Statuses: []string{"blocked", "done", "working", "working"}},
	}
}

func sessionModel(t *testing.T, env *plugin.Env, recent []string, cols, rows int) model {
	t.Helper()
	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, sessionList(), recent, theme.Defaults(), Toggle{Prefix: "ctrl+b"})
	m.setSize(cols, rows)
	return m
}

func headings(m model) []string {
	var out []string
	for _, l := range m.lines {
		if l.isHeading() {
			out = append(out, l.heading)
		}
	}
	return out
}

func TestAnEmptyQueryGroupsTheList(t *testing.T) {
	m := sessionModel(t, testEnv(t), []string{"jump"}, 72, 16)

	if got, want := headings(m), []string{"NEEDS YOU", "RECENT", "ACTIONS", "OPEN"}; !slices.Equal(got, want) {
		t.Errorf("headings = %v, want %v", got, want)
	}
	// A blocked agent cannot go on until it is answered, so it leads.
	if got, want := ids(m.ranked), []string{"pane:blocked", "pane:done", "jump", "split", "pane:working"}; !slices.Equal(got, want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want the agent that needs you selected", m.cursor)
	}
}

func TestAQueryListsItsMatchesWithoutGroups(t *testing.T) {
	m := typeQuery(t, sessionModel(t, testEnv(t), nil, 72, 16), "go")

	if got := headings(m); len(got) != 0 {
		t.Errorf("a query's matches are drawn under headings %v", got)
	}
}

func TestAListOfOneGroupHasNoHeading(t *testing.T) {
	var ran []string
	if got := headings(testModel(t, nil, &ran)); len(got) != 0 {
		t.Errorf("a list of commands alone is drawn under headings %v", got)
	}
}

func TestAHeadingIsNotARow(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 72, 16)

	// The first line of the list is the heading over what needs you.
	if index, ok := m.rowAt(0, headerRows); ok {
		t.Errorf("the heading is read as row %d", index)
	}
	if index, ok := m.rowAt(0, headerRows+1); !ok || index != 0 {
		t.Errorf("the line under the heading is row %d (%v), want the first row", index, ok)
	}
}

func TestMovingOntoAGroupBringsItsHeading(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 72, 7)
	rows := m.rows()

	// Down to the end and back to the top, which a window this short has to
	// scroll for.
	for range len(m.ranked) - 1 {
		m.step(1)
	}
	if m.offset == 0 {
		t.Fatal("the window did not scroll to the last row")
	}
	for range len(m.ranked) - 1 {
		m.step(-1)
	}
	if m.offset != 0 {
		t.Errorf("offset = %d on the first row, want its heading on show", m.offset)
	}
	if got := strings.Split(m.View(), "\n")[headerRows]; !strings.Contains(got, "NEEDS YOU") {
		t.Errorf("first line of a %d-line window = %q, want the heading", rows, got)
	}
}

func TestTheRuleCountsTheAgentsByState(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 72, 12)
	if got := m.topRule(); !strings.Contains(got, "● 1 blocked") || !strings.Contains(got, "● 2 working") {
		t.Errorf("rule = %q, want the agents counted by state", got)
	}
	if lipgloss.Width(m.topRule()) != 72 {
		t.Errorf("rule is %d wide, want the popup's width", lipgloss.Width(m.topRule()))
	}

	// A narrow popup keeps the counts and drops the names of the states.
	narrow := sessionModel(t, testEnv(t), nil, 32, 12)
	if got := narrow.topRule(); strings.Contains(got, "blocked") || !strings.Contains(got, "● 1") {
		t.Errorf("rule = %q, want the bare counts", got)
	}
}

func TestTheLabelNamesTheScreen(t *testing.T) {
	var picked string
	m := chooserModel(t, &picked, worktreeChoices)
	if got := m.header(); !strings.Contains(got, "PALETTE") {
		t.Errorf("header = %q, want PALETTE", got)
	}
	scoped := m
	scoped.enter(palette.ScopeWorkspaces)
	if got := scoped.header(); !strings.Contains(got, "WORKSPACES") {
		t.Errorf("header = %q in a scope, want WORKSPACES", got)
	}
	if got := choose(t, m, "worktree").header(); !strings.Contains(got, modePick) {
		t.Errorf("header = %q on a list of targets, want %s", got, modePick)
	}

	var ran []string
	confirm, _ := send(t, confirmModel(t, &ran), tea.KeyMsg{Type: tea.KeyEnter})
	if got := confirm.header(); !strings.Contains(got, modeConfirm) {
		t.Errorf("header = %q while asking, want %s", got, modeConfirm)
	}
}

// The label takes the room it needs and no more: the query starts a blank
// after it, whichever scope is on.
func TestTheQueryStartsRightAfterTheLabel(t *testing.T) {
	for _, scope := range []palette.Scope{palette.ScopePalette, palette.ScopePanes, palette.ScopeWorkspaces} {
		m := sessionModel(t, testEnv(t), nil, 72, 12)
		m.enter(scope)
		header := ansi.Strip(typeQuery(t, m, "x").header())
		got := lipgloss.Width(header[:strings.Index(header, "x")])
		if want := len(" "+scope.Label()+" ") + 2; got != want {
			t.Errorf("under %s the query starts at %d, want %d", scope.Label(), got, want)
		}
	}
}

func TestANarrowPopupHasNoPreview(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, previewMinCols-1, 12)
	if m.previewWidth() != 0 || m.previewTarget() != "" {
		t.Errorf("a %d-column popup draws a preview", m.cols())
	}
}

func previewServer(t *testing.T, text string) (*plugintest.Server, *plugin.Env) {
	t.Helper()
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPaneRead, herdr.PaneReadResponse{Read: herdr.PaneReadResult{Text: text}})
	return server, server.Env(plugintest.StateDir(t.TempDir()))
}

func TestTheSelectedAgentsScreenIsPreviewed(t *testing.T) {
	server, env := previewServer(t, "Allow Codex to apply this edit?\n› 1. Yes\n\n\n")
	m := sessionModel(t, env, nil, 120, 14)

	if m.previewTarget() != "p-blocked" {
		t.Fatalf("preview target = %q, want the selected agent's pane", m.previewTarget())
	}
	m, cmd := send(t, m, previewTickMsg{seq: m.previewSeq})
	if cmd == nil {
		t.Fatal("the preview was not read")
	}
	m, _ = send(t, m, cmd())

	var params herdr.PaneReadParams
	for _, call := range server.Calls() {
		if call.Method == herdr.MethodPaneRead {
			if err := json.Unmarshal(call.Params, &params); err != nil {
				t.Fatal(err)
			}
		}
	}
	if params.PaneID != "p-blocked" || params.Source != herdr.ReadSourceVisible {
		t.Errorf("read %+v, want the visible screen of the selected pane", params)
	}

	view := m.View()
	if !strings.Contains(view, "Allow Codex to apply this edit?") {
		t.Errorf("the preview does not show the pane's screen:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if width := lipgloss.Width(line); width > 120 {
			t.Errorf("line %d is %d wide: %q", i, width, line)
		}
	}
}

// The size arrives after the first frame, and it is what makes room for the
// preview, so that is when the screen is first asked for.
func TestTheFirstResizeAsksForThePreview(t *testing.T) {
	_, env := previewServer(t, "screen")
	m := newModel(context.Background(), env, &herdr.PluginInvocationContext{}, sessionList(), nil, theme.Defaults(), Toggle{})
	seq := m.previewSeq

	m, cmd := send(t, m, tea.WindowSizeMsg{Width: 120, Height: 14})
	if m.previewSeq == seq || cmd == nil {
		t.Error("a resize that made room for the preview did not ask for it")
	}
}

func TestMovingOffAPaneDropsItsPendingRead(t *testing.T) {
	_, env := previewServer(t, "screen")
	m := sessionModel(t, env, nil, 120, 14)
	stale := m.previewSeq

	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.previewSeq == stale {
		t.Fatal("moving to another pane did not move the preview on")
	}
	if _, cmd := send(t, m, previewTickMsg{seq: stale}); cmd != nil {
		t.Error("a read was asked for the pane already left")
	}
	m, _ = send(t, m, previewMsg{seq: stale, pane: "p-blocked", text: "old"})
	if m.preview.text == "old" {
		t.Error("the screen of the pane already left was kept")
	}
}

func TestACommandIsPreviewedAsWhatItDoes(t *testing.T) {
	m := typeQuery(t, sessionModel(t, testEnv(t), nil, 120, 12), "split")

	panel := strings.Join(m.previewLines(m.previewWidth(), m.rows()), "\n")
	if !strings.Contains(panel, "split pane right") || !strings.Contains(panel, "prefix+v") {
		t.Errorf("the card does not say what the command is and its key:\n%s", panel)
	}
}

func TestAClickOnThePreviewRunsNothing(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 120, 14)
	if _, ok := m.rowAt(m.listWidth()+2, headerRows+1); ok {
		t.Error("a click on the preview is read as a click on the row beside it")
	}
}

func TestNoLineOutgrowsAWidePopup(t *testing.T) {
	for _, cols := range []int{previewMinCols, 140, 200} {
		m := sessionModel(t, testEnv(t), nil, cols, 12)
		m.preview = preview{pane: "p-blocked", text: strings.Repeat("宽", 200) + "\n\tindented\x1b[31m red"}
		for i, line := range strings.Split(m.View(), "\n") {
			if width := lipgloss.Width(line); width > cols {
				t.Errorf("at %d columns line %d is %d wide: %q", cols, i, width, line)
			}
		}
	}
}

func TestAScreenIsCutToItsLastLines(t *testing.T) {
	got := screenLines("one\ntwo\x07\nthree\n\n  \n", 2)
	if want := []string{"two", "three"}; !slices.Equal(got, want) {
		t.Errorf("screenLines = %q, want %q", got, want)
	}
}

// A popup wide enough for the preview can still be too short for the heading
// and the rule it draws over a screen.
func TestAShortWidePopupDrawsThePreview(t *testing.T) {
	for _, height := range []int{3, 4, 5, 6} {
		m := sessionModel(t, testEnv(t), nil, 120, height)
		m.preview = preview{pane: m.previewTarget(), text: "one\ntwo\nthree"}
		if lines := strings.Split(m.View(), "\n"); len(lines) != m.rows()+chrome {
			t.Errorf("at height %d the view is %d lines, want %d", height, len(lines), m.rows()+chrome)
		}
	}
}

// A window one line high has room for the selected row or for the heading over
// it, and the row is what enter runs.
func TestAOneLineWindowShowsTheSelectedRow(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 72, 5)
	m = typeQuery(t, m, "x")
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyBackspace})

	if len(headings(m)) == 0 {
		t.Fatal("the empty query did not lay the list out in groups")
	}
	if got := m.lines[m.offset]; got.isHeading() || got.row != m.cursor {
		t.Errorf("the window shows %+v, want the selected row %d", got, m.cursor)
	}
}

// A rebuild of what is open, which every change of an agent's state brings,
// leaves the window where the reader had it.
func TestARebuildKeepsTheWindowWhereItWas(t *testing.T) {
	list := sessionList()
	for i := range 30 {
		list.Commands = append(list.Commands, palette.Entry{ID: fmt.Sprintf("c%d", i), Title: fmt.Sprintf("command %02d", i), Type: "Herdr"})
	}
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, list, nil, theme.Defaults(), Toggle{})
	m.setSize(72, 14)
	for range 15 {
		m.step(1)
	}
	if m.offset == 0 {
		t.Fatal("the window did not scroll")
	}
	// Up a few rows, so the selection is inside the window rather than on its
	// last line, where any offset would put it back.
	for range 3 {
		m.step(-1)
	}
	offset, cursor := m.offset, m.cursor

	m.setOpen(list.Session)
	if m.offset != offset || m.cursor != cursor {
		t.Errorf("after a rebuild offset = %d and cursor = %d, want %d and %d", m.offset, m.cursor, offset, cursor)
	}
}

func TestACardSaysWhatACommandRuns(t *testing.T) {
	list := sessionList()
	list.Commands = append(list.Commands, palette.Entry{
		ID: "cfg", Title: "new tab in workspace directory", Type: palette.TypeCustom,
		Description: `ws="$HERDR_ACTIVE_WORKSPACE_ID"; "$HERDR_BIN_PATH" tab create --workspace "$ws" --focus`,
	})
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, list, nil, theme.Defaults(), Toggle{})
	m.setSize(120, 14)
	m = typeQuery(t, m, "workspace directory")

	panel := strings.Join(m.previewLines(m.previewWidth(), m.rows()), "\n")
	if !strings.Contains(panel, "runs") || !strings.Contains(panel, "tab create") {
		t.Errorf("the card does not show the command line whole:\n%s", panel)
	}
}

func TestWrapKeepsEveryWordWithinTheWidth(t *testing.T) {
	text := "open the thing\nand a-word-far-longer-than-the-line here"
	lines := wrap(text, 12)
	for _, line := range lines {
		if lipgloss.Width(line) > 12 {
			t.Errorf("line %q is wider than 12", line)
		}
	}
	if got := strings.ReplaceAll(strings.Join(lines, ""), " ", ""); got != strings.ReplaceAll(strings.ReplaceAll(text, " ", ""), "\n", "") {
		t.Errorf("wrap lost text: %q", lines)
	}
}

func TestTheCaretIsABarAtTheInsertionPoint(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 72, 12)
	if got := ansi.Strip(m.header()); !strings.Contains(got, caret+searchPlaceholder) {
		t.Errorf("header = %q, want the caret in front of the placeholder", got)
	}

	m = typeQuery(t, m, "split")
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if got := ansi.Strip(m.header()); !strings.Contains(got, "spl"+caret+"it") {
		t.Errorf("header = %q, want the caret between the characters it sits between", got)
	}
	// Reverse video is how the field draws its block, and it is only written
	// out where the terminal takes styling at all.
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(profile)
	if header := m.header(); strings.Contains(header, "\x1b[7m") || strings.Contains(header, ";7m") {
		t.Errorf("the query line still draws a block: %q", header)
	}
}

func TestALongQueryKeepsTheCaretOnShow(t *testing.T) {
	m := sessionModel(t, testEnv(t), nil, 32, 12)
	m = typeQuery(t, m, strings.Repeat("abcdefgh ", 8)+"end")
	header := ansi.Strip(m.header())
	if !strings.Contains(header, "end"+caret) {
		t.Errorf("header = %q, want the end of the query and the caret after it", header)
	}
	if w := lipgloss.Width(m.header()); w > 32 {
		t.Errorf("header is %d wide", w)
	}
	for range 80 {
		m, _ = send(t, m, tea.KeyMsg{Type: tea.KeyHome})
	}
	if header := ansi.Strip(m.header()); !strings.Contains(header, caret+"abcdefgh") {
		t.Errorf("header = %q after home, want the caret at the start", header)
	}
}

// The agent the palette was opened from needs nobody to go to it, so it does
// not lead the list however long it has been waiting.
func TestWhereThePaletteIsDoesNotNeedYou(t *testing.T) {
	list := sessionList()
	for i := range list.Session.Entries {
		if list.Session.Entries[i].ID == "pane:blocked" {
			list.Session.Entries[i].Here = true
		}
	}
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, list, nil, theme.Defaults(), Toggle{})
	m.setSize(72, 16)
	if got := m.ranked[0].Entry.ID; got != "pane:done" {
		t.Errorf("first row is %q, want the finished agent ahead of the one the palette is in", got)
	}
}
