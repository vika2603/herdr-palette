package ui

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// fieldModel is a palette holding a script that asks for a branch, an
// environment picked from two, and an optional token. ran records the values
// each run was given.
func fieldModel(t *testing.T, cols int, ran *[][]string) model {
	t.Helper()
	record := func(_ context.Context, x palette.Exec) error {
		*ran = append(*ran, x.Args)
		return nil
	}
	entries := []palette.Entry{
		{
			ID: "script:deploy", Title: "Deploy", Type: palette.TypeCustom, Kind: palette.KindCustom,
			Arguments: []palette.Argument{
				{Name: "branch", Env: "HP_BRANCH", Placeholder: "branch", Type: palette.ArgumentText},
				{Name: "env", Env: "HP_ENV", Placeholder: "environment", Type: palette.ArgumentDropdown, Options: []palette.Choice{
					{Title: "Staging", Value: "staging"},
					{Title: "Production", Value: "production"},
				}},
				{Name: "token", Env: "HP_TOKEN", Placeholder: "token", Type: palette.ArgumentPassword, Optional: true},
			},
			Run: record,
		},
		{ID: "script:lazygit", Title: "Lazygit", Type: palette.TypeCustom, Kind: palette.KindCustom, Run: record},
	}
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{},
		palette.List{Commands: entries}, nil, theme.Defaults(), Toggle{})
	m.setSize(cols, 12)
	return typeQuery(t, m, "deploy")
}

func pressAll(t *testing.T, m model, keys ...tea.KeyPressMsg) model {
	t.Helper()
	for _, key := range keys {
		m, _ = send(t, m, key)
	}
	return m
}

var (
	tabKey      = tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTabKey = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	enterKey    = tea.KeyPressMsg{Code: tea.KeyEnter}
	escKey      = tea.KeyPressMsg{Code: tea.KeyEscape}
	downKey     = tea.KeyPressMsg{Code: tea.KeyDown}
)

var clearKey = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}

// runEnter presses enter and runs what it asked for, reporting whether it
// ran.
func runEnter(t *testing.T, m model) (model, bool) {
	t.Helper()
	m, cmd := send(t, m, enterKey)
	if cmd == nil {
		return m, false
	}
	msg, ok := cmd().(ranMsg)
	return m, ok && msg.err == nil
}

func TestASelectedScriptShowsAFieldForEachArgument(t *testing.T) {
	var ran [][]string
	m := fieldModel(t, 72, &ran)
	header := ansi.Strip(m.header(m.layout()))
	for _, name := range []string{"deploy", "branch", "environment ▾", "token"} {
		if !strings.Contains(header, name) {
			t.Errorf("header = %q, want %q on it", header, name)
		}
	}
	if !strings.Contains(m.footer(), "arguments") {
		t.Errorf("footer = %q, want tab named as the way into the fields", m.footer())
	}
	// With nothing typed, the line names the selected command in place of the
	// search placeholder.
	empty := pressAll(t, m, clearKey)
	selectRow(t, &empty, "script:deploy")
	if header := ansi.Strip(empty.header(empty.layout())); !strings.Contains(header, "Deploy") || strings.Contains(header, searchPlaceholder) {
		t.Errorf("header = %q, want the command's title as the placeholder", header)
	}

	// The query keeps the keys until tab, and a row without arguments has no
	// fields.
	m = typeQuery(t, m, "x")
	if m.query.Value() != "deployx" || m.inField() {
		t.Errorf("query = %q, in a field = %v, want the query typed into", m.query.Value(), m.inField())
	}
	// Moved onto with the keys, a row without arguments leaves the query line
	// as it was.
	m = pressAll(t, m, clearKey)
	for m.ranked[m.cursor].Entry.ID != "script:lazygit" {
		m = pressAll(t, m, downKey)
	}
	if header := ansi.Strip(m.header(m.layout())); strings.Contains(header, "branch") || !strings.Contains(header, searchPlaceholder) {
		t.Errorf("header = %q on a row without arguments, want no fields and the search placeholder", header)
	}
}

