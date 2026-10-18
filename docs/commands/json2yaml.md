# `json2yaml`

Convert a JSON document to YAML.

## Usage

```sh
agent-utils json2yaml [file]
```

The inverse of [`yaml2json`](yaml2json.md). Key order and number representation are preserved, so large integers never lose precision. Strings that would read as YAML booleans or numbers (`"yes"`, `"123"`) stay quoted strings, which makes the conversion round-trip safe: feeding the output back through [`yaml2json`](yaml2json.md) reproduces the original document. Invalid input is reported with its line and column.

## Examples

```sh
agent-utils json2yaml config.json
cat config.json | agent-utils json2yaml > config.yaml
cat config.json | agent-utils json2yaml | agent-utils yaml2json   # round trip
```

---

[All commands](../../README.md#commands)
