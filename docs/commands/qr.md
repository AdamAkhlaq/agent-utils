# `qr`

Generate a QR code PNG from text, or decode a QR code image back to its text.

## Usage

```sh
agent-utils qr [-s px] [-o file] <text>
agent-utils qr -d [-o file] [image]
```

## Flags

| Flag        | Description                                     |
| ----------- | ----------------------------------------------- |
| `-d`        | Decode a QR code image (PNG or JPEG) to text.   |
| `-o <file>` | Write output to a file instead of `stdout`.     |
| `-s <px>`   | Image size in pixels when encoding (default 256). |

Decoding works best on clean images (screenshots, generated files); expect lower success on photos than a phone camera scanner.

## Examples

```sh
agent-utils qr -o qr.png "https://example.com"
agent-utils qr -s 512 "wifi password" > code.png
agent-utils qr -d qr.png                # https://example.com
agent-utils qr -d < screenshot.png
```

---

[All commands](../../README.md#commands)
