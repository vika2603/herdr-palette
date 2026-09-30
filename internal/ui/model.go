package ui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// ranMsg carries the outcome of the command the user chose.
type ranMsg struct {
	epoch int
	err   error
}

// changedMsg says herdr reported a change to what is open, and openMsg carries
// the rebuilt rows. A failed rebuild keeps the rows the popup already has: a
// stale row goes to a pane that is gone, which herdr answers with an error the
// popup shows.
type (
	changedMsg struct{}
	openMsg    struct{ session palette.Session }
)

// choicesMsg carries what an entry can act on, once herdr has answered with
// the list. The entry travels with it, and the query the command list was
// filtered by when it was asked for: both can have moved on while the call
// was out.
type choicesMsg struct {
	epoch   int
	entry   palette.Entry
	query   string
	choices []palette.Choice
	err     error
}

// chooser is the entry waiting for its target while the palette shows what it
// can act on. query is what the command list was filtered by, which comes back
// when the choice is abandoned.
type chooser struct {
	entry palette.Entry
	query string
}

type model struct {
	ctx        context.Context
	env        *plugin.Env
	invocation *herdr.PluginInvocationContext

	// source is the commands, fixed while the popup is up, and what the
	// session holds. targets is what an entry can act on while that is up.
	source  palette.List
	targets []palette.Entry
	// scope is what the rows are drawn from, and home the scope the popup was
	// opened in: esc closes the popup from there rather than leaving it.
	scope palette.Scope
	home  palette.Scope
	// changes carries a signal per batch of herdr events, at most one waiting.
	changes <-chan struct{}
	recent  []string
	// choosing is set while the rows are an entry's targets rather than the
	// command list, and confirming while a row that cannot be undone is
	// waiting for the second keystroke that runs it. closing says the row
	// waiting closes what a row of the list goes to rather than running a
	// command, which leaves the popup up.
	choosing   *chooser
	confirming *palette.Entry
	closing    bool
	// replying is the blocked agent the keyboard has been handed to, whose
	// screen stands in the list's place until esc gives it back.
	replying *palette.Ranked
	// queued are keys pressed for the agent while the last ones were still on
	// their way, and sending whether some are: the socket takes each request
	// on a connection of its own, so two in flight could land out of order.
	queued  []string
	sending bool
	// closer is the toggle key, which closes the popup from inside: herdr
	// hands every key to a popup while one is up.
	closer closer
	// state is shared by every copy of the model, so the program's exit can
	// wait for a write a command started.
	state *stateGate

	query  textinput.Model
	ranked []palette.Ranked
	// lines is the list as it is drawn, with the headings of its groups, and
	// offset the first of them on show. cursor indexes ranked: a heading is
	// never selected.
	lines  []line
	cursor int
	offset int
	// widths are what the columns of a row are drawn to.
	widths widths
	// prefix is the key a prefix chord starts with, which the key column
	// spells out.
	prefix string

	// epoch counts the screens the popup has walked out of. A request to the
	// socket carries the one it was made in, and an answer from an earlier
	// screen is dropped: acting on it would pull the popup back to a step the
	// user has already left.
	epoch int

	styles  styles
	failure string
	// notice is what the footer says in place of a failure when nothing went
	// wrong: an entry with nothing to act on.
	notice string
	// pending is set while what a keystroke asked for is out on the socket,
	// so a slow answer does not read as a keystroke that went nowhere.
	pending       bool
	width, height int

	// preview is the last screen read for the preview, and previewSeq moves
	// on whenever the pane it should show changes.
	previewEnabled bool
	preview        preview
	previewSeq     int
	previewReads   int
}

