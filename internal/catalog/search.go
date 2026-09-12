package catalog

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

const (
	// searchLines is how far back each pane is searched. herdr's own copy mode
	// searches the pane you are in; what this command is for is the pane you
	// are not in, so it trades depth for covering all of them.
	searchLines = 200
	// searchRows caps what a session of many busy panes can put in the list.
	// The matcher is quick enough for this, and a query that needs more than
	// the last few hundred lines of every pane is a query for the pane itself.
	searchRows = 2000
)

// searchEntries is the command that looks through what every pane has
// printed. herdr searches a pane's scrollback one pane at a time, from inside
// it; finding which pane something was printed in is what has no answer.
func searchEntries() []palette.Entry {
	return []palette.Entry{{
		ID:    "herdr:pane.search",
		Title: "search what the panes have printed",
		Type:  groupHerdr,
		Choices: &palette.Choices{
			Label: "Line to find",
			Empty: "no pane has printed anything",
			List:  paneLines,
		},
		Run: func(ctx context.Context, e palette.Exec) error {
			_, err := e.Client.PaneFocus(ctx, herdr.PaneTarget{PaneID: e.Chosen})
			return err
		},
	}}
}

// paneLines is the tail of every pane's output, a row per line, each carrying
// the pane it was printed in. The rows are read one pane at a time: a socket
// client is not documented to take concurrent calls, and a session holds few
// enough panes that the wait is one the popup already says it is having.
func paneLines(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	snapshot, err := e.Client.SessionSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	workspaces := make(map[string]string, len(snapshot.Snapshot.Workspaces))
	for _, workspace := range snapshot.Snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}

	var choices []palette.Choice
	for _, pane := range snapshot.Snapshot.Panes {
		text, err := recentOutput(ctx, e.Client, pane.PaneID, searchLines)
		if err != nil {
			// One pane that cannot be read is not worth losing the rest of
			// the session's output over.
			continue
		}
		where := whichPane(pane, workspaces[pane.WorkspaceID])
		for _, line := range strings.Split(text, "\n") {
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			choices = append(choices, palette.Choice{
				Value:  pane.PaneID,
				Title:  line,
				Detail: where,
			})
			if len(choices) == searchRows {
				return choices, nil
			}
		}
	}
	return choices, nil
}

// whichPane names the pane a line was printed in, for the column beside it:
// what it is known by, and the workspace it sits in, which is what tells two
// panes of the same name apart.
func whichPane(pane herdr.PaneInfo, workspace string) string {
	name := ""
	for _, value := range []*string{pane.Label, pane.Agent, pane.TerminalTitleStripped} {
		if name = herdr.Value(value); name != "" {
			break
		}
	}
	if name == "" {
		if cwd := herdr.Value(pane.Cwd); cwd != "" {
			name = filepath.Base(cwd)
		}
	}
	if workspace == "" {
		return name
	}
	return workspace + " / " + name
}
