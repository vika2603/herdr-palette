package palette

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/manifest"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

const deployHeader = `#!/bin/sh
# @palette.title Deploy
# @palette.mode popup
# @palette.width 80%
# @palette.height 20
# @palette.confirm true
# @palette.argument1 { "name": "branch", "type": "text", "placeholder": "branch to deploy" }
# @palette.argument2 { "name": "env", "type": "dropdown", "data": [{ "title": "Staging", "value": "staging" }, { "title": "Branch preview", "value": "preview" }] }
# @palette.argument3 { "name": "token", "type": "password", "optional": true, "percentEncoded": true }
`

func TestScriptHeaderDeclaresArgumentsAndAPopupSize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "deploy.sh", deployHeader+"exit 0\n")
	entry := scriptByID(t, readScriptEntries(t, dir), path)

	if !entry.Confirm {
		t.Error("@palette.confirm true did not mark the script to be confirmed")
	}
	if len(entry.Arguments) != 3 {
		t.Fatalf("arguments = %+v, want three", entry.Arguments)
	}
	branch, env, token := entry.Arguments[0], entry.Arguments[1], entry.Arguments[2]
	if branch.Name != "branch" || branch.Env != "HP_BRANCH" || branch.Placeholder != "branch to deploy" || branch.Type != ArgumentText || branch.Optional {
		t.Errorf("branch = %+v", branch)
	}
	if env.Placeholder != "env" || env.Type != ArgumentDropdown || len(env.Options) != 2 || env.Options[1] != (Choice{Title: "Branch preview", Value: "preview"}) {
		t.Errorf("env = %+v, want the name as placeholder and both options", env)
	}
	if token.Env != "HP_TOKEN" || token.Type != ArgumentPassword || !token.Optional || !token.PercentEncoded {
		t.Errorf("token = %+v", token)
	}
}

func TestScriptHeaderRejectsBadArgumentsAndSizes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	for _, test := range []struct {
		header string
		want   string
	}{
		{`# @palette.argument1 { "name": "q", "type": "text", "optinal": true }`, `unknown field "optinal"`},
		{`# @palette.argument1 { "type": "text" }`, `name ""`},
		{`# @palette.argument1 { "name": "1st", "type": "text" }`, `name "1st"`},
		{`# @palette.argument1 { "name": "from city", "type": "text" }`, `name "from city"`},
		{`# @palette.argument1 { "name": "q", "type": "number" }`, `type "number"`},
		{`# @palette.argument1 { "name": "q", "type": "dropdown" }`, "dropdown needs data"},
		{`# @palette.argument1 { "name": "q", "type": "dropdown", "data": [{ "title": "A" }] }`, "needs a title and a value"},
		{`# @palette.argument1 { "name": "q", "type": "text", "data": [] }`, "only for a dropdown"},
		{`# @palette.argument1 { "name": "q", "type": "text", "command": "ls" }`, "only for a dropdown"},
		{`# @palette.argument1 { "name": "q", "type": "dropdown", "command": "ls", "data": [{ "title": "A", "value": "a" }] }`, "not both"},
		{`# @palette.argument1 { "name": "q", "type": "text" } trailing`, "after the argument"},
		{`# @palette.argument2 { "name": "q", "type": "text" }`, "@palette.argument2 needs @palette.argument1"},
		{"# @palette.argument1 { \"name\": \"q\", \"type\": \"text\" }\n# @palette.argument2 { \"name\": \"Q\", \"type\": \"text\" }", "both passed in HP_Q"},
		{`# @palette.argument7 { "name": "q", "type": "text" }`, "unknown script field @palette.argument7"},
		{`# @palette.argument01 { "name": "q", "type": "text" }`, "unknown script field @palette.argument01"},
		{"# @palette.width 80%", "need @palette.mode popup"},
		{"# @palette.confirm yes", "must be true or false"},
		{"# @palette.mode popup\n# @palette.width wide", `size "wide"`},
		{"# @palette.mode popup\n# @palette.height 0", "at least one cell"},
		{"# @palette.mode popup\n# @palette.height 120%", "between 1% and 100%"},
	} {
		dir := t.TempDir()
		path := writePaletteScript(t, dir, "bad.sh", "#!/bin/sh\n"+test.header+"\nexit 0\n")
		entries, err := ScriptEntries("herdr.palette", []string{dir})
		if err == nil || !strings.Contains(err.Error(), filepath.Base(path)) || !strings.Contains(err.Error(), test.want) {
			t.Errorf("header %q: error = %v, want the filename and %q", test.header, err, test.want)
		}
		if len(entries) != 0 {
			t.Errorf("header %q: listed %d scripts, want the invalid one left out", test.header, len(entries))
		}
	}
}

func TestADropdownCommandListsOneOptionPerLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	argument, err := parseArgument(`{ "name": "branch", "type": "dropdown", "command": "printf 'main\\n\\n%s\\t%s\\n' \"$HERDR_ACTIVE_PANE_ID\" \"$(pwd)\"" }`)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	options, err := argument.LoadOptions(context.Background(), &herdr.PluginInvocationContext{FocusedPaneID: new("w1:p2"), FocusedPaneCwd: &dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Choice{{Title: "main", Value: "main"}, {Title: "w1:p2", Value: dir}}
	if !slices.Equal(options, want) {
		t.Errorf("options = %+v, want %+v: a blank line skipped, a tab between title and value", options, want)
	}

	argument.Command = "echo no repository >&2; exit 2"
	if _, err := argument.LoadOptions(context.Background(), &herdr.PluginInvocationContext{}, nil); err == nil || !strings.Contains(err.Error(), "no repository") {
		t.Errorf("error = %v, want what the command printed to stderr", err)
	}
}

// A command past its time is ended with everything it started, so a hung
// child does not outlive the palette's wait for it.
func TestASlowDropdownCommandIsEndedWithItsChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command")
	}
	defer func(previous time.Duration) { optionsTimeout = previous }(optionsTimeout)
	optionsTimeout = 200 * time.Millisecond
	marker := "61.4" + strconv.Itoa(os.Getpid()%1000)
	argument := Argument{Name: "branch", Type: ArgumentDropdown, Command: "(sleep " + marker + "; echo late); echo main"}

	_, err := argument.LoadOptions(context.Background(), &herdr.PluginInvocationContext{}, nil)
	if err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("error = %v, want the timeout reported", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for exec.Command("pgrep", "-f", "sleep "+marker).Run() == nil {
		if time.Now().After(deadline) {
			_ = exec.Command("pkill", "-f", "sleep "+marker).Run()
			t.Fatal("the command's child is still running after the timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestShellScriptReceivesItsArgumentsByPositionAndName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix positional arguments")
	}
	dir := t.TempDir()
	header := strings.Replace(deployHeader, "# @palette.mode popup\n# @palette.width 80%\n# @palette.height 20\n", "", 1)
	path := writePaletteScript(t, dir, "deploy.sh", header+
		`printf '%s\n' "$#" "$1" "$2" "$3" "$HP_BRANCH" "$HP_ENV" "$HP_TOKEN" > "$PALETTE_TEST_OUTPUT"`+"\n")
	out := filepath.Join(t.TempDir(), "seen")
	t.Setenv("PALETTE_TEST_OUTPUT", out)
	ctx := focusedContext()
	ctx.FocusedPaneCwd = &dir
	entry := scriptByID(t, readScriptEntries(t, dir), path)

	if err := entry.Run(context.Background(), Exec{Ctx: ctx, Args: []string{"feature/x y", "preview", "a b/ü"}}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	var data []byte
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if data, _ = os.ReadFile(out); len(data) > 0 {
			break
		}
	}
	want := []string{"3", "feature/x y", "preview", "a%20b%2F%C3%BC", "feature/x y", "preview", "a%20b%2F%C3%BC"}
	if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); !slices.Equal(got, want) {
		t.Errorf("script saw %q, want %q", got, want)
	}
}

// An optional argument left empty still takes its position, so the ones after
// it do not move up into its place.
func TestAnEmptyArgumentKeepsItsPosition(t *testing.T) {
	values := argumentValues([]Argument{{Name: "a"}, {Name: "b"}, {Name: "c"}}, []string{"", "two"})
	if want := []string{"", "two", ""}; !slices.Equal(values, want) {
		t.Errorf("values = %q, want %q", values, want)
	}
}

