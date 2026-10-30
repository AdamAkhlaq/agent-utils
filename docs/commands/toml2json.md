# `toml2json`

Convert a TOML document to pretty-printed JSON.

## Usage

```sh
agent-utils toml2json [file]
```

Keys are emitted in sorted order: TOML decodes into unordered maps, so sorting is what makes the output deterministic (same input, byte-identical output, always). Integers and floats keep their types, and integers survive exactly up to the full signed 64-bit range. TOML datetimes become JSON strings: offset date-times in RFC 3339, local date-times, dates, and times as their literal text (`1979-05-27T07:32:00`, `1979-05-27`, `07:32:00`). A UTF-8 byte order mark at the start of the input is skipped. Input must be UTF-8: an invalid byte is an error (exit 1) that names its line and column rather than being replaced with U+FFFD, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding. Output pipes straight into `jq` or `json-fmt`. Invalid TOML is reported with its line and column. Output is limited to 64 MiB or 64 times the input size, whichever is larger, because pretty-printing indents every line by its nesting depth: 20 KB nested 9,999 levels deep would print 200 MB. When the output would be larger, the command fails with exit 1 and an error that names the limit, before anything is written.

## Examples

```sh
agent-utils toml2json Cargo.toml
cat pyproject.toml | agent-utils toml2json | jq -r .project.name
agent-utils toml2json config.toml | agent-utils json-fmt -c   # minified
```

---

[All commands](../../README.md#commands)
