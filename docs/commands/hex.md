# `hex`

Hex-encode or -decode data.

## Usage

```sh
agent-utils hex [-d] [file]
```

## Flags

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Surrounding whitespace is ignored when decoding, so piped shell output decodes cleanly.

## Examples

```sh
printf "hi" | agent-utils hex         # 6869
echo "6869" | agent-utils hex -d      # hi
```

---

[All commands](../../README.md#commands)
