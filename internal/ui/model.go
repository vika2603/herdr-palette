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

type model struct {
	ctx        context.Context
	env        *plugin.Env
	invocation *herdr.PluginInvocationContext

	entries []palette.Entry
	recent  []string

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
	entries []palette.Entry,
	recent []string,
	colours theme.Theme,
) model {
	query := textinput.New()
	query.Prompt = "› "
	query.Placeholder = "Search commands"
	query.Focus()

	m := model{
		ctx:        ctx,
		env:        env,
		invocation: invocation,
		entries:    entries,
		recent:     recent,
		query:      query,
		styles:     newStyles(colours),
	}
	m.rank()
	return m
}

func (m model) Init() tea.Cmd { return textinput.Blink }

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

// choose runs the selected entry.
func (m model) choose() (tea.Model, tea.Cmd) {
	if len(m.ranked) == 0 {
		return m, nil
	}
	return m, m.run(m.ranked[m.cursor].Entry)
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

// execute runs the entry, or hands it over to run outside the popup.
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

	err := entry.Run(m.ctx, palette.Exec{Client: client, Ctx: m.invocation})
	if herdr.IsCode(err, herdr.ErrCodeUIBusy) {
		return relay()
	}
	return err
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
