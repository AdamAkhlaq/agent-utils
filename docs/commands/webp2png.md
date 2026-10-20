# `webp2png`

Convert a WebP image to PNG.

## Usage

```sh
agent-utils webp2png [input [output]]
```

Handles lossy and lossless WebP, with or without transparency (alpha is preserved). Animated WebP is not supported and fails with exit `1`. The reverse direction (`png2webp`) is deliberately not offered: there is no maintained pure-Go WebP encoder, and a cgo one would break the single static binary. Takes an optional input file and output file (`webp2png in.webp out.png`); with one argument the PNG goes to `stdout`, with none the WebP is read from `stdin` too. An image over 128 megapixels (width x height above 128,000,000) fails with exit `1` before it is decoded, so memory use stays bounded whatever size a file declares. Within the limit, memory use follows the pixel count, not the file size: a WebP of a few kilobytes at 128 megapixels can take up to about 1.4 GB.

## Examples

```sh
agent-utils webp2png photo.webp photo.png
agent-utils webp2png photo.webp > photo.png
agent-utils webp2png < photo.webp > photo.png
```

---

[All commands](../../README.md#commands)
