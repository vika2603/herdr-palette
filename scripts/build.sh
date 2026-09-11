#!/bin/sh
# Produces bin/palette, the binary both manifest entrypoints run. herdr clones
# the repository and runs this during `herdr plugin install`.
set -eu

cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
	echo "build.sh: no Go toolchain; install Go and retry" >&2
	exit 1
fi

mkdir -p bin
# Through a temporary name: `go build -o` truncates its target first, and a
# keypress during that window finds no binary and fails silently.
go build -o bin/palette.new ./cmd/palette
mv bin/palette.new bin/palette