func newModel(
	ctx context.Context,
	env *plugin.Env,
	invocation *herdr.PluginInvocationContext,
	list palette.List,
	recent []string,
	colours theme.Theme,
	toggle Toggle,
) model {
	styles := newStyles(colours)

	// The label naming the screen stands where a prompt would.
	query := textinput.New()
	query.Placeholder = searchPlaceholder
	// The field owns editing; View positions the framework's native cursor.
	query.SetVirtualCursor(false)
	query.Focus()

	m := model{
		ctx:            ctx,
		env:            env,
		invocation:     invocation,
		source:         list,
		scope:          palette.ScopePalette,
		home:           palette.ScopePalette,
		prefix:         toggle.Prefix,
		recent:         recent,
		query:          query,
		styles:         styles,
		closer:         newCloser(toggle),
		state:          &stateGate{},
		previewEnabled: true,
	}
	m.changes = watch(ctx, env)
	m.rank()
	return m
}

// setSize takes the popup's dimensions and keeps the selection visible.
func (m *model) setSize(width, height int) {
	m.width, m.height = width, height
	m.reveal()
}

// openIn opens the popup in a scope before the first frame, which is how it
// opens already narrowed to what the key that opened it asked for.
func (m *model) openIn(scope palette.Scope) {
	m.home = scope
	m.enter(scope)
}

// enter draws the rows from the scope. The word that named it is not a query
// for what is in it, so the query starts over.
func (m *model) enter(scope palette.Scope) {
	m.scope = scope
	m.query.SetValue("")
	m.query.Placeholder = placeholder(scope)
	m.failure, m.notice = "", ""
	m.rank()
}

// leave puts the command list back. Once the scope the popup opened in has
// been left, esc closes the popup the way it does from the command list.
func (m *model) leave() {
	m.home = palette.ScopePalette
	m.enter(palette.ScopePalette)
}

// narrowed reports whether a scope other than the command list is on.
func (m model) narrowed() bool { return m.scope != palette.ScopePalette }

// Init starts following the session. The terminal blinks the native cursor;
// the text field's virtual cursor is not drawn.
func (m model) Init() tea.Cmd { return tea.Batch(listen(m.changes), tea.RequestBackgroundColor) }

// Update answers a message, and asks for the preview again once the pane it
// should show has changed, whatever the message did to change it.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	target := m.previewTarget()
	next, cmd := m.update(msg)
	updated, ok := next.(model)
	if !ok {
		return next, cmd
	}
	if after := updated.previewTarget(); after != target {
		updated.previewSeq++
		if after != "" {
			cmd = tea.Batch(cmd, updated.previewAfter(previewSettle))
		}
	}
	return updated, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		lipgloss.SetHasDarkBackground(msg.IsDark())
		return m, nil

	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case ranMsg:
		if msg.epoch != m.epoch {
			// The screen it was run from has been left, so its outcome has no
			// say in what the popup shows next. That it failed is still worth
			// saying: the command went out and did not work, wherever the
			// reader is now.
			if msg.err != nil {
				m.pending = false
				m.failure = msg.err.Error()
			}
			return m, nil
		}
		if msg.err == nil {
			return m, tea.Quit
		}
		// A command that failed leaves the popup open with the reason, so the
		// keystroke is not lost silently.
		m.pending = false
		m.failure = msg.err.Error()
		return m, nil

	case closedMsg:
		if msg.epoch == m.epoch {
			m.pending = false
		}
		if msg.err != nil {
			m.failure = msg.err.Error()
		} else {
			m.notice = "closed " + msg.name
		}
		return m, nil

	case changedMsg:
		return m, tea.Batch(m.reload(), listen(m.changes))

	case openMsg:
		m.setOpen(msg.session)
		if m.replying != nil {
			return m, m.followReply()
		}
		return m, nil

	case previewTickMsg:
		if msg.seq != m.previewSeq || m.previewTarget() == "" {
			return m, nil
		}
		return m, m.readPreview(m.previewTarget(), false)

	case previewEchoMsg:
		if msg.seq != m.previewSeq || m.previewTarget() == "" {
			return m, nil
		}
		return m, m.readPreview(m.previewTarget(), true)

	case previewMsg:
		if msg.seq != m.previewSeq {
			return m, nil
		}
		if msg.read > m.preview.read {
			m.preview = preview{pane: msg.pane, text: msg.text, err: msg.err, read: msg.read}
		}
		if msg.echo {
			return m, nil
		}
		if m.replying != nil {
			return m, m.previewAfter(replyRefresh)
		}
		return m, m.previewAfter(previewRefresh)

	case replySentMsg:
		m.sending = false
		if msg.err != nil {
			m.failure = msg.err.Error()
		}
		if m.replying == nil {
			m.queued = nil
			return m, nil
		}
		// Read once the key has had time to show, beside the refresh already
		// running: restarting that for every key would hold every read off
		// for as long as keys keep coming.
		seq := m.previewSeq
		echo := tea.Tick(replyEcho, func(time.Time) tea.Msg { return previewEchoMsg{seq: seq} })
		return m, tea.Batch(m.flushReply(), echo)

	case choicesMsg:
		if msg.epoch != m.epoch {
			return m, nil
		}
		m.choices(msg)
		return m, nil

	case tea.KeyPressMsg:
		return m.key(msg)

	case tea.PasteMsg:
		if m.closer.armed {
			m.closer.armed = false
			return m, nil
		}
		if m.replying != nil {
			return m.queueReply(replyTextKeys(msg.Content))
		}
		if m.confirming != nil {
			m.confirming = nil
			return m, nil
		}
		return m.editQuery(msg)

	case tea.MouseClickMsg:
		return m.mouse(msg.Mouse())
	case tea.MouseWheelMsg:
		return m.mouse(msg.Mouse())
	}
	return m, nil
}

