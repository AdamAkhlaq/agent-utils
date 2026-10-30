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

Pretty-printed output is limited to 64 MiB or 64 times the input size, whichever is larger, because indentation grows with nesting depth: 20 KB of brackets nested 9,999 levels deep would print 200 MB, and 800 MB at `-indent 8`. Over the limit, `json-fmt` exits `1` with an error that names the limit and points to `-c`, before printing anything. The input is checked before it is indented, so malformed input fails on its syntax first. `-c` and `-check` are not limited, since their output is never larger than the input.

## Examples

```sh
cat data.json | agent-utils json-fmt
agent-utils json-fmt -indent 4 data.json
cat data.json | agent-utils json-fmt -c
agent-utils json-fmt -check data.json && echo "valid"
```

---

[All commands](../../README.md#commands)
