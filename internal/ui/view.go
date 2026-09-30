package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// tone is the set of styles one row is drawn in. The selected row's differ by
// the band behind it, which every segment carries itself: a style wrapped
// around text that already contains escape sequences would be cut short by
// the first reset inside it.
type tone struct {
	text   lipgloss.Style
	meta   lipgloss.Style
	faint  lipgloss.Style
	match  lipgloss.Style
	marker lipgloss.Style
	origin lipgloss.Style
	// status is what a row's state is drawn in, by herdr's name for it.
	status map[string]lipgloss.Style
}

// styles are the popup's rendered colours.
type styles struct {
	plain, picked tone

	rule  lipgloss.Style
	thumb lipgloss.Style
	fail  lipgloss.Style
	// chip is the label naming the screen, and heavy the stretch of rule
	// under it; the danger pair is the same on a screen asking about
	// something that cannot be undone.
	chip, chipDanger   lipgloss.Style
	heavy, heavyDanger lipgloss.Style
}

func newStyles(colours theme.Theme) styles {
	band := lipgloss.NewStyle().Background(colours.Selected)
	fg := func(colour lipgloss.TerminalColor) lipgloss.Style { return lipgloss.NewStyle().Foreground(colour) }
	// Reversed rather than given a background of the accent: the label's text
	// is then the terminal's own background, whatever that is.
	chip := func(colour lipgloss.TerminalColor) lipgloss.Style { return fg(colour).Reverse(true).Bold(true) }

	return styles{
		plain: tone{
			text:   lipgloss.NewStyle(),
			meta:   fg(colours.Meta),
			faint:  fg(colours.Faint),
			match:  fg(colours.Match).Bold(true),
			marker: lipgloss.NewStyle(),
			origin: fg(colours.Accent),
			status: statusStyles(colours, lipgloss.NewStyle()),
		},
		picked: tone{
			text:  band.Bold(true),
			meta:  band.Foreground(colours.Meta),
			faint: band.Foreground(colours.Faint),
			// Underlined as well as coloured: the band behind the selected
			// row and the query's letters are separate colours that a
			// terminal with no room for both may round to the same one.
			match:  band.Foreground(colours.Match).Bold(true).Underline(true),
			marker: band.Foreground(colours.Accent),
			origin: band.Foreground(colours.Accent),
			status: statusStyles(colours, band),
		},

		rule:        fg(colours.Rule),
		thumb:       fg(colours.Scrollbar),
		fail:        fg(colours.Failure),
		chip:        chip(colours.Accent),
		chipDanger:  chip(colours.Failure),
		heavy:       fg(colours.Accent),
		heavyDanger: fg(colours.Failure),
	}
}

func statusStyles(colours theme.Theme, base lipgloss.Style) map[string]lipgloss.Style {
	styles := make(map[string]lipgloss.Style, len(colours.Status))
	for status, colour := range colours.Status {
		styles[status] = base.Foreground(colour)
	}
	return styles
}

func (s styles) tone(selected bool) tone {
	if selected {
		return s.picked
	}
	return s.plain
}

// state is the colour of a state by its name, or the detail's own dim one for
// a state with no colour of its own.
func (t tone) state(status string) lipgloss.Style {
	if style, ok := t.status[status]; ok {
		return style
	}
	return t.meta
}

// Sizes used until the first resize message arrives.
const (
	defaultCols = 72
	defaultRows = 12
)

// minCols is the narrowest a row can be drawn in: the selection and a blank,
// one title cell, a blank, the origin marker and the scrollbar.
// Below it a row cannot be built at all, so the rows are drawn to this width
// and the popup is the one that does not fit them, rather than the arithmetic
// coming apart.
const minCols = 6

// searchPlaceholder stands in the empty query line of the command list. A
// scope names what it lists there instead, and a list of targets what it is
// collecting.
const searchPlaceholder = "Search commands"

func placeholder(scope palette.Scope) string {
	if scope == palette.ScopePalette {
		return searchPlaceholder
	}
	return "Search " + scope.Name
}

