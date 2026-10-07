//go:build !unix && !windows

package palette

import "os/exec"

// detach has no equivalent on a platform other than unix and Windows, which
// are the ones the manifest ships.
func detach(*exec.Cmd) {}

// processExists cannot be answered without a signal, so the wait for the popup
// falls back to its timeout.
func processExists(int) bool { return true }

// killTree leaves cancelling to os/exec, which kills the command itself.
func killTree(*exec.Cmd) {}
