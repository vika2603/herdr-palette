package ui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// A script that asks for arguments carries a field for each on the query
// line, after the query, while its row is selected. The fields are all on show
// at once and tab moves between them, so a script is filled in and run from
// one line rather than one argument at a time.
const (
	// fieldGap is the blank between the query and the first field, after the
	// column the caret takes at the end of the query.
	fieldGap = 2
	// maxField and minField bound the text a field shows; a longer value
	// scrolls inside it. minQuery is what the query keeps beside the fields
	// before they are narrowed to leave it room.
	maxField = 24
	minField = 6
	minQuery = 10
	// dropMarker follows a dropdown's text.
	dropMarker = " ▾"
	// boxChrome is what a field takes beyond its text: a cap either side.
	boxChrome = 2
)

// The caps that round a field's ends: Powerline's half circles, from the
// private use area Nerd Fonts fill, drawn in the field's shade.
const (
	capLeft  = "\ue0b6"
	capRight = "\ue0b4"
)

// argForm is what has been entered for the arguments of one row. It belongs
// to that row: once another row is selected it is left behind, and coming
// back starts the fields over.
type argForm struct {
	id string
	// fields hold what is typed: an argument's value, or the text a dropdown's
	// options are filtered by while it has the focus.
	fields []textinput.Model
	// option is the option each dropdown has, an index into its options or -1
	// when it has none, and picked whether the dropdown has had the focus:
	// from then on its option is its value, and until then it has none.
	option []int
	picked []bool
	// loaded are the options a dropdown's command listed, requested whether
	// it has been started, and listedFor the values of the arguments before
	// it the latest run was given. loading is whether that run is still out,
	// and failed whether it failed.
	loaded    [][]palette.Choice
	requested []bool
	listedFor []string
	loading   []bool
	failed    []bool
	// focus is the argument being typed into, -1 while the query has the keys.
	focus int
}

func newArgForm(entry palette.Entry) *argForm {
	n := len(entry.Arguments)
	f := &argForm{
		id:        entry.ID,
		fields:    make([]textinput.Model, n),
		option:    make([]int, n),
		picked:    make([]bool, n),
		loaded:    make([][]palette.Choice, n),
		requested: make([]bool, n),
		listedFor: make([]string, n),
		loading:   make([]bool, n),
		failed:    make([]bool, n),
		focus:     -1,
	}
	for i := range f.fields {
		field := textinput.New()
		field.Prompt = ""
		field.SetVirtualCursor(false)
		field.Focus()
		f.fields[i] = field
		if entry.Arguments[i].Command != "" {
			f.option[i] = -1
		}
	}
	return f
}

// clone is a copy the model can change without changing the form an earlier
// copy of the model holds.
func (f *argForm) clone() *argForm {
	c := *f
	c.fields = slices.Clone(f.fields)
	c.option = slices.Clone(f.option)
	c.picked = slices.Clone(f.picked)
	c.loaded = slices.Clone(f.loaded)
	c.requested = slices.Clone(f.requested)
	c.listedFor = slices.Clone(f.listedFor)
	c.loading = slices.Clone(f.loading)
	c.failed = slices.Clone(f.failed)
	return &c
}

// options are what dropdown i picks from: those its header lists, or those
// its command listed.
func (f *argForm) options(entry palette.Entry, i int) []palette.Choice {
	if entry.Arguments[i].Command != "" {
		return f.loaded[i]
	}
	return entry.Arguments[i].Options
}

// shown are the indexes of dropdown i's options its filter keeps, best match
// first.
func (f *argForm) shown(entry palette.Entry, i int) []int {
	return palette.FilterChoices(f.options(entry, i), f.fields[i].Value())
}

// filtered gives dropdown i the best match for its filter, or no option when
// nothing matches.
func (f *argForm) filtered(entry palette.Entry, i int) {
	f.option[i] = -1
	if shown := f.shown(entry, i); len(shown) > 0 {
		f.option[i] = shown[0]
	}
}

// step moves dropdown i's option through the options its filter keeps.
func (f *argForm) step(entry palette.Entry, i, by int) {
	shown := f.shown(entry, i)
	if len(shown) == 0 {
		return
	}
	at := slices.Index(shown, f.option[i])
	if at < 0 {
		f.option[i] = shown[0]
		return
	}
	f.option[i] = shown[(at+by+len(shown))%len(shown)]
}

