package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	openMsg    struct{ open []palette.Entry }
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

	// commands is fixed while the popup is up, open is what the session holds,
	// and entries is the two of them as the list is ranked.
	commands []palette.Entry
	open     []palette.Entry
	entries  []palette.Entry
	// changes carries a signal per batch of herdr events, at most one waiting.
	changes <-chan struct{}
	recent  []string
	// choosing is set while the rows are an entry's targets rather than the
	// command list, and confirming while a row that cannot be undone is
	// waiting for the second keystroke that runs it.
	choosing   *chooser
	confirming *palette.Entry

	query  textinput.Model
	ranked []palette.Ranked
	cursor int
	offset int
	// keyWidth is the width of the key column, zero when nothing on show is
	// bound to a key.
	keyWidth int

	// epoch counts the screens the popup has walked out of. A request to the
	// socket carries the one it was made in, and an answer from an earlier
	// screen is dropped: acting on it would pull the popup back to a step the
	// user has already left.
	epoch int

	styles  styles
	failure string
	// pending is set while what a keystroke asked for is out on the socket,
	// so a slow answer does not read as a keystroke that went nowhere.
	pending       bool
	width, height int
}

func newModel(
	ctx context.Context,
	env *plugin.Env,
	invocation *herdr.PluginInvocationContext,
	list palette.List,
	recent []string,
	colours theme.Theme,
) model {
	styles := newStyles(colours)

	query := textinput.New()
	query.Prompt = queryPrompt
	query.Placeholder = searchPlaceholder
	query.Width = queryWidth(defaultCols)
	query.PromptStyle = styles.prompt
	query.Focus()

	m := model{
		ctx:        ctx,
		env:        env,
		invocation: invocation,
		commands:   list.Commands,
		open:       list.Open,
		recent:     recent,
		query:      query,
		styles:     styles,
	}
	m.changes = watch(ctx, env)
	m.collect()
	m.rank()
	return m
}

// setSize takes the popup's size. The query line is sized with it: what the
// field shows of a long query is the columns left over, and a field that keeps
// drawing all of it wraps the line and pushes every row below it down.
func (m *model) setSize(width, height int) {
	m.width, m.height = width, height
	m.query.Width = queryWidth(m.cols())
	m.offset = scroll(m.offset, m.cursor, m.rows())
}

// start fills the query before the first frame, which is how the popup opens
// already narrowed to what the key that opened it asked for.
func (m *model) start(text string) {
	m.query.SetValue(text)
	m.rank()
}

func (m model) Init() tea.Cmd { return tea.Batch(textinput.Blink, listen(m.changes)) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			// A list that stays is a screen the command is used from, so it
			// is asked for again rather than closing over what just changed.
			if m.staying() {
				return m, m.list(m.choosing.entry)
			}
			return m, tea.Quit
		}
		// A command that failed leaves the popup open with the reason, so the
		// keystroke is not lost silently.
		m.pending = false
		m.failure = msg.err.Error()
		return m, nil

	case changedMsg:
		return m, tea.Batch(m.reload(), listen(m.changes))

	case openMsg:
		m.setOpen(msg.open)
		return m, nil

	case choicesMsg:
		if msg.epoch != m.epoch {
			return m, nil
		}
		m.choices(msg)
		return m, nil

	case tea.KeyMsg:
		return m.key(msg)

	case tea.MouseMsg:
		return m.mouse(msg)
	}
	return m, nil
}

// wheelStep is how many rows one wheel notch moves the selection.
const wheelStep = 3

