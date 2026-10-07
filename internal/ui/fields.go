package ui

import (
	"slices"
	"strconv"
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
	// scrolls inside it. Fields narrowed to minField that still do not fit
	// scroll along the line instead, since a field narrower than that no
	// longer says what it holds. minQuery is what the query keeps beside the
	// fields before they are narrowed to leave it room.
	maxField = 24
	minField = 12
	minQuery = 10
	// dropMarker follows a dropdown's text.
	dropMarker = " ▾"
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
	// first is the first field on show when they do not all fit the line.
	first int
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

func (e extent) contains(x int) bool { return x >= e.start && x < e.start+e.width }

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
	// fields are where each field is drawn, caps included, and of no width
	// for one scrolled out of sight. first and end bound the run on show, and
	// before and after are where the counts of those out of sight are drawn.
	fields        []extent
	first, end    int
	before, after extent
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
			// The column the caret takes after the text is kept whether the
			// field has the keys or not, so taking and leaving the focus
			// does not move the fields after it.
			b.width = max(lipgloss.Width(b.text)+1, lipgloss.Width(argument.Placeholder))
		default:
			b.text, b.empty = argument.Placeholder, true
			b.width = lipgloss.Width(b.text)
		}
		b.width = min(max(b.width, 1), maxField)
		boxes[i] = b
		total += b.width + m.styles.fieldChrome + 1
	}
	// The text is what is narrowed, so a dropdown keeps its marker beside
	// minField of it.
	text := func(b box) int {
		if b.argument.Type == palette.ArgumentDropdown {
			return b.width - lipgloss.Width(dropMarker)
		}
		return b.width
	}
	for total > room-minQuery {
		widest := 0
		for i := range boxes {
			if text(boxes[i]) > text(boxes[widest]) {
				widest = i
			}
		}
		if text(boxes[widest]) <= minField {
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
	from, to := 0, 0
	queryRoom := room
	if fielded {
		boxes = m.boxes(entry, f, room)
		from, to = m.shownFields(boxes, f, room-minQuery)
		used := fieldGap + m.scrollWidth(from, to, len(boxes))
		for _, b := range boxes[from:to] {
			used += b.width + m.styles.fieldChrome + 1
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
	out.first, out.end = from, to
	out.fields = make([]extent, len(boxes))
	// A count is a way to the fields it counts, brighter under the pointer.
	count := func(text string, at extent) string {
		if m.pointer.Y == queryRow && at.contains(m.pointer.X) {
			return m.styles.plain.meta.Render(text)
		}
		return m.styles.plain.faint.Render(text)
	}
	if from > 0 {
		text := scrollBefore(from)
		out.before = extent{col, lipgloss.Width(text)}
		b.WriteString(count(text, out.before) + " ")
		col += out.before.width + 1
	}
	for i, bx := range boxes {
		if i < from || i >= to {
			continue
		}
		if i > from {
			b.WriteString(" ")
			col++
		}
		focused := inField && f.focus == i
		width := bx.width + m.styles.fieldChrome
		at := extent{col, width}
		// Nothing is drawn over the query line, so the field under the
		// pointer is the one it is over. The one being typed into is marked
		// by the rule under it instead.
		look := m.styles.field
		if !focused && m.pointer.Y == queryRow && at.contains(m.pointer.X) {
			look = m.styles.fieldHovered
		}
		text, cursor := m.boxText(bx, f, i, focused, look)
		b.WriteString(text)
		out.fields[i] = at
		if focused {
			out.mark = extent{col, width}
			dropdown := bx.argument.Type == palette.ArgumentDropdown
			if dropdown {
				out.drop = extent{col, width}
			}
			// A dropdown shows a caret only while a filter is typed into it.
			if !dropdown || f.fields[i].Value() != "" {
				out.cursor = col + m.styles.fieldLead + cursor
			}
		}
		col += width
	}
	if to < len(boxes) {
		text := scrollAfter(len(boxes) - to)
		out.after = extent{col + 1, lipgloss.Width(text)}
		b.WriteString(" " + count(text, out.after))
	}
	out.text = b.String()
	return out
}

// scrollBefore and scrollAfter count the fields scrolled out of sight on
// either side.
func scrollBefore(n int) string { return "‹" + strconv.Itoa(n) }
func scrollAfter(n int) string  { return strconv.Itoa(n) + "›" }

// scrollWidth is what the counts of the fields out of sight take beside the
// run from, to of all of them, a blank between each and the fields included.
func (m model) scrollWidth(from, to, all int) int {
	width := 0
	if from > 0 {
		width += lipgloss.Width(scrollBefore(from)) + 1
	}
	if to < all {
		width += lipgloss.Width(scrollAfter(all-to)) + 1
	}
	return width
}

// shownFields is the run of fields on show, from the form's first, moved only
// as far as it takes to keep the field being typed into in sight: a field
// tabbed to past the end brings one more in at that end rather than the run
// starting over around it.
func (m model) shownFields(boxes []box, f *argForm, room int) (from, to int) {
	first, focus := 0, -1
	if f != nil {
		first, focus = min(f.first, len(boxes)-1), f.focus
	}
	if focus >= 0 && focus < first {
		first = focus
	}
	fits := func(from, to int) bool {
		width := fieldGap + m.scrollWidth(from, to, len(boxes))
		for _, b := range boxes[from:to] {
			width += b.width + m.styles.fieldChrome + 1
		}
		return width <= room
	}
	if fits(0, len(boxes)) {
		return 0, len(boxes)
	}
	for focus >= 0 && first < focus && !fits(first, focus+1) {
		first++
	}
	to = first + 1
	for to < len(boxes) && fits(first, to+1) {
		to++
	}
	return first, to
}

// boxText draws a field in look between its ends, and the column of the caret
// inside its text when it has the focus.
func (m model) boxText(bx box, f *argForm, i int, focused bool, look fieldLook) (line string, cursor int) {
	style := look.text
	if bx.empty {
		style = look.empty
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
	return look.left + style.Render(text) + look.right, cursor
}

// menu is how the options of the dropdown being chosen from are laid out over
// the list: the columns they take, and either a status in place of the
// options or the run of the options its filter keeps that is on show.
type menu struct {
	start, width int
	status       string
	// shown are the options the filter keeps, indexes into options, first the
	// one drawn on the top line, rows how many are drawn, and highlighted the
	// one the dropdown has, an index into shown.
	options     []palette.Choice
	shown       []int
	first, rows int
	highlighted int
}

// menu lays out the dropdown open on the query line l over the list, a
// menu of no width when none is open.
func (m model) menu(l queryLine) menu {
	lines := m.rows()
	if l.drop.width == 0 || lines == 0 {
		return menu{}
	}
	entry, _ := m.fielded()
	f := m.form()
	u := menu{options: f.options(entry, f.focus), shown: f.shown(entry, f.focus)}

	switch {
	case f.loading[f.focus]:
		u.status = "loading…"
	case len(u.options) == 0:
		u.status = "no options"
	case len(u.shown) == 0:
		u.status = "no match"
	}

	cols := m.cols()
	// A status is read whole even where the field is narrower than it.
	u.width = min(max(l.drop.width, lipgloss.Width(u.status)+3), cols)
	u.start = max(min(l.drop.start, cols-u.width), 0)
	if u.status != "" {
		u.rows = 1
		return u
	}
	u.highlighted = max(slices.Index(u.shown, f.option[f.focus]), 0)
	u.rows = min(len(u.shown), lines)
	u.first = max(u.highlighted-u.rows+1, 0)
	return u
}

// dropdown draws the options of the dropdown being chosen from over the lines
// under its field, the way a menu opens under the control it belongs to.
func (m model) dropdown(lines []string, l layout) []string {
	u := l.menu
	if u.width == 0 {
		return lines
	}
	row := func(at int, marker string, markerStyle, style lipgloss.Style, text string) {
		text = truncate(text, max(u.width-3, 1))
		text += strings.Repeat(" ", max(u.width-3-lipgloss.Width(text), 0))
		drawn := markerStyle.Render(marker) + style.Render(" "+text+" ")
		lines[at] = splice(lines[at], u.start, u.width, drawn)
	}
	if u.status != "" {
		row(0, " ", m.styles.option, m.styles.option, u.status)
		return lines
	}
	hovered := l.hover.of(targetOption)
	for r := range u.rows {
		at := u.first + r
		switch {
		case at == u.highlighted:
			row(r, selectionMarker, m.styles.optionMarker, m.styles.optionPicked, u.options[u.shown[at]].Title)
		case u.shown[at] == hovered:
			row(r, " ", m.styles.optionHovered, m.styles.optionHovered, u.options[u.shown[at]].Title)
		default:
			row(r, " ", m.styles.option, m.styles.option, u.options[u.shown[at]].Title)
		}
	}
	return lines
}

// at is the option the pointer at x, y is over, an index into options, or -1
// over a status. over reports whether the pointer is on the menu at all.
func (u menu) at(x, y int) (option int, over bool) {
	r := y - headerRows
	if u.width == 0 || r < 0 || r >= u.rows || x < u.start || x >= u.start+u.width {
		return -1, false
	}
	if u.status != "" {
		return -1, true
	}
	return u.shown[u.first+r], true
}

// fieldAt is the field drawn at column x, or -1.
func (l queryLine) fieldAt(x int) int {
	for i, field := range l.fields {
		if field.contains(x) {
			return i
		}
	}
	return -1
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
