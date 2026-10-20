# `bmp2png`

Convert a BMP image to PNG.

## Usage

```sh
agent-utils bmp2png [input [output]]
```

Takes an optional input file and output file (`bmp2png in.bmp out.png`); with one argument the PNG goes to `stdout`, with none the BMP is read from `stdin` too. An image over 128 megapixels (width x height above 128,000,000) fails with exit `1` before it is decoded, so memory use stays bounded whatever size a file declares. BMP is uncompressed, so within the limit memory use stays in proportion to the file's size.

## Examples

```sh
agent-utils bmp2png scan.bmp scan.png
agent-utils bmp2png scan.bmp > scan.png
agent-utils bmp2png < scan.bmp > scan.png
```

---

[All commands](../../README.md#commands)
