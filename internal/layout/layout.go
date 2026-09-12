// Package layout keeps the tab arrangements the palette saves.
//
// herdr exports a tab's layout and applies one back, but it stores none: the
// arrangement lives as long as the tab does. The palette therefore writes them
// down in the plugin's state directory, under the name they were saved with.
package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

// File is where the saved layouts live, in the plugin's state directory.
const File = "layouts.json"

// Saved is one arrangement under the name it was saved with. The tree is kept
// as it was written rather than decoded, so a layout herdr describes in a way
// this plugin does not know survives being listed next to the others.
type Saved struct {
	Name string          `json:"name"`
	Root json.RawMessage `json:"root"`
}

type file struct {
	Layouts []Saved `json:"layouts"`
}

// List is every saved layout, most recently saved first.
func List(env *plugin.Env) []Saved {
	var stored file
	// A file that cannot be read only means there is nothing to apply, which
	// the palette says the same way it says nothing was ever saved.
	if err := env.ReadStateJSON(File, &stored); err != nil {
		return nil
	}
	return stored.Layouts
}

// Save writes the arrangement down under name, replacing a layout of the same
// name and putting it first.
func Save(env *plugin.Env, name string, root herdr.LayoutNode) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("a layout needs a name to be saved under")
	}
	tree, err := json.Marshal(withoutPaneIDs(root))
	if err != nil {
		return err
	}

	layouts := append([]Saved{{Name: name, Root: tree}}, without(List(env), name)...)
	return env.WriteStateJSON(File, file{Layouts: layouts})
}

// Remove forgets the layout saved under name.
func Remove(env *plugin.Env, name string) error {
	layouts := List(env)
	left := without(layouts, name)
	if len(left) == len(layouts) {
		return fmt.Errorf("no layout is saved as %q", name)
	}
	return env.WriteStateJSON(File, file{Layouts: left})
}

// Root is the tree saved under name, decoded the way herdr's own apply
// parameters decode one.
func Root(env *plugin.Env, name string) (herdr.LayoutNode, error) {
	for _, saved := range List(env) {
		if saved.Name != name {
			continue
		}
		// LayoutNode is a union the client decodes by its type tag, and the
		// apply parameters are where that decoder is reachable from here.
		var params herdr.LayoutApplyParams
		wrapped, err := json.Marshal(struct {
			Root json.RawMessage `json:"root"`
		}{Root: saved.Root})
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(wrapped, &params); err != nil {
			return nil, fmt.Errorf("the layout saved as %q cannot be read: %w", name, err)
		}
		return params.Root, nil
	}
	return nil, fmt.Errorf("no layout is saved as %q", name)
}

func without(layouts []Saved, name string) []Saved {
	left := make([]Saved, 0, len(layouts))
	for _, saved := range layouts {
		if saved.Name != name {
			left = append(left, saved)
		}
	}
	return left
}

// withoutPaneIDs drops the panes' ids, which name the panes the arrangement
// was exported from. A saved layout outlives them, and applying one with the
// ids left in would move those panes rather than open the arrangement again.
//
// Decoding a layout produces the value form of each variant, which is what an
// exported tree is made of.
func withoutPaneIDs(node herdr.LayoutNode) herdr.LayoutNode {
	switch node := node.(type) {
	case herdr.LayoutNodePane:
		node.PaneID = nil
		return node
	case herdr.LayoutNodeSplit:
		node.First = withoutPaneIDs(node.First)
		node.Second = withoutPaneIDs(node.Second)
		return node
	default:
		return node
	}
}
