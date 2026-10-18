# `bmp2png`

Convert a BMP image to PNG.

## Usage

```sh
agent-utils bmp2png [input [output]]
```

Takes an optional input file and output file (`bmp2png in.bmp out.png`); with one argument the PNG goes to `stdout`, with none the BMP is read from `stdin` too.

## Examples

```sh
agent-utils bmp2png scan.bmp scan.png
agent-utils bmp2png scan.bmp > scan.png
agent-utils bmp2png < scan.bmp > scan.png
```

---

[All commands](../../README.md#commands)