// mouse moves the selection with the wheel and runs the row a click lands on.
// The pointer on its own moves nothing: the selection belongs to the keyboard,
// and a row taken over by where the pointer came to rest is a row the next
// enter would run unread.
func (m model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.move(-wheelStep)
		return m, nil
	case tea.MouseButtonWheelDown:
		m.move(wheelStep)
		return m, nil
	case tea.MouseButtonLeft:
		index, ok := m.rowAt(msg.Y)
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
				return m, m.run(entry)
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
// rows that would have been there had the window been taller.
func (m model) rowAt(y int) (int, bool) {
	if y < headerRows || y >= headerRows+m.rows() {
		return 0, false
	}
	index := m.offset + y - headerRows
	if index >= len(m.ranked) {
		return 0, false
	}
	return index, true
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	return m.keyList(msg)
}

func (m model) keyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// A row waiting to be confirmed takes the next keystroke whatever it is:
	// enter runs it, anything else puts the question away. Nothing reaches
	// the query in between, so the answer cannot be typed into it by mistake.
	if m.confirming != nil {
		entry := *m.confirming
		m.confirming = nil
		if msg.String() == "enter" {
			m.pending = true
			return m, m.run(entry)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		// A list of targets is a step inside the palette, so esc goes back to
		// the commands rather than closing the popup.
		if m.choosing != nil {
			m.abandon()
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
		// A list that stays is a screen of states rather than of targets, and
		// turning one over is what tab reads as. Elsewhere it types nothing
		// and does nothing.
		if m.staying() {
			return m.choose()
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
	}

	var cmd tea.Cmd
	before := m.query.Value()
	m.query, cmd = m.query.Update(msg)
	if m.query.Value() != before {
		m.failure = ""
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
	// The list an entry picks its target from is not itself the act, so the
	// question waits until there is something to ask about.
	if entry.Confirm && entry.Choices == nil {
		m.confirming = &entry
		m.failure = ""
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

// staying reports whether the list on show is one the commands are run from
// rather than picked out of.
func (m model) staying() bool {
	return m.choosing != nil && m.choosing.entry.Choices.Stays
}

// choices shows what the entry can act on, or says why there is nothing to
// show. Either way the popup stays open: the keystroke that asked for the list
// is answered where it was made.
//
// A list asked for again, after a row of it ran, replaces the rows under the
// query and the selection they were picked with, so the screen does not move
// out from under the next keystroke.
func (m *model) choices(msg choicesMsg) {
	m.pending = false
	switch {
	case msg.err != nil:
		m.failure = msg.err.Error()
	case len(msg.choices) == 0:
		if m.choosing != nil {
			m.abandon()
		}
		m.failure = msg.entry.Choices.Empty
	default:
		again := m.choosing != nil && m.choosing.entry.ID == msg.entry.ID
		if !again {
			m.choosing = &chooser{entry: msg.entry, query: msg.query}
			m.query.SetValue("")
			m.query.Placeholder = msg.entry.Choices.Label
		}

		cursor := m.cursor
		m.entries = palette.ChoiceEntries(msg.entry, msg.choices)
		m.failure = ""
		m.rank()
		if again {
			m.cursor = clamp(cursor, len(m.ranked))
			m.offset = scroll(m.offset, m.cursor, m.rows())
		}
	}
}

// abandon leaves the targets for the command list, filtered the way it was.
// Whatever is still out on the socket belongs to the list being left, so the
// popup stops waiting for it: an answer arriving afterwards would otherwise
// put the targets back up, or close the popup on the way out of them.
func (m *model) abandon() {
	m.query.SetValue(m.choosing.query)
	m.query.Placeholder = searchPlaceholder
	m.choosing = nil
	m.failure = ""
	m.epoch++
	m.pending = false
	m.collect()
	m.rank()
}

func (m model) run(entry palette.Entry) tea.Cmd {
	epoch := m.epoch
	return func() tea.Msg {
		err := m.execute(entry)
		if err == nil {
			// A failed write only costs the recent order, so it does not turn
			// a command that ran into a command that reports failure.
			_ = palette.WriteRecent(m.env, entry.ID, m.recent)
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
		open, err := palette.OpenEntries(m.ctx, m.env.Client())
		if err != nil {
			return nil
		}
		return openMsg{open: open}
	}
}

// setOpen takes the rebuilt rows without moving the selection off the entry it
// is on, which would otherwise jump under the user as an agent changes state.
// While a list of targets is up the rows are not the session's, so the rebuilt
// ones are kept for the way back instead of being shown.
func (m *model) setOpen(open []palette.Entry) {
	// While a list of targets is up the rows are not the session's, so the
	// rebuilt ones are kept for the way back instead of being shown.
	if m.choosing != nil {
		m.open = open
		return
	}

	var selected string
	if m.cursor < len(m.ranked) {
		selected = m.ranked[m.cursor].Entry.ID
	}

	m.open = open
	m.collect()
	m.rank()

	for i, ranked := range m.ranked {
		if ranked.Entry.ID == selected {
			m.cursor = i
			break
		}
	}
	m.offset = scroll(m.offset, m.cursor, m.rows())

	// The question names the row under the selection. If the rebuild moved the
	// selection off it — the pane it went to has gone — the question is about
	// something no longer on show, so it goes with it.
	if m.confirming != nil && (m.cursor >= len(m.ranked) || m.ranked[m.cursor].Entry.ID != m.confirming.ID) {
		m.confirming = nil
	}
}

func (m *model) collect() {
	m.entries = append(append(make([]palette.Entry, 0, len(m.commands)+len(m.open)), m.commands...), m.open...)
}

func (m *model) rank() {
	entries, text := m.entries, m.query.Value()
	if rest, only := strings.CutPrefix(text, GoesPrefix); only {
		// The prefix narrows the list to what it goes to rather than opening a
		// screen of its own: what follows it filters those rows the way it
		// filters any others, and deleting it puts the commands back.
		entries, text = goesOnly(entries), rest
	}

	m.ranked = palette.Rank(entries, text, m.recent)
	m.cursor, m.offset = 0, 0

	m.keyWidth = 0
	for _, ranked := range m.ranked {
		m.keyWidth = max(m.keyWidth, len([]rune(ranked.Entry.Key)))
	}
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
	m.offset = scroll(m.offset, m.cursor, m.rows())
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
	m.offset = scroll(m.offset, m.cursor, m.rows())
}

// settle clears what the footer is holding over the list. Moving the selection
// is the reader having taken in a failure, and it puts away a question the
// next keystroke is no longer answering; the footer is also where the keys and
// what is on show live, so neither is held there for the rest of the popup.
func (m *model) settle() {
	m.failure = ""
	m.confirming = nil
}

// queryPrompt stands in front of the query, and queryWidth is how much of a
// long query is on show: the columns left once the prompt and the cursor's own
// have been taken. Without it the line grows past the popup and wraps, which
// pushes every row below it down.
const queryPrompt = "› "

func queryWidth(cols int) int {
	return max(cols-lipgloss.Width(queryPrompt)-1, 1)
}

// GoesPrefix narrows the list to the rows that go somewhere already open. It
// is one character rather than a step of its own, so reaching a pane by name
// stays a single keystroke longer than typing the name.
const GoesPrefix = "@"

// goesOnly keeps the rows that focus something rather than run a command.
func goesOnly(entries []palette.Entry) []palette.Entry {
	out := make([]palette.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Goes {
			out = append(out, entry)
		}
	}
	return out
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

// scroll keeps the cursor inside the visible window.
func scroll(offset, cursor, rows int) int {
	if rows <= 0 {
		return 0
	}
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+rows {
		return cursor - rows + 1
	}
	return offset
}
