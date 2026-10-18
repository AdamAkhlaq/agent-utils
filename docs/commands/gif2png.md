# `gif2png`

Convert a GIF image to PNG.

## Usage

```sh
agent-utils gif2png [input [output]]
```

For an animated GIF only the first frame is converted; the animation itself is not preserved (PNG is a single-image format). Takes an optional input file and output file (`gif2png in.gif out.png`); with one argument the PNG goes to `stdout`, with none the GIF is read from `stdin` too.

## Examples

```sh
agent-utils gif2png icon.gif icon.png
agent-utils gif2png animation.gif > first-frame.png
agent-utils gif2png < icon.gif > icon.png
```

---

[All commands](../../README.md#commands)