func TestTabMovesIntoTheFieldsAndTypingFillsThem(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")

	if m.query.Value() != "deploy" {
		t.Errorf("query = %q, want it left as it was", m.query.Value())
	}
	header := []rune(ansi.Strip(m.header(m.layout())))
	start := slices.Index(header, []rune(capLeft)[0])
	if start < 0 || string(header[start:start+5]) != capLeft+"main" {
		t.Fatalf("header = %q, want the branch typed into its field", string(header))
	}
	// The query has given up the keys, so the line names the command instead.
	if text := string(header); !strings.Contains(text, "Deploy") || strings.Contains(text, "deploy") {
		t.Errorf("header = %q, want the command's title in place of the query", text)
	}
	if cursor := m.View().Cursor; cursor == nil || cursor.X != start+1+len("main") || cursor.Y != queryRow {
		t.Errorf("cursor = %+v, want it after main at column %d", cursor, start+1+len("main"))
	}
	// The rule's mark follows the keys from the label to the field.
	rule := []rune(ansi.Strip(m.topRule(m.layout())))
	if mark := slices.Index(rule, '━'); mark != start {
		t.Errorf("the rule's mark starts at %d, want under the field at %d: %q", mark, start, string(rule))
	}
	if !strings.Contains(m.footer(), "next") || !strings.Contains(m.footer(), "previous") {
		t.Errorf("footer = %q, want the keys that move between the fields", m.footer())
	}
}

func TestEnterGoesToWhatIsMissingBeforeItRuns(t *testing.T) {
	var ran [][]string
	m, ok := runEnter(t, fieldModel(t, 72, &ran))
	if ok || len(ran) != 0 {
		t.Fatal("the script ran without its branch")
	}
	if m.args.focus != 0 || !strings.Contains(m.footer(), "branch is required") {
		t.Errorf("focus = %d, footer = %q, want the keys on the branch and the reason", m.args.focus, m.footer())
	}

	m, ok = runEnter(t, typeQuery(t, m, "main"))
	if ok || m.args.focus != 1 || !strings.Contains(m.footer(), "env is required") {
		t.Fatalf("ran = %v, focus = %d, footer = %q, want the keys on the dropdown next", ok, m.args.focus, m.footer())
	}

	// The dropdown has had the focus, so its highlighted option is its value,
	// and the optional token can stay empty.
	if _, ok = runEnter(t, m); !ok {
		t.Fatal("the script did not run once everything it needs was entered")
	}
	if want := []string{"main", "staging", ""}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

func TestADropdownIsChosenWithTheArrowsUnderItsField(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")
	m = pressAll(t, m, tabKey, downKey)

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Staging") || !strings.Contains(view, "Production") {
		t.Errorf("view does not show the options:\n%s", view)
	}
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"Production") || strings.Contains(header, "environment") {
		t.Errorf("header = %q, want the highlighted option in the field", header)
	}
	if cursor := m.View().Cursor; cursor != nil {
		t.Errorf("cursor = %+v, want none until a filter is typed", cursor)
	}
	if _, ok := runEnter(t, m); !ok {
		t.Fatal("enter in the dropdown did not run the script")
	}
	if want := []string{"main", "production", ""}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

func TestTypingInADropdownFiltersItsOptions(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")
	m = typeQuery(t, pressAll(t, m, tabKey), "prd")

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Production") || strings.Contains(view, "Staging") {
		t.Errorf("view does not show only the matching option:\n%s", view)
	}
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"prd") {
		t.Errorf("header = %q, want the filter in the field", header)
	}
	if cursor := m.View().Cursor; cursor == nil {
		t.Error("no cursor while a filter is typed")
	}

	// Nothing matching leaves the dropdown without a value.
	none := typeQuery(t, m, "zz")
	if !strings.Contains(ansi.Strip(none.View().Content), "no match") {
		t.Errorf("view does not say nothing matches:\n%s", ansi.Strip(none.View().Content))
	}
	if _, ok := runEnter(t, none); ok {
		t.Fatal("the script ran with a dropdown that matched nothing")
	}

	// Leaving the field drops the filter and keeps the option it matched.
	m = pressAll(t, m, escKey)
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"Production") {
		t.Errorf("header = %q, want the matched option kept", header)
	}
	if _, ok := runEnter(t, m); !ok {
		t.Fatal("the script did not run")
	}
	if want := []string{"main", "production", ""}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

