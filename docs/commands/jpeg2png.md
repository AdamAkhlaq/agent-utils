# `jpeg2png`

Convert a JPEG image to PNG.

## Usage

```sh
agent-utils jpeg2png [input [output]]
```

Takes an optional input file and output file (`jpeg2png in.jpg out.png`); with one argument the PNG goes to `stdout`, with none the JPEG is read from `stdin` too.

## Examples

```sh
agent-utils jpeg2png photo.jpg photo.png
agent-utils jpeg2png photo.jpg > photo.png
agent-utils jpeg2png < photo.jpg > photo.png
```

---

[All commands](../../README.md#commands)
