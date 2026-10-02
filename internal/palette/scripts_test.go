package palette

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func writePaletteScript(t testing.TB, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writePortablePaletteScript(t testing.TB, dir, name, title, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		body := "REM @palette.title " + title + "\r\nREM @palette.mode " + mode + "\r\n@echo off\r\n"
		return writePaletteScript(t, dir, name+".cmd", body)
	}
	body := "#!/bin/sh\n# @palette.title " + title + "\n# @palette.mode " + mode + "\nexit 0\n"
	return writePaletteScript(t, dir, name+".sh", body)
}

func scriptByID(t *testing.T, entries []Entry, path string) Entry {
	t.Helper()
	id := "script:" + path
	for _, entry := range entries {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("no script entry with ID %q among %v", id, scriptEntryIDs(entries))
	return Entry{}
}

func scriptEntryIDs(entries []Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}

func readScriptEntries(t *testing.T, dir string) []Entry {
	t.Helper()
	entries, err := ScriptEntries("herdr.palette", []string{dir})
	if err != nil {
		t.Fatalf("ScriptEntries(%q) = %v", dir, err)
	}
	return entries
}

func TestScriptEntriesDiscoverOnlyRunnableScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable and shebang discovery")
	}
	dir := t.TempDir()
	primary := writePaletteScript(t, dir, "Open Git.sh", "#!/bin/sh\n# @palette.title Git status\n# @palette.mode pane\nexit 0\n")
	writePaletteScript(t, dir, ".hidden.sh", "#!/bin/sh\nexit 0\n")
	writePaletteScript(t, dir, "plain.sh", "exit 0\n")
	withoutExec := writePaletteScript(t, dir, "not-executable.sh", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(withoutExec, 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writePaletteScript(t, nested, "child.sh", "#!/bin/sh\nexit 0\n")
	link := filepath.Join(dir, "Git link.sh")
	if err := os.Symlink(primary, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	entries := readScriptEntries(t, dir)
	if got, want := scriptEntryIDs(entries), []string{"script:" + link}; !slices.Equal(got, want) {
		t.Fatalf("discovered %v, want %v", got, want)
	}
	entry := scriptByID(t, entries, link)
	if entry.Title != "Git status" || entry.Detail != "pane" || entry.Description != link {
		t.Errorf("entry = %+v, want metadata and the absolute script path", entry)
	}
	if entry.Type != TypeCustom || entry.Kind != KindCustom || !entry.AlwaysRelay {
		t.Errorf("entry = %+v, want a relayed custom command", entry)
	}
	if got := (List{Commands: entries}).Rows(ScopeCommands); len(got) != 1 {
		t.Errorf("commands scope listed %d scripts, want 1", len(got))
	}
	if got := Rank(entries, "git status", nil); len(got) != 1 || got[0].Entry.ID != entry.ID {
		t.Errorf("search for the script title matched %v, want the single resolved script", ids(got))
	}
}

func TestScriptEntriesAcceptNoDirectories(t *testing.T) {
	for _, dirs := range [][]string{nil, {}} {
		entries, err := ScriptEntries("herdr.palette", dirs)
		if err != nil || len(entries) != 0 {
			t.Errorf("ScriptEntries(%v) = %v, %v, want empty entries without error", dirs, entries, err)
		}
	}
}

func TestScriptEntriesLoadMultipleDirectoriesWithDistinctPaths(t *testing.T) {
	firstDir, secondDir := t.TempDir(), t.TempDir()
	firstPath := writePortablePaletteScript(t, firstDir, "one", "Shared tool", "shell")
	secondPath := writePortablePaletteScript(t, secondDir, "two", "Shared tool", "popup")
	entries, err := ScriptEntries("herdr.palette", []string{firstDir, secondDir})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + firstPath, "script:" + secondPath}; !slices.Equal(got, want) {
		t.Fatalf("entries = %v, want %v in directory order", got, want)
	}
	for _, test := range []struct {
		path string
		mode string
	}{{firstPath, "shell"}, {secondPath, "popup"}} {
		entry := scriptByID(t, entries, test.path)
		if entry.Title != "Shared tool" || entry.Description != test.path || !strings.Contains(entry.Detail, test.path) || !strings.Contains(entry.Detail, test.mode) {
			t.Errorf("entry = %+v, want the shared title, mode, and distinguishing path", entry)
		}
	}
}

func TestScriptEntriesDeduplicateAliasesInDirectoryOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating file and directory symlinks needs Windows privileges")
	}
	primaryDir, aliasDir := t.TempDir(), t.TempDir()
	primary := writePortablePaletteScript(t, primaryDir, "tool", "Primary", "shell")
	alias := filepath.Join(aliasDir, "alias.sh")
	if err := os.Symlink(primary, alias); err != nil {
		t.Skipf("file symlinks unavailable: %v", err)
	}
	dirLink := filepath.Join(t.TempDir(), "linked-dir")
	if err := os.Symlink(aliasDir, dirLink); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}

	entries, err := ScriptEntries("herdr.palette", []string{aliasDir, aliasDir, dirLink, primaryDir})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + alias}; !slices.Equal(got, want) {
		t.Errorf("aliases discovered %v, want first reference %v", got, want)
	}
	if entries[0].Description != alias || entries[0].Title != "Primary" {
		t.Errorf("first reference entry = %+v, want alias path and metadata", entries[0])
	}

	entries, err = ScriptEntries("herdr.palette", []string{primaryDir, aliasDir})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + primary}; !slices.Equal(got, want) {
		t.Errorf("reversed directories discovered %v, want first reference %v", got, want)
	}
}