const (
	// chrome is the row budget the list does not get: the query line, the two
	// rules, and the help line.
	chrome = 4
	// headerRows is how many lines precede the first line of the list: the
	// query line and the rule under it. A click's row is counted from there.
	headerRows = 2
	// margins includes the selection and its blank, the blank and origin
	// marker at the right edge, and the scrollbar.
	margins = 5
	// columnGap is the blank between two columns of a row.
	columnGap = 2
	// sectionGap separates the title and key from right-aligned metadata.
	sectionGap = 4
	// detailShare is the fraction of the list the detail column may take, and
	// minDetail the width below which it is left out: what the row is comes
	// first, and a couple of letters of context explain nothing. A detail
	// shorter than that is kept whole.
	detailShare = 3
	minDetail   = 10
	// minContent is what a row keeps for what it is and the detail beside it
	// before the columns to its right are worth their width: a title of the
	// length these commands run to, and the least detail worth drawing. Below
	// it the key column is left out first and the source after it, rather
	// than shortened, since half a key names nothing, and what an agent is
	// doing is worth more than the key beside a command.
	minContent = 34
	// minTitle is the least title a row keeps once the detail is in: below
	// it the detail goes, since a row has to say what it is.
	minTitle = 12
	// maxSource caps the source column: a plugin's name can run long, and
	// the rest of it is in the line under the list.
	maxSource = 16
)

// selectionMarker stands in the leading column of the selected row. The band
// behind the row says the same thing, but the marker survives a terminal or a
// theme that draws no background.
const selectionMarker = "▌"

// originMarker stays on the invoking place while selection moves elsewhere.
const originMarker = "•"

// queryLead is what precedes the query on its line: a blank, the label naming
// the screen with its padding, and the blank after it. It follows the label
// rather than reserving room for the longest one, so the query moves when the
// label changes; a scope starts on an empty query, and a question's label is up
// only while nothing is typed into the query.
func (m model) queryLead() int {
	label, _ := m.mode()
	return 1 + len(label) + 2 + 1
}

// Labels naming the screen.
const (
	modePick    = "PICK"
	modeConfirm = "CONFIRM"
)

// summaryOrder is the states the rule under the query counts, in the order
// they are named: the ones waiting on you first.
var summaryOrder = []string{"blocked", "done", "working"}

// minRuleRun is the least of the rule left between the label's stretch of it
// and the count at its end, so the two do not read as one.
const minRuleRun = 6

func (m model) cols() int {
	if m.width <= 0 {
		return defaultCols
	}
	return max(m.width, minCols)
}

// rows is how many lines of the list fit under the query line.
func (m model) rows() int {
	if m.height <= 0 {
		return defaultRows
	}
	if rows := m.height - chrome; rows > 0 {
		return rows
	}
	return 1
}

func (m model) View() tea.View {
	lines := []string{m.header(), m.topRule()}
	lines = append(lines, m.body()...)
	lines = append(lines, m.rule(), m.footer())
	column := m.queryCursorColumn()
	visible := m.replying == nil && m.confirming == nil && m.width > column && m.height >= len(lines)
	view := tea.NewView(strings.Join(lines, "\n"))
	view.MouseMode = tea.MouseModeCellMotion
	if visible {
		view.Cursor = tea.NewCursor(column, 0)
	}
	return view
}

// mode is the label naming the screen on show, and whether that screen is
// asking about something that cannot be undone.
func (m model) mode() (string, bool) {
	switch {
	case m.replying != nil:
		return modeReply, false
	case m.confirming != nil:
		return modeConfirm, true
	case m.choosing != nil:
		return modePick, false
	}
	return m.scope.Label(), false
}

func (m model) header() string {
	label, danger := m.mode()
	chip := m.styles.chip
	if danger {
		chip = m.styles.chipDanger
	}
	lead := " " + chip.Render(" "+label+" ") + " "
	if m.replying != nil {
		// What is typed goes to the agent, so the line names it rather than
		// showing a query nothing is typed into.
		name := strings.TrimPrefix(m.replying.Entry.Title, palette.GoTo)
		return truncate(lead+m.styles.plain.text.Bold(true).Render(name), m.cols())
	}
	return truncate(lead+m.queryLine(max(m.cols()-m.queryLead(), 1)), m.cols())
}

