package palette

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/vika2603/herdr-client/herdr"
)

// Argument types a script header can declare, the ones Raycast script
// commands declare.
const (
	ArgumentText     = "text"
	ArgumentPassword = "password"
	ArgumentDropdown = "dropdown"
)

// maxArguments is how many arguments a script can declare, as many as a
// Raycast script command can.
const maxArguments = 3

// ArgEnvPrefix is in front of the name of the environment variable each
// argument's value is passed in, beside its position.
const ArgEnvPrefix = "HP_"

// ArgsEnv carries the values a script runs with to the run pane, as a JSON
// array, so the pane passes them on as positional arguments.
const ArgsEnv = "HERDR_PALETTE_ARGS"

// Argument is a value a script asks for before it runs. The palette collects
// it on the query line and passes it as a positional argument and in the
// environment variable Env.
type Argument struct {
	Name string
	Env  string
	// Placeholder is what the field shows while it is empty.
	Placeholder string
	Type        string
	Optional    bool
	// PercentEncoded passes the value encoded the way encodeURIComponent
	// encodes it, for a script that puts it in a URL.
	PercentEncoded bool
	// Options are what a dropdown picks from: the title is shown, the value
	// passed.
	Options []Choice
	// Command is a shell command line whose output lists a dropdown's options
	// in place of Options, run when the dropdown first gets the focus.
	Command string
}

// Pass is what the script receives for the value entered.
func (a Argument) Pass(value string) string {
	if a.PercentEncoded {
		return encodeURIComponent(value)
	}
	return value
}

// argumentValues is one value per declared argument, as the script receives
// it. A header that gained an argument after the values were collected gets
// an empty value for it rather than shifting the positions of the others.
func argumentValues(arguments []Argument, entered []string) []string {
	values := make([]string, len(arguments))
	for i, argument := range arguments {
		if i < len(entered) {
			values[i] = argument.Pass(entered[i])
		}
	}
	return values
}

// encodeURIComponent leaves the characters JavaScript's function of that name
// leaves and percent-encodes every other byte of the UTF-8 text, so a space
// becomes %20 rather than the + of a query string.
func encodeURIComponent(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || strings.IndexByte("-_.!~*'()", c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

var argumentName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// argumentHeader is the JSON of one @palette.argumentN line. The fields are
// Raycast's, with name added for the environment variable.
type argumentHeader struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Placeholder    string `json:"placeholder"`
	Optional       bool   `json:"optional"`
	PercentEncoded bool   `json:"percentEncoded"`
	Command        string `json:"command"`
	Data           []struct {
		Title *string `json:"title"`
		Value *string `json:"value"`
	} `json:"data"`
}

// parseArgument reads one argument's JSON. Fields it does not know are an
// error, so a misspelled one is reported rather than silently doing nothing.
func parseArgument(text string) (Argument, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	var header argumentHeader
	if err := decoder.Decode(&header); err != nil {
		return Argument{}, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return Argument{}, errors.New("text after the argument's JSON object")
	}

	if !argumentName.MatchString(header.Name) {
		return Argument{}, fmt.Errorf("name %q must start with a letter and hold only letters, digits and _", header.Name)
	}
	argument := Argument{
		Name:           header.Name,
		Env:            ArgEnvPrefix + strings.ToUpper(header.Name),
		Placeholder:    header.Placeholder,
		Type:           header.Type,
		Optional:       header.Optional,
		PercentEncoded: header.PercentEncoded,
		Command:        header.Command,
	}
	if argument.Placeholder == "" {
		argument.Placeholder = argument.Name
	}
	switch header.Type {
	case ArgumentText, ArgumentPassword:
		if header.Data != nil {
			return Argument{}, errors.New("data is only for a dropdown")
		}
		if header.Command != "" {
			return Argument{}, errors.New("command is only for a dropdown")
		}
	case ArgumentDropdown:
		if header.Command != "" {
			if header.Data != nil {
				return Argument{}, errors.New("dropdown takes data or command, not both")
			}
			break
		}
		if len(header.Data) == 0 {
			return Argument{}, errors.New("dropdown needs data or command")
		}
		for i, option := range header.Data {
			if option.Title == nil || *option.Title == "" || option.Value == nil {
				return Argument{}, fmt.Errorf("data item %d needs a title and a value", i+1)
			}
			argument.Options = append(argument.Options, Choice{Title: *option.Title, Value: *option.Value})
		}
	default:
		return Argument{}, fmt.Errorf("type %q is not text, password or dropdown", header.Type)
	}
	return argument, nil
}

// optionsTimeout bounds how long a dropdown's command may take to list its
// options; the popup is waiting on it. A variable so a test need not wait it
// out.
var optionsTimeout = 5 * time.Second

// LoadOptions runs the dropdown's command where a script would run, with the
// same HERDR_ACTIVE_ variables and env added, and reads one option per line
// of its output: the line is both title and value, or title and value
// separated by a tab.
func (a Argument) LoadOptions(ctx context.Context, c *herdr.PluginInvocationContext, env map[string]string) ([]Choice, error) {
	ctx, cancel := context.WithTimeout(ctx, optionsTimeout)
	defer cancel()
	cmd := ShellCommand(ctx, a.Command)
	killTree(cmd)
	// A child that left the process group and kept stdout would otherwise
	// hold the read open after the group is killed.
	cmd.WaitDelay = time.Second
	cmd.Dir = herdr.Value(c.FocusedPaneCwd)
	cmd.Env = cmd.Environ()
	for name, value := range activeEnv(c) {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	for name, value := range env {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%s: options took longer than %s", a.Name, optionsTimeout)
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n"); line != "" {
				return nil, fmt.Errorf("%s: %s", a.Name, line)
			}
		}
		return nil, fmt.Errorf("%s: %w", a.Name, err)
	}
	return parseOptions(out), nil
}

func parseOptions(out []byte) []Choice {
	var options []Choice
	for line := range bytes.Lines(out) {
		text := strings.TrimRight(string(line), "\r\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		title, value, ok := strings.Cut(text, "\t")
		if !ok {
			value = title
		}
		options = append(options, Choice{Title: title, Value: value})
	}
	return options
}

// declaredArguments checks the arguments a header declared, by number, and
// returns them in order: numbered from one without a gap, each passed in an
// environment variable of its own.
func declaredArguments(numbered [maxArguments]*Argument) ([]Argument, error) {
	var arguments []Argument
	owner := make(map[string]int)
	for i, argument := range numbered {
		if argument == nil {
			continue
		}
		if i > 0 && numbered[i-1] == nil {
			return nil, fmt.Errorf("@palette.argument%d needs @palette.argument%d", i+1, i)
		}
		if j, taken := owner[argument.Env]; taken {
			return nil, fmt.Errorf("@palette.argument%d and @palette.argument%d are both passed in %s", j+1, i+1, argument.Env)
		}
		owner[argument.Env] = i
		arguments = append(arguments, *argument)
	}
	return arguments, nil
}
