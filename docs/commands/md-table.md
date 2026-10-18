# `md-table`

Render a JSON array or CSV as a GitHub-flavored Markdown pipe table, with cells padded so the pipes align vertically in monospace: exact alignment an agent would otherwise approximate by hand.

## Usage

```sh
agent-utils md-table [-csv] [file]
```

## Flags

| Flag   | Description                                        |
| ------ | -------------------------------------------------- |
| `-csv` | Input is CSV with a header row instead of JSON.    |

The default input is a JSON array of objects: one row per object, with the columns being the union of keys in first-seen order (the first object's key order exactly as written in the source, then keys that only appear in later objects appended in first-encountered order). Missing keys render as empty cells. A JSON array of arrays is also accepted, with the first inner array as the header row; shorter rows are padded with empty cells, and a row wider than the header is an error. Strings render as-is, numbers keep their exact source form (`1.10` stays `1.10`), booleans render as `true`/`false`, and `null` becomes an empty cell. Nested objects and arrays are rejected: table cells are flat. With `-csv`, the first record is the header row, handled per RFC 4180 like [`csv2json`](csv2json.md) (pipe through `csv2json -sep` first for other separators). Empty input and an empty JSON array are errors: a table with no rows has no meaning.

Pipes in cells are escaped as `\|` and newlines become `<br>`, so any cell value survives the trip. Column widths are computed in runes; East Asian wide characters occupy two terminal columns and may still misalign visually (exact display-width handling would need data tables beyond the stdlib).

## Examples

```sh
echo '[{"name":"Jane","age":30},{"name":"Bob","city":"Oslo","age":4}]' | agent-utils md-table
# | name | age | city |
# | ---- | --- | ---- |
# | Jane | 30  |      |
# | Bob  | 4   | Oslo |
agent-utils md-table -csv report.csv
echo '[["metric","value"],["p99","250ms"]]' | agent-utils md-table
agent-utils csv2json -sep ';' european.csv | agent-utils md-table
```

---

[All commands](../../README.md#commands)
