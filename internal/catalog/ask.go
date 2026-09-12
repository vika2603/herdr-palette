package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

var errNoSelection = errors.New("no text is selected")

const (
	// askLines is how much of a pane's output an agent is given to look at.
	// Enough for a stack trace or a failing test's output; a whole scrollback
	// would be mostly noise, and the agent reads the pane itself if it needs
	// more.
	askLines = 200
	// waitLimit is how long the entry that watches an agent stays up. The wait
	// runs in a process of its own outside the popup, so it needs an end: an
	// agent left running overnight should not leave one behind.
	waitLimit = time.Hour
)

// stops are the states an agent is in when it is no longer working, which is
// what waiting for one is waiting for.
var stops = []herdr.AgentStatus{
	herdr.AgentStatusDone,
	herdr.AgentStatusBlocked,
	herdr.AgentStatusIdle,
}

// askEntries are the commands that hand an agent something to look at. herdr
// carries the selection and the panes' output, and prompts an agent, but
// nothing joins the two: what is on screen has no way to reach an agent
// without being copied there by hand.
func askEntries() []palette.Entry {
	return []palette.Entry{
		{
			// The selection travels in the invocation context, so the text is
			// the one that was selected when the key opened the palette rather
			// than whatever is selected by the time the command runs.
			ID:             "herdr:agent.ask.selection",
			Title:          "ask an agent about the selection",
			Type:           groupHerdr,
			NeedsSelection: true,
			Choices:        agentChoices("Agent to ask about the selection"),
			Input:          &palette.Input{Label: "What to ask"},
			Run: func(ctx context.Context, e palette.Exec) error {
				selection := herdr.Value(e.Ctx.SelectedText)
				if selection == "" {
					return errNoSelection
				}
				return ask(ctx, e, selection)
			},
		},
		{
			ID:      "herdr:agent.ask.output",
			Title:   "ask an agent about this pane's output",
			Type:    groupHerdr,
			Choices: agentChoices("Agent to ask about the output"),
			Input:   &palette.Input{Label: "What to ask"},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				text, err := recentOutput(ctx, e.Client, id, askLines)
				if err != nil {
					return err
				}
				if text == "" {
					return errors.New("the pane has printed nothing to ask about")
				}
				return ask(ctx, e, text)
			},
		},
		{
			// The wait lasts as long as the agent does, which is far longer
			// than a popup, so this is handed to the entrypoint that runs
			// outside one rather than tried here first.
			ID:          "herdr:agent.watch",
			Title:       "tell me when an agent stops",
			Type:        groupHerdr,
			AlwaysRelay: true,
			Choices:     agentChoices("Agent to watch"),
			Run:         watchAgent,
		},
	}
}

func agentChoices(label string) *palette.Choices {
	return &palette.Choices{
		Label: label,
		Empty: "no pane in the session runs an agent",
		List:  sessionAgents,
	}
}

// ask sends the agent what was collected, with the question in front of it.
// The question comes first because it is what the agent is being asked to do
// with what follows, and the text can run to hundreds of lines.
func ask(ctx context.Context, e palette.Exec, text string) error {
	_, err := e.Client.AgentPrompt(ctx, herdr.AgentPromptParams{
		Target: e.Chosen,
		Text:   e.Input + "\n\n" + strings.TrimRight(text, "\n"),
	})
	return err
}

// recentOutput is the tail of what a pane has printed, unwrapped so a line
// that ran past the pane's width reads as the one line it is, and without the
// escape sequences that colour it.
func recentOutput(ctx context.Context, client *herdr.Client, paneID string, tail uint32) (string, error) {
	lines := tail
	strip := true
	read, err := client.PaneRead(ctx, herdr.PaneReadParams{
		PaneID:    paneID,
		Source:    herdr.ReadSourceRecentUnwrapped,
		Format:    herdr.ReadFormatText,
		Lines:     &lines,
		StripANSI: &strip,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(read.Read.Text, "\n \t"), nil
}

// watchAgent waits for the agent to stop and says so. It runs outside the
// popup, where a failure has nowhere to be shown, so the notification is the
// whole point rather than a report on the way out.
//
// The agent is read first: the wait answers with it only when it stops, and
// running past the limit is the one outcome that still has something to say.
func watchAgent(ctx context.Context, e palette.Exec) error {
	watched, err := e.Client.AgentGet(ctx, herdr.AgentTarget{Target: e.Chosen})
	if err != nil {
		return fmt.Errorf("watching %s: %w", e.Chosen, err)
	}
	name := agentName(watched.Agent)

	timeout := uint64(waitLimit.Milliseconds())
	waited, err := e.Client.AgentWait(ctx, herdr.AgentWaitParams{
		Target:    e.Chosen,
		Until:     stops,
		TimeoutMs: &timeout,
	})

	// herdr answers a wait that ran out with a timeout, which here is news
	// rather than a failure: the agent is still working, and saying so is what
	// the watch was for.
	if herdr.IsCode(err, herdr.ErrCodeTimeout) {
		return notify(ctx, e,
			fmt.Sprintf("%s is still working", name),
			fmt.Sprintf("after %s", waitLimit))
	}
	if err != nil {
		return fmt.Errorf("watching %s: %w", name, err)
	}
	return notify(ctx, e,
		fmt.Sprintf("%s is %s", agentName(waited.Agent), waited.Agent.AgentStatus),
		herdr.Value(waited.Agent.Cwd))
}

func notify(ctx context.Context, e palette.Exec, title, body string) error {
	_, err := e.Client.NotificationShow(ctx, herdr.NotificationShowParams{
		Title: title,
		Body:  &body,
	})
	return err
}

// agentName is what an agent goes by: the name it was given, then what it is.
func agentName(agent herdr.AgentInfo) string {
	for _, value := range []*string{agent.Name, agent.Agent} {
		if name := herdr.Value(value); name != "" {
			return name
		}
	}
	return "the agent"
}