func TestADropdownCommandListsItsOptionsOnceEntered(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	var ran [][]string
	m := fieldModel(t, 72, &ran)
	entry := m.source.Commands[0]
	entry.Arguments = []palette.Argument{{
		Name: "branch", Env: "HP_BRANCH", Placeholder: "branch", Type: palette.ArgumentDropdown,
		Command: `printf 'main\nRelease 1\trelease/1\n'`,
	}}
	m.source.Commands[0] = entry
	m.rank()

	m, cmd := send(t, m, tabKey)
	if cmd == nil || !strings.Contains(ansi.Strip(m.View().Content), "loading") {
		t.Fatalf("entering the dropdown did not start its command:\n%s", ansi.Strip(m.View().Content))
	}
	m = drain(t, m, cmd)
	m = typeQuery(t, m, "rel")
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"rel") {
		t.Errorf("header = %q, want the filter in the field", header)
	}
	if _, ok := runEnter(t, m); !ok {
		t.Fatal("the script did not run")
	}
	if want := []string{"release/1"}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

// commandModel is fieldModel's script asking for a repository and then a
// branch its command lists.
func commandModel(t *testing.T, command string, ran *[][]string) model {
	t.Helper()
	m := fieldModel(t, 72, ran)
	entry := m.source.Commands[0]
	entry.Arguments = []palette.Argument{
		{Name: "repo", Env: "HP_REPO", Placeholder: "repo", Type: palette.ArgumentText},
		{Name: "branch", Env: "HP_BRANCH", Placeholder: "branch", Type: palette.ArgumentDropdown, Command: command},
	}
	m.source.Commands[0] = entry
	m.rank()
	return m
}

func TestADropdownCommandIsListedAgainForNewValuesBeforeIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	var ran [][]string
	m := commandModel(t, `printf '%s-main\n%s-dev\n' "$HP_REPO" "$HP_REPO"`, &ran)
	m = typeQuery(t, pressAll(t, m, tabKey), "api")
	m, cmd := send(t, m, tabKey)
	m = pressAll(t, drain(t, m, cmd), downKey)
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"api-dev") {
		t.Fatalf("header = %q, want the options listed for api", header)
	}

	// The repository changes after the branch was picked: enter goes back to
	// the branch and lists it again rather than running with api-dev.
	m = pressAll(t, m, shiftTabKey, clearKey)
	m = typeQuery(t, m, "web")
	m, cmd = send(t, m, enterKey)
	if len(ran) != 0 || m.args.focus != 1 || !strings.Contains(m.footer(), "listed again") {
		t.Fatalf("ran = %q, focus = %d, footer = %q, want the branch listed again first", ran, m.args.focus, m.footer())
	}
	// Until the new options arrive, the old one is not run with either.
	m, ok := runEnter(t, m)
	if ok || len(ran) != 0 || !strings.Contains(m.footer(), "still loading") {
		t.Fatalf("ran = %q, footer = %q, want the script held while the branch loads", ran, m.footer())
	}
	m = drain(t, m, cmd)
	if _, ok := runEnter(t, m); !ok {
		t.Fatal("the script did not run")
	}
	if want := []string{"web", "web-main"}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