// wheelStep is how many rows one wheel notch moves the selection.
const wheelStep = 3

// mouse moves the selection with the wheel and runs the row a click lands on.
// The pointer on its own moves nothing: the selection belongs to the keyboard,
// and a row taken over by where the pointer came to rest is a row the next
// enter would run unread.
func (m model) mouse(msg tea.Mouse) (tea.Model, tea.Cmd) {
	// The agent's screen is not a list, and a click is not an answer to it.
	if m.replying != nil {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.move(-wheelStep)
		return m, nil
	case tea.MouseWheelDown:
		m.move(wheelStep)
		return m, nil
	case tea.MouseLeft:
		index, ok := m.rowAt(msg.X, msg.Y)
		if !ok {
			// A click off the rows is not an answer, so it puts the question
			// away rather than leaving it up over a list being read.
			m.settle()
			return m, nil
		}
		if m.confirming != nil {
			// A click on the row that is waiting answers it the way enter
			// does; one on any other row puts the question away and takes the
			// selection there, the way any other key puts it away.
			entry := *m.confirming
			m.confirming = nil
			if index == m.cursor {
				m.pending = true
				return m, m.answer(entry)
			}
			m.cursor = index
			return m, nil
		}
		m.cursor = index
		return m.choose()
	}
	return m, nil
}

// rowAt is the entry the pointer is over, and whether it is over one at all.
// A row has to have been drawn: the rule under the list and the line below it
// are inside the popup too, and the arithmetic alone would read them as the
// rows that would have been there had the window been taller. A group's
// heading and the preview beside the list are not rows either.
func (m model) rowAt(x, y int) (int, bool) {
	if y < headerRows || y >= headerRows+m.rows() || x >= m.listWidth() {
		return 0, false
	}
	at := m.offset + y - headerRows
	if at >= len(m.lines) || m.lines[at].isHeading() {
		return 0, false
	}
	return m.lines[at].row, true
}

func (m model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	// The key that opened the palette closes it. It comes here rather than
	// to the action, because herdr hands every key to a popup while one is up.
	closes, taken := m.closer.press(msg)
	if closes {
		return m, tea.Quit
	}
	if taken {
		return m, nil
	}
	if m.replying != nil {
		return m.keyReply(msg)
	}
	return m.keyList(msg)
}

