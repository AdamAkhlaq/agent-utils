# agent-utils

> Small, exact command-line utilities that AI agents call instead of doing the work in the model, bundled into one static Go binary.

[![CI](https://github.com/AdamAkhlaq/agent-utils/actions/workflows/ci.yml/badge.svg)](https://github.com/AdamAkhlaq/agent-utils/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/AdamAkhlaq/agent-utils)](https://github.com/AdamAkhlaq/agent-utils/releases/latest)
[![Go 1.25+](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Agents constantly need small, exact transformations: decode this JWT, convert this YAML, check whether an IP is in a subnet, find the invisible character that breaks a string comparison. Done "by hand" in the model, these are slow, cost tokens, and are sometimes subtly wrong. `agent-utils` does them instantly and exactly, behind one predictable CLI contract built for agents first and equally usable from scripts and by hand.

## Quick start

```sh
# macOS or Linux, amd64 or arm64: download the latest release into the current directory
curl -fsSL "https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/agent-utils_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xz agent-utils
sudo mv agent-utils /usr/local/bin/
```

```sh
$ openssl s_client -connect example.com:443 -showcerts </dev/null 2>/dev/null \
    | agent-utils cert-decode | jq -c '.[0] | {subject, notAfter, expired}'
{"subject":"CN=example.com","notAfter":"2026-12-25T22:56:35Z","expired":false}

$ printf 'caf\xc3\xa9 \xe2\x80\x8bbar' | agent-utils unicode -check | jq -c '.findings[] | {codepoint, name}'
{"codepoint":"U+200B","name":"ZERO WIDTH SPACE"}

$ agent-utils cidr contains 10.0.0.0/16 10.0.42.1
true

$ agent-utils semver -check '>=1.28.0' v1.27.4; echo "exit $?"
false
agent-utils: semver: v1.27.4 does not satisfy ">=1.28.0"
exit 3
```

Then run `agent-utils help` for the full command list, or `agent-utils <command> -h` for any command's usage, flags, and examples.

## Why agent-utils

**Instead of the model doing it.** A model asked to decode base64, compute a subnet's broadcast address, or convert epoch milliseconds to Tokyo time produces a plausible answer. A tool produces the right one, without spending tokens on the work or pulling a large file into the context window: `agent-utils base64 -d blob.b64 > out.png` never shows the agent a single byte.

**Instead of jq, yq, openssl, date, and friends.** `agent-utils` fills the gaps between the classic tools rather than replacing them; most of its JSON output is meant to be piped into `jq`. What it adds:

- **One contract for every operation.** Input from `stdin` or a file argument, results on `stdout`, diagnostics on `stderr`, and the [same exit codes](#the-contract) everywhere. An agent learns the pattern once instead of learning a dozen tools' flags, output formats, and exit-code conventions.
- **Structured output where the classic tool prints prose.** `cert-decode` emits JSON where `openssl x509 -text` emits a report; `cidr info` emits JSON fields where subnet calculators print tables.
- **Identical everywhere.** One static binary with no runtime dependencies behaves the same on macOS, Linux, and Windows, with none of the GNU-versus-BSD differences in `date`, `sed`, or `stat`, and an embedded timezone database.
- **Strict instead of forgiving.** Ambiguous input fails loudly with a specific message: multi-document YAML, partial SemVer versions, colors with alpha, CSV with duplicate headers. Nothing is silently guessed, defaulted, or truncated.
- **Safe to allowlist.** Every command except `video` runs fully offline and only touches the files you name, so an agent can be given blanket permission to run it.

## Install

### Release binary

Download the archive for your platform from the [latest release](https://github.com/AdamAkhlaq/agent-utils/releases/latest): macOS, Linux, and Windows, each for amd64 and arm64. Archive names are stable (`agent-utils_<os>_<arch>.tar.gz`, `.zip` on Windows), so `releases/latest/download/<name>` always points at the newest build, as in the [quick start](#quick-start).

To verify a download, fetch `checksums.txt` from the same release and check it:

```sh
curl -fsSLO https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/agent-utils_linux_amd64.tar.gz \
     -O https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/checksums.txt
shasum -a 256 --check --ignore-missing checksums.txt   # agent-utils_linux_amd64.tar.gz: OK
```

On macOS, a binary downloaded through a browser is quarantined by Gatekeeper because it is not notarized. Clear the flag with `xattr -d com.apple.quarantine ./agent-utils`; downloads made with `curl` are not affected.

### `go install`

Requires Go 1.25 or newer.

```sh
go install github.com/adamakhlaq/agent-utils/v2@latest
```

This puts an `agent-utils` binary in `$(go env GOPATH)/bin`.

### From source

```sh
git clone https://github.com/AdamAkhlaq/agent-utils.git
cd agent-utils
go build -o agent-utils .
```

## Use it with your agent

The tool documents itself, so an agent only needs to know it exists and that it should prefer it. `agent-utils help` prints the command list in about 45 lines, and `agent-utils <command> -h` prints one command's usage, flags, and examples.

### Claude Code

Add this to your project's `CLAUDE.md` (or `~/.claude/CLAUDE.md` for every project):

```markdown
## agent-utils

`agent-utils` is installed: exact, deterministic CLI utilities for encoding and
decoding, converting JSON/YAML/TOML/XML/CSV, hashes, UUIDs, CIDR and SemVer math,
timestamps, X.509 and JWT decoding, Unicode inspection, and image conversion.
Prefer it over doing these transformations yourself. Run `agent-utils help` for the
command list and `agent-utils <command> -h` for usage. Exit codes: 0 success,
1 failure, 2 usage error, 3 a yes/no check answered no.
```

To let Claude Code run it without asking each time, allow it in `.claude/settings.json` (or `~/.claude/settings.json`):

```json
{
  "permissions": {
    "allow": ["Bash(agent-utils *)"]
  }
}
```

Claude Code checks each command in a pipeline separately, so `agent-utils yaml2json a.yaml | jq .` still needs `jq` to be allowed on its own.

### Codex, Cursor, and other agents

Put the same snippet in `AGENTS.md`, which these agents read. Claude Code reads `AGENTS.md` only when there is no `CLAUDE.md`; to share one file between both, keep the snippet in `AGENTS.md` and add the line `@AGENTS.md` to `CLAUDE.md`.

### Building an integration

`agent-utils commands` emits a JSON manifest of every command with its usage lines, flags (name, type, default, description), and examples: everything needed to generate tool definitions for an agent framework. See [`commands`](docs/commands/commands.md).

## Commands

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
| Meta | [`commands`](docs/commands/commands.md) | List every command with its usage, flags, and examples as JSON |
| Meta | [`version`](docs/commands/version.md) | Print the binary's version |

Each command links to its full reference in [`docs/commands`](docs/commands).

## The contract

Every command follows the same rules, so a caller that has learned one command can drive them all.

- **Input** comes from the file named as the argument, or from `stdin` when there is none. A few commands take literal arguments instead (`cidr`, `color`, `qr`, `semver -compare`, `time`, `urlparse`), and the conversion commands accept an optional output file as a second argument; each command's `-h` shows its exact form.
- **Results** go to `stdout` and nothing else does; diagnostics go to `stderr`. There are no colors, prompts, progress bars, or interactive modes.
- **Structured results are JSON**, pretty-printed and ready for `jq`.
- **Output is deterministic**: the same input gives byte-identical output. The exceptions are the commands whose job is randomness (`uuid`, `password`, drawn from the OS's secure random source) or the present moment (`time` with no argument, and `cert-decode`'s `expired` field).
- **Exit codes** mean the same thing everywhere:

| Code | Meaning |
| ---- | ------- |
| `0`  | Success. `stdout` holds the answer; for a yes/no check, the answer is yes. |
| `1`  | Runtime failure: invalid input data, an unreadable file, a failed download. The reason is on `stderr`. |
| `2`  | Usage error: unknown command, unknown flag, or a malformed argument. |
| `3`  | A yes/no check answered no: `cidr contains`, `cidr overlaps`, `semver -check`, `json-fmt -check`, and `hash -c`. |

`unicode -check` is a report rather than a yes/no check: it exits `0` and its JSON lists what it found.

The exit codes make the commands natural building blocks for shell scripts, `Makefile`s, pre-commit hooks, and CI steps:

```sh
if agent-utils cidr contains 10.0.0.0/8 "$peer_ip" >/dev/null; then
  echo "peer is internal"
fi

agent-utils json-fmt -check config.json || exit 1
```

The examples here and in the docs assume a POSIX shell. On Windows, use Git Bash or WSL, or note that Windows PowerShell 5.1 re-encodes data passed through pipes, so pass binary files as arguments rather than piping them.

### Stability

`agent-utils` follows [Semantic Versioning](https://semver.org) for its CLI contract. Command names, flags, JSON output shapes, and exit-code meanings change only in a major release. New commands, new flags, and new JSON fields arrive in minor releases, so scripts should ignore fields they don't know.

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
