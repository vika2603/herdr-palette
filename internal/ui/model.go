package ui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/palette"
	"github.com/vika2603/herdr-palette/internal/theme"
)

// ranMsg carries the outcome of the command the user chose.
type ranMsg struct{ err error }

// changedMsg says herdr reported a change to what is open, and openMsg carries
// the rebuilt rows. A failed rebuild keeps the rows the popup already has: a
// stale row goes to a pane that is gone, which herdr answers with an error the
// popup shows.
type (
	changedMsg struct{}
	openMsg    struct{ open []palette.Entry }
)

// choicesMsg carries what an entry can act on, once herdr has answered with
// the list. The entry travels with it: the selection may have moved on while
// the call was out.
type choicesMsg struct {
	entry   palette.Entry
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
	// command list.
	choosing *chooser

	query  textinput.Model
	ranked []palette.Ranked
	cursor int
	offset int
	// keyWidth is the width of the key column, zero when nothing on show is
	// bound to a key.
	keyWidth int

	styles        styles
	failure       string
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
	query := textinput.New()
	query.Prompt = "› "
	query.Placeholder = searchPlaceholder
	query.Focus()

	m := model{
		ctx:        ctx,
		env:        env,
		invocation: invocation,
		commands:   list.Commands,
		open:       list.Open,
		recent:     recent,
		query:      query,
		styles:     newStyles(colours),
	}
	m.changes = watch(ctx, env)
	m.collect()
	m.rank()
	return m
}

func (m model) Init() tea.Cmd { return tea.Batch(textinput.Blink, listen(m.changes)) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.offset = scroll(m.offset, m.cursor, m.rows())
		return m, nil

	case ranMsg:
		if msg.err == nil {
			return m, tea.Quit
		}
		// A command that failed leaves the popup open with the reason, so the
		// keystroke is not lost silently.
		m.failure = msg.err.Error()
		return m, nil

	case changedMsg:
		return m, tea.Batch(m.reload(), listen(m.changes))

	case openMsg:
		m.setOpen(msg.open)
		return m, nil

	case choicesMsg:
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
		index := m.offset + msg.Y - headerRows
		if msg.Y < headerRows || index >= len(m.ranked) {
			return m, nil
		}
		m.cursor = index
		return m.choose()
	}
	return m, nil
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	return m.keyList(msg)
}

func (m model) keyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.move(1)
		return m, nil
	case "up", "ctrl+p":
		m.move(-1)
		return m, nil
	case "pgdown":
		m.move(m.rows())
		return m, nil
	case "pgup":
		m.move(-m.rows())
		return m, nil
	case "enter":
		return m.choose()
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
	if len(m.ranked) == 0 {
		return m, nil
	}
	entry := m.ranked[m.cursor].Entry
	if entry.Choices != nil {
		return m, m.list(entry)
	}
	return m, m.run(entry)
}

// list asks for the entry's targets. It runs off the update loop, the way a
// command does, so a slow socket does not hold up a keystroke.
func (m model) list(entry palette.Entry) tea.Cmd {
	return func() tea.Msg {
		choices, err := entry.Choices.List(m.ctx, palette.Exec{Client: m.env.Client(), Ctx: m.invocation})
		return choicesMsg{entry: entry, choices: choices, err: err}
	}
}

// choices shows what the entry can act on, or says why there is nothing to
// show. Either way the popup stays open: the keystroke that asked for the list
// is answered where it was made.
func (m *model) choices(msg choicesMsg) {
	switch {
	case msg.err != nil:
		m.failure = msg.err.Error()
	case len(msg.choices) == 0:
		m.failure = msg.entry.Choices.Empty
	default:
		m.choosing = &chooser{entry: msg.entry, query: m.query.Value()}
		m.entries = palette.ChoiceEntries(msg.entry, msg.choices)
		m.query.SetValue("")
		m.query.Placeholder = msg.entry.Choices.Label
		m.failure = ""
		m.rank()
	}
}

// abandon leaves the targets for the command list, filtered the way it was.
func (m *model) abandon() {
	m.query.SetValue(m.choosing.query)
	m.query.Placeholder = searchPlaceholder
	m.choosing = nil
	m.failure = ""
	m.collect()
	m.rank()
}

func (m model) run(entry palette.Entry) tea.Cmd {
	return func() tea.Msg {
		err := m.execute(entry)
		if err == nil {
			// A failed write only costs the recent order, so it does not turn
			// a command that ran into a command that reports failure.
			_ = palette.WriteRecent(m.env, entry.ID, m.recent)
		}
		return ranMsg{err: err}
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

	err := entry.Run(m.ctx, palette.Exec{Client: client, Ctx: m.invocation, Input: entry.Chosen})
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
}

func (m *model) collect() {
	m.entries = append(append(make([]palette.Entry, 0, len(m.commands)+len(m.open)), m.commands...), m.open...)
}

func (m *model) rank() {
	m.ranked = palette.Rank(m.entries, m.query.Value(), m.recent)
	m.cursor, m.offset = 0, 0

	m.keyWidth = 0
	for _, ranked := range m.ranked {
		m.keyWidth = max(m.keyWidth, len([]rune(ranked.Entry.Key)))
	}
}

func (m *model) move(by int) {
	if len(m.ranked) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	m.cursor = clamp(m.cursor+by, len(m.ranked))
	m.offset = scroll(m.offset, m.cursor, m.rows())
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
