package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

const (
	// modeReply names the screen that hands the keyboard to a blocked agent.
	modeReply = "REPLY"
	// replyEcho is how soon after a key reaches the agent its screen is read
	// again, and replyRefresh how often while the screen is being answered:
	// an agent redraws its question the moment a key lands, and a second is a
	// long time to wait to see what the key did.
	replyEcho    = 120 * time.Millisecond
	replyRefresh = 300 * time.Millisecond
)

// replySentMsg carries the outcome of keys handed to the agent.
type replySentMsg struct{ err error }

// canReply reports whether the row is an agent waiting on a question, which
// is the one thing the palette hands the keyboard over for: a working agent
// is reached with a prompt, and one that is done has nothing to answer. A tab
// or a workspace carries the status of the agents in it and a pane to preview
// it through, which need not be the agent that is waiting.
func canReply(ranked palette.Ranked) bool {
	return ranked.Entry.Kind == palette.KindAgent && ranked.Entry.Pane != "" &&
		herdr.AgentStatus(ranked.Status) == herdr.AgentStatusBlocked
}

// startReply hands the keyboard to the selected agent, whose screen takes the
// place of the list. The preview is asked for straight away and at the reply
// rate from then on.
func (m model) startReply() (tea.Model, tea.Cmd) {
	ranked := m.ranked[m.cursor]
	m.replying = &ranked
	m.failure, m.notice = "", ""
	m.confirming = nil
	return m, m.restartPreview(0)
}

// stopReply gives the keyboard back to the list, saying why when it was not
// the reader who left.
func (m *model) stopReply(notice string) tea.Cmd {
	m.replying = nil
	m.queued = nil
	m.notice = notice
	return m.restartPreview(previewSettle)
}

// restartPreview begins a new read of the preview after wait, ending the one
// running, whose refresh would otherwise keep its own pace alongside.
func (m *model) restartPreview(wait time.Duration) tea.Cmd {
	m.previewSeq++
	if m.previewTarget() == "" {
		return nil
	}
	return m.previewAfter(wait)
}

// keyReply takes a keystroke while the keyboard is the agent's. Esc is the
// palette's own, so a key meant to leave never tells the agent no; the rest
// that herdr can deliver go to the agent as they were pressed.
func (m model) keyReply(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		return m, m.stopReply("")
	}
	return m.queueReply(replyKeys(msg))
}

// queueReply keeps typed keys and pasted text on one ordered send path.
func (m model) queueReply(keys []string) (tea.Model, tea.Cmd) {
	if len(keys) == 0 {
		return m, nil
	}
	m.failure = ""
	m.queued = append(m.queued, keys...)
	return m, m.flushReply()
}

// flushReply sends what is queued for the agent, unless keys are already on
// their way: those are answered first, and what was queued behind them goes
// out then, in the order it was pressed.
func (m *model) flushReply() tea.Cmd {
	if m.sending || len(m.queued) == 0 || m.replying == nil {
		return nil
	}
	keys, pane := m.queued, m.replying.Entry.Pane
	m.queued, m.sending = nil, true
	// Sent to the pane rather than through agent.send_keys, which takes only
	// an agent herdr started under a name: a detected one is refused. Whether
	// the pane is waiting on an answer is the palette's own check.
	return func() tea.Msg {
		_, err := m.env.Client().PaneSendKeys(m.ctx, herdr.PaneSendKeysParams{PaneID: pane, Keys: keys})
		return replySentMsg{err: err}
	}
}

// replyNamed are the keys herdr takes by the name the popup's terminal reports
// them under.
var replyNamed = map[string]bool{
	"enter": true, "tab": true, "shift+tab": true, "backspace": true,
	"up": true, "down": true, "left": true, "right": true,
}

// replyKeys is a keystroke as herdr's pane.send_keys spells it: every character on
// its own, the space by name, and the named keys an answer is made with.
// ctrl with a letter is passed on too, which is how an agent is interrupted;
// ctrl+c stays the palette's, which closes it. An alt chord is dropped rather
// than sent as its bare key, which on a numbered dialog would pick an option.
func replyKeys(msg tea.KeyPressMsg) []string {
	if msg.Mod&tea.ModAlt != 0 {
		return nil
	}
	if msg.Text != "" {
		return replyTextKeys(msg.Text)
	}
	if msg.Code == tea.KeySpace && msg.Mod == 0 {
		return []string{"space"}
	}
	name := msg.String()
	if replyNamed[name] {
		return []string{name}
	}
	if letter, ok := strings.CutPrefix(name, "ctrl+"); ok && len(letter) == 1 && letter[0] >= 'a' && letter[0] <= 'z' {
		return []string{name}
	}
	return nil
}

// A paste can contain line breaks and tabs. Herdr refuses a whole call over
// one key it cannot spell, so unsupported control characters are left out.
func replyTextKeys(text string) []string {
	keys := make([]string, 0, len([]rune(text)))
	for _, r := range text {
		switch {
		case r == ' ':
			keys = append(keys, "space")
		case r == '\n' || r == '\r':
			keys = append(keys, "enter")
		case r == '\t':
			keys = append(keys, "tab")
		case r < 0x20 || r == 0x7f:
		default:
			keys = append(keys, string(r))
		}
	}
	return keys
}

// followReply keeps the agent being answered in step with the session. An
// agent that is no longer waiting has been answered, so the keyboard goes
// back to the list; one that has gone is said to have gone.
func (m *model) followReply() tea.Cmd {
	for _, entry := range m.source.Session.Entries {
		if entry.ID != m.replying.Entry.ID {
			continue
		}
		if herdr.AgentStatus(entry.Status) == herdr.AgentStatusBlocked {
			m.replying.Entry = entry
			m.replying.Detail, m.replying.Status = entry.Detail, entry.Status
			return nil
		}
		return m.stopReply(strings.TrimPrefix(entry.Title, palette.GoTo) + " is no longer waiting")
	}
	return m.stopReply(strings.TrimPrefix(m.replying.Entry.Title, palette.GoTo) + " is gone")
}

// replyLines is the agent's screen over the whole of the list's room.
func (m model) replyLines(height int) []string {
	width := max(m.cols()-2, 1)
	content := m.screenPanel(*m.replying, width, height)
	lines := make([]string, height)
	for i := range lines {
		if i < len(content) {
			lines[i] = " " + truncate(content[i], width)
		}
	}
	return lines
}