func TestEncodeURIComponentMatchesJavaScript(t *testing.T) {
	if got, want := encodeURIComponent("split pane/ü?a=1&b!*'()~-_."), "split%20pane%2F%C3%BC%3Fa%3D1%26b!*'()~-_."; got != want {
		t.Errorf("encodeURIComponent() = %q, want %q", got, want)
	}
}

func TestPopupScriptOpensAtItsSizeWithItsArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "deploy.sh", deployHeader+"exit 0\n")
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{}).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	ctx := focusedContext()
	ctx.FocusedPaneCwd = &dir
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	entry.Args = []string{"main", "staging", ""}

	// The palette hands every script over, so the values have to survive the
	// handover to the exec entrypoint.
	if err := Relay(context.Background(), env.Client(), env, entry, ctx); err != nil {
		t.Fatalf("Relay() = %v", err)
	}
	pending, ok := ReadPending(env)
	if !ok {
		t.Fatal("nothing was handed over")
	}
	pending.PID = 0
	if err := RunPending(context.Background(), env, readScriptEntries(t, dir), pending); err != nil {
		t.Fatalf("RunPending() = %v", err)
	}

	var params herdr.PluginPaneOpenParams
	for _, call := range server.Calls() {
		if call.Method == herdr.MethodPluginPaneOpen {
			decode(t, call.Params, &params)
		}
	}
	if params.Width == nil || *params.Width != herdr.PopupSize(manifest.PopupSize{Percent: 80}) {
		t.Errorf("width = %v, want 80%%", params.Width)
	}
	if params.Height == nil || *params.Height != herdr.PopupSize(manifest.PopupSize{Cells: 20}) {
		t.Errorf("height = %v, want 20 cells", params.Height)
	}
	var args []string
	if err := json.Unmarshal([]byte(params.Env[ArgsEnv]), &args); err != nil || !slices.Equal(args, []string{"main", "staging", ""}) {
		t.Errorf("%s = %q, want the values for the run pane", ArgsEnv, params.Env[ArgsEnv])
	}
	for name, want := range map[string]string{"HP_BRANCH": "main", "HP_ENV": "staging", "HP_TOKEN": ""} {
		if got, ok := params.Env[name]; !ok || got != want {
			t.Errorf("%s = %q (set %v), want %q", name, got, ok, want)
		}
	}
}

// A run pane inherits the environment it is opened with, so a command without
// arguments clears the variable rather than leaving one from elsewhere.
func TestACommandWithoutArgumentsClearsThem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	dir := t.TempDir()
	path := writePaletteScript(t, dir, "tool.sh", "#!/bin/sh\n# @palette.mode popup\nexit 0\n")
	server := plugintest.NewServer(t).Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	ctx := focusedContext()
	ctx.FocusedPaneCwd = &dir
	entry := scriptByID(t, readScriptEntries(t, dir), path)
	if err := entry.Run(context.Background(), Exec{Client: env.Client(), Ctx: ctx, Env: env}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	var params herdr.PluginPaneOpenParams
	decode(t, server.Calls()[0].Params, &params)
	if got, ok := params.Env[ArgsEnv]; !ok || got != "" {
		t.Errorf("%s = %q (set %v), want it set empty", ArgsEnv, got, ok)
	}
	if params.Width != nil || params.Height != nil {
		t.Errorf("size = %v x %v, want herdr's default for a script that names none", params.Width, params.Height)
	}
}

func TestAScriptCanAskForSixValues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture")
	}
	var header strings.Builder
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&header, "# @palette.argument%d { \"name\": \"v%d\", \"type\": \"text\" }\n", i, i)
	}
	dir := t.TempDir()
	writePaletteScript(t, dir, "six.sh", "#!/bin/sh\n"+header.String()+"exit 0\n")
	entries, err := ScriptEntries("herdr.palette", []string{dir})
	if err != nil || len(entries) != 1 || len(entries[0].Arguments) != 6 {
		t.Fatalf("entries = %+v, err = %v, want one script asking for six values", entries, err)
	}
	if got := entries[0].Arguments[5].Env; got != "HP_V6" {
		t.Errorf("sixth argument passed in %q, want HP_V6", got)
	}
}
