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

Columns are the union of keys across all objects, in first-seen order (the input is decoded token by token, so the document's own key order is preserved). A key missing from an object, or set to `null`, becomes an empty cell. Strings are written as-is (fields with embedded separators, quotes, or newlines are quoted per RFC 4180), numbers keep their exact source form (`1.10` stays `1.10`, large integers never lose precision), and booleans become `true`/`false`. Nested objects and arrays are rejected with the offending element and key rather than silently flattened, as are inputs that are not an array of objects. Note that CSV carries no type information: feeding the output back through [`csv2json`](csv2json.md) yields all-string values (and an empty string for `null` or a missing key); a string comes back exactly as it was, line breaks included. A record that would otherwise be read back wrong has its first field quoted: an empty value or header in a one-column table is written as `""` rather than as a blank line, which CSV readers skip, and a first header that starts with U+FEFF is quoted so it is not taken for a byte order mark. A double quote, CR, LF, or U+FFFD cannot be the `-sep` separator (exit 2). A UTF-8 byte order mark at the start of the input is skipped.

## Examples

```sh
printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv
agent-utils json2csv data.json > data.csv
agent-utils json2csv -sep ';' data.json
agent-utils csv2json data.csv | agent-utils json2csv   # round trip
```

---

[All commands](../../README.md#commands)
