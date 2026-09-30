package ui

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func replyServer(t *testing.T) (*plugintest.Server, model) {
	t.Helper()
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPaneSendKeys, herdr.OKResponse{}).
		Reply(herdr.MethodPaneRead, herdr.PaneReadResponse{Read: herdr.PaneReadResult{Text: "Allow this edit?\n› 1. Yes\n  2. No"}})
	m := sessionModel(t, server.Env(plugintest.StateDir(t.TempDir())), nil, 72, 14)
	return server, m
}

// sent is every list of keys the agent was handed, in order.
func sent(t *testing.T, server *plugintest.Server) [][]string {
	t.Helper()
	var out [][]string
	for _, call := range server.Calls() {
		if call.Method != herdr.MethodPaneSendKeys {
			continue
		}
		var params herdr.PaneSendKeysParams
		if err := json.Unmarshal(call.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.PaneID != "p-blocked" {
			t.Errorf("keys went to %q, want the agent being answered", params.PaneID)
		}
		out = append(out, params.Keys)
	}
	return out
}

func press(t *testing.T, m model, msg tea.KeyPressMsg) (model, tea.Cmd) {
	t.Helper()
	return send(t, m, msg)
}

func runes(text string) tea.KeyPressMsg { return tea.KeyPressMsg{Text: text} }

func TestTabHandsTheKeyboardToABlockedAgent(t *testing.T) {
	withSpelling(t, "darwin")
	_, m := replyServer(t)
	if !canReply(m.ranked[m.cursor]) {
		t.Fatalf("the first row is %q, want the blocked agent", m.ranked[m.cursor].Entry.ID)
	}
	if !strings.Contains(m.footer(), "⇥ reply") {
		t.Errorf("footer = %q, want the key that answers the agent", m.footer())
	}

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.replying == nil {
		t.Fatal("tab on a blocked agent did not hand it the keyboard")
	}
	if got := m.header(); !strings.Contains(got, modeReply) || !strings.Contains(got, "frontend") {
		t.Errorf("header = %q, want the label and the agent's name", got)
	}
	if m.previewTarget() != "p-blocked" {
		t.Errorf("preview target = %q on a popup too narrow for a preview, want the agent's screen", m.previewTarget())
	}
	view := m.View().Content
	if strings.Contains(view, "NEEDS YOU") {
		t.Errorf("the list is still on show while answering:\n%s", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 72 {
			t.Errorf("line %d is %d wide", i, w)
		}
	}
}

func TestOnlyABlockedAgentCanBeAnswered(t *testing.T) {
	_, m := replyServer(t)
	m.step(1) // the done agent
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.replying != nil {
		t.Errorf("tab on %q handed it the keyboard", m.ranked[m.cursor].Entry.ID)
	}
}

func TestKeysGoToTheAgentInTheOrderPressed(t *testing.T) {
	server, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})

	m, first := press(t, m, runes("1"))
	if first == nil {
		t.Fatal("a key was not sent")
	}
	// Pressed while the first is on its way: it waits rather than racing it.
	m, second := press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if second != nil {
		t.Fatal("a second key was sent while the first was still on its way")
	}
	m, after := send(t, m, first())
	// What comes back sends the queued key and asks for the screen again; the
	// send is the first of the two.
	batch, ok := after().(tea.BatchMsg)
	if !ok || len(batch) == 0 || batch[0] == nil {
		t.Fatal("the key queued behind the first was never sent")
	}
	m, _ = send(t, m, batch[0]())

	if got, want := sent(t, server), [][]string{{"1"}, {"enter"}}; !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("sent %v, want %v", got, want)
	}
	if m.sending || len(m.queued) != 0 {
		t.Errorf("sending = %v with %v queued after every key went out", m.sending, m.queued)
	}
}

func TestEscLeavesWithoutTellingTheAgent(t *testing.T) {
	server, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m, cmd := press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.replying != nil {
		t.Fatal("esc did not give the keyboard back")
	}
	if cmd != nil {
		if _, isSend := cmd().(replySentMsg); isSend {
			t.Error("esc was sent to the agent")
		}
	}
	if len(sent(t, server)) != 0 {
		t.Errorf("sent %v, want nothing", sent(t, server))
	}
}

