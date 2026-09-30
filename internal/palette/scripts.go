package palette

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/vika2603/herdr-palette/internal/keys"
)

// ScriptEnv carries a script path to the run pane without composing a shell
// command. The script's interpreter remains responsible for its contents.
const ScriptEnv = "HERDR_PALETTE_SCRIPT"

const scriptHeaderLimit = 4 * 1024

type scriptMetadata struct {
	title string
	mode  string
}

// ScriptEntries reads the configured directories of user scripts. Discovery
// never runs a script and only reads its initial comments, bounded by scriptHeaderLimit.
// A missing directory is normal before the first script has been added; other
// failures leave the valid entries available alongside an explanation.
func ScriptEntries(own string, dirs []string) ([]Entry, error) {
	seen := make(map[string]bool)
	var directories []fs.FileInfo
	var entries []Entry
	var failures []error

nextDirectory:
	for _, dir := range dirs {
		// Paths that differ only in case can still identify one directory.
		// Comparing the few configured directories by file identity avoids
		// rescanning it without assuming a case-insensitive filesystem.
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			for _, previous := range directories {
				if os.SameFile(info, previous) {
					continue nextDirectory
				}
			}
			directories = append(directories, info)
		}
		found, err := scriptDirectory(own, dir, seen)
		entries = append(entries, found...)
		if err != nil {
			failures = append(failures, err)
		}
	}
	// Equal titles from different files remain separate commands. Show their
	// paths in the existing detail column so they can be told apart before
	// running either one, including when the preview is not visible.
	titles := make(map[string]int)
	for _, entry := range entries {
		titles[entry.Title]++
	}
	for i := range entries {
		if titles[entries[i].Title] > 1 {
			entries[i].Detail = joinNames(entries[i].Detail, entries[i].Description)
		}
	}
	return entries, errors.Join(failures...)
}

func scriptDirectory(own, dir string, seen map[string]bool) ([]Entry, error) {
	if dir == "" {
		return nil, nil
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("script directory: %w", err)
	}
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scripts unavailable: %w", err)
	}
	// Windows can report a missing path when ReadDir is given a file. Check
	// its type before treating a missing directory as an optional empty list.
	if !info.IsDir() {
		return nil, fmt.Errorf("scripts unavailable: %s is not a directory", dir)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scripts unavailable: %w", err)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("script directory %s: %w", dir, err)
	}

	var entries []Entry
	var failures []error
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") || file.IsDir() {
			continue
		}
		path := filepath.Join(dir, file.Name())
		realPath := filepath.Join(realDir, file.Name())
		if file.Type()&os.ModeSymlink != 0 {
			realPath, err = filepath.EvalSymlinks(path)
			if err != nil {
				failures = append(failures, fmt.Errorf("script %s: %w", path, err))
				continue
			}
		}
		if seen[realPath] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("script %s: %w", path, err))
			continue
		}
		if !scriptFile(runtime.GOOS, path, info) {
			continue
		}
		seen[realPath] = true
		meta, ok, err := readScript(path, runtime.GOOS)
		if err != nil {
			failures = append(failures, fmt.Errorf("script %s: %w", path, err))
			continue
		}
		if !ok {
			continue
		}
		if meta.title == "" {
			meta.title = strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		}
		entry := entryFor(own, configured{
			id:     "script:" + path,
			title:  meta.title,
			script: path,
			window: herdrWindow(meta.mode),
		})
		entry.Description = path
		entry.Search = path
		entry.Detail = meta.mode
		// A shell script can itself ask herdr to open a pane. It must run
		// after the palette closes, just like a plugin action, rather than
		// fail with ui_busy outside the palette's control.
		entry.AlwaysRelay = true
		entries = append(entries, entry)
	}
	return entries, errors.Join(failures...)
}

func scriptFile(goos, path string, info fs.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	if goos == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ps1", ".cmd", ".bat":
			return true
		}
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

