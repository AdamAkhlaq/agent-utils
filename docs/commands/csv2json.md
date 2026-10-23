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

The first row is the header row; every later row becomes one object with the headers as keys, in column order. All values remain JSON strings: CSV carries no type information, and inferring types corrupts data (a zip code `01234` must not become the number `1234`). Fields are read per RFC 4180: a quoted field keeps every byte between its quotes, so embedded separators and line breaks survive exactly (a CRLF inside quotes stays `\r\n`), and `""` inside quotes stands for one `"`. Records end at LF or CRLF, and empty lines are skipped wherever they appear (a line holding only a space is a record). Malformed CSV is an error (exit 1) that names its line and column: a bare `"` in an unquoted field, anything but a separator or line break after a closing quote, a quote that is never closed, and a carriage return outside quotes that is not part of a CRLF (classic Mac line endings). A record with a different number of fields from the header row is an error that names its line. Duplicate header names are rejected rather than silently overwritten. A double quote, CR, LF, or U+FFFD cannot be the `-sep` separator (exit 2). A UTF-8 byte order mark at the start of the input (Excel's "CSV UTF-8" format writes one) is skipped, so it never becomes part of the first header. Output pipes straight into `jq`.

## Examples

```sh
printf 'name,zip\nJane,01234' | agent-utils csv2json
agent-utils csv2json data.csv | jq -r '.[0].name'
agent-utils csv2json -sep ';' european.csv
printf 'a\tb\n1\t2' | agent-utils csv2json -sep '\t'
```

---

[All commands](../../README.md#commands)