// queryLine draws text continuously. View.Cursor supplies the native cursor
// to the framework so input methods can locate the insertion point.
func (m model) queryLine(room int) string {
	s := m.styles.plain
	runes := []rune(m.query.Value())
	if len(runes) == 0 {
		return s.faint.Render(truncate(m.query.Placeholder, room))
	}
	at := min(max(m.query.Position(), 0), len(runes))
	from, to := window(runes, at, room)
	return s.text.Render(string(runes[from:to]))
}

func (m model) queryCursorColumn() int {
	runes := []rune(m.query.Value())
	at := min(max(m.query.Position(), 0), len(runes))
	from, _ := window(runes, at, max(m.cols()-m.queryLead(), 1))
	return m.queryLead() + lipgloss.Width(string(runes[from:at]))
}

// window is the part of the query on show, which is all of it until it
// outgrows the line. One column remains available for the native cursor when
// the insertion point is at the end of the value.
func window(runes []rune, at, room int) (from, to int) {
	for from < at && lipgloss.Width(string(runes[from:at]))+1 > room {
		from++
	}
	for to = at; to < len(runes); to++ {
		if lipgloss.Width(string(runes[from:to+1]))+1 > room {
			break
		}
	}
	return from, to
}

// topRule is the rule under the query line: heavier under the label, in its
// colour, with how many agents are in each state at its end.
func (m model) topRule() string {
	label, danger := m.mode()
	heavy := m.styles.heavy
	if danger {
		heavy = m.styles.heavyDanger
	}
	cols, chip := m.cols(), len(label)+2
	if cols < 1+chip {
		return m.styles.rule.Render(strings.Repeat("─", cols))
	}

	tail := ""
	if summary := m.summary(cols - 1 - chip - minRuleRun); summary != "" {
		tail = " " + summary + " " + m.styles.rule.Render("─")
	}
	run := cols - 1 - chip - lipgloss.Width(tail)
	return m.styles.rule.Render("─") + heavy.Render(strings.Repeat("━", chip)) +
		m.styles.rule.Render(strings.Repeat("─", run)) + tail
}

// summary counts the agents of the session by state, in no more than room
// columns once the blanks and the rule's last dash around it are taken. It
// drops the names of the states before it drops the counts, since the colour
// of each count says the same.
func (m model) summary(room int) string {
	counts := map[string]int{}
	for _, status := range m.source.Session.Statuses {
		counts[status]++
	}

	s := m.styles.plain
	var named, bare []string
	for _, status := range summaryOrder {
		n := counts[status]
		if n == 0 {
			continue
		}
		count := s.state(status).Render("● " + strconv.Itoa(n))
		named = append(named, count+s.meta.Render(" "+status))
		bare = append(bare, count)
	}
	for _, parts := range [][]string{named, bare} {
		if text := strings.Join(parts, "  "); text != "" && lipgloss.Width(text)+3 <= room {
			return text
		}
	}
	return ""
}

// body is the list, with the preview beside it where the popup is wide
// enough for one.
func (m model) body() []string {
	if m.replying != nil {
		return m.replyLines(m.rows())
	}
	list := m.listLines(m.listWidth())
	width := m.previewWidth()
	if width == 0 {
		return list
	}
	panel := m.previewLines(width, len(list))
	for i := range list {
		list[i] += panel[i]
	}
	return list
}

func (m model) listLines(width int) []string {
	rows := m.rows()
	c := m.columns(width)
	lines := make([]string, 0, rows)

	if len(m.ranked) == 0 {
		lines = append(lines, m.fill(m.styles.plain.meta.Render(truncate("  "+m.empty(), width-1)), width-1)+m.scrollbar(0))
	}
	for i := m.offset; i < len(m.lines) && len(lines) < rows; i++ {
		l := m.lines[i]
		if l.isHeading() {
			heading := m.styles.plain.faint.Render(truncate("  "+l.heading, width-1))
			lines = append(lines, m.fill(heading, width-1)+m.scrollbar(len(lines)))
			continue
		}
		lines = append(lines, m.row(l.row, c)+m.scrollbar(len(lines)))
	}
	for len(lines) < rows {
		lines = append(lines, strings.Repeat(" ", width-1)+m.scrollbar(len(lines)))
	}
	return lines
}