// A question over the row hides the form without leaving it, so options
// arriving then still end the loading.
func TestOptionsArrivingUnderAQuestionEndTheLoading(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	var ran [][]string
	m := commandModel(t, "echo main", &ran)
	m = typeQuery(t, pressAll(t, m, tabKey), "api")
	m, cmd := send(t, m, tabKey)
	entry := m.ranked[m.cursor].Entry
	m.confirming = &entry
	m = drain(t, m, cmd)
	m.confirming = nil
	if m.args == nil || m.args.loading[1] {
		t.Fatal("the dropdown is still loading after its options arrived")
	}
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the option that arrived", header)
	}
}

func TestAFailedDropdownCommandRunsAgainWhenEnteredAgain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	marker := filepath.Join(t.TempDir(), "tried")
	var ran [][]string
	m := commandModel(t, "test -f '"+marker+"' || { touch '"+marker+"'; echo offline >&2; exit 1; }; echo main", &ran)
	m = typeQuery(t, pressAll(t, m, tabKey), "api")
	m, cmd := send(t, m, tabKey)
	m = drain(t, m, cmd)
	if !strings.Contains(m.footer(), "offline") {
		t.Fatalf("footer = %q, want the command's error", m.footer())
	}
	if _, cmd = send(t, m, downKey); cmd != nil {
		t.Fatal("the failed command ran again without the dropdown being entered again")
	}

	m = pressAll(t, m, shiftTabKey)
	m, cmd = send(t, m, tabKey)
	if cmd == nil {
		t.Fatal("entering the dropdown again did not run its command")
	}
	m = drain(t, m, cmd)
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the options from the second run", header)
	}
}

func TestAConfirmedScriptAsksWithItsValuesAndKeepsThemOnCancel(t *testing.T) {
	var ran [][]string
	m := fieldModel(t, 120, &ran)
	entry := m.source.Commands[0]
	entry.Confirm = true
	m.source.Commands[0] = entry
	m.rank()

	m = typeQuery(t, pressAll(t, m, tabKey), "main")
	m = pressAll(t, m, tabKey, tabKey)
	m, _ = send(t, m, tea.PasteMsg{Content: "s3cret"})
	m, ok := runEnter(t, m)
	if ok || len(ran) != 0 || m.confirming == nil {
		t.Fatalf("ran = %q, confirming = %v, want the question first", ran, m.confirming != nil)
	}
	footer := ansi.Strip(m.footer())
	if !strings.Contains(footer, "main · staging · ••••••") || strings.Contains(footer, "s3cret") {
		t.Errorf("footer = %q, want the values with the password hidden", footer)
	}

	// Any other key cancels, and the fields keep what was entered.
	m = pressAll(t, m, escKey)
	if m.confirming != nil || m.args == nil {
		t.Fatalf("confirming = %v, args = %v, want the question gone and the values kept", m.confirming != nil, m.args)
	}
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the branch kept", header)
	}

	m, _ = runEnter(t, m)
	if _, ok = runEnter(t, m); !ok {
		t.Fatal("enter on the question did not run the script")
	}
	if want := []string{"main", "staging", "s3cret"}; len(ran) != 1 || !slices.Equal(ran[0], want) {
		t.Errorf("ran with %q, want %q", ran, want)
	}
}

func TestEscGivesTheKeysBackAndKeepsWhatWasEntered(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")
	m = pressAll(t, m, escKey)
	if m.inField() {
		t.Fatal("esc left the keys in the field")
	}
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the branch kept", header)
	}

	// A query that keeps the row selected keeps its values; another row
	// selected leaves them behind with the row.
	m = typeQuery(t, pressAll(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace}), "y")
	if header := ansi.Strip(m.header(m.layout())); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the branch kept while the row stays selected", header)
	}
	m = pressAll(t, m, clearKey)
	for m.ranked[m.cursor].Entry.ID == "script:deploy" {
		m = pressAll(t, m, downKey)
	}
	for m.ranked[m.cursor].Entry.ID != "script:deploy" {
		m = pressAll(t, m, downKey)
	}
	if header := ansi.Strip(m.header(m.layout())); strings.Contains(header, "main") {
		t.Errorf("header = %q, want the fields started over", header)
	}

	// Shift+tab from the first field goes back to the query as well.
	m = pressAll(t, m, tabKey, shiftTabKey)
	if m.inField() {
		t.Error("shift+tab from the first field left the keys in it")
	}
}

