# `json2csv`

Convert a JSON array of flat objects to CSV with a header row: the inverse of [`csv2json`](csv2json.md).

## Usage

```sh
agent-utils json2csv [-sep s] [file]
```

## Flags

| Flag       | Description                                                    |
| ---------- | -------------------------------------------------------------- |
| `-sep <s>` | Field separator: one character, or `\t` for tab (default `,`). |

Columns are the union of keys across all objects, in first-seen order (the input is decoded token by token, so the document's own key order is preserved). A key missing from an object, or set to `null`, becomes an empty cell. Strings are written as-is (fields with embedded separators, quotes, or newlines are quoted per RFC 4180), numbers keep their exact source form (`1.10` stays `1.10`, large integers never lose precision), and booleans become `true`/`false`. Nested objects and arrays are rejected with the offending element and key rather than silently flattened, as are inputs that are not an array of objects. Note that CSV carries no type information: feeding the output back through [`csv2json`](csv2json.md) yields all-string values (and an empty string for `null` or a missing key); a string comes back exactly as it was, line breaks included. A record that would otherwise be read back wrong has its first field quoted: an empty value or header in a one-column table is written as `""` rather than as a blank line, which CSV readers skip, and the first field of a record is quoted when it starts with U+FEFF, so a first header is not taken for a byte order mark. A double quote, CR, LF, or U+FFFD cannot be the `-sep` separator (exit 2). A UTF-8 byte order mark at the start of the input is skipped. Input must be UTF-8: an invalid byte is an error (exit 1) that names its line and column rather than being replaced with U+FFFD, and input that starts with a UTF-16 or UTF-32 byte order mark is reported as that encoding. A `\uXXXX` escape of an unpaired UTF-16 surrogate (such as `\ud800`) has no UTF-8 form, so it is an error too (exit 1) that names the line and column of the string that holds it. Output is limited to 64 MiB or 64 times the input size, whichever is larger, because every row has a cell for every column, and the columns are every key found in any object: 379 KB of JSON, 30,000 objects that each have their own key, would print 900 MB of empty cells and separators. The output is measured before any of it is written, so input like that fails with exit 1 and an error that names the limit, and nothing goes to `stdout`. Converting objects with different keys separately gives each group its own columns and its own limit.

## Examples

```sh
printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv
agent-utils json2csv data.json > data.csv
agent-utils json2csv -sep ';' data.json
agent-utils csv2json data.csv | agent-utils json2csv   # round trip
```

---

[All commands](../../README.md#commands)