// focusOn moves the keys to argument i, or to the query for -1. A dropdown
// that gets the focus takes its option as its value; one that loses it drops
// its filter and keeps the option.
func (f *argForm) focusOn(entry palette.Entry, i int) {
	if f.focus >= 0 && f.focus != i && entry.Arguments[f.focus].Type == palette.ArgumentDropdown {
		f.fields[f.focus].SetValue("")
	}
	f.focus = i
	if i >= 0 && f.failed[i] {
		// Entering it again is how a failed command is retried.
		f.requested[i], f.failed[i] = false, false
	}
	if i >= 0 && entry.Arguments[i].Type == palette.ArgumentDropdown {
		f.picked[i] = true
		if f.option[i] < 0 {
			f.filtered(entry, i)
		}
	}
}

// value is what was entered for argument i: the text typed, or the value of
// the option a dropdown has.
func (f *argForm) value(entry palette.Entry, i int) string {
	argument := entry.Arguments[i]
	if argument.Type != palette.ArgumentDropdown {
		return f.fields[i].Value()
	}
	if !f.picked[i] || f.option[i] < 0 {
		return ""
	}
	return f.options(entry, i)[f.option[i]].Value
}

func (f *argForm) values(entry palette.Entry) []string {
	values := make([]string, len(entry.Arguments))
	for i := range values {
		values[i] = f.value(entry, i)
	}
	return values
}

// missing is the first argument the script cannot run without that has no
// value yet, or -1.
func (f *argForm) missing(entry palette.Entry) int {
	for i, argument := range entry.Arguments {
		if argument.Optional {
			continue
		}
		if argument.Type == palette.ArgumentDropdown && (!f.picked[i] || f.option[i] < 0) || argument.Type != palette.ArgumentDropdown && f.fields[i].Value() == "" {
			return i
		}
	}
	return -1
}

// above is what the arguments before argument i pass, by environment
// variable, which a dropdown's command is given so its options can depend on
// them, and the same values as one string to tell when they have changed.
func (f *argForm) above(entry palette.Entry, i int) (map[string]string, string) {
	env := make(map[string]string, i)
	values := make([]string, i)
	for j, argument := range entry.Arguments[:i] {
		values[j] = argument.Pass(f.value(entry, j))
		env[argument.Env] = values[j]
	}
	return env, strings.Join(values, "\x00")
}

// stale reports whether dropdown i's options were listed for values before
// it that have changed since.
func (f *argForm) stale(entry palette.Entry, i int) bool {
	if entry.Arguments[i].Command == "" || !f.requested[i] {
		return false
	}
	_, key := f.above(entry, i)
	return key != f.listedFor[i]
}

// optionsMsg carries the options a dropdown's command listed for the form of
// row id, given the values before it that key joins.
type optionsMsg struct {
	id      string
	index   int
	key     string
	options []palette.Choice
	err     error
}

// loadOptions starts the command of the dropdown that has the focus, the first
// time it is entered and again whenever the values before it have changed. It
// runs off the update loop, and only once the dropdown is entered, so moving
// over the row runs nothing.
func (m *model) loadOptions() tea.Cmd {
	f := m.form()
	if f == nil || f.focus < 0 {
		return nil
	}
	entry, _ := m.fielded()
	i := f.focus
	argument := entry.Arguments[i]
	if argument.Command == "" || f.requested[i] && !f.stale(entry, i) {
		return nil
	}
	env, key := f.above(entry, i)
	f = f.clone()
	f.requested[i], f.listedFor[i], f.loading[i] = true, key, true
	m.args = f
	ctx, invocation := m.ctx, m.invocation
	return func() tea.Msg {
		options, err := argument.LoadOptions(ctx, invocation, env)
		return optionsMsg{id: entry.ID, index: i, key: key, options: options, err: err}
	}
}

// optionsLoaded gives a dropdown the options its command listed, if the form
// that asked for them is still the one being filled in and nothing before the
// dropdown has changed since. The option it had stays when the new options
// still hold its value. The form is looked up by its row rather than through
// form, which a question over the row hides: an answer arriving then still
// has to end the loading.
func (m *model) optionsLoaded(msg optionsMsg) {
	f := m.args
	if f == nil || f.id != msg.id || m.cursor >= len(m.ranked) || m.ranked[m.cursor].Entry.ID != msg.id {
		return
	}
	if !f.loading[msg.index] || f.listedFor[msg.index] != msg.key {
		return
	}
	entry := m.ranked[m.cursor].Entry
	f = f.clone()
	i := msg.index
	kept := f.picked[i] && f.option[i] >= 0
	had := f.value(entry, i)
	f.loading[i] = false
	f.failed[i] = msg.err != nil
	f.loaded[i] = msg.options
	if msg.err != nil {
		m.failure = msg.err.Error()
	}
	f.filtered(entry, i)
	if kept && f.fields[i].Value() == "" {
		if at := slices.IndexFunc(msg.options, func(c palette.Choice) bool { return c.Value == had }); at >= 0 {
			f.option[i] = at
		}
	}
	m.args = f
}

