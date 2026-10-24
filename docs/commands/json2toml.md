# `json2toml`

Convert a JSON document to TOML.

## Usage

```sh
agent-utils json2toml [file]
```

The inverse of [`toml2json`](toml2json.md). Keys are emitted in sorted order, so the same input always produces identical output. Integers stay integers and floats stay floats (`1.0` becomes `1.0`, never `1`). Values TOML cannot represent fail loudly with exit 1 and the offending path: a non-object root, any `null` (TOML has no null), and integers outside the signed 64-bit range (converting them to floats would silently lose precision). Heterogeneous arrays are fine, TOML 1.0 allows them. Invalid JSON is reported with its line and column. A UTF-8 byte order mark at the start of the input is skipped. Input must be UTF-8: an invalid byte is an error (exit 1) that names its line and column rather than being replaced with U+FFFD, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding.

## Examples

```sh
agent-utils json2toml config.json
cat config.json | agent-utils json2toml > config.toml
cat config.json | agent-utils json2toml | agent-utils toml2json   # round trip
```

---

[All commands](../../README.md#commands)
