#!/bin/sh
# setup.sh — one command that prepares everything needed to test this city locally.
#
#   ./setup.sh
#
# Run this FIRST in a fresh environment (new machine, new container, new
# session). Nothing else has to be done before it. When it finishes,
#
#   robocity-sim run main.go
#
# runs your controller against the real game engine straight away.
#
# What it does:
#   1. installs the SimCode local test tooling (the `robocity-sim` CLI, which
#      carries the Go client library) if it isn't installed already;
#   2. makes sure `robocity-sim` is on your PATH;
#   3. runs a 1-tick local test, which downloads + caches the game engine under
#      ~/.cache/simcode/ and proves the whole setup actually works.
#
# It is idempotent and fast to re-run: if the tool is already installed it does
# not reinstall anything, and with the engine already cached the check takes a
# moment. If setup cannot complete it stops and says what was missing.
#
# This is LOCAL TESTING ONLY. It does not deploy anything — deploying is a
# separate action (commit + push to the default branch, then resync the city).

set -eu

TOOLS_PKG="github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest"
ORIG_PATH="$PATH"   # your shell's PATH, before this script adds anything to its own

say()  { printf '%s\n' "$*"; }
fail() { printf 'setup.sh: %s\n' "$*" >&2; exit 1; }

here=$(dirname -- "$0")
cd -- "$here"

# Append one line to a file WITHOUT ever welding onto its last line: a file
# that doesn't end in a newline is common, and `foo >> file` would produce
# `export EDITOR=vimexport PATH=...` — silently changing the user's variable.
append_line() {
	file=$1
	text=$2
	# Command substitution strips trailing newlines, so a non-empty result
	# means the last byte is NOT a newline.
	if [ -s "$file" ] && [ -n "$(tail -c 1 -- "$file" 2>/dev/null)" ]; then
		printf '\n' >>"$file"
	fi
	printf '%s\n' "$text" >>"$file"
}

# Put a directory on PATH for FUTURE shells, so the next session (or the next
# command an assistant runs, which is usually a brand-new shell) just works.
# This is the only thing this script writes outside the repo; it touches only
# shell startup files, one line each, at most once, and names every file it
# changed. The three files are not redundant: a login sh/bash reads ~/.profile,
# an interactive bash reads ~/.bashrc, zsh reads neither.
persist_on_path() {
	dir=$1
	line="export PATH=\"$dir:\$PATH\"  # added by SimCode setup.sh"
	touched=0
	for rc in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.zshrc"; do
		[ -f "$rc" ] || continue
		if grep -Fq "$dir" "$rc" 2>/dev/null; then
			touched=1        # already there — nothing to do
			continue
		fi
		append_line "$rc" "$line" || continue
		say "PATH:     added $dir to $rc (for future shells)"
		touched=1
	done
	if [ "$touched" -eq 0 ]; then
		append_line "$HOME/.profile" "$line" || return 0
		say "PATH:     added $dir to $HOME/.profile (for future shells)"
	fi
}

# Where `go install` puts binaries — used both to find an already-installed
# tool and to recover after installing.
go_bin_dir() {
	gobin=$(go env GOBIN 2>/dev/null || true)
	if [ -z "$gobin" ]; then
		gopath=$(go env GOPATH 2>/dev/null || true)
		if [ -n "$gopath" ]; then gobin="$gopath/bin"; fi
	fi
	printf '%s\n' "$gobin"
}

# --- 1. the Go toolchain --------------------------------------------------
# Checked even when the tool is already installed: `robocity-sim run main.go`
# compiles your controller, so it needs `go` on PATH every time. Without this
# check a missing toolchain would surface as a confusing engine warning below.
command -v go >/dev/null 2>&1 ||
	fail "go not found on PATH. Install Go 1.22+ (https://go.dev/dl/) — or, if it is
  installed, add its bin directory to PATH (commonly /usr/local/go/bin) — and
  re-run ./setup.sh"

# --- 2. the tooling -------------------------------------------------------
toolbin=""
if command -v robocity-sim >/dev/null 2>&1; then
	say "tooling:  already installed ($(command -v robocity-sim)) — skipping install"
