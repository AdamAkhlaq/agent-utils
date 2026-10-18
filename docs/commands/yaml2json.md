# `yaml2json`

Convert a YAML document to pretty-printed JSON.

## Usage

```sh
agent-utils yaml2json [file]
```

Key order and number representation are preserved, anchors and aliases are resolved, and YAML 1.2 semantics apply (`yes`/`no` stay strings, not booleans). Input with multiple `---` documents is rejected rather than silently truncated. Output pipes straight into `jq` or `json-fmt`.

## Examples

```sh
agent-utils yaml2json config.yaml
cat config.yaml | agent-utils yaml2json | jq .service.name
agent-utils yaml2json config.yaml | agent-utils json-fmt -c   # minified
```

---

[All commands](../../README.md#commands)