func (m model) keyList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A row waiting to be confirmed takes the next keystroke whatever it is:
	// enter runs it, anything else puts the question away. Nothing reaches
	// the query in between, so the answer cannot be typed into it by mistake.
	if m.confirming != nil {
		entry := *m.confirming
		m.confirming = nil
		if msg.String() == "enter" {
			m.pending = true
			return m, m.answer(entry)
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+o":
		if m.cols() < previewMinCols {
			m.notice = "widen the palette to show a preview"
			return m, nil
		}
		m.previewEnabled = !m.previewEnabled
		m.preview = preview{}
		m.failure, m.notice = "", ""
		return m, nil
	case "esc":
		// A list of targets or a scope is a step inside the palette, so esc
		// goes back to the commands rather than closing the popup.
		if m.choosing != nil {
			m.abandon()
			return m, nil
		}
		if m.scope != m.home {
			m.leave()
			return m, nil
		}
		return m, tea.Quit
	case "down", "ctrl+n":
		m.step(1)
		return m, nil
	case "up", "ctrl+p":
		m.step(-1)
		return m, nil
	case "pgdown":
		m.move(m.rows())
		return m, nil
	case "pgup":
		m.move(-m.rows())
		return m, nil
	case "enter":
		return m.choose()
	case "tab":
		// A query that starts the name of a scope completes to it. Otherwise,
		// on an agent that is waiting, tab moves the keyboard over to the
		// agent. Elsewhere it types nothing and does nothing.
		//
		// Not while a command is out: its answer would close the popup or
		// put a list up under the agent's screen.
		if scope, ok := m.completion(); ok {
			m.enter(scope)
			return m, nil
		}
		if m.choosing == nil && !m.pending && m.cursor < len(m.ranked) && canReply(m.ranked[m.cursor]) {
			return m.startReply()
		}
		return m, nil
	case "ctrl+x":
		if m.canClose() {
			return m.startClose()
		}
		return m, nil
	case "backspace":
		// An empty query is nothing left to delete, so the key reads as
		// undoing the step that opened the targets. With something typed it
		// belongs to the field, which is what the rest of this falls to.
		if m.choosing != nil && m.query.Value() == "" {
			m.abandon()
			return m, nil
		}
		if m.narrowed() && m.query.Value() == "" {
			m.leave()
			return m, nil
		}
	}

	return m.editQuery(msg)
}

func (m model) editQuery(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	before := m.query.Value()
	m.query, cmd = m.query.Update(msg)
	if m.query.Value() != before {
		m.failure, m.notice = "", ""
		m.rank()
	}
	return m, cmd
}

// choose runs the selected entry, or asks herdr what it can act on when the
// entry picks its target from a list. A row of such a list carries the target
// and no list of its own, so choosing one runs the entry.
func (m model) choose() (tea.Model, tea.Cmd) {
	// A keystroke while the last one is still out on the socket would run the
	// command twice, which for anything that creates something leaves two of
	// it. The answer is on its way, and the footer says so.
	if m.pending || len(m.ranked) == 0 {
		return m, nil
	}
	entry := m.ranked[m.cursor].Entry
	if entry.Scope.Name != "" {
		m.enter(entry.Scope)
		return m, nil
	}
	// The list an entry picks its target from is not itself the act, so the
	// question waits until there is something to ask about.
	if entry.Confirm && entry.Choices == nil {
		m.confirming, m.closing = &entry, false
		m.failure, m.notice = "", ""
		return m, nil
	}
	m.pending = true
	if entry.Choices != nil {
		return m, m.list(entry)
	}
	return m, m.run(entry)
}

// list asks for the entry's targets. It runs off the update loop, the way a
// command does, so a slow socket does not hold up a keystroke.
func (m model) list(entry palette.Entry) tea.Cmd {
	epoch, query := m.epoch, m.query.Value()
	return func() tea.Msg {
		choices, err := entry.Choices.List(m.ctx, palette.Exec{Client: m.env.Client(), Ctx: m.invocation, Env: m.env})
		return choicesMsg{epoch: epoch, entry: entry, query: query, choices: choices, err: err}
	}
}

