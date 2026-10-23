# agent-utils

> Small, exact command-line utilities that AI agents call instead of doing the work in the model, bundled into one static Go binary.

[![CI](https://github.com/AdamAkhlaq/agent-utils/actions/workflows/ci.yml/badge.svg)](https://github.com/AdamAkhlaq/agent-utils/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/AdamAkhlaq/agent-utils)](https://github.com/AdamAkhlaq/agent-utils/releases/latest)
[![Go 1.26+](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Agents constantly need small, exact transformations: decode this JWT, convert this YAML, check whether an IP is in a subnet, find the invisible character that breaks a string comparison. Done "by hand" in the model, these are slow, cost tokens, and are sometimes subtly wrong. `agent-utils` does them instantly and exactly, behind one predictable CLI contract built for agents first and equally usable from scripts and by hand.

## Quick start

```sh
# macOS or Linux, amd64 or arm64: download the latest release into the current directory
curl -fsSL "https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/agent-utils_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | tar -xzf - agent-utils
sudo mv agent-utils /usr/local/bin/
```

```sh
$ openssl s_client -connect example.com:443 -showcerts </dev/null 2>/dev/null \
    | agent-utils cert-decode | jq -c '.[0] | {subject, expired}'
{"subject":"CN=example.com","expired":false}

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

**Instead of a throwaway script.** An agent can write `python3 -c 'import base64, sys; ...'`, but that is code to generate and debug, an interpreter that may be missing or a different version, and an improvised error path. `agent-utils base64 -d` costs a few tokens and fails the same way every time.

**Instead of jq, yq, openssl, date, and friends.** `agent-utils` fills the gaps between the classic tools rather than replacing them; most of its JSON output is meant to be piped into `jq`. What it adds:

- **One contract for every operation.** Input from `stdin` or a file argument, results on `stdout`, diagnostics on `stderr`, and the [same exit codes](#the-contract) everywhere. An agent learns the pattern once instead of learning a dozen tools' flags, output formats, and exit-code conventions.
- **Structured output where the classic tool prints prose.** `cert-decode` emits JSON where `openssl x509 -text` emits a report; `cidr info` emits JSON fields where subnet calculators print tables.
- **Identical everywhere.** One static binary with no runtime dependencies behaves the same on macOS, Linux, and Windows, with an embedded timezone database and none of the GNU-versus-BSD differences that trip up scripts (GNU `base64` wraps its output at 76 columns and BSD's doesn't; GNU `date -d` is BSD `date -j -f`).
- **Strict instead of forgiving.** Ambiguous input fails loudly with a specific message: multi-document YAML, partial SemVer versions, colors with alpha, CSV with duplicate headers. Nothing is silently guessed, defaulted, or truncated.
- **Easy to allowlist.** Every command except `video` runs offline and writes only to `stdout` or to an output path you pass explicitly, replacing that file only when the command succeeds. An agent can be given blanket permission to run it.
- **No server.** It is a plain CLI, so it works with any agent that can run a shell command, with no tool schemas taking up the context window.

## Install

### Release binary

Download the archive for your platform from the [latest release](https://github.com/AdamAkhlaq/agent-utils/releases/latest): macOS, Linux, and Windows, each for amd64 and arm64. Archive names are stable (`agent-utils_<os>_<arch>.tar.gz`, `.zip` on Windows), so `releases/latest/download/<name>` always points at the newest build, as in the [quick start](#quick-start).

To verify a download, fetch `checksums.txt` from the same release and check it (on macOS, use `shasum -a 256` in place of `sha256sum`):

```sh
curl -fsSLO https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/agent-utils_linux_amd64.tar.gz \
     -O https://github.com/AdamAkhlaq/agent-utils/releases/latest/download/checksums.txt
sha256sum --check --ignore-missing checksums.txt   # agent-utils_linux_amd64.tar.gz: OK
```

On macOS, a binary downloaded through a browser is quarantined by Gatekeeper because it is not notarized. Clear the flag with `xattr -d com.apple.quarantine ./agent-utils`; downloads made with `curl` are not affected.

### `go install`

Requires Go 1.26 or newer.

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

`agent-utils` is installed: exact, deterministic CLI utilities for
base64/hex/URL encoding, JSON/YAML/TOML/XML/CSV conversion, JSON Schema
validation, Markdown tables, hashes, UUIDs and passwords, exact arithmetic,
CIDR, SemVer, and date math, timestamps, timezones, cron schedules, X.509, JWT,
and URL decoding, Unicode inspection, ANSI stripping, case and slug conversion,
colors, QR codes, image conversion, and shell/JSON/HTML/regex quoting. Prefer it
over doing these transformations yourself. Run `agent-utils help` for the
command list and `agent-utils <command> -h` for usage. Exit codes: 0 success,
1 failure, 2 usage error, 3 a yes/no check answered no.
```

To let Claude Code run it without asking each time, allow it in `.claude/settings.json` (or `~/.claude/settings.json`). The deny rule keeps `video`, the one command that reaches the network, behind a prompt; deny rules take precedence over allow rules.

```json
{
  "permissions": {
    "allow": ["Bash(agent-utils *)"],
    "deny": ["Bash(agent-utils video *)"]
  }
}
```

Claude Code checks each command in a pipeline separately, so `agent-utils yaml2json a.yaml | jq .` still needs `jq` to be allowed on its own, and a redirect such as `> out.png` is checked against your file-editing rules.

### Codex, Cursor, and other agents

Put the same snippet in `AGENTS.md`, which these agents read. Recent versions of Claude Code read `AGENTS.md` too, but only when the project has no `CLAUDE.md`; to share one file, keep the snippet in `AGENTS.md` and add the line `@AGENTS.md` to `CLAUDE.md`.

### Building an integration

`agent-utils commands` emits a JSON manifest of every command with its usage lines, flags (name, JSON Schema type, default, description), and examples, which is a starting point for generating tool definitions for an agent framework. It does not yet say which flags are required or mutually exclusive. See [`commands`](docs/commands/commands.md).

## Commands

| Category | Command | What it does |
| -------- | ------- | ------------ |
| Encode | [`base64`](docs/commands/base64.md) | Base64-encode or -decode data |
| Encode | [`hex`](docs/commands/hex.md) | Hex-encode or -decode data |
| Encode | [`url`](docs/commands/url.md) | URL-encode or -decode data |
| Encode | [`quote`](docs/commands/quote.md) | Quote or escape text for shell, JSON, HTML, or regex, and decode the first three |
| Encode | [`jwt-decode`](docs/commands/jwt-decode.md) | Decode a JWT's header and payload as JSON (no verification) |
| Format | [`json-fmt`](docs/commands/json-fmt.md) | Pretty-print, minify, or validate JSON |
| Format | [`json-schema`](docs/commands/json-schema.md) | Validate JSON against a JSON Schema, reporting every violation as JSON |
| Format | [`json2toml`](docs/commands/json2toml.md) | Convert JSON to TOML |
| Format | [`json2yaml`](docs/commands/json2yaml.md) | Convert JSON to YAML |
| Format | [`toml2json`](docs/commands/toml2json.md) | Convert TOML to JSON |
| Format | [`xml2json`](docs/commands/xml2json.md) | Convert XML to JSON |
| Format | [`yaml2json`](docs/commands/yaml2json.md) | Convert YAML to JSON |
| Format | [`csv2json`](docs/commands/csv2json.md) | Convert CSV with a header row to a JSON array |
| Format | [`json2csv`](docs/commands/json2csv.md) | Convert a JSON array of objects to CSV |
| Image | [`qr`](docs/commands/qr.md) | Generate a QR code PNG, or decode one from a PNG, JPEG, GIF, BMP, TIFF, or WebP image |
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
| Math | [`calc`](docs/commands/calc.md) | Evaluate an arithmetic expression exactly, with no floating point |
| Network | [`cidr`](docs/commands/cidr.md) | CIDR/subnet math: describe, test, and split prefixes |
| Time | [`time`](docs/commands/time.md) | Print or convert timestamps across formats and timezones, add durations, or diff two dates |
| Time | [`cron`](docs/commands/cron.md) | Validate a cron expression and list its next run times |
| SemVer | [`semver`](docs/commands/semver.md) | Sort, compare, or constraint-check SemVer 2.0.0 versions |
| Download | [`video`](docs/commands/video.md) | Download a video via yt-dlp, printing the saved file path |
| Meta | [`commands`](docs/commands/commands.md) | List every command with its usage, flags, and examples as JSON |
| Meta | [`version`](docs/commands/version.md) | Print the binary's version |

Each command links to its full reference in [`docs/commands`](docs/commands).

## The contract

Every command follows the same rules, so a caller that has learned one command can drive them all.

- **Input** comes from the file named as the argument, or from `stdin` when there is none. Some commands take values on the command line instead: `cidr`, `semver -compare`, `qr` (the text to encode), `time`, and `video` (the URL) take only literal values, and `time` never reads `stdin`; `calc`, `color`, `cron`, `urlparse`, and `semver -check` take a literal value or fall back to `stdin`. Each command's `-h` shows its exact form. Commands that read a CSV, JSON, PEM, TOML, XML, or YAML document skip a UTF-8 byte order mark at the very start of their input (an encoding signature some Windows tools write). Everything else keeps it as data: commands that work on raw bytes or characters (`hash`, `base64`, `unicode`, `strip-ansi`) and commands that read a single value (`quote -d`, `urlparse`, `semver`).
- **Output files** are opt-in: the image commands and `strip-ansi` accept an output path as a second argument, and `qr` takes `-o`. The result is written to a temporary file beside it and renamed into place only on success, so a failed command never damages an existing file, and the output may be the input file.
- **Results** go to `stdout` and nothing else does; diagnostics go to `stderr`. There are no colors, prompts, progress bars, or interactive modes.
- **Structured results are JSON**, pretty-printed and ready for `jq`.
- **Output is deterministic**: the same input gives byte-identical output. The exceptions depend on something outside the input by design: randomness (`uuid` and `password`, drawn from the OS's secure random source), the clock (`time` with no argument or `now`, `cron` without `-from`, and `cert-decode`'s `expired` field), the machine (`time -z local` and `cron -z local`), or the network (`video`).
- **Exit codes** mean the same thing everywhere:

| Code | Meaning |
| ---- | ------- |
| `0`  | Success. `stdout` holds the answer; for a yes/no check, the answer is yes. |
| `1`  | Runtime failure: invalid input data (from `stdin`, a file, or a value argument such as a version, prefix, color, URL, or timestamp), an unreadable file, a failed download. The reason is on `stderr`. |
| `2`  | Usage error: unknown command or flag, a bad flag value, or the wrong number of arguments. |
| `3`  | A yes/no check answered no: `cidr contains`, `cidr overlaps`, `semver -check`, `json-fmt -check`, `json-schema`, and `hash -c`. |

`unicode -check` is a report rather than a yes/no check: it exits `0` and its JSON lists what it found. A command stopped by `SIGINT` or `SIGTERM` (Ctrl-C, or an agent's tool timeout) removes any unfinished output file and exits `130` or `143`, the shell convention.

The exit codes make the commands natural building blocks for shell scripts, `Makefile`s, pre-commit hooks, and CI steps:

```sh
if agent-utils cidr contains 10.0.0.0/8 "$peer_ip" >/dev/null; then
  echo "peer is internal"
fi

agent-utils json-fmt -check config.json || exit 1
```

The examples here and in the docs assume a POSIX shell (macOS, Linux, Git Bash, or WSL). Windows PowerShell 5.1 re-encodes pipes and `>` redirects as text, which corrupts binary data: pass files as arguments and use output-file arguments, or use PowerShell 7.4 or newer.

### Stability

`agent-utils` follows [Semantic Versioning](https://semver.org) for its CLI contract. Command names, flags, JSON output shapes, and exit-code meanings change only in a major release. New commands, new flags, and new JSON fields arrive in minor releases, so scripts should ignore fields they don't know.

## Contributing

Bug reports, command proposals, and pull requests are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) covers the development setup, how the code is organized, and the checklist for adding a command. Questions belong in [Discussions](https://github.com/AdamAkhlaq/agent-utils/discussions). To report a vulnerability, follow the [security policy](SECURITY.md); everyone taking part is expected to follow the [code of conduct](CODE_OF_CONDUCT.md).

## License

Released under the [MIT License](LICENSE).
