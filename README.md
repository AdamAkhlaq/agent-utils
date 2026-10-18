# agent-utils

> An agent-first toolbox: small, deterministic command-line utilities an AI agent can call instead of doing the work itself, bundled into a single Go binary.

[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8.svg)](https://go.dev)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

AI agents constantly need small, exact transformations: decode this base64, validate this JSON, convert this config, generate a UUID. Doing these "by hand" in the model is slow, token-expensive, and probabilistic; a purpose-built tool is instant, free, and exact. `agent-utils` packages those operations behind one consistent, machine-friendly CLI. Humans and shell scripts get the same benefits, but every design decision starts from the question: *what makes this trivially usable by an agent?*

## Agent-first design

- **Discoverable**: `agent-utils commands` emits the full command list as JSON; each command answers `-h`. An agent can learn the whole tool surface in two calls.
- **Deterministic**: a real transformation every time, never a plausible guess. Commands with no inherent randomness (like `lorem`) are deterministic by design.
- **Predictable contract**: every command reads `stdin` (or a file argument), writes results to `stdout`, keeps diagnostics on `stderr`, and exits `0` (success), `1` (runtime failure), `2` (usage error), or `3` (a yes/no check answered no). No colors, no prompts, no interactivity, ever.
- **Structured output where data is structured**: `jwt-decode`, `yaml2json`, and `commands` emit clean JSON that pipes straight into `jq`.
- **Loud failures**: bad input and misused flags produce a specific error and a non-zero exit code; nothing is silently ignored or truncated.
- **No runtime**: one static Go binary drops into any sandbox or container with no setup, keeping large data out of the model's context window entirely.
- **Local-first**: everything except `video` runs fully offline; nothing is sent anywhere.

## Installation

### Download a release binary

Grab the archive for your platform from the [latest release](https://github.com/AdamAkhlaq/agent-utils/releases/latest) (macOS, Linux, and Windows; amd64 and arm64), unpack it, and put `agent-utils` on your `PATH`. Checksums ship alongside the archives.

### Install with `go install`

**Requirements:** Go 1.25 or newer; install it with `brew install go` or from [go.dev/dl](https://go.dev/dl/).

```sh
go install github.com/adamakhlaq/agent-utils@latest
```

This puts a `agent-utils` binary on your `PATH` (in `$(go env GOPATH)/bin`).

### Build from source

```sh
git clone https://github.com/adamakhlaq/agent-utils.git
cd agent-utils
go build -o agent-utils .
./agent-utils
```

> **Tip:** if you use it interactively a lot, alias it to something shorter, e.g. `alias au=agent-utils`.

## Usage

```sh
agent-utils <command> [flags] [file]
```

Commands read input from the file argument when one is given, otherwise from `stdin`, and write results to `stdout`. Flags go before the filename. Run `agent-utils` with no arguments for a human-readable command list, or `agent-utils commands` for the same list as JSON.

## Commands

> This project is under active development; the command set below grows with each release.

### At a glance

| Category | Command | What it does |
| -------- | ------- | ------------ |
| Encode | [`base64`](docs/commands/base64.md) | Base64-encode or -decode data |
| Encode | [`hex`](docs/commands/hex.md) | Hex-encode or -decode data |
| Encode | [`url`](docs/commands/url.md) | URL-encode or -decode data |
| Encode | [`jwt-decode`](docs/commands/jwt-decode.md) | Decode a JWT's header and payload as JSON (no verification) |
| Format | [`json-fmt`](docs/commands/json-fmt.md) | Pretty-print, minify, or validate JSON |
| Format | [`json2toml`](docs/commands/json2toml.md) | Convert JSON to TOML |
| Format | [`json2yaml`](docs/commands/json2yaml.md) | Convert JSON to YAML |
| Format | [`toml2json`](docs/commands/toml2json.md) | Convert TOML to JSON |
| Format | [`xml2json`](docs/commands/xml2json.md) | Convert XML to JSON |
| Format | [`yaml2json`](docs/commands/yaml2json.md) | Convert YAML to JSON |
| Format | [`csv2json`](docs/commands/csv2json.md) | Convert CSV with a header row to a JSON array |
| Format | [`json2csv`](docs/commands/json2csv.md) | Convert a JSON array of objects to CSV |
| Image | [`qr`](docs/commands/qr.md) | Generate or decode QR code PNGs |
| Image | [`png2jpeg`](docs/commands/png2jpeg.md) | Convert PNG to JPEG |
| Image | [`jpeg2png`](docs/commands/jpeg2png.md) | Convert JPEG to PNG |
| Image | [`img-resize`](docs/commands/img-resize.md) | Resize a PNG or JPEG, keeping its format |
| Image | [`webp2png`](docs/commands/webp2png.md) | Convert WebP to PNG |
| Image | [`gif2png`](docs/commands/gif2png.md) | Convert GIF to PNG (first frame of an animation) |
| Image | [`bmp2png`](docs/commands/bmp2png.md) | Convert BMP to PNG |
| Image | [`tiff2png`](docs/commands/tiff2png.md) | Convert TIFF to PNG (first page of a multi-page file) |
| Inspect | [`cert-decode`](docs/commands/cert-decode.md) | Decode X.509 certificates (PEM or DER) to a JSON array |
| Inspect | [`filetype`](docs/commands/filetype.md) | Identify a file's MIME type and image dimensions |
| Inspect | [`hash`](docs/commands/hash.md) | Checksum data with sha256/sha1/sha512/md5, or verify one |
| Inspect | [`unicode`](docs/commands/unicode.md) | Inspect text codepoints, find invisible characters, normalize NFC/NFD |
| Inspect | [`urlparse`](docs/commands/urlparse.md) | Parse a URL into its components as JSON |
| Generate | [`uuid`](docs/commands/uuid.md) | Generate random v4 UUIDs |
| Generate | [`password`](docs/commands/password.md) | Generate secure random passwords |
| Generate | [`lorem`](docs/commands/lorem.md) | Generate deterministic lorem ipsum filler text |
| Text | [`slugify`](docs/commands/slugify.md) | Turn text into a lowercase hyphenated slug |
| Text | [`case`](docs/commands/case.md) | Convert between snake, camel, pascal, kebab, screaming case |
| Text | [`strip-ansi`](docs/commands/strip-ansi.md) | Remove ANSI escape sequences from text |
| Text | [`md-table`](docs/commands/md-table.md) | Render a JSON array or CSV as a GitHub-flavored Markdown table |
| Color | [`color`](docs/commands/color.md) | Convert a color between hex, rgb, and hsl |
| Network | [`cidr`](docs/commands/cidr.md) | CIDR/subnet math: describe, test, and split prefixes |
| Time | [`time`](docs/commands/time.md) | Print or convert timestamps across formats and timezones |
| Version | [`semver`](docs/commands/semver.md) | Sort, compare, or constraint-check SemVer 2.0.0 versions |
| Download | [`video`](docs/commands/video.md) | Download a video via yt-dlp, printing the saved file path |
| Meta | [`commands`](docs/commands/commands.md) | List every command as JSON for tool discovery |
| Meta | [`version`](docs/commands/version.md) | Print the binary's version |

Each command links to its full reference in [`docs/commands`](docs/commands), and `agent-utils <command> -h` prints its usage, flags, and examples.

## Scripting and automation

Every command follows the same contract (`stdin` in, `stdout` out, diagnostics on `stderr`, meaningful exit codes), so `agent-utils` composes naturally with the rest of the shell:

```sh
# Pipe anything through a utility
cat photo.png | agent-utils base64 > photo.b64

# Use exit codes for control flow
if agent-utils base64 -d < input.b64 > /dev/null; then
  echo "valid base64"
fi
```

That makes it easy to drop into shell scripts, `Makefile`s, pre-commit hooks, or CI steps.

## Wiring it into an agent

Give an agent access to the binary and one line of instruction; the tool teaches itself from there:

```
You have `agent-utils`, a CLI of exact local utilities. Run `agent-utils commands`
to see what it offers, and prefer it over doing transformations yourself.
```

Patterns that work well in practice:

- **Discovery first**: `agent-utils commands` returns JSON the agent can parse; `agent-utils <command> -h` documents flags on demand.
- **Trust the exit code**: `0` means the output is the answer; `1` means the input or operation was bad (the reason is on `stderr`); `2` means the invocation itself was wrong; `3` means a yes/no check (`cidr contains`, `semver -check`, `json-fmt -check`, `hash -c`) answered no.
- **Keep data out of context**: `agent-utils base64 -d blob.b64 > out.png` transforms a file without the agent ever reading its contents.
- **Sandbox-friendly**: one static binary, no runtime, no network (except `video`), so it can be allowlisted broadly in restricted environments.

## Architecture

A few deliberate choices shape the codebase:

- **`main.go` at the root** keeps a single-binary project simple (the `cmd/` layout is for repositories that build several binaries).
- **Utility logic lives under `internal/`**, using Go's import boundary: nothing outside this module can import these packages, signalling that this is an application rather than a reusable library.
- **One package per category** (`encode`, `format`, `img`, ...). In Go the package is the unit of organisation; files within a package are split purely for readability.
- **Two layers.** Utility logic is a pure, CLI-agnostic core (typically `io.Reader` -> `io.Writer`) with a thin CLI layer for flags and dispatch, so core functions are easy to test and to call directly.

## Contributing

Issues and pull requests are welcome. All changes land through pull requests: `main` is protected, and CI (gofmt, vet, tests, build) must pass before merging. Before submitting, run:

```sh
go fmt ./... && go vet ./... && go test ./...
```

Keep changes small and focused: one utility or fix per PR, tests alongside the code, and commit messages in [Conventional Commits](https://www.conventionalcommits.org) style (`feat:`, `fix:`, ...), which the release changelog is generated from.

## Releases

Releases follow [semantic versioning](https://semver.org). Pushing a `vX.Y.Z` tag triggers the release workflow, which cross-compiles the binaries, generates the changelog from commit messages, and publishes a GitHub Release.

## License

Released under the [MIT License](LICENSE).