// choices shows what the entry can act on, or says why there is nothing to
// show. Either way the popup stays open: the keystroke that asked for the list
// is answered where it was made.
func (m *model) choices(msg choicesMsg) {
	m.pending = false
	switch {
	case msg.err != nil:
		m.failure = msg.err.Error()
	case len(msg.choices) == 0:
		m.notice = msg.entry.Choices.Empty
	default:
		m.choosing = &chooser{entry: msg.entry, query: msg.query}
		m.query.SetValue("")
		m.query.Placeholder = msg.entry.Choices.Label
		m.targets = palette.ChoiceEntries(msg.entry, msg.choices)
		m.failure, m.notice = "", ""
		m.rank()
	}
}

// abandon leaves the targets for the command list, filtered the way it was.
// Whatever is still out on the socket belongs to the list being left, so the
// popup stops waiting for it: an answer arriving afterwards would otherwise
// put the targets back up, or close the popup on the way out of them.
func (m *model) abandon() {
	m.query.SetValue(m.choosing.query)
	m.query.Placeholder = placeholder(m.scope)
	m.choosing, m.targets = nil, nil
	m.failure, m.notice = "", ""
	m.epoch++
	m.pending = false
	m.rank()
}

func (m model) run(entry palette.Entry) tea.Cmd {
	epoch := m.epoch
	return func() tea.Msg {
		err := m.execute(entry)
		if err == nil {
			// A failed write only costs the recent order, so it does not turn
			// a command that ran into a command that reports failure.
			m.state.write(func() { _ = palette.WriteRecent(m.env, entry.ID, m.recent) })
		}
		return ranMsg{epoch: epoch, err: err}
	}
}

// execute runs the entry, or hands it over to run outside the popup. A row
// picked from a list of targets carries it, which is the entry's input.
//
// herdr allows one popup at a time and answers the second with ui_busy, which
// is the signal to hand over: a command that wants a popup cannot run while
// the palette's own is up.
func (m model) execute(entry palette.Entry) error {
	client := m.env.Client()
	if entry.Input != nil {
		return palette.RelayPrompt(m.ctx, client, m.env, entry, m.invocation)
	}

	relay := func() error {
		return palette.Relay(m.ctx, client, m.env, entry, m.invocation)
	}
	if entry.AlwaysRelay {
		return relay()
	}

	err := entry.Run(m.ctx, palette.Exec{Client: client, Ctx: m.invocation, Chosen: entry.Chosen, Env: m.env})
	if herdr.IsCode(err, herdr.ErrCodeUIBusy) {
		return relay()
	}
	return err
}

// reload rebuilds the rows that go to what is open. It runs off the update
// loop, so a slow socket does not hold up a keystroke.
func (m model) reload() tea.Cmd {
	return func() tea.Msg {
		session, err := palette.OpenSession(m.ctx, m.env.Client())
		if err != nil {
			return nil
		}
		return openMsg{session: session}
	}
}

// setOpen takes the rebuilt rows without moving the selection off the entry it
// is on, which would otherwise jump under the user as an agent changes state.
// While a list of targets is up the rows are not the session's, so the rebuilt
// ones are kept for the way back instead of being shown.
func (m *model) setOpen(session palette.Session) {
	m.source.Session = session
	if m.choosing != nil {
		return
	}

	var selected string
	if m.cursor < len(m.ranked) {
		selected = m.ranked[m.cursor].Entry.ID
	}
	offset, at := m.offset, m.cursor

	m.rank()

	// A selected row that has gone, such as one just closed, leaves the
	// selection where it was, on the row that took its place: the next one to
	// close is picked from there, not from the top.
	m.cursor = min(at, max(len(m.ranked)-1, 0))
	for i, ranked := range m.ranked {
		if ranked.Entry.ID == selected {
			m.cursor = i
			break
		}
	}
	// The window stays where it was rather than jumping back to the top with
	// the rebuild, unless that would leave the selection out of it.
	m.offset = min(offset, max(len(m.lines)-1, 0))
	m.reveal()

	// The question names the row under the selection. If the rebuild moved the
	// selection off it — the pane it went to has gone — the question is about
	// something no longer on show, so it goes with it.
	if m.confirming != nil && (m.cursor >= len(m.ranked) || m.ranked[m.cursor].Entry.ID != m.confirming.ID) {
		m.confirming = nil
	}
}

