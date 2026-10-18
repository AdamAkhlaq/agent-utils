# `url`

URL-encode or -decode data, with query-component semantics (space becomes `+`).

## Usage

```sh
agent-utils url [-d] [file]
```

## Flags

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding.

## Examples

```sh
printf "a b&c" | agent-utils url        # a+b%26c
echo "a+b%26c" | agent-utils url -d     # a b&c
```

---

[All commands](../../README.md#commands)
