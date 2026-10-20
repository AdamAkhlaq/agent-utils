# `gif2png`

Convert a GIF image to PNG.

## Usage

```sh
agent-utils gif2png [input [output]]
```

For an animated GIF only the first frame is converted; the animation itself is not preserved (PNG is a single-image format). Takes an optional input file and output file (`gif2png in.gif out.png`); with one argument the PNG goes to `stdout`, with none the GIF is read from `stdin` too. An image whose logical screen is over 128 megapixels (width x height above 128,000,000) fails with exit `1` before it is decoded, so memory use stays bounded whatever size a file declares. Within the limit, a GIF takes about 1 byte per pixel, about 150 MB at 128 megapixels.

## Examples

```sh
agent-utils gif2png icon.gif icon.png
agent-utils gif2png animation.gif > first-frame.png
agent-utils gif2png < icon.gif > icon.png
```

---

[All commands](../../README.md#commands)