func TestCtrlCStillClosesThePalette(t *testing.T) {
	_, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if _, cmd := press(t, m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil || cmd() != tea.Quit() {
		t.Error("ctrl+c while answering did not close the palette")
	}
}

// An agent that has been answered goes back to work, and there is nothing
// left to answer, so the list comes back.
func TestAnAnsweredAgentGivesTheKeyboardBack(t *testing.T) {
	_, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})

	session := sessionList().Session
	open := session.Entries
	for i := range open {
		if open[i].ID == "pane:blocked" {
			open[i].Status, open[i].Detail = "working", "working · codex"
		}
	}
	m, _ = send(t, m, openMsg{session: session})
	if m.replying != nil {
		t.Fatal("the keyboard stayed with an agent that is no longer waiting")
	}
	if !strings.Contains(m.notice, "frontend is no longer waiting") {
		t.Errorf("notice = %q, want to be told why the list is back", m.notice)
	}
}

func TestAKeyIsSpelledTheWayHerdrSendsIt(t *testing.T) {
	for _, tc := range []struct {
		msg  tea.KeyPressMsg
		want []string
	}{
		{runes("1"), []string{"1"}},
		{runes("y n"), []string{"y", "space", "n"}},
		{tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, []string{"space"}},
		{tea.KeyPressMsg{Code: tea.KeyEnter}, []string{"enter"}},
		{tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}, []string{"shift+tab"}},
		{tea.KeyPressMsg{Code: tea.KeyDown}, []string{"down"}},
		{tea.KeyPressMsg{Code: tea.KeyBackspace}, []string{"backspace"}},
		{tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}, []string{"ctrl+d"}},
		{tea.KeyPressMsg{Code: tea.KeyF5}, nil},
	} {
		if got := replyKeys(tc.msg); !slices.Equal(got, tc.want) {
			t.Errorf("replyKeys(%q) = %v, want %v", tc.msg.String(), got, tc.want)
		}
	}
}

func TestAnAltChordAndAPasteAreSentAsHerdrCanTakeThem(t *testing.T) {
	if got := replyKeys(tea.KeyPressMsg{Code: '1', Mod: tea.ModAlt}); got != nil {
		t.Errorf("alt+1 sent %v, want nothing: a bare 1 picks an option", got)
	}
	paste := tea.PasteMsg{Content: "yes\nno\tx\x01"}
	if got, want := replyTextKeys(paste.Content), []string{"y", "e", "s", "enter", "n", "o", "tab", "x"}; !slices.Equal(got, want) {
		t.Errorf("paste sent %v, want %v", got, want)
	}
}

func TestTabDoesNotAnswerWhileACommandIsOut(t *testing.T) {
	_, m := replyServer(t)
	m.pending = true
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.replying != nil {
		t.Error("tab handed the keyboard over while a command was still out")
	}
}

// Keys that keep coming must not hold the screen's refresh off: the read after
// a key runs beside the refresh rather than restarting it.
func TestAKeyDoesNotRestartTheRefresh(t *testing.T) {
	_, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	seq := m.previewSeq

	m, _ = send(t, m, replySentMsg{})
	if m.previewSeq != seq {
		t.Fatal("a key that went out restarted the refresh")
	}
	m, read := send(t, m, previewEchoMsg{seq: seq})
	if read == nil {
		t.Fatal("the screen was not read after the key")
	}
	msg, ok := read().(previewMsg)
	if !ok || !msg.echo {
		t.Fatalf("read answered %#v, want a read made for the key", msg)
	}
	if _, again := send(t, m, msg); again != nil {
		t.Error("the read made for a key started a refresh of its own")
	}
}

func TestASlowReadDoesNotOverwriteALaterOne(t *testing.T) {
	_, m := replyServer(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	seq := m.previewSeq
	m, _ = send(t, m, previewMsg{seq: seq, read: 2, pane: "p-blocked", text: "new", echo: true})
	m, _ = send(t, m, previewMsg{seq: seq, read: 1, pane: "p-blocked", text: "old", echo: true})
	if m.preview.text != "new" {
		t.Errorf("preview = %q, want the later read kept", m.preview.text)
	}
}
