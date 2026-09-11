//go:build !unix

package palette

import "os/exec"

// detach has no equivalent outside unix; the manifest ships linux and macos
// only.
func detach(*exec.Cmd) {}

// processExists cannot be answered without a signal, so the wait for the popup
// falls back to its timeout.
func processExists(int) bool { return true }
