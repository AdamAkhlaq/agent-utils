# dev-utils

> A growing collection of small, fast command-line utilities for everyday development tasks — bundled into a single Go binary that runs entirely on your machine.

[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8.svg)](https://go.dev)
[![Go Version](https://img.shields.io/badge/go-1.24%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`dev-utils` is a personal toolbox of the little utilities I reach for constantly — converting an image, formatting a blob of JSON, generating a QR code or a UUID, pulling down a video — without hunting for a website or wiring up a one-off script each time. Each utility is a small subcommand of one consistent CLI, so the things I use daily live behind a single, predictable command.

It is **local-first** (the core utilities are pure offline transforms — nothing leaves your machine), ships as **one self-contained binary**, and is designed to be equally usable by a human at a terminal, a shell script, or an AI agent.

## Why this project exists

I built `dev-utils` primarily as a way to **learn Go** by solving real problems I run into, rather than working through disconnected exercises. Every utility is an excuse to learn another part of the language and standard library — interfaces, I/O streaming, error handling, table-driven testing, modules, and CLI design — while producing something I actually use afterwards.

That origin shapes the codebase: it leans on the standard library, keeps dependencies minimal, and favours small, idiomatic, well-tested building blocks over cleverness. It is also **open source and continually evolving** — whenever I hit a task I'd rather not do by hand, I add a utility for it.

## Features

- **Single binary** — written in Go, compiles to one static executable with no runtime or interpreter to install.
- **Local-first** — core utilities run fully offline; nothing is sent anywhere.
- **Composable** — every command reads from `stdin` (or a file) and writes to `stdout`, with errors on `stderr` and meaningful exit codes, so commands pipe together and slot into scripts.
- **Consistent interface** — predictable subcommand and flag conventions across every utility.
- **Ever-growing** — new utilities are added over time; the structure makes adding one straightforward.

## Installation

**Requirements:** Go 1.24 or newer (developed on Go 1.26) — install it with `brew install go` (or from [go.dev/dl](https://go.dev/dl/)). A few commands shell out to external tools — see [External dependencies](#external-dependencies).

### Install with `go install`

```sh
go install github.com/<your-username>/dev-utils@latest
```

This puts a `dev-utils` binary on your `PATH` (in `$(go env GOPATH)/bin`).

### Build from source

```sh
git clone https://github.com/<your-username>/dev-utils.git
cd dev-utils
go build -o dev-utils .
./dev-utils --help
```

> **Tip:** the command is `dev-utils`. If you use it interactively a lot, alias it — e.g. `alias dev=dev-utils`.

## Usage

General form:

```sh
dev-utils <command> [flags] [input]
```

Most commands accept a file argument or, if none is given, read from `stdin`, and write the result to `stdout`.

```sh
# Convert an image
dev-utils png2jpeg logo.png logo.jpg

# Pretty-print JSON (or pipe it in)
dev-utils json-fmt response.json
cat response.json | dev-utils json-fmt

# Generate a QR code for a URL
dev-utils qr "https://example.com" -o qr.png

# Generate a UUID
dev-utils uuid

# Download a video
dev-utils video "https://example.com/watch?v=..."
```

List everything with `dev-utils --help`, or get details for one command with `dev-utils <command> --help`.

## Available utilities

The set grows over time. Current and planned categories:

| Category     | Examples                                                       |
| ------------ | -------------------------------------------------------------- |
| **image**    | `png2jpeg`, `jpeg2png`, resize, compress, `svg2png`, QR encode |
| **format**   | `json-fmt`, `json-min`, `json-validate`, JSON ⇄ YAML           |
| **encode**   | base64, URL, hex encode/decode; JWT decode                     |
| **generate** | `uuid`, secure password/token, lorem ipsum                     |
| **text**     | case conversion, slugify, diff, count                          |
| **download** | `video` (and other fetch-and-save helpers)                     |

## Scripting & automation

Because every command follows the `stdin → stdout`, errors-to-`stderr`, exit-code conventions, `dev-utils` composes naturally with the rest of the shell — it's a set of building blocks, not a walled garden:

```sh
# Chain utilities together
cat config.yaml | dev-utils yaml2json | dev-utils json-min > config.min.json

# Use exit codes for control flow
if dev-utils json-validate payload.json; then
  echo "valid"
fi
```

That makes it easy to drop into shell scripts, `Makefile`s, pre-commit hooks, or CI steps.

## Use with AI agents

The same design that makes `dev-utils` script-friendly makes it a clean tool for AI agents to call. An agent can run a deterministic utility instead of attempting the operation itself, which is faster, cheaper, and exact:

- **Discoverable** — `--help` lists every command and its flags.
- **Deterministic** — a real conversion, hash, or UUID every time, rather than a probabilistic guess.
- **Structured output** — commands that return data support `--json` for machine-readable results.
- **Predictable I/O and exit codes** — output on `stdout`, diagnostics on `stderr`, `0`/non-zero to signal success.
- **No runtime** — a single static binary drops into any sandbox or container with no setup.

In practice this offloads well-specified work (format conversion, encoding, extraction) from the model onto fast local code, keeping large data out of the context window entirely.

## A continually evolving toolkit

This repository is intentionally open-ended. New utilities are added whenever a recurring task is worth automating, and the layout is designed so that adding one is a small, self-contained change: a new function in the relevant package plus a registered subcommand and a test.

## Project structure

```
dev-utils/
├── main.go              # entry point: builds the command registry and dispatches
├── go.mod
├── internal/
│   ├── cli/             # command routing, flag parsing, help output
│   ├── image/           # image conversion, resizing, QR encoding
│   ├── format/          # JSON / YAML formatting and validation
│   ├── encode/          # base64, URL, hex, JWT
│   ├── generate/        # UUIDs, passwords, lorem ipsum
│   ├── text/            # case conversion, slugify, diff
│   └── download/        # video and other fetch utilities (shell out to system tools)
└── README.md
```

A few deliberate choices:

- **`main.go` at the root** keeps a single-binary project simple (the `cmd/` layout is for repositories that build several binaries).
- **`internal/`** uses Go's import-boundary feature: nothing outside this module can import these packages, signalling that this is an application rather than a reusable library.
- **One package per category.** In Go the _package_ is the unit of organisation; files within a package are split purely for readability.
- Utility logic is written against `io.Reader` / `io.Writer` and kept independent of the CLI layer, so the core functions are easy to test and to call directly.

## External dependencies

Most commands are pure Go and need nothing extra. A few delegate to mature system tools and will report a clear error if the tool isn't installed:

- **`video`** — requires `yt-dlp` (and `ffmpeg` for some formats): `brew install yt-dlp ffmpeg`
- **`svg2png`** — requires an SVG renderer: `brew install resvg`

## License

Released under the [MIT License](LICENSE).