// candidates is what the query is ranked against: an entry's targets while
// they are up, and the scope's rows otherwise, read afresh so a rebuilt session
// shows in them.
func (m model) candidates() []palette.Entry {
	if m.choosing != nil {
		return m.targets
	}
	return m.source.Rows(m.scope)
}

// completion is the scope tab would complete the query to. Only the command
// list narrows: inside a scope or a list of targets there is nothing to
// complete to.
func (m model) completion() (palette.Scope, bool) {
	if m.narrowed() || m.choosing != nil || m.pending {
		return palette.Scope{}, false
	}
	return palette.Complete(m.query.Value())
}

func (m *model) rank() {
	entries, text := m.candidates(), m.query.Value()

	m.ranked = palette.Rank(entries, text, m.recent)
	// A list nobody has searched yet is laid out in groups; a query's matches
	// are one list in the order they matched, which the groups would break.
	if strings.TrimSpace(text) == "" && m.choosing == nil {
		m.ranked, m.lines = grouped(m.ranked)
		// A scope holds rows of a kind, where headings that tell commands from
		// places say nothing; the order they give is kept.
		if m.narrowed() {
			m.lines = ungrouped(len(m.ranked))
		}
	} else {
		m.lines = ungrouped(len(m.ranked))
	}
	m.cursor, m.offset = 0, 0
	m.widths = measure(m.ranked, m.prefix)
	// A window one line high would otherwise show the heading over the first
	// row rather than the row selected.
	m.reveal()
}

// step moves the selection one row and goes round at the ends, so the far end
// of a long list is one keystroke away.
func (m *model) step(by int) {
	if len(m.ranked) == 0 {
		m.move(by)
		return
	}
	m.settle()
	m.cursor = (m.cursor + by + len(m.ranked)) % len(m.ranked)
	m.reveal()
}

// move takes the selection by a page or a turn of the wheel, which stop at the
// ends: going round would carry the reader past what they were looking at.
func (m *model) move(by int) {
	m.settle()
	if len(m.ranked) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	m.cursor = clamp(m.cursor+by, len(m.ranked))
	m.reveal()
}

// settle clears what the footer is holding over the list. Moving the selection
// is the reader having taken in a failure, and it puts away a question the
// next keystroke is no longer answering; the footer is also where the keys and
// what is on show live, so neither is held there for the rest of the popup.
func (m *model) settle() {
	m.failure, m.notice = "", ""
	m.confirming = nil
}

func clamp(index, length int) int {
	if length == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

// reveal scrolls the window to the selected row. Moving up onto the first row
// of a group brings its heading into the window with it, so a row is never on
// show without the group it belongs to having been said.
func (m *model) reveal() {
	at := m.lineOf(m.cursor)
	top := at
	if top > 0 && m.lines[top-1].isHeading() && m.rows() > 1 {
		top--
	}
	m.offset = scroll(m.offset, top, at, m.rows())
}

// lineOf is the line the row is drawn on.
func (m model) lineOf(row int) int {
	for i, l := range m.lines {
		if l.row == row {
			return i
		}
	}
	return 0
}

// scroll keeps the lines from top to bottom inside the visible window.
func scroll(offset, top, bottom, rows int) int {
	if rows <= 0 {
		return 0
	}
	if top < offset {
		return top
	}
	if bottom >= offset+rows {
		return bottom - rows + 1
	}
	return offset
}
