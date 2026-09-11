package ui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// ranMsg carries the outcome of the command the user chose.
type ranMsg struct{ err error }

type model struct {
	ctx        context.Context
	client     *herdr.Client
	env        *plugin.Env
	invocation *herdr.PluginInvocationContext

	entries []palette.Entry
	recent  []string

	query  textinput.Model
	ranked []palette.Ranked
	cursor int
	offset int

	// pending is the entry waiting for its input. The input screen is open
	// while it is set.
	pending *palette.Entry
	input   textinput.Model

	failure       string
	width, height int
}

func newModel(
	ctx context.Context,
	client *herdr.Client,
	env *plugin.Env,
	invocation *herdr.PluginInvocationContext,
	entries []palette.Entry,
	recent []string,
) model {
	query := textinput.New()
	query.Prompt = "› "
	query.Placeholder = "Search commands"
	query.Focus()

	input := textinput.New()
	input.Prompt = "› "

	m := model{
		ctx:        ctx,
		client:     client,
		env:        env,
		invocation: invocation,
		entries:    entries,
		recent:     recent,
		query:      query,
		input:      input,
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
		m.pending = nil
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
// The input screen takes no mouse input: there is nothing to point at.
func (m model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.pending != nil || msg.Action != tea.MouseActionPress {
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
	if m.pending != nil {
		return m.keyInput(msg)
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

func (m model) keyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.pending = nil
		m.input.Blur()
		m.query.Focus()
		return m, textinput.Blink
	case "enter":
		value := m.input.Value()
		if value == "" {
			return m, nil
		}
		return m, m.run(*m.pending, value)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// choose runs the selected entry, or opens the input screen first when the
// entry needs a value.
func (m model) choose() (tea.Model, tea.Cmd) {
	if len(m.ranked) == 0 {
		return m, nil
	}
	entry := m.ranked[m.cursor].Entry
	if entry.Input == nil {
		return m, m.run(entry, "")
	}

	m.pending = &entry
	m.failure = ""
	m.input.SetValue(entry.Initial(m.invocation))
	m.input.CursorEnd()
	m.query.Blur()
	m.input.Focus()
	return m, textinput.Blink
}

func (m model) run(entry palette.Entry, value string) tea.Cmd {
	return func() tea.Msg {
		err := entry.Run(m.ctx, palette.Exec{
			Client: m.client,
			Ctx:    m.invocation,
			Input:  value,
		})
		if err == nil {
			// A failed write only costs the recent order, so it does not turn
			// a command that ran into a command that reports failure.
			_ = palette.WriteRecent(m.env, entry.ID, m.recent)
		}
		return ranMsg{err: err}
	}
}

func (m *model) rank() {
	m.ranked = palette.Rank(m.entries, m.query.Value(), m.recent)
	m.cursor, m.offset = 0, 0
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