func readScript(path, goos string) (scriptMetadata, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return scriptMetadata{}, false, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, scriptHeaderLimit+1))
	if err != nil {
		return scriptMetadata{}, false, err
	}
	truncated := len(data) > scriptHeaderLimit
	if truncated {
		data = data[:scriptHeaderLimit]
	}
	if goos != "windows" && !strings.HasPrefix(string(data), "#!") {
		return scriptMetadata{}, false, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if goos == "windows" && ext == ".ps1" {
		data, err = powerShellHeader(data)
		if err != nil {
			return scriptMetadata{}, false, err
		}
	}
	meta, err := parseScriptHeader(data, goos == "windows" && (ext == ".cmd" || ext == ".bat"), truncated)
	return meta, true, err
}

// Windows PowerShell's file redirection writes UTF-16. Decode the bounded
// header only; PowerShell still receives and executes the original file.
func powerShellHeader(data []byte) ([]byte, error) {
	if len(data) < 2 {
		return data, nil
	}
	if len(data) >= 4 && (string(data[:4]) == "\xff\xfe\x00\x00" || string(data[:4]) == "\x00\x00\xfe\xff") {
		return nil, errors.New("UTF-32 script headers are not supported; use UTF-8 or UTF-16")
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "\xff\xfe":
		order = binary.LittleEndian
	case "\xfe\xff":
		order = binary.BigEndian
	default:
		return data, nil
	}
	data = data[2:]
	if len(data)%2 != 0 {
		return nil, errors.New("incomplete UTF-16 script header")
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2:])
	}
	return []byte(string(utf16.Decode(units))), nil
}

func parseScriptHeader(data []byte, batch, truncated bool) (scriptMetadata, error) {
	meta := scriptMetadata{mode: keys.TypeShell}
	seen := make(map[string]bool)
	lines := strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		comment, ok := scriptComment(line, batch)
		if line != "" && !ok {
			return meta, nil
		}
		if i == len(lines)-1 && truncated {
			return meta, fmt.Errorf("script header exceeds %d bytes", scriptHeaderLimit)
		}
		field, ok := strings.CutPrefix(strings.TrimSpace(comment), "@palette.")
		if !ok {
			continue
		}
		key, value := field, ""
		if n := strings.IndexAny(field, " \t"); n >= 0 {
			key, value = field[:n], strings.TrimSpace(field[n:])
		}
		if value == "" {
			return meta, fmt.Errorf("@palette.%s needs a value", key)
		}
		if seen[key] {
			return meta, fmt.Errorf("duplicate @palette.%s", key)
		}
		seen[key] = true
		switch key {
		case "title":
			meta.title = value
		case "mode":
			switch value {
			case keys.TypeShell, keys.TypePane, keys.TypePopup:
				meta.mode = value
			default:
				return meta, fmt.Errorf("unknown script mode %q (use shell, pane or popup)", value)
			}
		default:
			return meta, fmt.Errorf("unknown script field @palette.%s", key)
		}
	}
	return meta, nil
}

func scriptComment(line string, batch bool) (string, bool) {
	if !batch {
		return strings.CutPrefix(line, "#")
	}
	if comment, ok := strings.CutPrefix(line, "::"); ok {
		return comment, true
	}
	if len(line) >= 3 && strings.EqualFold(line[:3], "rem") && (len(line) == 3 || line[3] == ' ' || line[3] == '\t') {
		return line[3:], true
	}
	return "", false
}

// ScriptCommand executes a script as a file, preserving its interpreter and
// keeping the path out of shell source. It also rechecks files after discovery,
// since the script may have been removed or replaced while the palette was up.
func ScriptCommand(ctx context.Context, path string) (*exec.Cmd, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("script path must be absolute: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("script: %w", err)
	}
	if !scriptFile(runtime.GOOS, path, info) {
		return nil, fmt.Errorf("%s is not a runnable script", path)
	}
	if _, ok, err := readScript(path, runtime.GOOS); err != nil {
		return nil, fmt.Errorf("script %s: %w", path, err)
	} else if !ok {
		return nil, fmt.Errorf("script %s needs a shebang", path)
	}
	cmd := scriptCommand(ctx, path)
	if cmd.Err != nil {
		return nil, fmt.Errorf("script %s: %w", path, cmd.Err)
	}
	return cmd, nil
}