// fielded is the selected row when it is a script asking for arguments, which
// the query line then carries fields for. Not while a list of targets, a
// question or an agent's screen is up: the line belongs to them then.
func (m model) fielded() (palette.Entry, bool) {
	if m.choosing != nil || m.replying != nil || m.confirming != nil || m.cursor >= len(m.ranked) {
		return palette.Entry{}, false
	}
	entry := m.ranked[m.cursor].Entry
	return entry, len(entry.Arguments) > 0
}

// form is what has been entered for the selected row, nil while nothing has.
func (m model) form() *argForm {
	entry, ok := m.fielded()
	if !ok || m.args == nil || m.args.id != entry.ID {
		return nil
	}
	return m.args
}

// inField reports whether the keys go to one of the fields rather than the
// query.
func (m model) inField() bool {
	f := m.form()
	return f != nil && f.focus >= 0
}

// editForm is the selected row's form to change, started over if there is
// none yet.
func (m model) editForm(entry palette.Entry) *argForm {
	if f := m.form(); f != nil {
		return f.clone()
	}
	return newArgForm(entry)
}

// focusField moves the keys to argument i of the selected row, or back to the
// query for -1.
func (m *model) focusField(entry palette.Entry, i int) {
	f := m.editForm(entry)
	f.focusOn(entry, i)
	m.args = f
	m.failure, m.notice = "", ""
}

// filled is the entry ready to run with what was entered for it. When an
// argument it cannot run without has no value, the keys go to that argument
// instead and the footer says what it is waiting for.
func (m *model) filled(entry palette.Entry) (palette.Entry, bool) {
	f := m.editForm(entry)
	// A dropdown still listing its options, or listed for values that have
	// changed since, is checked again before the script runs with an option
	// that may no longer apply.
	for i := range entry.Arguments {
		reason := ""
		switch {
		case f.loading[i]:
			reason = " is still loading"
		case f.picked[i] && f.stale(entry, i):
			reason = " is listed again for the values before it"
		}
		if reason != "" {
			f.focusOn(entry, i)
			m.args = f
			m.failure = entry.Arguments[i].Name + reason
			return entry, false
		}
	}
	if missing := f.missing(entry); missing >= 0 {
		f.focusOn(entry, missing)
		m.args = f
		m.failure = entry.Arguments[missing].Name + " is required"
		return entry, false
	}
	m.args = f
	entry.Args = f.values(entry)
	return entry, true
}

// keyField answers a key while one of the fields has the focus.
func (m model) keyField(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	entry, _ := m.fielded()
	f := m.args.clone()
	last := len(entry.Arguments) - 1
	dropdown := entry.Arguments[f.focus].Type == palette.ArgumentDropdown

	switch msg.String() {
	case "esc":
		// Back to the query, keeping what was entered while the row stays
		// selected.
		f.focusOn(entry, -1)
		m.args = f
		return m, nil
	case "tab":
		f.focusOn(entry, min(f.focus+1, last))
		m.args, m.failure = f, ""
		return m, nil
	case "shift+tab":
		f.focusOn(entry, f.focus-1)
		m.args, m.failure = f, ""
		return m, nil
	case "enter":
		m.args = f
		return m.choose()
	case "ctrl+o":
		return m.togglePreview()
	case "down", "ctrl+n", "up", "ctrl+p":
		if dropdown {
			by := 1
			if s := msg.String(); s == "up" || s == "ctrl+p" {
				by = -1
			}
			f.step(entry, f.focus, by)
			m.args = f
		}
		return m, nil
	}
	return m.editField(f, msg)
}

// editField passes a key or a paste to the field that has the focus. What is
// typed into a dropdown filters its options, and the best match becomes its
// option.
func (m model) editField(f *argForm, msg tea.Msg) (tea.Model, tea.Cmd) {
	entry, _ := m.fielded()
	var cmd tea.Cmd
	before := f.fields[f.focus].Value()
	f.fields[f.focus], cmd = f.fields[f.focus].Update(msg)
	if f.fields[f.focus].Value() != before {
		m.failure, m.notice = "", ""
		if entry.Arguments[f.focus].Type == palette.ArgumentDropdown {
			f.filtered(entry, f.focus)
		}
	}
	m.args = f
	return m, cmd
}

// extent is a run of columns on a line.
type extent struct{ start, width int }

