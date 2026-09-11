//go:build unix

package palette

import (
	"os/exec"
	"syscall"
)

// detach puts the command in a session of its own, out of reach of the signals
// that end the popup.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
