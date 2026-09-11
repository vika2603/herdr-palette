package main

import (
	"testing"

	"github.com/vika2603/herdr-client/plugin/plugintest"
)

// The manifest and the registered entrypoints have to agree: herdr runs the
// binary with the id from the manifest, and an id with no handler fails at
// the keypress.
func TestManifestMatchesTheRegisteredEntrypoints(t *testing.T) {
	plugintest.CheckManifest(t, "../../herdr-plugin.toml", newPlugin())
}
