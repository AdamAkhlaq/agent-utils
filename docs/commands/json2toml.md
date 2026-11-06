# `json2toml`

Convert a JSON document to TOML.

## Usage

```sh
agent-utils json2toml [file]
```

The inverse of [`toml2json`](toml2json.md). Keys are emitted in sorted order, so the same input always produces identical output. Integers stay integers and floats stay floats (`1.0` becomes `1.0`, never `1`). Values TOML cannot represent fail loudly with exit 1 and the offending path: a non-object root, any `null` (TOML has no null), and integers outside the signed 64-bit range (converting them to floats would silently lose precision). Heterogeneous arrays are fine, TOML 1.0 allows them. Invalid JSON is reported with its line and column. A UTF-8 byte order mark at the start of the input is skipped. Input must be UTF-8: an invalid byte is an error (exit 1) that names its line and column rather than being replaced with U+FFFD, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding. A `\uXXXX` escape of an unpaired UTF-16 surrogate (such as `\ud800`) has no UTF-8 form, so it is an error too (exit 1) that names the line and column of the string that holds it. Output is limited to 64 MiB or 64 times the input size, whichever is larger, because every table header spells out its table's full dotted key (`[a.b.c]`): 60 KB of objects nested 10,000 levels deep would print 100 MB of headers. When the output would be larger, the command fails with exit 1 and an error that names the limit, before anything is written.

## Examples

```sh
agent-utils json2toml config.json
cat config.json | agent-utils json2toml > config.toml
cat config.json | agent-utils json2toml | agent-utils toml2json   # round trip
```

---

[All commands](../../README.md#commands)
