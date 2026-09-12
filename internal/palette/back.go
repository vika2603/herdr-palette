package palette

import (
	"context"
	"fmt"
	"strings"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

// The kinds of place a recent id names, the part in front of the colon of the
// ids the session rows carry. Every other id ran a command, which is not
// somewhere to go back to.
const (
	backWorkspace = "workspace"
	backTab       = "tab"
	backPane      = "pane"
)

// backTitle heads what this action tells the user. It opens no popup, so a
// notification is the only way anything reaches them.
const backTitle = "back"

// Back goes to the place the palette last went to, without opening the popup.
// It follows the recent order the palette keeps rather than the pane herdr
// last focused, so it returns to whatever was reached through the palette,
// workspace, tab or pane alike.
func Back(ctx context.Context, client *herdr.Client, env *plugin.Env) error {
	// An entrypoint invoked outside a workspace has no context, which only
	// means nothing is left out for being where the session already is.
	invocation, _ := env.Context()

	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		err = fmt.Errorf("open panes unavailable: %w", err)
		report(ctx, client, backTitle, err)
		return err
	}

	var failure error
	for _, target := range backTargets(ReadRecent(env), snapshot.Snapshot, invocation) {
		if failure = target.focus(ctx, client); failure == nil {
			return nil
		}
	}
	if failure != nil {
		// The target was in the snapshot, so a refusal is a failure of its
		// own rather than something that is no longer there.
		report(ctx, client, backTitle, failure)
		return failure
	}

	body := "nothing the palette went to is still open"
	_, _ = client.NotificationShow(ctx, herdr.NotificationShowParams{
		Title: "Nowhere to go back to",
		Body:  &body,
	})
	return nil
}

// backTarget is one place the recent order says the palette went to.
type backTarget struct {
	kind string
	id   string
}

// backTargets is where to go, most recent first: the ids that name a place,
// minus where the session already is and minus what is no longer open.
//
// What is still there is read from the snapshot rather than found by focusing
// one id after another. Ids outlive the session they were written in, so the
// front of the list can be a long run of dead ones, and a refused focus does
// not say whether the target is gone or the call failed for another reason.
func backTargets(recent []string, snapshot herdr.SessionSnapshot, invocation *herdr.PluginInvocationContext) []backTarget {
	targets := make([]backTarget, 0, len(recent))
	for _, id := range recent {
		target, ok := backTargetOf(id)
		if !ok || target.here(invocation) || !target.open(snapshot) {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

// backTargetOf reads the place an entry id goes to, and reports whether it
// goes anywhere at all.
func backTargetOf(id string) (backTarget, bool) {
	kind, target, ok := strings.Cut(id, ":")
	if !ok || target == "" {
		return backTarget{}, false
	}
	switch kind {
	case backWorkspace, backTab, backPane:
		return backTarget{kind: kind, id: target}, true
	}
	return backTarget{}, false
}

// here reports whether the target is where the action was invoked from, which
// is the one place going back would not move.
func (t backTarget) here(invocation *herdr.PluginInvocationContext) bool {
	if invocation == nil {
		return false
	}
	switch t.kind {
	case backWorkspace:
		return herdr.Value(invocation.WorkspaceID) == t.id
	case backTab:
		return herdr.Value(invocation.TabID) == t.id
	default:
		return herdr.Value(invocation.FocusedPaneID) == t.id
	}
}

func (t backTarget) open(snapshot herdr.SessionSnapshot) bool {
	switch t.kind {
	case backWorkspace:
		for _, workspace := range snapshot.Workspaces {
			if workspace.WorkspaceID == t.id {
				return true
			}
		}
	case backTab:
		for _, tab := range snapshot.Tabs {
			if tab.TabID == t.id {
				return true
			}
		}
	default:
		for _, pane := range snapshot.Panes {
			if pane.PaneID == t.id {
				return true
			}
		}
	}
	return false
}

func (t backTarget) focus(ctx context.Context, client *herdr.Client) error {
	switch t.kind {
	case backWorkspace:
		_, err := client.WorkspaceFocus(ctx, herdr.WorkspaceTarget{WorkspaceID: t.id})
		return err
	case backTab:
		_, err := client.TabFocus(ctx, herdr.TabTarget{TabID: t.id})
		return err
	default:
		_, err := client.PaneFocus(ctx, herdr.PaneTarget{PaneID: t.id})
		return err
	}
}
