# `json2yaml`

Convert a JSON document to YAML.

## Usage

```sh
agent-utils json2yaml [file]
```

The inverse of [`yaml2json`](yaml2json.md). The output is block-style YAML that reads back as the same document in YAML 1.1 and 1.2 readers (go-yaml, PyYAML, and Ruby's Psych are checked in tests): key order is kept, numbers keep their digits, and a string never reads back as a number, boolean, or null. A UTF-8 byte order mark at the start of the input is skipped. Every error below exits with code `1`, and nothing is written to `stdout` unless the whole document converts.

## Details

**Numbers.** Every number is written with the digits it has in the input, so nothing is rounded: `0.933333333333333348`, `12345678901234567.5`, and `100000000000000000000` stay as written. A number with an exponent gains a decimal point and a sign on the exponent if it lacks them (`1e3` becomes `1.0e+3`), because YAML 1.1 readers, `yaml2json` included, read `1e3` as a string.

**Strings.** A string is written plain when no reader could take it for anything else: it starts with a letter, one of `_/$^(`, `./`, `../`, or a dot before a letter (`.github`); it is not `yes`, `no`, `on`, `off`, `y`, `n`, `true`, `false`, `null`, `.inf`, or `.nan` in any case; and it has no tab, no `: ` or ` #`, and no trailing space or `:`. Anything else is double-quoted, with escapes for quotes, backslashes, control characters, and the characters YAML 1.1 reads as line breaks (`\u0085`, `\u2028`, `\u2029`). So `"yes"`, `"123"`, `"0.1.0"`, and `"2001-12-14"` stay quoted strings, while `hello world`, `app:1`, and `https://example.com/a#b` are plain. A string of several lines is written as a literal block (`|`), unless a line ends in whitespace, its first non-empty line starts with whitespace, or it holds a carriage return or a character that does not print; then it is double-quoted.

**Keys.** Keys follow the same rules as strings. A key of exactly `<<` is written `!!str "<<"`, since some readers (Ruby's Psych) take even a quoted `<<` key for a merge key, and a key that ends in `<<` is quoted (go-yaml reads it as a merge key otherwise). A key longer than 1,024 characters, the most YAML allows on one line before the `:`, is written in the explicit `? key` form. A string of several lines right after either of these two key forms is double-quoted, as go-yaml cannot read a literal block there.

**Errors.** Invalid JSON is reported with its line and column, and so are a duplicate key in one object (YAML does not allow one), invalid UTF-8, and a `\uXXXX` escape of an unpaired UTF-16 surrogate (such as `\ud800`), which has no UTF-8 form.

**Reading it back with `yaml2json`.** `yaml2json` reads integers past the 64-bit range and floats past the 64-bit range as strings, and floats only to 64-bit precision, so a round trip keeps every value within those ranges.

## Examples

```sh
agent-utils json2yaml config.json
cat config.json | agent-utils json2yaml > config.yaml
cat config.json | agent-utils json2yaml | agent-utils yaml2json   # round trip
```

---

[All commands](../../README.md#commands)
