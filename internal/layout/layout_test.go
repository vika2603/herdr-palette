package layout

import (
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func testEnv(t *testing.T) *plugin.Env {
	t.Helper()
	return plugintest.Env(plugintest.StateDir(t.TempDir()))
}

// exported is the shape herdr answers layout.export with: a tree of splits
// whose leaves name the panes it was exported from.
func exported() herdr.LayoutNode {
	return herdr.LayoutNodeSplit{
		Direction: herdr.SplitDirectionRight,
		Ratio:     0.5,
		First:     herdr.LayoutNodePane{PaneID: new("w1:p1"), Cwd: new("/repo")},
		Second:    herdr.LayoutNodePane{PaneID: new("w1:p2"), Cwd: new("/repo/sub")},
	}
}

// A saved layout outlives the panes it was exported from, so their ids are
// dropped: with them in, applying it would move those panes instead of opening
// the arrangement again.
func TestASavedLayoutKeepsTheArrangementWithoutThePanes(t *testing.T) {
	env := testEnv(t)
	if err := Save(env, "work", exported(), nil); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	root, err := Root(env, "work")
	if err != nil {
		t.Fatalf("Root() = %v", err)
	}
	split, ok := root.(herdr.LayoutNodeSplit)
	if !ok {
		t.Fatalf("root is %T, want the split it was saved as", root)
	}
	if split.Direction != herdr.SplitDirectionRight || split.Ratio != 0.5 {
		t.Errorf("split = %+v, want the direction and ratio it was saved with", split)
	}

	panes := herdr.LayoutPanes(root)
	if len(panes) != 2 {
		t.Fatalf("the layout has %d panes, want the two it was saved with", len(panes))
	}
	for _, pane := range panes {
		if pane.PaneID != nil {
			t.Errorf("pane keeps the id %q it was exported from", *pane.PaneID)
		}
	}
	if herdr.Value(panes[1].Cwd) != "/repo/sub" {
		t.Errorf("pane cwd = %q, want the directory it was saved with", herdr.Value(panes[1].Cwd))
	}
}

func TestSavingUnderAUsedNameReplacesItAndComesFirst(t *testing.T) {
	env := testEnv(t)
	for _, name := range []string{"work", "review", "work"} {
		if err := Save(env, name, exported(), nil); err != nil {
			t.Fatalf("Save(%q) = %v", name, err)
		}
	}

	saved := List(env)
	if len(saved) != 2 {
		t.Fatalf("kept %d layouts, want one per name", len(saved))
	}
	if saved[0].Name != "work" {
		t.Errorf("first layout is %q, want the one just saved", saved[0].Name)
	}
}

func TestANamelessLayoutIsNotSaved(t *testing.T) {
	env := testEnv(t)
	if err := Save(env, "  ", exported(), nil); err == nil {
		t.Fatal("Save() kept a layout with no name to find it by")
	}
	if len(List(env)) != 0 {
		t.Error("a layout was written despite the failure")
	}
}

func TestForgettingALayout(t *testing.T) {
	env := testEnv(t)
	if err := Save(env, "work", exported(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Remove(env, "work"); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if len(List(env)) != 0 {
		t.Error("the layout is still saved")
	}
	if err := Remove(env, "work"); err == nil {
		t.Error("Remove() reported no error for a layout that is not saved")
	}
}

// Nothing saved and nothing readable are the same to the palette: there is no
// layout to open.
func TestNothingSavedYet(t *testing.T) {
	env := testEnv(t)
	if got := List(env); len(got) != 0 {
		t.Errorf("List() = %v, want nothing", got)
	}
	if _, err := Root(env, "work"); err == nil {
		t.Error("Root() returned a layout that was never saved")
	}
}