func TestAPasswordIsNeverDrawn(t *testing.T) {
	var ran [][]string
	m := pressAll(t, fieldModel(t, 120, &ran), tabKey)
	m = typeQuery(t, m, "main")
	m = pressAll(t, m, tabKey, tabKey)
	m, _ = send(t, m, tea.PasteMsg{Content: "s3cret"})

	if view := ansi.Strip(m.View().Content); strings.Contains(view, "s3cret") || !strings.Contains(view, "••••••") {
		t.Errorf("the password shows, or its mask does not:\n%s", view)
	}
	if _, ok := runEnter(t, m); !ok || len(ran) != 1 || ran[0][2] != "s3cret" {
		t.Errorf("ran with %q, want the password itself passed", ran)
	}
}

func TestTheCardShowsWhatTheScriptWillReceive(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 120, &ran), tabKey), "main")
	panel := ansi.Strip(strings.Join(m.previewLines(m.previewWidth(), m.rows()), "\n"))
	for _, want := range []string{"$1", "main", "$2", "env", "$3", "token"} {
		if !strings.Contains(panel, want) {
			t.Errorf("card does not show %q:\n%s", want, panel)
		}
	}
}

func TestFieldsNeverOutgrowThePopup(t *testing.T) {
	for _, cols := range []int{16, 24, 40, 72, 120} {
		var ran [][]string
		m := pressAll(t, fieldModel(t, cols, &ran), tabKey)
		m = typeQuery(t, m, strings.Repeat("feature/long-branch-name ", 3))
		m = pressAll(t, m, tabKey)
		for i, line := range strings.Split(m.View().Content, "\n") {
			if width := lipgloss.Width(line); width > cols {
				t.Errorf("at %d columns line %d is %d wide: %q", cols, i, width, ansi.Strip(line))
			}
		}
		if cursor := m.View().Cursor; cursor != nil && cursor.X >= cols {
			t.Errorf("at %d columns the cursor is at %d, off the popup", cols, cursor.X)
		}
	}
}

// drain runs what a key asked for and hands back the options a dropdown's
// command listed.
func drain(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if c != nil {
				m = drain(t, m, c)
			}
		}
	case optionsMsg:
		m, _ = send(t, m, msg)
	}
	return m
}

