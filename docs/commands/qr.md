# `qr`

Generate a QR code PNG from text, or decode a QR code image back to its text.

## Usage

```sh
agent-utils qr [-s px] [-o file] <text>
agent-utils qr -d [-o file] [image]
```

## Flags

| Flag        | Description                                                          |
| ----------- | -------------------------------------------------------------------- |
| `-d`        | Decode a QR code image (PNG, JPEG, GIF, BMP, TIFF, or WebP) to text. |
| `-o <file>` | Write output to a file instead of `stdout`.                          |
| `-s <px>`   | Image size in pixels when encoding (default 256, at most 11313).     |

Decoding works best on clean images (screenshots, generated files); expect lower success on photos than a phone camera scanner. Light-on-dark codes (as in dark-mode screenshots) are read too. When the image decodes but no code in it can be read, `qr -d` exits `1` with `no readable QR code found in the image`.

Images are limited to 128 megapixels (width x height up to 128,000,000), so memory use stays bounded whatever size a file declares or a flag requests: `-s` above 11313 fails with exit `2`, and with `-d` an image over the limit fails with exit `1` before it is decoded. Within the limit, memory use follows the pixel count, not the file size: up to about 3.6 GB for a progressive CMYK JPEG of 128 megapixels, and about 2 GB for a PNG. A TIFF passed as a file argument is read in place; from `stdin`, a TIFF whose header follows its pixel data is held in memory once more while it is read.

## Examples

```sh
agent-utils qr -o qr.png "https://example.com"
agent-utils qr -s 512 "wifi password" > code.png
agent-utils qr -d qr.png                # https://example.com
agent-utils qr -d < screenshot.png
```

---

[All commands](../../README.md#commands)