// fill pads a rendered line with blanks to width.
func (m model) fill(rendered string, width int) string {
	return rendered + strings.Repeat(" ", max(width-lipgloss.Width(rendered), 0))
}

// columns holds the row width and metadata budgets, zero when omitted.
type columns struct {
	width, detail, source, key int
}

func span(width int) int {
	if width == 0 {
		return 0
	}
	return width + columnGap
}

// columns budgets room for the longest detail, source and key on show. Keys
// follow their titles; details and sources each align to a right-hand edge.
// The budgets give way in the order that keeps titles readable.
func (m model) columns(width int) columns {
	body := max(width-margins, 1)
	c := columns{
		width:  width,
		detail: min(m.widths.detail, width/detailShare),
		source: m.widths.source,
		key:    m.widths.key,
	}
	if c.detail < min(m.widths.detail, minDetail) {
		c.detail = 0
	}
	// In a scope of places the label already says what every row is, and the
	// detail says which pane runs an agent.
	if m.narrowed() && m.scope.Places() {
		c.source = 0
	}
	room := func() int { return body - span(c.detail) - span(c.source) - span(c.key) }
	if c.key > 0 && room()+c.detail < minContent {
		c.key = 0
	}
	if c.source > 0 && room()+c.detail < minContent {
		c.source = 0
	}
	if c.detail > 0 && room() < minTitle {
		c.detail = 0
	}
	return c
}

// widths are the widest detail, source and key among the rows, used to decide
// which metadata can fit beside a title.
type widths struct {
	detail, source, key int
}

func measure(ranked []palette.Ranked, prefix string) widths {
	var w widths
	for _, r := range ranked {
		w.detail = max(w.detail, detailWidth(r))
		w.source = max(w.source, lipgloss.Width(source(r.Entry)))
		lead, key := spelling.keycap(r.Entry.Key, prefix)
		w.key = max(w.key, lipgloss.Width(lead+key))
	}
	w.source = min(w.source, maxSource)
	return w
}

// detailWidth measures the displayed detail, including the compact agent
// state icon instead of the searchable state name.
func detailWidth(r palette.Ranked) int {
	if r.Detail == "" {
		return 0
	}
	if trailingStatus(r) {
		name := strings.TrimSuffix(r.Detail, " · "+r.Status)
		if icon := statusIcon(r.Status); icon != "" {
			return lipgloss.Width(name + " " + icon)
		}
		return lipgloss.Width(name)
	}
	if r.Status != "" {
		return lipgloss.Width(r.Detail) + 2
	}
	return lipgloss.Width(r.Detail)
}

// source is where a row comes from, which the row draws in a column of its
// own at the right rather than in front of the title.
func source(entry palette.Entry) string {
	return strings.TrimSuffix(entry.Namespace(), ": ")
}

