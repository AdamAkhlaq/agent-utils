# `jpeg2png`

Convert a JPEG image to PNG.

## Usage

```sh
agent-utils jpeg2png [input [output]]
```

Takes an optional input file and output file (`jpeg2png in.jpg out.png`); with one argument the PNG goes to `stdout`, with none the JPEG is read from `stdin` too. An image over 128 megapixels (width x height above 128,000,000) fails with exit `1` before it is decoded, so memory use stays bounded whatever size a file declares. Within the limit, memory use follows the pixel count, not the file size, and a progressive CMYK JPEG takes the most: a 2 MB one of 128 megapixels takes about 3.4 GB.

## Examples

```sh
agent-utils jpeg2png photo.jpg photo.png
agent-utils jpeg2png photo.jpg > photo.png
agent-utils jpeg2png < photo.jpg > photo.png
```

---

[All commands](../../README.md#commands)