func click(t *testing.T, m model, x, y int) (model, tea.Cmd) {
	t.Helper()
	return send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// notRun fails the test when a command asked for runs a script.
func notRun(t *testing.T, cmd tea.Cmd, what string) {
	t.Helper()
	if cmd == nil {
		return
	}
	if _, ok := cmd().(ranMsg); ok {
		t.Fatalf("%s ran the script", what)
	}
}

// openDropdown types a branch into the deploy script m has selected and opens
// its environment dropdown, the first option, Staging, highlighted.
func openDropdown(t *testing.T, m model) model {
	t.Helper()
	return pressAll(t, typeQuery(t, pressAll(t, m, tabKey), "main"), tabKey)
}

// darkBackground is what a dark terminal reports. The answer sets lipgloss's
// shared idea of the background, which the test puts back.
func darkBackground(t *testing.T, m model) model {
	t.Helper()
	previous := lipgloss.HasDarkBackground()
	t.Cleanup(func() { lipgloss.SetHasDarkBackground(previous) })
	m, _ = send(t, m, tea.BackgroundColorMsg{Color: color.RGBA{R: 0x1a, G: 0x1b, B: 0x26, A: 0xff}})
	return m
}

// surfaces names every surface colour the theme can be given.
var surfaces = map[string]string{
	"hover_background": "#222222", "menu_background": "#333333", "menu_hover_background": "#444444",
}

// The options are drawn over the list, so a click on one picks it rather than
// running the row under it.
func TestAClickOnAnOptionPicksIt(t *testing.T) {
	var ran [][]string
	m := openDropdown(t, fieldModel(t, 72, &ran))
	drop := m.line().drop

	m, cmd := click(t, m, drop.start+2, headerRows+1)
	notRun(t, cmd, "a click on an option")
	if f := m.form(); f == nil || f.focus != 1 || f.value(m.ranked[m.cursor].Entry, 1) != "production" {
		t.Fatalf("form = %+v, want Production picked with the dropdown still open", f)
	}
	if _, ok := runEnter(t, m); !ok || !slices.Equal(ran[0], []string{"main", "production", ""}) {
		t.Errorf("ran with %q, want the clicked option", ran)
	}
}

func TestAClickOnAFieldFocusesIt(t *testing.T) {
	var ran [][]string
	m := fieldModel(t, 72, &ran)
	fields := m.line().fields

	m, _ = click(t, m, fields[1].start+1, queryRow)
	if f := m.form(); f == nil || f.focus != 1 {
		t.Fatalf("form = %+v, want the dropdown focused", f)
	}
	if m.line().drop.width == 0 {
		t.Error("the clicked dropdown did not open")
	}

	// A second click on the open dropdown closes it.
	m, _ = click(t, m, fields[1].start+1, queryRow)
	if m.inField() {
		t.Error("a click on the open dropdown did not close it")
	}

	m, _ = click(t, m, fields[0].start+1, queryRow)
	if f := m.form(); f == nil || f.focus != 0 {
		t.Fatalf("form = %+v, want the text field focused", f)
	}
	// A click on the query line away from the fields gives the keys back to
	// the query.
	m, _ = click(t, m, 2, queryRow)
	if m.inField() {
		t.Error("a click on the query did not give it the keys")
	}
}

func TestAClickAwayFromAnOpenDropdownOnlyClosesIt(t *testing.T) {
	var ran [][]string
	m := openDropdown(t, fieldModel(t, 72, &ran))

	m, cmd := click(t, m, 2, headerRows)
	notRun(t, cmd, "a click away from the dropdown")
	if m.inField() {
		t.Error("the dropdown stayed open")
	}
	if f := m.form(); f == nil || f.fields[0].Value() != "main" {
		t.Errorf("form = %+v, want what was entered kept", f)
	}
}

// A click off the rows while a text field has the keys gives them back to the
// query, the way a click on the list does.
func TestAClickOffTheRowsLeavesATextField(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")

	m, cmd := click(t, m, 2, headerRows+5)
	notRun(t, cmd, "a click off the rows")
	if m.inField() {
		t.Error("the text field kept the keys")
	}
}

func TestTheWheelGoesThroughTheOptionsAndLeavesTheList(t *testing.T) {
	var ran [][]string
	m := pressAll(t, fieldModel(t, 72, &ran), clearKey)
	if len(m.ranked) < 2 || m.ranked[m.cursor].Entry.ID != "script:deploy" {
		t.Fatalf("want deploy selected among several rows, got %d rows", len(m.ranked))
	}
	m = openDropdown(t, m)
	drop := m.line().drop
	entry := m.ranked[m.cursor].Entry

	m, _ = send(t, m, tea.MouseWheelMsg{X: drop.start + 2, Y: headerRows, Button: tea.MouseWheelDown})
	if got := m.form().value(entry, 1); got != "production" {
		t.Errorf("after the wheel down the dropdown has %q, want production", got)
	}

	// Off the menu the wheel would move the selection and leave the form
	// behind, so it does nothing while a field has the keys.
	m, _ = send(t, m, tea.MouseWheelMsg{X: drop.start + drop.width + 2, Y: headerRows + 2, Button: tea.MouseWheelDown})
	if m.ranked[m.cursor].Entry.ID != "script:deploy" || m.form() == nil {
		t.Errorf("the wheel off the menu moved the selection to %s", m.ranked[m.cursor].Entry.ID)
	}
}

// The pointer marks what it is over and moves nothing: the next enter still
// runs what the keys selected.
func TestThePointerMarksWhatItIsOverAndMovesNothing(t *testing.T) {
	var ran [][]string
	m := darkBackground(t, pressAll(t, fieldModel(t, 72, &ran), clearKey))
	selected := m.cursor
	c := m.columns(m.listWidth())
	plain := m.row(1, c, false)

	m, _ = send(t, m, tea.MouseMotionMsg{X: 4, Y: headerRows + 1})
	if at := m.layout().hover; at != (target{targetRow, 1}) || m.cursor != selected {
		t.Fatalf("hovering %+v with the cursor at %d, want row 1 hovered and the cursor left at %d", at, m.cursor, selected)
	}
	if hovered := ansi.Strip(m.row(1, c, true)); hovered != ansi.Strip(plain) {
		t.Errorf("hovered row = %q, want the plain row's text without the selected row's bar", hovered)
	}
	band := fmt.Sprint(m.styles.hovered.text.GetBackground())
	if band == fmt.Sprint(m.colours.Selected) || band == fmt.Sprint(m.styles.plain.text.GetBackground()) {
		t.Errorf("hovered band = %s, want a shade of its own between the background and the selected row's", band)
	}
	menu := fmt.Sprint(m.styles.option.GetBackground())
	if menu == fmt.Sprint(m.colours.Rule) || fmt.Sprint(m.styles.optionHovered.GetBackground()) == menu {
		t.Errorf("menu = %s, want a shade of the band's in place of the rule's, and the hovered option apart from it", menu)
	}

	m = openDropdown(t, m)
	drop := m.line().drop
	entry := m.ranked[m.cursor].Entry
	m, _ = send(t, m, tea.MouseMotionMsg{X: drop.start + 2, Y: headerRows + 1})
	if at := m.layout().hover; at != (target{targetOption, 1}) || m.form().value(entry, 1) != "staging" {
		t.Errorf("hovering %+v with %q picked, want Production hovered and Staging kept", at, m.form().value(entry, 1))
	}

	fields := m.line().fields
	m, _ = send(t, m, tea.MouseMotionMsg{X: fields[2].start + 1, Y: queryRow})
	if at := m.layout().hover; at != (target{targetField, 2}) || m.form().focus != 1 {
		t.Errorf("hovering %+v with focus on %d, want the token field hovered and the dropdown kept", at, m.form().focus)
	}
	rule := []rune(ansi.Strip(m.topRule(m.layout())))
	if rule[fields[2].start] != '━' {
		t.Errorf("rule under the hovered field = %q, want it marked", string(rule))
	}
}

// Surfaces the theme names are drawn in from the start, before the terminal
// has reported the background the derived ones are shades of.
func TestConfiguredSurfacesNeedNoBackground(t *testing.T) {
	colours := theme.Load(nil, theme.Custom{Colours: surfaces})
	m := newModel(context.Background(), testEnv(t), &herdr.PluginInvocationContext{}, palette.List{}, nil, colours, Toggle{})

	check := func(when string) {
		t.Helper()
		for _, c := range []struct {
			name string
			got  lipgloss.TerminalColor
			want string
		}{
			{"hovered row", m.styles.hovered.text.GetBackground(), surfaces["hover_background"]},
			{"menu", m.styles.option.GetBackground(), surfaces["menu_background"]},
			{"hovered option", m.styles.optionHovered.GetBackground(), surfaces["menu_hover_background"]},
		} {
			if fmt.Sprint(c.got) != c.want {
				t.Errorf("%s %s = %v, want the configured colour", c.name, when, c.got)
			}
		}
	}
	check("before the background")
	m = darkBackground(t, m)
	check("after the background")
}