else
	# Cheap probe of Go's bin directory BEFORE deciding to reinstall: re-running
	# in the same shell that couldn't see the tool must not pay for a full
	# download + cgo build.
	gobin=$(go_bin_dir)
	if [ -n "$gobin" ] && [ -x "$gobin/robocity-sim" ]; then
		toolbin=$gobin
		PATH="$toolbin:$PATH"
		export PATH
		say "tooling:  already installed ($toolbin/robocity-sim) — skipping install"
		say "PATH:     $toolbin is not on your PATH"
		persist_on_path "$toolbin"
	else
		say "tooling:  robocity-sim not found — installing $TOOLS_PKG"

		# The tool loads the engine over a cgo bridge, so it needs a C compiler.
		cc=${CC:-}
		if [ -z "$cc" ]; then
			for candidate in cc gcc clang; do
				if command -v "$candidate" >/dev/null 2>&1; then cc=$candidate; break; fi
			done
		fi
		[ -n "$cc" ] ||
			fail "no C compiler found (looked for cc, gcc, clang). robocity-sim needs CGO,
  so install one first:
      Debian/Ubuntu:  sudo apt-get install build-essential
      macOS:          xcode-select --install
  then re-run ./setup.sh"

		CGO_ENABLED=1 go install "$TOOLS_PKG" ||
			fail "go install $TOOLS_PKG failed — the output above names the real cause
  (no network, CGO disabled, missing C toolchain, ...)."

		if ! command -v robocity-sim >/dev/null 2>&1; then
			# Installed, but Go's bin directory is not on PATH.
			toolbin=$(go_bin_dir)
			{ [ -n "$toolbin" ] && [ -x "$toolbin/robocity-sim" ]; } ||
				fail "go install reported success but robocity-sim was not found. Add
  \"\$(go env GOPATH)/bin\" to PATH and re-run ./setup.sh"
			PATH="$toolbin:$PATH"
			export PATH
			say "PATH:     robocity-sim installed to $toolbin, which is not on your PATH"
			persist_on_path "$toolbin"
		fi
		say "tooling:  installed ($(command -v robocity-sim))"
	fi
fi

# --- 3. how YOUR shell will invoke it -------------------------------------
# The PATH this script built applies to this script only. Everything printed
# below must be a command the user can actually paste into their own shell.
toolpath=$(command -v robocity-sim)
if (PATH="$ORIG_PATH"; export PATH; command -v robocity-sim >/dev/null 2>&1); then
	on_path=1
	invoke="robocity-sim"
else
	on_path=0
	invoke="$toolpath"
	[ -n "$toolbin" ] || toolbin=$(dirname -- "$toolpath")
fi

# --- 4. the engine + a real 1-tick test -----------------------------------
# The engine is downloaded from the server on first use and cached under
# ~/.cache/simcode/, so this both warms the cache and verifies the install.
# --canonical on purpose: setup runs BEFORE this repo is linked to a city, and a
# run refuses to guess a world it cannot resolve (it will not silently substitute
# one). Your real runs, from a linked repo, use your city's world with no flag.
say "engine:   running a 1-tick local test to warm the engine cache..."
if robocity-sim run main.go --canonical --ticks 1 >/dev/null 2>&1; then
	say "engine:   ready — local runs start instantly from now on"
else
	say "engine:   WARNING: the warm-up run did not complete."
	say "          Setup itself is done; this usually means the server is"
	say "          unreachable (offline), and the engine will be downloaded on"
	say "          your first real run. To see the actual error, run it directly:"
	say "              $invoke run main.go --canonical --ticks 1"
fi

# --- 5. what to do next ---------------------------------------------------
say ""
if [ "$on_path" -eq 1 ]; then
	say "Setup complete. Test your controller with:"
	say "    robocity-sim run main.go"
else
	say "Setup complete — but robocity-sim is NOT on this shell's PATH yet."
	say "Run this once in your current shell (it is already saved for new shells):"
	say "    export PATH=\"$toolbin:\$PATH\""
	say "Then test your controller with:"
	say "    robocity-sim run main.go"
	say "Or call it by full path right now:"
	say "    $invoke run main.go"
fi
