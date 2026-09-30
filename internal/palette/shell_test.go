package palette

import "testing"

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// cmd.exe reads what follows /c as it stands, so the line keeps its own quotes
// and operators: quoting it as one argument would hand cmd.exe a line that
// starts with a quote and escapes it had never been given.
func TestAWindowsCommandLineIsHandedToCmdAsWritten(t *testing.T) {
	line := `git log --format="%h %s" -3 & echo "done"`
	got := windowsCommandLine(`C:\Windows\System32\cmd.exe`, line)
	if want := `"C:\Windows\System32\cmd.exe" /d /c ` + line; got != want {
		t.Errorf("command line = %s, want %s", got, want)
	}
}

func TestTheWindowsShellIsTheOneComSpecNames(t *testing.T) {
	if got := comSpec(env(map[string]string{"ComSpec": `D:\tools\cmd.exe`})); got != `D:\tools\cmd.exe` {
		t.Errorf("comSpec = %q, want the one ComSpec names", got)
	}
	if got := comSpec(env(nil)); got != defaultComSpec {
		t.Errorf("comSpec = %q, want %q when ComSpec names none", got, defaultComSpec)
	}
}

func TestTheWindowsEditorComesFromTheEnvironmentThenNotepad(t *testing.T) {
	path := `C:\Users\vika\AppData\Roaming\herdr\config.toml`
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"VISUAL": "code --wait", "EDITOR": "nvim"}, `code --wait "` + path + `"`},
		{map[string]string{"EDITOR": "nvim"}, `nvim "` + path + `"`},
		{nil, `notepad "` + path + `"`},
	} {
		if got := editorLine("windows", env(c.env), path); got != c.want {
			t.Errorf("editorLine(%v) = %s, want %s", c.env, got, c.want)
		}
	}
}

// Everywhere else the shell that runs the line picks the editor, so the line
// names none of its own.
func TestAPosixEditorIsPickedByTheShell(t *testing.T) {
	got := editorLine("darwin", env(map[string]string{"EDITOR": "nvim"}), "/tmp/it's.toml")
	if want := `${VISUAL:-${EDITOR:-vi}} '/tmp/it'\''s.toml'`; got != want {
		t.Errorf("editorLine = %s, want %s", got, want)
	}
}