func (m model) row(index int, c columns) string {
	ranked := m.ranked[index]
	entry := ranked.Entry
	selected := index == m.cursor
	t := m.styles.tone(selected)

	marker := t.text.Render(" ")
	if selected {
		marker = t.marker.Render(selectionMarker)
	}

	lead, key := "", ""
	if c.key > 0 {
		lead, key = spelling.keycap(entry.Key, m.prefix)
	}
	keyWidth := lipgloss.Width(lead + key)
	keySpan := 0
	if keyWidth > 0 {
		keySpan = keyWidth + columnGap
	}
	detail := ""
	if c.detail > 0 && ranked.Detail != "" {
		detail = m.detail(ranked, c.detail, t)
	}
	detailRoom := lipgloss.Width(detail)
	from := ""
	if c.source > 0 {
		name := truncate(source(entry), c.source)
		from = paint(name, before(ranked.Matched, len([]rune(name))), t.faint, t.match)
	}
	rightWidth := detailRoom + lipgloss.Width(from)
	if detail != "" && from != "" {
		rightWidth += columnGap
	}
	rightSpan := 0
	if rightWidth > 0 {
		rightSpan = rightWidth + sectionGap
	}
	// Only this row's actual metadata reserves room. A long plugin name
	// elsewhere does not leave an empty source slot beside an agent's state.
	room := max(c.width-margins-keySpan-rightSpan, 1)
	// The matched positions index the whole name, namespace first.
	namespace := len([]rune(entry.Namespace()))
	title := m.title(entry, truncate(entry.Title, room), shift(ranked.Matched, namespace), t)

	var b strings.Builder
	b.WriteString(marker)
	b.WriteString(t.text.Render(" "))
	b.WriteString(title)

	gap := t.text.Render(strings.Repeat(" ", columnGap))
	if keyWidth > 0 {
		b.WriteString(gap)
		if lead != "" {
			b.WriteString(t.faint.Render(lead))
		}
		if key != "" {
			b.WriteString(t.meta.Render(key))
		}
	}
	if rightWidth > 0 {
		start := c.width - 3 - rightWidth
		b.WriteString(t.text.Render(strings.Repeat(" ", max(start-lipgloss.Width(b.String()), sectionGap))))
		b.WriteString(detail)
		if detail != "" && from != "" {
			b.WriteString(gap)
		}
		b.WriteString(from)
	}
	b.WriteString(t.text.Render(strings.Repeat(" ", max(c.width-2-lipgloss.Width(b.String()), 0))))
	if entry.Here {
		b.WriteString(t.origin.Render(originMarker))
	} else {
		b.WriteString(t.text.Render(" "))
	}
	return b.String()
}

// title draws what a row is, with the runes the query matched picked out. On
// a row that goes somewhere the words saying so are drawn faint: they are on
// every such row, and the name after them is what tells the rows apart.
func (m model) title(entry palette.Entry, text string, matched []int, t tone) string {
	verb := 0
	if entry.Goes() && strings.HasPrefix(entry.Title, palette.GoTo) {
		verb = min(len([]rune(palette.GoTo)), len([]rune(text)))
	}
	runes := []rune(text)
	return paint(string(runes[:verb]), matched, t.faint, t.match) +
		paint(string(runes[verb:]), shift(matched, verb), t.text, t.match)
}

// detail replaces agent state names with coloured icons, leaving the name
// dim. Matching the full state name highlights its icon.
func (m model) detail(ranked palette.Ranked, width int, t tone) string {
	if trailingStatus(ranked) {
		name := strings.TrimSuffix(ranked.Detail, " · "+ranked.Status)
		stateAt := len([]rune(name)) + len([]rune(" · "))
		style := t.state(ranked.Status)
		if len(shift(ranked.DetailMatched, stateAt)) > 0 {
			style = t.match
		}
		icon := statusIcon(ranked.Status)
		if icon == "" {
			return paint(truncate(name, width), before(ranked.DetailMatched, len([]rune(name))), t.meta, t.match)
		}
		state := style.Render(icon)
		nameRoom := width - lipgloss.Width(icon) - 1
		if nameRoom <= 0 {
			return state
		}
		return state + t.meta.Render(" ") +
			paint(truncate(name, nameRoom), before(ranked.DetailMatched, len([]rune(name))), t.meta, t.match)
	}
	dot := ""
	room := width
	if ranked.Status != "" {
		dot = t.state(ranked.Status).Render("● ")
		room -= 2
	}
	text := []rune(truncate(ranked.Detail, room))
	stateEnd := 0
	if ranked.Status != "" && (ranked.Detail == ranked.Status || strings.HasPrefix(ranked.Detail, ranked.Status+" · ")) {
		stateEnd = min(len([]rune(ranked.Status)), len(text))
	}
	rendered := dot + paint(string(text[:stateEnd]), before(ranked.DetailMatched, stateEnd), t.state(ranked.Status), t.match) +
		paint(string(text[stateEnd:]), shift(ranked.DetailMatched, stateEnd), t.meta, t.match)
	return rendered
}

func statusIcon(status string) string {
	switch status {
	case "working":
		return "●"
	case "blocked":
		return "!"
	case "done":
		return "✓"
	case "idle":
		return "○"
	default:
		return ""
	}
}

