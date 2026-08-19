# agent-utils

> An agent-first toolbox: small, deterministic command-line utilities an AI agent can call instead of doing the work itself, bundled into a single Go binary.

[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8.svg)](https://go.dev)
[![Go Version](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

AI agents constantly need small, exact transformations: decode this base64, validate this JSON, convert this config, generate a UUID. Doing these "by hand" in the model is slow, token-expensive, and probabilistic; a purpose-built tool is instant, free, and exact. `agent-utils` packages those operations behind one consistent, machine-friendly CLI. Humans and shell scripts get the same benefits, but every design decision starts from the question: *what makes this trivially usable by an agent?*

## Agent-first design

- **Discoverable**: `agent-utils commands` emits the full command list as JSON; each command answers `-h`. An agent can learn the whole tool surface in two calls.
- **Deterministic**: a real transformation every time, never a plausible guess. Commands with no inherent randomness (like `lorem`) are deterministic by design.
- **Predictable contract**: every command reads `stdin` (or a file argument), writes results to `stdout`, keeps diagnostics on `stderr`, and exits `0` (success), `1` (runtime failure), or `2` (usage error). No colors, no prompts, no interactivity, ever.
- **Structured output where data is structured**: `jwt-decode`, `yaml2json`, and `commands` emit clean JSON that pipes straight into `jq`.
- **Loud failures**: bad input and misused flags produce a specific error and a non-zero exit code; nothing is silently ignored or truncated.
- **No runtime**: one static Go binary drops into any sandbox or container with no setup, keeping large data out of the model's context window entirely.
- **Local-first**: everything except `video` runs fully offline; nothing is sent anywhere.

## Installation

### Download a release binary

Grab the archive for your platform from the [latest release](https://github.com/AdamAkhlaq/agent-utils/releases/latest) (macOS, Linux, and Windows; amd64 and arm64), unpack it, and put `agent-utils` on your `PATH`. Checksums ship alongside the archives.

### Install with `go install`

**Requirements:** Go 1.26 or newer; install it with `brew install go` or from [go.dev/dl](https://go.dev/dl/).

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
| Encode | [`base64`](#base64) | Base64-encode or -decode data |
| Encode | [`hex`](#hex) | Hex-encode or -decode data |
| Encode | [`url`](#url) | URL-encode or -decode data |
| Encode | [`jwt-decode`](#jwt-decode) | Decode a JWT's header and payload as JSON (no verification) |
| Format | [`json-fmt`](#json-fmt) | Pretty-print, minify, or validate JSON |
| Format | [`json2toml`](#json2toml) | Convert JSON to TOML |
| Format | [`json2yaml`](#json2yaml) | Convert JSON to YAML |
| Format | [`toml2json`](#toml2json) | Convert TOML to JSON |
| Format | [`yaml2json`](#yaml2json) | Convert YAML to JSON |
| Format | [`csv2json`](#csv2json) | Convert CSV with a header row to a JSON array |
| Format | [`json2csv`](#json2csv) | Convert a JSON array of objects to CSV |
| Image | [`qr`](#qr) | Generate or decode QR code PNGs |
| Image | [`png2jpeg`](#png2jpeg) | Convert PNG to JPEG |
| Image | [`jpeg2png`](#jpeg2png) | Convert JPEG to PNG |
| Inspect | [`filetype`](#filetype) | Identify a file's MIME type and image dimensions |
| Inspect | [`hash`](#hash) | Checksum data with sha256/sha1/sha512/md5, or verify one |
| Inspect | [`urlparse`](#urlparse) | Parse a URL into its components as JSON |
| Generate | [`uuid`](#uuid) | Generate random v4 UUIDs |
| Generate | [`password`](#password) | Generate secure random passwords |
| Generate | [`lorem`](#lorem) | Generate deterministic lorem ipsum filler text |
| Text | [`slugify`](#slugify) | Turn text into a lowercase hyphenated slug |
| Text | [`case`](#case) | Convert between snake, camel, pascal, kebab, screaming case |
| Color | [`color`](#color) | Convert a color between hex, rgb, and hsl |
| Time | [`time`](#time) | Print or convert timestamps across formats and timezones |
| Version | [`semver`](#semver) | Sort, compare, or constraint-check SemVer 2.0.0 versions |
| Download | [`video`](#video) | Download a video via yt-dlp |
| Meta | [`commands`](#commands-1) | List every command as JSON for tool discovery |
| Meta | [`version`](#version) | Print the binary's version |

Full documentation for each command follows, in alphabetical order.

### `base64`

Base64-encode or -decode data.

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Like system `base64`, input is encoded byte for byte (`echo` appends a newline; use `printf` to encode an exact string), and newlines in base64 input are ignored when decoding.

```sh
printf "hi" | agent-utils base64      # aGk=
echo "aGk=" | agent-utils base64 -d   # hi
agent-utils base64 photo.png > photo.b64
agent-utils base64 -d photo.b64 > photo.png
```

### `case`

Convert identifier casing: snake, camel, pascal, kebab, or SCREAMING_SNAKE.

| Flag           | Description                                                          |
| -------------- | -------------------------------------------------------------------- |
| `-to <target>` | Target casing: `snake`, `camel`, `pascal`, `kebab`, or `screaming`. Required. |

Words are detected by splitting on runs of non-alphanumeric characters (spaces, `-`, `_`, punctuation) and at camel boundaries: before an upper letter that follows a lower letter or digit, and before the last upper of an all-upper run followed by a lower letter, so acronym runs stay whole (`HTTPServer` is `HTTP` + `Server`). Every word is then lowercased before joining, so acronyms normalize (`HTTPServer` in camel is `httpServer`, not `hTTPServer`); preserving their capitalization would need a dictionary.

```sh
printf "camelCase" | agent-utils case -to snake       # camel_case
printf "snake_case" | agent-utils case -to camel      # snakeCase
printf "kebab-case" | agent-utils case -to pascal     # KebabCase
printf "HTTPServer" | agent-utils case -to snake      # http_server
printf "hello world" | agent-utils case -to screaming # HELLO_WORLD
agent-utils case -to kebab name.txt
```

### `color`

Convert a color between hex, rgb, and hsl: exact conversion math an agent would otherwise approximate. The color is a literal argument or comes from stdin.

| Flag         | Description                                                     |
| ------------ | --------------------------------------------------------------- |
| `-to <form>` | Output form: `hex`, `rgb`, or `hsl`. Required unless `-json`.   |
| `-json`      | Print all three forms as one JSON object (mutually exclusive with `-to`). |

Accepted input forms, case-insensitive: `#rgb` or `#rrggbb` hex (leading `#` optional), `rgb(255, 136, 0)` or the bare triple `255,136,0` with integer components 0-255, and `hsl(30, 100%, 50%)` with an integer hue 0-360 (360 means 0) and integer percentages. Alpha forms (`#rrggbbaa`, `rgba()`, `hsla()`) are rejected with a specific error, never silently stripped. There is no default output form: requiring `-to` keeps the output independent of the input's form, so scripts never guess.

Output is canonical: lowercase `#rrggbb`, `rgb(r, g, b)`, and `hsl(h, s%, l%)` with the hue in 0-359 and integer percentages. The canonical space is 8-bit RGB (hsl input converts to RGB first), and hsl components are rounded to the nearest integer, halves away from zero. That rounding makes hex to hsl lossy for near neighbors: 16.7M RGB values map onto roughly 3.7M rounded HSL triples, so for example `#ff8801` reads back from its hsl form as `#ff8800`. Common colors (primaries, greys, and anything authored as integer hsl) round-trip exactly.

```sh
agent-utils color -to hsl '#ff8800'            # hsl(32, 100%, 50%)
agent-utils color -to hex 'rgb(255, 136, 0)'   # #ff8800
agent-utils color -to rgb 'hsl(32, 100%, 50%)' # rgb(255, 136, 0)
printf '#663399' | agent-utils color -to hsl   # hsl(270, 50%, 40%)
agent-utils color -json '#ff8800' | jq .hsl.h  # 32
```

### `commands`

List every command as JSON: the machine-readable counterpart to the bare `agent-utils` help text, meant for tool discovery by agents and scripts.

```sh
agent-utils commands                          # [{"name": "base64", "summary": "..."}, ...]
agent-utils commands | jq -r '.[].name'       # command names, one per line
```

### `csv2json`

Convert CSV with a header row to a pretty-printed JSON array of objects.

| Flag       | Description                                                    |
| ---------- | -------------------------------------------------------------- |
| `-sep <s>` | Field separator: one character, or `\t` for tab (default `,`). |

The first row is the header row; every later row becomes one object with the headers as keys, in column order. All values remain JSON strings: CSV carries no type information, and inferring types corrupts data (a zip code `01234` must not become the number `1234`). Quoted fields (embedded separators, quotes, newlines) and CRLF line endings are handled per RFC 4180, and duplicate header names are rejected rather than silently overwritten. Output pipes straight into `jq`.

```sh
printf 'name,zip\nJane,01234' | agent-utils csv2json
agent-utils csv2json data.csv | jq -r '.[0].name'
agent-utils csv2json -sep ';' european.csv
printf 'a\tb\n1\t2' | agent-utils csv2json -sep '\t'
```

### `filetype`

Identify a file's MIME type from its content and, for images, its dimensions - so a file can be inspected without reading its contents into context.

| Flag    | Description                                                             |
| ------- | ----------------------------------------------------------------------- |
| `-json` | Print a JSON object with `mime`, `bytes`, and image `width`/`height`.   |

Detection uses the file's magic bytes (the first 512), never the filename, so a mislabelled file is still identified correctly. The whole input is read: the byte count needs it, and image dimensions can sit deep in the file (progressive JPEGs). `width` and `height` appear only for decodable PNG, JPEG, and GIF images; unrecognized content reports `application/octet-stream`.

```sh
agent-utils filetype photo.png                    # image/png
agent-utils filetype -json photo.png              # {"mime": "image/png", "bytes": 340, "width": 256, "height": 256}
curl -s https://example.com/asset | agent-utils filetype
agent-utils filetype -json photo.png | jq -r .mime
```

### `hash`

Compute a checksum of data, or verify one.

| Flag        | Description                                                                           |
| ----------- | ------------------------------------------------------------------------------------- |
| `-a <algo>` | Hash algorithm: `sha256` (default), `sha1`, `sha512`, or `md5`.                       |
| `-c <hex>`  | Verify: compare the digest to this checksum (case-insensitive, whitespace trimmed).   |

Input is streamed through the hasher, so hashing large files is fine. `md5` and `sha1` are provided for integrity checks and content addressing, not for security. Verify mode prints nothing on success and communicates through the exit code (`0` match, `1` mismatch), making it script- and agent-friendly.

```sh
printf "abc" | agent-utils hash                        # ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
agent-utils hash release.tar.gz                        # sha256 of the file
agent-utils hash -a md5 release.tar.gz                 # md5 instead
agent-utils hash -c "$(cut -d' ' -f1 release.sha256)" release.tar.gz && echo "ok"
```

### `hex`

Hex-encode or -decode data.

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding, so piped shell output decodes cleanly.

```sh
printf "hi" | agent-utils hex         # 6869
echo "6869" | agent-utils hex -d      # hi
```

### `jpeg2png`

Convert a JPEG image to PNG.

Takes an optional input file and output file (`jpeg2png in.jpg out.png`); with one argument the PNG goes to `stdout`, with none the JPEG is read from `stdin` too.

```sh
agent-utils jpeg2png photo.jpg photo.png
agent-utils jpeg2png photo.jpg > photo.png
agent-utils jpeg2png < photo.jpg > photo.png
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
cat data.json | agent-utils json-fmt
agent-utils json-fmt -indent 4 data.json
cat data.json | agent-utils json-fmt -c
agent-utils json-fmt -check data.json && echo "valid"
```

### `json2csv`

Convert a JSON array of flat objects to CSV with a header row: the inverse of `csv2json`.

| Flag       | Description                                                    |
| ---------- | -------------------------------------------------------------- |
| `-sep <s>` | Field separator: one character, or `\t` for tab (default `,`). |

Columns are the union of keys across all objects, in first-seen order (the input is decoded token by token, so the document's own key order is preserved). A key missing from an object, or set to `null`, becomes an empty cell. Strings are written as-is (fields with embedded separators, quotes, or newlines are quoted per RFC 4180), numbers keep their exact source form (`1.10` stays `1.10`, large integers never lose precision), and booleans become `true`/`false`. Nested objects and arrays are rejected with the offending element and key rather than silently flattened, as are inputs that are not an array of objects. Note that CSV carries no type information: feeding the output back through `csv2json` yields all-string values.

```sh
printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv
agent-utils json2csv data.json > data.csv
agent-utils json2csv -sep ';' data.json
agent-utils csv2json data.csv | agent-utils json2csv   # round trip
```

### `json2toml`

Convert a JSON document to TOML.

The inverse of `toml2json`. Keys are emitted in sorted order, so the same input always produces identical output. Integers stay integers and floats stay floats (`1.0` becomes `1.0`, never `1`). Values TOML cannot represent fail loudly with exit 1 and the offending path: a non-object root, any `null` (TOML has no null), and integers outside the signed 64-bit range (converting them to floats would silently lose precision). Heterogeneous arrays are fine, TOML 1.0 allows them. Invalid JSON is reported with its line and column.

```sh
agent-utils json2toml config.json
cat config.json | agent-utils json2toml > config.toml
cat config.json | agent-utils json2toml | agent-utils toml2json   # round trip
```

### `json2yaml`

Convert a JSON document to YAML.

The inverse of `yaml2json`. Key order and number representation are preserved, so large integers never lose precision. Strings that would read as YAML booleans or numbers (`"yes"`, `"123"`) stay quoted strings, which makes the conversion round-trip safe: feeding the output back through `yaml2json` reproduces the original document. Invalid input is reported with its line and column.

```sh
agent-utils json2yaml config.json
cat config.json | agent-utils json2yaml > config.yaml
cat config.json | agent-utils json2yaml | agent-utils yaml2json   # round trip
```

### `jwt-decode`

Decode a JWT's header and payload into one pretty-printed JSON document.

This only decodes; it does **not** verify the signature, so never treat the claims as authentic on this basis alone. Claim order and number representation are preserved exactly. A leading `Bearer ` (as pasted from an `Authorization` header) is ignored, and the output is a single JSON object, so it pipes straight into `jq`.

```sh
printf '%s' "$TOKEN" | agent-utils jwt-decode
agent-utils jwt-decode token.txt
printf '%s' "$TOKEN" | agent-utils jwt-decode | jq -r .payload.exp
```

### `lorem`

Generate lorem ipsum filler text.

| Flag     | Description                                |
| -------- | ------------------------------------------ |
| `-w <n>` | Number of words.                           |
| `-p <n>` | Number of paragraphs (default 1).          |

`-w` and `-p` are mutually exclusive. Output is deterministic: the canonical Lorem Ipsum passage, cycled for word counts and repeated for paragraphs, so the same command always produces the same text. This command reads no input.

```sh
agent-utils lorem                 # one canonical paragraph
agent-utils lorem -p 3            # three paragraphs, blank-line separated
agent-utils lorem -w 40           # exactly 40 words
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
agent-utils password
agent-utils password -l 32 -n 5
agent-utils password -no-symbols
```

### `png2jpeg`

Convert a PNG image to JPEG.

| Flag         | Description                        |
| ------------ | ---------------------------------- |
| `-q <1-100>` | JPEG quality (default 85).         |

Takes an optional input file and output file, like `jpeg2png`. Transparent and semi-transparent pixels are composited onto a white background, since JPEG has no transparency.

```sh
agent-utils png2jpeg in.png out.jpg
agent-utils png2jpeg -q 95 in.png out.jpg
agent-utils png2jpeg < in.png > out.jpg
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
agent-utils qr -o qr.png "https://example.com"
agent-utils qr -s 512 "wifi password" > code.png
agent-utils qr -d qr.png                # https://example.com
agent-utils qr -d < screenshot.png
```

### `semver`

Sort, compare, or constraint-check versions per [Semantic Versioning 2.0.0](https://semver.org), with the spec's exact precedence rules (pre-release ordering, numeric vs alphanumeric identifiers, build metadata) instead of a lexical approximation.

| Flag                  | Description                                                                  |
| --------------------- | ---------------------------------------------------------------------------- |
| `-r`                  | Sort in descending order (sort mode only).                                   |
| `-compare`            | Compare two version arguments by precedence: print `-1`, `0`, or `1`.        |
| `-check <constraint>` | Test a version against a constraint: print `true`/`false`, exit `0`/`1`.     |

The three modes are mutually exclusive. By default, versions are read one per line (from `stdin` or a file argument) and sorted ascending by precedence; blank lines are skipped, and any invalid version aborts with its line number and value. Versions with equal precedence (differing only in build metadata or a `v` prefix) are ordered by their original text, byte-wise, so the output is deterministic and independent of input order. `-compare` takes exactly two version arguments; `-check` takes the version as an argument or from `stdin`, and its constraint is space-separated comparators (`=`, `>`, `>=`, `<`, `<=` directly followed by a version), all of which must hold. Ecosystem-specific range syntax (npm's `^`, `~`, `||`) is rejected.

A single leading `v` is accepted on any input version (`v1.2.3`, as in git tags) and preserved in the output. Everything else is strict per spec: partial versions (`1.2`) and leading zeros (`1.02.0`) are rejected with specific errors. Comparison and constraints use pure SemVer precedence, where a pre-release orders below its release: `2.0.0-rc.1` satisfies `<2.0.0`, and `1.2.0-rc.1` does not satisfy `>=1.2.0`.

```sh
git tag | agent-utils semver -r | head -1                       # newest tag by precedence
printf '1.0.0-beta.11\n1.0.0-beta.2\n' | agent-utils semver     # beta.2 before beta.11
agent-utils semver -compare 1.2.3 1.10.0                        # -1
agent-utils semver -check ">=1.2.0 <2.0.0" v1.5.3               # true
if agent-utils semver -check ">=1.28.0" "$(kubectl version -o json | jq -r .serverVersion.gitVersion)" >/dev/null; then
  echo "cluster is new enough"
fi
```

### `slugify`

Turn text into a lowercase hyphenated slug: runs of anything that isn't a letter or digit become single hyphens, with none leading or trailing.

Unicode letters are kept and lowercased, not transliterated (`Café` becomes `café`, not `cafe`).

```sh
printf "Hello, World!" | agent-utils slugify   # hello-world
printf "A -- Messy___Title (2024)" | agent-utils slugify   # a-messy-title-2024
agent-utils slugify title.txt
```

### `time`

Print or convert a timestamp. With no argument, prints the current time; with one, parses it from any accepted form: `now`, epoch seconds or milliseconds (told apart by magnitude), RFC 3339, RFC 1123, or `2006-01-02` with an optional `15:04[:05]`.

| Flag           | Description                                                        |
| -------------- | ------------------------------------------------------------------ |
| `-z <zone>`    | Output timezone: an IANA name, `UTC` (default), or `local`.        |
| `-f <format>`  | Output format: `rfc3339` (default), `unix`, `unix-ms`, `date`, `time`. |
| `-layout <l>`  | Custom Go time layout instead of `-f`.                             |
| `-json`        | Print every representation at once as JSON.                        |

Output defaults to RFC 3339 in UTC, deliberately: local-zone defaults would make output machine-dependent. `-f`, `-layout`, and `-json` are mutually exclusive. The IANA timezone database is embedded in the binary, so zones work identically on every platform, including Windows.

```sh
agent-utils time                              # 2026-08-17T19:45:20Z
agent-utils time 1755459000                   # epoch seconds to RFC 3339
agent-utils time -z Asia/Tokyo 1755459000     # same instant, Tokyo wall clock
agent-utils time -f unix "2026-08-17T19:45:20Z"   # RFC 3339 to epoch
agent-utils time -json now                    # unix, unix_ms, rfc3339, utc, date, time, weekday, zone
```

### `toml2json`

Convert a TOML document to pretty-printed JSON.

Keys are emitted in sorted order: TOML decodes into unordered maps, so sorting is what makes the output deterministic (same input, byte-identical output, always). Integers and floats keep their types, and integers survive exactly up to the full signed 64-bit range. TOML datetimes become JSON strings: offset date-times in RFC 3339, local date-times, dates, and times as their literal text (`1979-05-27T07:32:00`, `1979-05-27`, `07:32:00`). Output pipes straight into `jq` or `json-fmt`. Invalid TOML is reported with its line and column.

```sh
agent-utils toml2json Cargo.toml
cat pyproject.toml | agent-utils toml2json | jq -r .project.name
agent-utils toml2json config.toml | agent-utils json-fmt -c   # minified
```

### `url`

URL-encode or -decode data, with query-component semantics (space becomes `+`).

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding.

```sh
printf "a b&c" | agent-utils url        # a+b%26c
echo "a+b%26c" | agent-utils url -d     # a b&c
```

### `urlparse`

Parse a URL into its components as pretty-printed JSON: exact scheme/host/port/query splitting an agent would otherwise eyeball out of a long string. The URL is a literal argument, or the first line of stdin (trimmed of surrounding whitespace) when no argument is given.

The output schema is fixed: every field is always present, with `""` for absent string components and `{}` for an absent query, so scripts never need existence checks.

| Field | Description |
| ----- | ----------- |
| `scheme` | URL scheme, lowercase (`https`). |
| `opaque` | Scheme-specific part of non-hierarchical URLs (`mailto:a@b.com` puts `a@b.com` here). |
| `user` | Username from the userinfo, percent-decoded. |
| `has_password` | Whether the userinfo carried a password. The password itself is never printed: URLs land in logs and transcripts, and the redaction loses nothing an agent legitimately needs. |
| `host` | Hostname alone: no port, no IPv6 brackets (`[::1]:8080` gives `::1`). |
| `port` | Port as a string, `""` if none; no scheme-default is inferred. |
| `path` | Path, percent-decoded (`/a%20b` gives `/a b`). |
| `raw_query` | Query string exactly as it appeared, undecoded. |
| `query` | Object mapping each key to an array of percent-decoded values, preserving repeated keys in order of appearance. Keys are sorted, so output is deterministic. |
| `fragment` | Fragment without the `#`, percent-decoded. |

Parsing is strict where it matters: an unparseable URL or a malformed percent-escape in the query is an error (exit 1), never a silently dropped component. One `net/url` behavior to know: input without a scheme still parses, but as a bare path - `example.com/x` reports `host: ""` and `path: "example.com/x"`. If you mean a host, say `https://example.com/x`.

```sh
agent-utils urlparse 'https://example.com:8443/a%20b?tag=x&tag=y&q=x%26y#frag'
# {
#   "scheme": "https",
#   "opaque": "",
#   "user": "",
#   "has_password": false,
#   "host": "example.com",
#   "port": "8443",
#   "path": "/a b",
#   "raw_query": "tag=x&tag=y&q=x%26y",
#   "query": {
#     "q": ["x&y"],
#     "tag": ["x", "y"]
#   },
#   "fragment": "frag"
# }
agent-utils urlparse 'https://example.com/search?q=hello+world' | jq -r '.query.q[0]'   # hello world
pbpaste | agent-utils urlparse | jq -r .host
agent-utils urlparse 'https://user:hunter2@example.com/' | jq .has_password             # true, password never printed
```

### `uuid`

Generate random (version 4) UUIDs, one per line.

| Flag         | Description                          |
| ------------ | ------------------------------------ |
| `-n <count>` | How many to generate (default 1).    |

UUIDs are drawn from the operating system's cryptographically secure random source. This command reads no input.

```sh
agent-utils uuid          # e.g. 8b28f3f4-9d51-4a7b-b8a2-52c62c54cbf5
agent-utils uuid -n 5     # five, one per line
```

### `version`

Print the binary's version as a bare, script-friendly value.

```sh
agent-utils version     # v1.2.0
```

### `video`

Download a video by driving [`yt-dlp`](https://github.com/yt-dlp/yt-dlp), streaming its progress output through to the terminal.

| Flag       | Description                                     |
| ---------- | ----------------------------------------------- |
| `-o <dir>` | Output directory (default: current directory).  |
| `-audio`   | Download the audio track only.                  |

Requires `yt-dlp` on your `PATH` (`brew install yt-dlp`); the command fails with a clear message if it's missing. `-audio` extraction, and merging of high-resolution formats, additionally use `ffmpeg` (`brew install ffmpeg`). Unlike the pure transforms above, this command talks to the network.

```sh
agent-utils video "https://www.youtube.com/watch?v=..."
agent-utils video -audio -o ~/Music "https://www.youtube.com/watch?v=..."
```

### `yaml2json`

Convert a YAML document to pretty-printed JSON.

Key order and number representation are preserved, anchors and aliases are resolved, and YAML 1.2 semantics apply (`yes`/`no` stay strings, not booleans). Input with multiple `---` documents is rejected rather than silently truncated. Output pipes straight into `jq` or `json-fmt`.

```sh
agent-utils yaml2json config.yaml
cat config.yaml | agent-utils yaml2json | jq .service.name
agent-utils yaml2json config.yaml | agent-utils json-fmt -c   # minified
```

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
- **Trust the exit code**: `0` means the output is the answer; `1` means the input or operation was bad (the reason is on `stderr`); `2` means the invocation itself was wrong.
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
