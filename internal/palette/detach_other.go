//go:build !unix

package palette

import "os/exec"

// detach has no equivalent outside unix; the manifest ships linux and macos
// only.
func detach(*exec.Cmd) {}