// Searchable agent details store the state last; their display puts its icon
// first. Tab and workspace summaries store the state first.
func trailingStatus(r palette.Ranked) bool {
	return r.Status != "" && r.Entry.Kind != palette.KindTab && r.Entry.Kind != palette.KindWorkspace &&
		strings.HasSuffix(r.Detail, " · "+r.Status)
}

// scrollbar draws where the visible window sits in the whole list, one column
// at the right edge of the list; y is the line of the window it is drawn on.
// It stays blank while everything fits, so rows do not shift as the query
// narrows the list — except beside the preview, where the column is also the
// line between the two.
func (m model) scrollbar(y int) string {
	rows, total := m.rows(), len(m.lines)
	if total <= rows {
		if m.previewWidth() > 0 {
			return m.styles.rule.Render("│")
		}
		return " "
	}

	// Scaled by how far the window can travel rather than by the list length,
	// so the thumb reaches the bottom exactly on the last page.
	size := max(rows*rows/total, 1)
	start := m.offset * (rows - size) / (total - rows)
	if y >= start && y < start+size {
		return m.styles.thumb.Render("┃")
	}
	return m.styles.rule.Render("│")
}

// paint draws text with the runes at the matched positions picked out, in runs
// of one style each, which keeps a screenful of rows down to a handful of
// styled spans.
func paint(text string, matched []int, base, hit lipgloss.Style) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	at := 0
	isMatch := func(index int) bool {
		for at < len(matched) && matched[at] < index {
			at++
		}
		return at < len(matched) && matched[at] == index
	}

	var out strings.Builder
	for start := 0; start < len(runes); {
		current := isMatch(start)
		end := start + 1
		for end < len(runes) && isMatch(end) == current {
			end++
		}
		style := base
		if current {
			style = hit
		}
		out.WriteString(style.Render(string(runes[start:end])))
		start = end
	}
	return out.String()
}

// shift moves the positions onto a slice of the row that starts at by.
func shift(matched []int, by int) []int {
	out := make([]int, 0, len(matched))
	for _, at := range matched {
		if at -= by; at >= 0 {
			out = append(out, at)
		}
	}
	return out
}

// before keeps the positions in front of cut.
func before(matched []int, cut int) []int {
	out := make([]int, 0, len(matched))
	for _, at := range matched {
		if at < cut {
			out = append(out, at)
		}
	}
	return out
}

func (m model) rule() string {
	return m.styles.rule.Render(strings.Repeat("─", m.cols()))
}

// empty is the line that stands where the rows would be when the query leaves
// none: what was searched is what it names.
func (m model) empty() string {
	switch {
	case m.choosing != nil:
		return "no match"
	case !m.narrowed():
		return "no command matches"
	case m.query.Value() == "":
		return m.scope.Empty
	}
	return "no " + m.scope.Noun() + " matches"
}

// hint is one key the footer names, with what it does.
type hint struct {
	key, does string
}

func (m model) hints(hints []hint) string {
	s := m.styles.plain
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, s.text.Render(h.key)+s.meta.Render(" "+h.does))
	}
	return strings.Join(parts, "  ")
}

