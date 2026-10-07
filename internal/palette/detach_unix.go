//go:build unix

package palette

import (
	"os"
	"os/exec"
	"syscall"
)

// detach puts the command in a session of its own, out of reach of the signals
// that end the popup.
func detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// processExists reports whether the process is still running. Signal zero is
// the portable existence check on unix.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// killTree makes cancelling the command end everything it started, not only
// the shell: the command leads a process group of its own, and the whole group
// is killed.
func killTree(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
