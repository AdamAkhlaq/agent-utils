# `csv2json`

Convert CSV with a header row to a pretty-printed JSON array of objects.

## Usage

```sh
agent-utils csv2json [-sep s] [file]
```

## Flags

| Flag       | Description                                                    |
| ---------- | -------------------------------------------------------------- |
| `-sep <s>` | Field separator: one character, or `\t` for tab (default `,`). |

The first row is the header row; every later row becomes one object with the headers as keys, in column order. All values remain JSON strings: CSV carries no type information, and inferring types corrupts data (a zip code `01234` must not become the number `1234`). Quoted fields (embedded separators, quotes, newlines) and CRLF line endings are handled per RFC 4180, and duplicate header names are rejected rather than silently overwritten. A UTF-8 byte order mark at the start of the input (Excel's "CSV UTF-8" format writes one) is skipped, so it never becomes part of the first header. Output pipes straight into `jq`.

## Examples

```sh
printf 'name,zip\nJane,01234' | agent-utils csv2json
agent-utils csv2json data.csv | jq -r '.[0].name'
agent-utils csv2json -sep ';' european.csv
printf 'a\tb\n1\t2' | agent-utils csv2json -sep '\t'
```

---

[All commands](../../README.md#commands)
