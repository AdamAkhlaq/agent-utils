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

Columns are the union of keys across all objects, in first-seen order (the input is decoded token by token, so the document's own key order is preserved). A key missing from an object, or set to `null`, becomes an empty cell. Strings are written as-is (fields with embedded separators, quotes, or newlines are quoted per RFC 4180), numbers keep their exact source form (`1.10` stays `1.10`, large integers never lose precision), and booleans become `true`/`false`. Nested objects and arrays are rejected with the offending element and key rather than silently flattened, as are inputs that are not an array of objects. Note that CSV carries no type information: feeding the output back through [`csv2json`](csv2json.md) yields all-string values.

## Examples

```sh
printf '[{"name":"Jane","zip":"01234"}]' | agent-utils json2csv
agent-utils json2csv data.json > data.csv
agent-utils json2csv -sep ';' data.json
agent-utils csv2json data.csv | agent-utils json2csv   # round trip
```

---

[All commands](../../README.md#commands)
