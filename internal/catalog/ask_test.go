package catalog

import (
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

// selectionContext is the palette opened over a pane with text selected, which
// herdr carries in the invocation context rather than leaving to be read back.
func selectionContext(selected string) *herdr.PluginInvocationContext {
	ctx := fullContext()
	ctx.SelectedText = &selected
	return ctx
}

func promptSent(t *testing.T, s *plugintest.Server) herdr.AgentPromptParams {
	t.Helper()
	var sent herdr.AgentPromptParams
	decode(t, paramsOf(t, s, herdr.MethodAgentPrompt), &sent)
	return sent
}

// The selection reaches an agent with the question in front of it: the
// question is what the agent is being asked to do with what follows, and what
// follows can run to hundreds of lines.
func TestAskingAnAgentAboutTheSelection(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	err := execute(t, env, "herdr:agent.ask.selection",
		step{chosen: "w1:p3", input: "why does this fail?"},
		selectionContext("panic: nil map\n"))
	if err != nil {
		t.Fatalf("asking about the selection: %v", err)
	}

	sent := promptSent(t, s)
	if sent.Target != "w1:p3" {
		t.Errorf("asked %q, want the agent that was picked", sent.Target)
	}
	if want := "why does this fail?\n\npanic: nil map"; sent.Text != want {
		t.Errorf("text = %q, want %q", sent.Text, want)
	}
}

// The entry is offered only with a selection, but the context it runs with is
// the one handed over, so it says what is missing rather than sending the
// question with nothing attached.
func TestAskingAboutNoSelectionSaysSo(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	err := execute(t, env, "herdr:agent.ask.selection",
		step{chosen: "w1:p3", input: "what is this?"}, fullContext())
	if err == nil {
		t.Fatal("the question went out with no selection behind it")
	}
	if saw(s, herdr.MethodAgentPrompt) {
		t.Error("an agent was prompted although there was nothing to ask about")
	}
}

// A pane's output reaches an agent without being copied there by hand. It is
// read unwrapped and without escape sequences: a line that ran past the pane's
// width is one line, and the colours are not what the agent is being asked
// about.
func TestAskingAnAgentAboutThePanesOutput(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	err := execute(t, env, "herdr:agent.ask.output",
		step{chosen: "w1:p3", input: "what failed?"}, fullContext())
	if err != nil {
		t.Fatalf("asking about the output: %v", err)
	}

	var read herdr.PaneReadParams
	decode(t, paramsOf(t, s, herdr.MethodPaneRead), &read)
	if read.PaneID != "p1" {
		t.Errorf("read pane %q, want the focused one", read.PaneID)
	}
	if read.Source != herdr.ReadSourceRecentUnwrapped {
		t.Errorf("read %q, want the output unwrapped", read.Source)
	}
	if read.StripANSI == nil || !*read.StripANSI {
		t.Error("the output was sent with its escape sequences")
	}
	if read.Lines == nil || *read.Lines != askLines {
		t.Errorf("lines = %v, want the tail the agent is given", read.Lines)
	}

	sent := promptSent(t, s)
	if !strings.HasPrefix(sent.Text, "what failed?\n\n") {
		t.Errorf("text = %q, want the question in front of the output", sent.Text)
	}
	if !strings.Contains(sent.Text, "panic: nil map") {
		t.Errorf("text = %q, want what the pane printed", sent.Text)
	}
	if strings.HasSuffix(sent.Text, "\n") {
		t.Errorf("text = %q, want the trailing blank lines off", sent.Text)
	}
}

// Watching an agent waits for it to stop and says so, which is the whole point
// rather than a report on the way out: it runs outside the popup, where there
// is nothing left to show a result in.
func TestWatchingAnAgentNotifiesWhenItStops(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:agent.watch", step{chosen: "w1:p3"}, fullContext()); err != nil {
		t.Fatalf("watching the agent: %v", err)
	}

	var waited herdr.AgentWaitParams
	decode(t, paramsOf(t, s, herdr.MethodAgentWait), &waited)
	if waited.Target != "w1:p3" {
		t.Errorf("waited on %q, want the agent that was picked", waited.Target)
	}
	if waited.TimeoutMs == nil {
		t.Error("the wait has no end, so a process could be left behind by an agent left running")
	}
	for _, status := range []herdr.AgentStatus{herdr.AgentStatusDone, herdr.AgentStatusBlocked, herdr.AgentStatusIdle} {
		if !waitsFor(waited.Until, status) {
			t.Errorf("%q is not waited for, and an agent in it has stopped", status)
		}
	}
	if waitsFor(waited.Until, herdr.AgentStatusWorking) {
		t.Error("working is waited for, which is the state being waited out")
	}

	var shown herdr.NotificationShowParams
	decode(t, paramsOf(t, s, herdr.MethodNotificationShow), &shown)
	// The agent was renamed, so that is what it is known by.
	if !strings.Contains(shown.Title, "reviewer") || !strings.Contains(shown.Title, "blocked") {
		t.Errorf("title = %q, want the agent and the state it stopped in", shown.Title)
	}
}

func waitsFor(until []herdr.AgentStatus, status herdr.AgentStatus) bool {
	for _, s := range until {
		if s == status {
			return true
		}
	}
	return false
}

// The watch is handed to the entrypoint outside the popup rather than tried in
// it: the wait lasts as long as the agent does.
func TestWatchingAnAgentRunsOutsideThePopup(t *testing.T) {
	if !entry(t, "herdr:agent.watch").AlwaysRelay {
		t.Error("the watch runs inside the popup, which it would hold open for as long as the agent runs")
	}
}

func saw(s *plugintest.Server, method string) bool {
	for _, call := range s.Calls() {
		if call.Method == method {
			return true
		}
	}
	return false
}

// A wait that runs out is news rather than a failure: the agent is still
// working, and saying so is what the watch was for. Reporting it as an error
// would put "timed out waiting for agent status" in front of someone whose
// agent is fine.
func TestAWatchThatRunsOutSaysTheAgentIsStillWorking(t *testing.T) {
	s := plugintest.NewServer(t).
		Reply(herdr.MethodAgentGet, herdr.AgentInfoResponse{Agent: herdr.AgentInfo{
			PaneID: "w1:p3", Agent: new("claude"), Name: new("reviewer"),
			AgentStatus: herdr.AgentStatusWorking,
		}}).
		Fail(herdr.MethodAgentWait, herdr.ErrCodeTimeout, "timed out waiting for agent status").
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:agent.watch", step{chosen: "w1:p3"}, fullContext()); err != nil {
		t.Fatalf("a wait that ran out was reported as a failure: %v", err)
	}

	var shown herdr.NotificationShowParams
	decode(t, paramsOf(t, s, herdr.MethodNotificationShow), &shown)
	if !strings.Contains(shown.Title, "reviewer") {
		t.Errorf("title = %q, want the agent it was watching", shown.Title)
	}
	if strings.Contains(strings.ToLower(shown.Title), "timed out") {
		t.Errorf("title = %q, want what it means rather than what herdr called it", shown.Title)
	}
}

// A target that is not there is a failure, which the wait running out is not.
func TestWatchingAnAgentThatIsNotThere(t *testing.T) {
	s := plugintest.NewServer(t).
		Fail(herdr.MethodAgentGet, herdr.ErrCodeAgentNotFound, "no such agent")
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:agent.watch", step{chosen: "gone"}, fullContext()); err == nil {
		t.Error("watching an agent that is not there was not reported")
	}
	if saw(s, herdr.MethodNotificationShow) {
		t.Error("a notification was shown for an agent that was never found")
	}
}
