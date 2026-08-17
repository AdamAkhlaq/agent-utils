# dev-utils

> Small, fast command-line utilities for everyday development tasks, bundled into a single Go binary that runs entirely on your machine.

[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8.svg)](https://go.dev)
[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`dev-utils` bundles the little utilities developers reach for constantly behind one consistent CLI, so everyday tasks never require a website, a language runtime, or a one-off script.

It is **local-first** (core utilities are pure offline transforms; nothing leaves your machine), ships as **one self-contained binary**, and is designed to be equally usable by a human at a terminal, a shell script, or an AI agent.

## Features

- **Single binary**: written in Go, compiles to one static executable with no runtime or interpreter to install.
- **Local-first**: core utilities run fully offline; nothing is sent anywhere.
- **Composable**: every command reads from `stdin` (or a file argument) and writes to `stdout`, with errors on `stderr` and consistent exit codes (`0` success, `1` failure, `2` usage error), so commands pipe together and slot into scripts.
- **Consistent interface**: predictable subcommand and flag conventions across every utility.
- **Ever-growing**: new utilities are added over time, and the architecture makes adding one a small, self-contained change.

## Installation

**Requirements:** Go 1.26 or newer; install it with `brew install go` or from [go.dev/dl](https://go.dev/dl/).

### Install with `go install`

```sh
go install github.com/adamakhlaq/dev-utils@latest
```

This puts a `dev-utils` binary on your `PATH` (in `$(go env GOPATH)/bin`).

### Build from source

```sh
git clone https://github.com/adamakhlaq/dev-utils.git
cd dev-utils
go build -o dev-utils .
./dev-utils
```

> **Tip:** if you use it interactively a lot, alias it to something shorter, e.g. `alias dv=dev-utils`. (Avoid `du`, which is the standard Unix disk-usage tool.)

## Usage

```sh
dev-utils <command> [flags] [file]
```

Commands read input from the file argument when one is given, otherwise from `stdin`, and write results to `stdout`. Flags go before the filename. Run `dev-utils` with no arguments to list every command.

## Commands

> This project is under active development; the command set below grows with each release.

### `base64`

Base64-encode or -decode data.

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Like system `base64`, input is encoded byte for byte (`echo` appends a newline; use `printf` to encode an exact string), and newlines in base64 input are ignored when decoding.

```sh
printf "hi" | dev-utils base64      # aGk=
echo "aGk=" | dev-utils base64 -d   # hi
dev-utils base64 photo.png > photo.b64
dev-utils base64 -d photo.b64 > photo.png
```

### `hex`

Hex-encode or -decode data.

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding, so piped shell output decodes cleanly.

```sh
printf "hi" | dev-utils hex         # 6869
echo "6869" | dev-utils hex -d      # hi
```

### `jpeg2png`

Convert a JPEG image to PNG.

Takes an optional input file and output file (`jpeg2png in.jpg out.png`); with one argument the PNG goes to `stdout`, with none the JPEG is read from `stdin` too.

```sh
dev-utils jpeg2png photo.jpg photo.png
dev-utils jpeg2png photo.jpg > photo.png
dev-utils jpeg2png < photo.jpg > photo.png
```

### `json-fmt`

Pretty-print, minify, or validate JSON.

| Flag          | Description                                                     |
| ------------- | --------------------------------------------------------------- |
| `-indent <n>` | Spaces per indentation level when pretty-printing (default 2).  |
| `-c`          | Compact (minify) instead of pretty-print.                       |
| `-check`      | Validate only: print nothing, exit `0` if valid and `1` if not. |

Formatting preserves the document exactly as written: key order and number representation are untouched, so large integers never lose precision. Invalid input is reported with its line and column.

```sh
cat data.json | dev-utils json-fmt
dev-utils json-fmt -indent 4 data.json
cat data.json | dev-utils json-fmt -c
dev-utils json-fmt -check data.json && echo "valid"
```

### `jwt-decode`

Decode a JWT's header and payload into one pretty-printed JSON document.

This only decodes; it does **not** verify the signature, so never treat the claims as authentic on this basis alone. Claim order and number representation are preserved exactly. A leading `Bearer ` (as pasted from an `Authorization` header) is ignored, and the output is a single JSON object, so it pipes straight into `jq`.

```sh
printf '%s' "$TOKEN" | dev-utils jwt-decode
dev-utils jwt-decode token.txt
printf '%s' "$TOKEN" | dev-utils jwt-decode | jq -r .payload.exp
```

### `lorem`

Generate lorem ipsum filler text.

| Flag     | Description                                |
| -------- | ------------------------------------------ |
| `-w <n>` | Number of words.                           |
| `-p <n>` | Number of paragraphs (default 1).          |

`-w` and `-p` are mutually exclusive. Output is deterministic: the canonical Lorem Ipsum passage, cycled for word counts and repeated for paragraphs, so the same command always produces the same text. This command reads no input.

```sh
dev-utils lorem                 # one canonical paragraph
dev-utils lorem -p 3            # three paragraphs, blank-line separated
dev-utils lorem -w 40           # exactly 40 words
```

### `password`

Generate random passwords, one per line, from a cryptographically secure random source.

| Flag          | Description                                       |
| ------------- | ------------------------------------------------- |
| `-l <len>`    | Password length (default 20).                     |
| `-n <count>`  | How many to generate (default 1).                 |
| `-no-symbols` | Letters and digits only.                          |

Characters are drawn uniformly (no modulo bias) from letters, digits, and the symbols `!@#$%^&*-_=+?`, a set chosen to paste safely into shells and config files. The default 20 characters with symbols gives roughly 124 bits of entropy. This command reads no input.

```sh
dev-utils password
dev-utils password -l 32 -n 5
dev-utils password -no-symbols
```

### `png2jpeg`

Convert a PNG image to JPEG.

| Flag         | Description                        |
| ------------ | ---------------------------------- |
| `-q <1-100>` | JPEG quality (default 85).         |

Takes an optional input file and output file, like `jpeg2png`. Transparent and semi-transparent pixels are composited onto a white background, since JPEG has no transparency.

```sh
dev-utils png2jpeg in.png out.jpg
dev-utils png2jpeg -q 95 in.png out.jpg
dev-utils png2jpeg < in.png > out.jpg
```

### `qr`

Generate a QR code PNG from text, or decode a QR code image back to its text.

| Flag        | Description                                     |
| ----------- | ----------------------------------------------- |
| `-d`        | Decode a QR code image (PNG or JPEG) to text.   |
| `-o <file>` | Write output to a file instead of `stdout`.     |
| `-s <px>`   | Image size in pixels when encoding (default 256). |

Decoding works best on clean images (screenshots, generated files); expect lower success on photos than a phone camera scanner.

```sh
dev-utils qr -o qr.png "https://example.com"
dev-utils qr -s 512 "wifi password" > code.png
dev-utils qr -d qr.png                # https://example.com
dev-utils qr -d < screenshot.png
```

### `slugify`

Turn text into a lowercase hyphenated slug: runs of anything that isn't a letter or digit become single hyphens, with none leading or trailing.

Unicode letters are kept and lowercased, not transliterated (`Café` becomes `café`, not `cafe`).

```sh
printf "Hello, World!" | dev-utils slugify   # hello-world
printf "A -- Messy___Title (2024)" | dev-utils slugify   # a-messy-title-2024
dev-utils slugify title.txt
```

### `url`

URL-encode or -decode data, with query-component semantics (space becomes `+`).

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding.

```sh
printf "a b&c" | dev-utils url        # a+b%26c
echo "a+b%26c" | dev-utils url -d     # a b&c
```

### `uuid`

Generate random (version 4) UUIDs, one per line.

| Flag         | Description                          |
| ------------ | ------------------------------------ |
| `-n <count>` | How many to generate (default 1).    |

UUIDs are drawn from the operating system's cryptographically secure random source. This command reads no input.

```sh
dev-utils uuid          # e.g. 8b28f3f4-9d51-4a7b-b8a2-52c62c54cbf5
dev-utils uuid -n 5     # five, one per line
```

### `video`

Download a video by driving [`yt-dlp`](https://github.com/yt-dlp/yt-dlp), streaming its progress output through to the terminal.

| Flag       | Description                                     |
| ---------- | ----------------------------------------------- |
| `-o <dir>` | Output directory (default: current directory).  |
| `-audio`   | Download the audio track only.                  |

Requires `yt-dlp` on your `PATH` (`brew install yt-dlp`); the command fails with a clear message if it's missing. `-audio` extraction, and merging of high-resolution formats, additionally use `ffmpeg` (`brew install ffmpeg`). Unlike the pure transforms above, this command talks to the network.

```sh
dev-utils video "https://www.youtube.com/watch?v=..."
dev-utils video -audio -o ~/Music "https://www.youtube.com/watch?v=..."
```

## Scripting and automation

Every command follows the same contract (`stdin` in, `stdout` out, diagnostics on `stderr`, meaningful exit codes), so `dev-utils` composes naturally with the rest of the shell:

```sh
# Pipe anything through a utility
cat photo.png | dev-utils base64 > photo.b64

# Use exit codes for control flow
if dev-utils base64 -d < input.b64 > /dev/null; then
  echo "valid base64"
fi
```

That makes it easy to drop into shell scripts, `Makefile`s, pre-commit hooks, or CI steps.

## Use with AI agents

The same design that makes `dev-utils` script-friendly makes it a clean tool for AI agents to call. An agent can run a deterministic utility instead of attempting the operation itself, which is faster, cheaper, and exact:

- **Discoverable**: running `dev-utils` lists every command and its summary.
- **Deterministic**: a real transformation every time, rather than a probabilistic guess.
- **Predictable I/O and exit codes**: output on `stdout`, diagnostics on `stderr`, and `0`/`1`/`2` to signal what happened.
- **No runtime**: a single static binary drops into any sandbox or container with no setup.

In practice this offloads well-specified work (conversion, encoding, extraction) from the model onto fast local code, keeping large data out of the context window entirely.

## Architecture

A few deliberate choices shape the codebase:

- **`main.go` at the root** keeps a single-binary project simple (the `cmd/` layout is for repositories that build several binaries).
- **Utility logic lives under `internal/`**, using Go's import boundary: nothing outside this module can import these packages, signalling that this is an application rather than a reusable library.
- **One package per category** (`encode`, `format`, `img`, ...). In Go the package is the unit of organisation; files within a package are split purely for readability.
- **Two layers.** Utility logic is a pure, CLI-agnostic core (typically `io.Reader` -> `io.Writer`) with a thin CLI layer for flags and dispatch, so core functions are easy to test and to call directly.

## Contributing

Issues and pull requests are welcome. Before submitting, run:

```sh
go fmt ./... && go vet ./... && go test ./...
```

Keep changes small and focused: one utility or fix per PR, tests alongside the code.

## License

Released under the [MIT License](LICENSE).
