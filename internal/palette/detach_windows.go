//go:build windows

package palette

import (
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// detach gives the command a console of its own, with no window, and a
// process group of its own. The popup's console goes when the popup closes and
// takes whatever is attached to it along; a hidden console rather than none
// keeps a console program the command starts from opening a window of its own.
func detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP
}

// stillActive is the exit code Windows reports for a process that has not
// exited, STILL_ACTIVE in its headers.
const stillActive = 259

// processExists reports whether the process is still running, the way herdr
// checks on Windows: a process that has exited can still be opened for as
// long as a handle to it is held, so the exit code is what says.
func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var code uint32
	return windows.GetExitCodeProcess(handle, &code) == nil && code == stillActive
}

// killTree makes cancelling the command end everything it started, not only
// cmd.exe: taskkill /T ends the process tree.
func killTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
		return kill.Run()
	}
}
