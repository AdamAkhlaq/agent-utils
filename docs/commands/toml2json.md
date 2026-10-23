# `toml2json`

Convert a TOML document to pretty-printed JSON.

## Usage

```sh
agent-utils toml2json [file]
```

Keys are emitted in sorted order: TOML decodes into unordered maps, so sorting is what makes the output deterministic (same input, byte-identical output, always). Integers and floats keep their types, and integers survive exactly up to the full signed 64-bit range. TOML datetimes become JSON strings: offset date-times in RFC 3339, local date-times, dates, and times as their literal text (`1979-05-27T07:32:00`, `1979-05-27`, `07:32:00`). A UTF-8 byte order mark at the start of the input is skipped. Output pipes straight into `jq` or `json-fmt`. Invalid TOML is reported with its line and column.

## Examples

```sh
agent-utils toml2json Cargo.toml
cat pyproject.toml | agent-utils toml2json | jq -r .project.name
agent-utils toml2json config.toml | agent-utils json-fmt -c   # minified
```

---

[All commands](../../README.md#commands)
