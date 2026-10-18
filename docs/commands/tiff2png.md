# `tiff2png`

Convert a TIFF image to PNG.

## Usage

```sh
agent-utils tiff2png [input [output]]
```

For a multi-page TIFF only the first page is converted. Takes an optional input file and output file (`tiff2png in.tiff out.png`); with one argument the PNG goes to `stdout`, with none the TIFF is read from `stdin` too.

## Examples

```sh
agent-utils tiff2png scan.tiff scan.png
agent-utils tiff2png scan.tiff > scan.png
agent-utils tiff2png < scan.tiff > scan.png
```

---

[All commands](../../README.md#commands)