// footer is the line under the list: what is on show on the left, and what the
// keys do on the right. A failure takes the whole line instead — the popup is
// too small for both — and so does a notice, drawn dim rather than as a
// failure: an entry with nothing to act on did not go wrong.
func (m model) footer() string {
	if m.failure != "" {
		return m.styles.fail.Render(truncate(" "+m.failure, m.cols()))
	}
	if m.notice != "" {
		return m.styles.plain.meta.Render(truncate(" "+m.notice, m.cols()))
	}

	// A row that cannot be undone takes the whole line: what it is about to do
	// is the only thing the next keystroke decides.
	if m.confirming != nil {
		question := " " + m.confirming.Name() + "?"
		if chosen := m.confirming.Chosen; chosen != "" {
			question = " " + m.confirming.Title + "?"
		}
		keys := m.fitHints([]hint{{spelling.name("enter"), "run"}, {"any key", "cancel"}})
		room := max(m.cols()-lipgloss.Width(keys)-2, 1)
		// The namespace repeats what the selected row above already says, and
		// the question mark is what makes the line a question rather than a
		// label, so the namespace goes before the mark does.
		if lipgloss.Width(question) > room {
			question = " " + m.confirming.Title + "?"
		}
		return m.spread(m.styles.fail.Render(truncate(question, room)), keys)
	}

	if m.replying != nil {
		name := strings.TrimPrefix(m.replying.Entry.Title, palette.GoTo)
		keys := m.fitHints([]hint{{spelling.name("esc"), "back"}})
		return m.spread(m.styles.plain.meta.Render(truncate(" keys go to "+name, max(m.cols()-lipgloss.Width(keys)-2, 1))), keys)
	}

	left := m.selectedName()
	act, back := hint{spelling.name("enter"), "run"}, hint{spelling.name("esc"), "close"}
	var more []hint
	if m.cursor < len(m.ranked) {
		switch entry := m.ranked[m.cursor].Entry; {
		case entry.Goes():
			act.does = "go"
		case entry.Scope.Name != "":
			act.does = "open"
		}
	}
	if scope, ok := m.completion(); ok {
		more = append(more, hint{spelling.name("tab"), scope.Name})
	} else if m.choosing == nil && !m.pending && m.cursor < len(m.ranked) && canReply(m.ranked[m.cursor]) {
		more = append(more, hint{spelling.name("tab"), "reply"})
	}
	if m.canClose() {
		more = append(more, closeHint(m.ranked[m.cursor].Entry))
	}
	if m.cols() >= previewMinCols {
		action := "show preview"
		if m.previewEnabled {
			action = "hide preview"
		}
		more = append(more, hint{spelling.name("ctrl+o"), action})
	}
	if m.scope != m.home {
		back.does = "back"
	}
	if m.choosing != nil {
		// The command the targets belong to is no longer on the list, so the
		// footer is where it stays legible.
		left, back = " "+m.choosing.entry.Name(), hint{spelling.name("esc"), "back"}
	}
	if m.pending {
		// What the keystroke asked for is still out on the socket. Knowing it
		// landed is worth more for the moment than what is selected.
		left = " working…"
	}

	// What is on the left gives way first, since it is also on show in the
	// list; the keys are dropped one at a time only once the line is too
	// narrow to leave the left anything worth reading.
	keys := m.fitHints(append(append([]hint{act}, more...), back))
	left = truncate(left, max(m.cols()-lipgloss.Width(keys)-2, 1))
	return m.spread(m.styles.plain.meta.Render(left), keys)
}

// fitHints is as many of the hints as leave the left of the footer room.
func (m model) fitHints(hints []hint) string {
	for n := len(hints); n > 0; n-- {
		if keys := m.hints(hints[:n]); m.fits(keys) {
			return keys
		}
	}
	return ""
}

// spread puts left at the start of the footer and keys at its end, with a
// column to spare at the right edge.
func (m model) spread(left, keys string) string {
	gap := max(m.cols()-lipgloss.Width(left)-lipgloss.Width(keys)-1, 1)
	return left + strings.Repeat(" ", gap) + keys + " "
}

// minName is what the left of the footer is worth keeping: enough for a row
// count, or for the first word of what is selected.
const minName = 10

// fits reports whether the keys stand on the footer with room left for what is
// on the left of it, a column between them and one at the right edge.
func (m model) fits(keys string) bool {
	return lipgloss.Width(keys)+minName+2 <= m.cols()
}

// selectedName is the selected row in full, which is what a row too narrow for
// its own title cannot show. An empty list leaves it blank: the line where the
// rows would be already says the query matched nothing.
func (m model) selectedName() string {
	if m.cursor >= len(m.ranked) {
		return ""
	}
	ranked := m.ranked[m.cursor]
	name := " " + ranked.Entry.Name()
	if ranked.Detail != "" {
		name += "  " + ranked.Detail
	}
	return name
}

// truncate cuts text to fit width columns. Width is counted in terminal cells
// rather than in runes: the row's budget is the popup's own columns, and a
// wide character such as a CJK ideograph or an emoji takes two of them. A row
// measured in runes overruns the popup and wraps, which pushes every row below
// it down a line.
func truncate(text string, width int) string {
	if width < 1 {
		width = 1
	}
	return ansi.Truncate(text, width, "…")
}
