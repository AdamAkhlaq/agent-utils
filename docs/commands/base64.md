# `base64`

Base64-encode or -decode data.

## Usage

```sh
agent-utils base64 [-d] [file]
```

## Flags

| Flag | Description               |
| ---- | ------------------------- |
| `-d` | Decode instead of encode. |

Like system `base64`, input is encoded byte for byte (`echo` appends a newline; use `printf` to encode an exact string), and newlines in base64 input are ignored when decoding.

## Examples

```sh
printf "hi" | agent-utils base64      # aGk=
echo "aGk=" | agent-utils base64 -d   # hi
agent-utils base64 photo.png > photo.b64
agent-utils base64 -d photo.b64 > photo.png
```

---

[All commands](../../README.md#commands)