func TestScriptEntriesContinueAfterFailuresInOtherDirectories(t *testing.T) {
	goodDir, invalidDir := t.TempDir(), t.TempDir()
	good := writePortablePaletteScript(t, goodDir, "good", "Good", "shell")
	invalid := writePortablePaletteScript(t, invalidDir, "invalid", "Bad", "split")
	notDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ScriptEntries("herdr.palette", []string{notDir, invalidDir, goodDir})
	if err == nil || !strings.Contains(err.Error(), notDir) || !strings.Contains(err.Error(), invalid) {
		t.Fatalf("errors = %v, want both failing paths", err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + good}; !slices.Equal(got, want) {
		t.Errorf("entries = %v, want the valid script from another directory", got)
	}
}

func TestScriptEntriesDeduplicateDirectoryCaseAliases(t *testing.T) {
	root := t.TempDir()
	dir, alias := filepath.Join(root, "Scripts"), filepath.Join(root, "scripts")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(alias); errors.Is(err, os.ErrNotExist) {
		t.Skip("filesystem distinguishes directory casing")
	} else if err != nil {
		t.Fatal(err)
	}
	path := writePortablePaletteScript(t, dir, "tool", "Tool", "shell")
	entries, err := ScriptEntries("herdr.palette", []string{dir, alias})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + path}; !slices.Equal(got, want) {
		t.Fatalf("directory aliases discovered %v, want %v", got, want)
	}
}

func TestScriptEntriesReloadAfterAFileChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "Tool.sh", "#!/bin/sh\n# @palette.title First\nexit 0\n")
	first := scriptByID(t, readScriptEntries(t, dir), path)
	if first.Title != "First" || first.Detail != "shell" {
		t.Errorf("first load = %+v, want the title and default mode", first)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n# @palette.title Second\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	second := scriptByID(t, readScriptEntries(t, dir), path)
	if second.Title != "Second" || second.ID != first.ID {
		t.Errorf("reloaded entry = %+v, want changed title and stable ID %q", second, first.ID)
	}
	added := writePaletteScript(t, dir, "Lazygit.sh", "#!/bin/sh\nexit 0\n")
	if got := len(readScriptEntries(t, dir)); got != 2 {
		t.Errorf("after adding a script: %d entries, want 2", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	remaining := readScriptEntries(t, dir)
	if got, want := scriptEntryIDs(remaining), []string{"script:" + added}; !slices.Equal(got, want) {
		t.Errorf("after removing a script: %v, want %v", got, want)
	}
	if remaining[0].Title != "Lazygit" {
		t.Errorf("filename title = %q, want the filename with its original casing", remaining[0].Title)
	}
}

func TestScriptEntriesReportInvalidMetadataWithoutHidingValidScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	good := writePaletteScript(t, dir, "good.sh", "#!/bin/sh\nexit 0\n")
	bad := writePaletteScript(t, dir, "bad.sh", "#!/bin/sh\n# @palette.mode split\nexit 0\n")
	entries, err := ScriptEntries("herdr.palette", []string{dir})
	if err == nil || !strings.Contains(err.Error(), filepath.Base(bad)) {
		t.Fatalf("invalid mode error = %v, want the script filename", err)
	}
	if got, want := scriptEntryIDs(entries), []string{"script:" + good}; !slices.Equal(got, want) {
		t.Errorf("entries = %v, want the valid script to remain", got)
	}

	for _, header := range []string{
		"# @palette.title\n",
		"# @palette.unknown nope\n",
		"# @palette.title One\n# @palette.title Two\n",
	} {
		if err := os.WriteFile(bad, []byte("#!/bin/sh\n"+header+"exit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := ScriptEntries("herdr.palette", []string{dir})
		if err == nil || !strings.Contains(err.Error(), filepath.Base(bad)) {
			t.Errorf("header %q: error = %v, want filename", header, err)
		}
	}
}

func TestScriptEntriesReadOnlyTheInitialComments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "one.sh", "#!/bin/sh\n# @palette.title First\necho body\n# @palette.title Ignored\n"+strings.Repeat("x", 1<<20))
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	if entry.Title != "First" {
		t.Errorf("title = %q, want only the initial comment block", entry.Title)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+strings.Repeat("# padding\n", 600)+"# @palette.title Beyond limit\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := ScriptEntries("herdr.palette", []string{dir})
	if err == nil || !strings.Contains(err.Error(), filepath.Base(path)) {
		t.Errorf("oversized header error = %v, want filename", err)
	}
}

func TestScriptEntriesHandleMissingAndUnusableDirectories(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if entries, err := ScriptEntries("herdr.palette", []string{dir}); err != nil || len(entries) != 0 {
		t.Errorf("missing directory = %v, %v, want empty entries without error", entries, err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("ordinary file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ScriptEntries("herdr.palette", []string{file}); err == nil {
		t.Error("a non-directory path was treated as an empty script directory")
	}
}

func TestShellScriptRunsInTheInvokingPaneWithItsContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "it's $here.sh", "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$HERDR_ACTIVE_WORKSPACE_ID\" \"$HERDR_ACTIVE_TAB_ID\" \"$HERDR_ACTIVE_PANE_ID\" \"$HERDR_ACTIVE_PANE_CWD\" > \"$PALETTE_TEST_OUTPUT\"\n")
	out := filepath.Join(t.TempDir(), "seen")
	t.Setenv("PALETTE_TEST_OUTPUT", out)
	ctx := focusedContext()
	ctx.FocusedPaneCwd = &dir
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	if err := entry.Run(context.Background(), Exec{Ctx: ctx}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	var data []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(out)
		if len(data) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	want := []string{dir, "w1", "w1:t2", "w1:p3", dir}
	if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !slices.Equal(got, want) {
		t.Errorf("script saw %q, want %q", got, want)
	}
}

func TestScriptDoesNotLaunchWithMissingDirectoryOrDeletedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "tool.sh", "#!/bin/sh\nexit 0\n")
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	ctx := focusedContext()
	missing := filepath.Join(t.TempDir(), "missing")
	ctx.FocusedPaneCwd = &missing
	if err := entry.Run(context.Background(), Exec{Ctx: ctx}); err == nil {
		t.Error("script launched although the invoking directory was gone")
	}
	ctx.FocusedPaneCwd = &dir
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := entry.Run(context.Background(), Exec{Ctx: ctx}); err == nil {
		t.Error("script launched although its file was deleted")
	}
}

func TestPaneAndPopupScriptsKeepTheInvokingContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell script fixture")
	}
	for _, test := range []struct {
		mode      string
		placement herdr.PluginPanePlacement
		target    bool
	}{
		{"pane", herdr.PluginPanePlacementZoomed, true},
		{"popup", herdr.PluginPanePlacementPopup, false},
	} {
		t.Run(test.mode, func(t *testing.T) {
			dir := t.TempDir()
			path := writePaletteScript(t, dir, "tool.sh", "#!/bin/sh\n# @palette.mode "+test.mode+"\nexit 0\n")
			server := plugintest.NewServer(t).Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
			env := server.Env(plugintest.StateDir(t.TempDir()))
			ctx := focusedContext()
			ctx.FocusedPaneCwd = &dir
			entry := scriptByID(t, readScriptEntries(t, dir), path)
			if err := entry.Run(context.Background(), Exec{Client: env.Client(), Ctx: ctx, Env: env}); err != nil {
				t.Fatalf("Run() = %v", err)
			}
			calls := server.Calls()
			if len(calls) != 1 || calls[0].Method != herdr.MethodPluginPaneOpen {
				t.Fatalf("calls = %v, want one pane open", calls)
			}
			var params herdr.PluginPaneOpenParams
			decode(t, calls[0].Params, &params)
			if params.Placement == nil || *params.Placement != test.placement {
				t.Errorf("placement = %v, want %q", params.Placement, test.placement)
			}
			if params.Cwd == nil || *params.Cwd != dir {
				t.Errorf("cwd = %v, want %q", params.Cwd, dir)
			}
			if got := params.TargetPaneID != nil; got != test.target {
				t.Errorf("has target pane = %v, want %v", got, test.target)
			}
			if test.target && *params.TargetPaneID != "w1:p3" {
				t.Errorf("target pane = %q, want w1:p3", *params.TargetPaneID)
			}
			for name, want := range map[string]string{
				"HERDR_ACTIVE_WORKSPACE_ID": "w1",
				"HERDR_ACTIVE_TAB_ID":       "w1:t2",
				"HERDR_ACTIVE_PANE_ID":      "w1:p3",
				"HERDR_ACTIVE_PANE_CWD":     dir,
				"HERDR_PALETTE_SCRIPT":      path,
			} {
				if got := params.Env[name]; got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
			if got := params.Env[RunEnv]; got != "" {
				t.Errorf("%s = %q, want no configured command", RunEnv, got)
			}
		})
	}
}

func TestRelayedScriptCanBeReloadedAndRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "tool.sh", "#!/bin/sh\n# @palette.mode pane\nexit 0\n")
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{}).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	ctx := focusedContext()
	ctx.FocusedPaneCwd = &dir
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	if err := Relay(context.Background(), env.Client(), env, entry, ctx); err != nil {
		t.Fatalf("Relay() = %v", err)
	}
	pending, ok := ReadPending(env)
	if !ok || pending.EntryID != entry.ID {
		t.Fatalf("pending = %+v, %v, want %q", pending, ok, entry.ID)
	}
	pending.PID = 0 // The palette process has closed before the exec action runs.
	if err := RunPending(context.Background(), env, readScriptEntries(t, dir), pending); err != nil {
		t.Fatalf("RunPending() = %v", err)
	}
	calls := server.Calls()
	if len(calls) != 2 || calls[0].Method != herdr.MethodPluginActionInvoke || calls[1].Method != herdr.MethodPluginPaneOpen {
		t.Errorf("calls = %v, want invoke then pane open", calls)
	}
}

func TestScriptCommandRunsTheScriptDirectly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shebang dispatch")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "it's $here.sh", "#!/bin/sh\nprintf 'ran'\n")
	cmd, err := ScriptCommand(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.Output()
	if err != nil || string(out) != "ran" {
		t.Errorf("ScriptCommand() output = %q, %v, want ran", out, err)
	}
}

func TestWindowsPowerShellScriptCanReadTerminalInput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell script execution")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "ask me.ps1", "Write-Output (Read-Host 'Prompt')\n")
	cmd, err := ScriptCommand(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = strings.NewReader("hello\n")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "hello") {
		t.Errorf("PowerShell output = %q, %v, want the input echoed", out, err)
	}
}

func TestWindowsBatchScriptLaunchesFromAPathWithShellCharacters(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows batch execution")
	}
	dir := t.TempDir()
	t.Setenv("PALETTE_TEST_VALUE", "expanded")
	path := writePaletteScript(t, dir, "it's %PALETTE_TEST_VALUE%! & here.cmd", "REM @palette.title Batch tool\r\n@echo off\r\necho ran\r\n")
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	if entry.Title != "Batch tool" {
		t.Errorf("batch title = %q, want metadata from REM comment", entry.Title)
	}
	cmd, err := ScriptCommand(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ran") {
		t.Errorf("batch output = %q, %v, want ran", out, err)
	}
}

func BenchmarkScriptEntries(b *testing.B) {
	if runtime.GOOS == "windows" {
		b.Skip("Unix shell script fixture")
	}
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprintf("scripts=%d", count), func(b *testing.B) {
			dir := b.TempDir()
			for i := range count {
				writePaletteScript(b, dir, fmt.Sprintf("script-%04d.sh", i), "#!/bin/sh\n# @palette.mode shell\nexit 0\n")
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := ScriptEntries("herdr.palette", []string{dir})
				if err != nil || len(entries) != count {
					b.Fatalf("ScriptEntries() = %d entries, %v, want %d", len(entries), err, count)
				}
			}
		})
	}
}
