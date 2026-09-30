package palette

import "strings"

// defaultComSpec is the shell herdr runs a command line under on Windows when
// ComSpec names none.
const defaultComSpec = `C:\Windows\System32\cmd.exe`

// comSpec is the shell a command line runs under on Windows, the one ComSpec
// names, as herdr picks it.
func comSpec(getenv func(string) string) string {
	if shell := getenv("ComSpec"); shell != "" {
		return shell
	}
	return defaultComSpec
}

// windowsCommandLine is the whole command line a configured command runs as on
// Windows: the shell, then /d /c, then the line exactly as it was written, the
// way herdr runs its own. cmd.exe reads what follows /c itself, so quoting the
// line as one argument would hand it quotes that were never in it.
func windowsCommandLine(shell, line string) string {
	return `"` + shell + `" /d /c ` + line
}

// editorLine is the command line that opens path in the user's editor. A
// POSIX shell picks the editor as it runs the line, from VISUAL, then EDITOR,
// then vi. cmd.exe has no such fallback, so on Windows the palette picks from
// the same variables itself, from the environment herdr passed it, and falls
// back to notepad, the one editor every Windows has.
func editorLine(goos string, getenv func(string) string, path string) string {
	if goos != "windows" {
		return "${VISUAL:-${EDITOR:-vi}} " + shellQuote(path)
	}
	editor := "notepad"
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if value := getenv(name); value != "" {
			editor = value
			break
		}
	}
	// A Windows path cannot hold a double quote, so wrapping it in them is
	// all cmd.exe needs.
	return editor + ` "` + path + `"`
}

// shellQuote wraps a path for the POSIX shell that runs the command line,
// which is the one thing about it this plugin composes rather than reads.
func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
