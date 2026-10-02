package ui

import (
	"context"
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
	header := ansi.Strip(m.header())
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
	if header := ansi.Strip(empty.header()); !strings.Contains(header, "Deploy") || strings.Contains(header, searchPlaceholder) {
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
	if header := ansi.Strip(m.header()); strings.Contains(header, "branch") || !strings.Contains(header, searchPlaceholder) {
		t.Errorf("header = %q on a row without arguments, want no fields and the search placeholder", header)
	}
}

func TestTabMovesIntoTheFieldsAndTypingFillsThem(t *testing.T) {
	var ran [][]string
	m := typeQuery(t, pressAll(t, fieldModel(t, 72, &ran), tabKey), "main")

	if m.query.Value() != "deploy" {
		t.Errorf("query = %q, want it left as it was", m.query.Value())
	}
	header := []rune(ansi.Strip(m.header()))
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
	rule := []rune(ansi.Strip(m.topRule()))
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
	if header := ansi.Strip(m.header()); !strings.Contains(header, capLeft+"Production") || strings.Contains(header, "environment") {
		t.Errorf("header = %q, want the highlighted option in the field", header)
	}
	if cursor := m.View().Cursor; cursor != nil {
		t.Errorf("cursor = %+v, want none in a field that takes no text", cursor)
	}
	m = typeQuery(t, m, "zz")
	if _, ok := runEnter(t, m); !ok {
		t.Fatal("enter in the dropdown did not run the script")
	}
	if want := []string{"main", "production", ""}; len(ran) != 1 || !slices.Equal(ran[0], want) {
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
	if header := ansi.Strip(m.header()); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the branch kept", header)
	}

	// A query that keeps the row selected keeps its values; another row
	// selected leaves them behind with the row.
	m = typeQuery(t, pressAll(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace}), "y")
	if header := ansi.Strip(m.header()); !strings.Contains(header, capLeft+"main") {
		t.Errorf("header = %q, want the branch kept while the row stays selected", header)
	}
	m = pressAll(t, m, clearKey)
	for m.ranked[m.cursor].Entry.ID == "script:deploy" {
		m = pressAll(t, m, downKey)
	}
	for m.ranked[m.cursor].Entry.ID != "script:deploy" {
		m = pressAll(t, m, downKey)
	}
	if header := ansi.Strip(m.header()); strings.Contains(header, "main") {
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