// queryLine is what follows the label as it is drawn: the query, and the
// selected script's fields after it.
type queryLine struct {
	text string
	// cursor is the column of the native cursor, -1 when there is none to
	// show: a dropdown is not typed into.
	cursor int
	// mark is what the rule under the line marks out, the label or the field
	// being typed into, and drop the dropdown being chosen from.
	mark extent
	drop extent
}

// box is one field as it is laid out: the text it shows and how wide that
// text is drawn.
type box struct {
	argument palette.Argument
	text     string
	empty    bool
	width    int
}

// boxes lays the fields out at the widths their text asks for, narrowed in
// turn, widest first, until they leave the query room beside them.
func (m model) boxes(entry palette.Entry, f *argForm, room int) []box {
	boxes := make([]box, len(entry.Arguments))
	total := fieldGap
	for i, argument := range entry.Arguments {
		b := box{argument: argument}
		switch {
		case argument.Type == palette.ArgumentDropdown:
			options := argument.Options
			if f != nil {
				options = f.options(entry, i)
			}
			b.text, b.empty = argument.Placeholder, true
			b.width = lipgloss.Width(argument.Placeholder)
			switch {
			case f != nil && f.focus == i && f.fields[i].Value() != "":
				b.text, b.empty = f.fields[i].Value(), false
				b.width = max(b.width, lipgloss.Width(b.text)+1)
			case f != nil && f.picked[i] && f.option[i] >= 0:
				b.text, b.empty = options[f.option[i]].Title, false
			}
			// As wide as its longest option, so the fields after it do not
			// move as the options are gone through.
			for _, option := range options {
				b.width = max(b.width, lipgloss.Width(option.Title))
			}
			b.width += lipgloss.Width(dropMarker)
		case f != nil && f.fields[i].Value() != "":
			b.text = f.fields[i].Value()
			if argument.Type == palette.ArgumentPassword {
				b.text = strings.Repeat("•", len([]rune(b.text)))
			}
			b.width = max(lipgloss.Width(b.text), lipgloss.Width(argument.Placeholder))
			if f.focus == i {
				b.width++
			}
		default:
			b.text, b.empty = argument.Placeholder, true
			b.width = lipgloss.Width(b.text)
		}
		b.width = min(max(b.width, 1), maxField)
		boxes[i] = b
		total += b.width + boxChrome + 1
	}
	for total > room-minQuery {
		widest := 0
		for i := range boxes {
			if boxes[i].width > boxes[widest].width {
				widest = i
			}
		}
		if boxes[widest].width <= minField {
			break
		}
		boxes[widest].width--
		total--
	}
	return boxes
}

// line lays out what follows the label: the query, the selected script's
// fields, and where the cursor and the rule's mark go.
func (m model) line() queryLine {
	label, _ := m.mode()
	lead := m.queryLead()
	room := max(m.cols()-lead, 1)
	out := queryLine{cursor: -1, mark: extent{1, len(label) + 2}}

	entry, fielded := m.fielded()
	f := m.form()
	var boxes []box
	queryRoom := room
	if fielded {
		boxes = m.boxes(entry, f, room)
		used := fieldGap
		for _, b := range boxes {
			used += b.width + boxChrome + 1
		}
		queryRoom = max(room-used, 1)
	}
	inField := f != nil && f.focus >= 0

	s := m.styles.plain
	var b strings.Builder
	shown := 0
	runes := []rune(m.query.Value())
	switch {
	case inField:
		// The query no longer takes the keys, so the line names the command
		// the fields belong to in its place.
		title := truncate(entry.Title, queryRoom)
		b.WriteString(s.text.Bold(true).Render(title))
		shown = lipgloss.Width(title)
	case len(runes) == 0:
		// On a script asking for arguments the empty line names it, so the
		// fields after it read as that script's.
		placeholder := m.query.Placeholder
		if fielded {
			placeholder = entry.Title
		}
		placeholder = truncate(placeholder, queryRoom)
		b.WriteString(s.faint.Render(placeholder))
		shown = lipgloss.Width(placeholder)
		out.cursor = lead
	default:
		at := min(max(m.query.Position(), 0), len(runes))
		from, to := window(runes, at, queryRoom)
		b.WriteString(s.text.Render(string(runes[from:to])))
		shown = lipgloss.Width(string(runes[from:to]))
		out.cursor = lead + lipgloss.Width(string(runes[from:at]))
	}
	if !fielded {
		out.text = b.String()
		return out
	}

	gap := 1 + fieldGap
	b.WriteString(strings.Repeat(" ", gap))
	col := lead + shown + gap
	for i, bx := range boxes {
		if i > 0 {
			b.WriteString(" ")
			col++
		}
		focused := inField && f.focus == i
		text, cursor := m.boxText(bx, f, i, focused)
		b.WriteString(text)
		width := bx.width + boxChrome
		if focused {
			out.mark = extent{col, width}
			dropdown := bx.argument.Type == palette.ArgumentDropdown
			if dropdown {
				out.drop = extent{col, width}
			}
			// A dropdown shows a caret only while a filter is typed into it.
			if !dropdown || f.fields[i].Value() != "" {
				out.cursor = col + 1 + cursor
			}
		}
		col += width
	}
	out.text = b.String()
	return out
}

