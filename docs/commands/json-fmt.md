# `json-fmt`

Pretty-print, minify, or validate JSON.

## Usage

```sh
agent-utils json-fmt [-indent n] [file]
agent-utils json-fmt -c [file]
agent-utils json-fmt -check [file]
```

## Flags

| Flag          | Description                                                     |
| ------------- | --------------------------------------------------------------- |
| `-indent <n>` | Spaces per indentation level when pretty-printing (default 2).  |
| `-c`          | Compact (minify) instead of pretty-print.                       |
| `-check`      | Validate only: print nothing, exit `0` if valid and `3` if not. |

Formatting preserves the document exactly as written: key order and number representation are untouched, so large integers never lose precision. Invalid input is reported with its line and column. A UTF-8 byte order mark at the start of the input is skipped, as `jq` does. Input must be UTF-8, as RFC 8259 requires: an invalid byte is reported with its line and column rather than passed through, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding. Either is an invalid document, so `-check` answers no (exit `3`).

## Examples

```sh
cat data.json | agent-utils json-fmt
agent-utils json-fmt -indent 4 data.json
cat data.json | agent-utils json-fmt -c
agent-utils json-fmt -check data.json && echo "valid"
```

---

[All commands](../../README.md#commands)
