set shell := ["zsh", "-cu"]

plugin_root := justfile_directory()
state_dir := env("XDG_STATE_HOME", env("HOME") / ".local/state") / "herdr/plugins/herdr.palette"

default: check

build:
    ./scripts/build.sh

test:
    go test -race ./...

lint:
    go vet ./...
    golangci-lint run ./...

fmt:
    gofmt -w cmd internal

check: build test lint

# Point herdr at this working tree.
link: build
    herdr plugin link {{plugin_root}}

unlink:
    herdr plugin unlink herdr.palette

# Open the palette without pressing the key.
open: build
    herdr plugin action invoke open --plugin herdr.palette

# Exit codes, stdout and stderr of every plugin command herdr ran.
logs:
    herdr plugin log list

# The recent-command order the palette keeps.
recent:
    cat {{state_dir}}/recent.json