// boxText draws a field between its rounded caps, and the column of the caret
// inside its text when it has the focus.
func (m model) boxText(bx box, f *argForm, i int, focused bool) (line string, cursor int) {
	style := m.styles.field
	if bx.empty {
		style = m.styles.fieldEmpty
	}
	if focused {
		style = style.Bold(true)
	}

	text := bx.text
	switch {
	case bx.argument.Type == palette.ArgumentDropdown:
		room := bx.width - lipgloss.Width(dropMarker)
		if focused && f.fields[i].Value() != "" {
			runes := []rune(f.fields[i].Value())
			at := min(max(f.fields[i].Position(), 0), len(runes))
			from, to := window(runes, at, room)
			text = string(runes[from:to])
			cursor = lipgloss.Width(string(runes[from:at]))
		} else {
			text = truncate(text, room)
		}
		text += strings.Repeat(" ", max(room-lipgloss.Width(text), 0)) + dropMarker
	case focused && !bx.empty:
		runes := []rune(bx.text)
		at := min(max(f.fields[i].Position(), 0), len(runes))
		from, to := window(runes, at, bx.width)
		text = string(runes[from:to])
		cursor = lipgloss.Width(string(runes[from:at]))
	default:
		text = truncate(text, bx.width)
	}
	text += strings.Repeat(" ", max(bx.width-lipgloss.Width(text), 0))
	return m.styles.fieldCap.Render(capLeft) + style.Render(text) + m.styles.fieldCap.Render(capRight), cursor
}

// dropdown draws the options of the dropdown being chosen from over the lines
// under its field, the way a menu opens under the control it belongs to.
func (m model) dropdown(lines []string) []string {
	l := m.line()
	if l.drop.width == 0 || len(lines) == 0 {
		return lines
	}
	entry, _ := m.fielded()
	f := m.form()
	options := f.options(entry, f.focus)
	shown := f.shown(entry, f.focus)

	status := ""
	switch {
	case f.loading[f.focus]:
		status = "loading…"
	case len(options) == 0:
		status = "no options"
	case len(shown) == 0:
		status = "no match"
	}

	cols := m.cols()
	// A status is read whole even where the field is narrower than it.
	width := min(max(l.drop.width, lipgloss.Width(status)+3), cols)
	start := max(min(l.drop.start, cols-width), 0)
	row := func(at int, marker, text string, style lipgloss.Style) {
		text = truncate(text, max(width-3, 1))
		text += strings.Repeat(" ", max(width-3-lipgloss.Width(text), 0))
		drawn := m.styles.optionMarker.Render(marker) + style.Render(" "+text+" ")
		lines[at] = splice(lines[at], start, width, drawn)
	}
	if status != "" {
		row(0, " ", status, m.styles.option)
		return lines
	}
	highlighted := max(slices.Index(shown, f.option[f.focus]), 0)
	rows := min(len(shown), len(lines))
	first := max(highlighted-rows+1, 0)
	for r := range rows {
		at := first + r
		marker, style := " ", m.styles.option
		if at == highlighted {
			marker, style = selectionMarker, m.styles.optionPicked
		}
		row(r, marker, options[shown[at]].Title, style)
	}
	return lines
}

// splice puts drawn over the columns from start, width wide, of a drawn line.
// A wide character cut in half on either side gives way to a blank.
func splice(line string, start, width int, drawn string) string {
	left := ansi.Truncate(line, start, "")
	left += strings.Repeat(" ", max(start-lipgloss.Width(left), 0))
	right := ansi.TruncateLeft(line, start+width, "")
	pad := lipgloss.Width(line) - start - width - lipgloss.Width(right)
	return left + drawn + strings.Repeat(" ", max(pad, 0)) + right
}

// passed is what the script will receive for argument i as things stand,
// hidden for a password.
func (m model) passed(entry palette.Entry, i int) string {
	f := m.form()
	if f == nil {
		return ""
	}
	argument := entry.Arguments[i]
	value := argument.Pass(f.value(entry, i))
	if argument.Type == palette.ArgumentPassword {
		value = strings.Repeat("•", len([]rune(value)))
	}
	return value
}
