# `png2jpeg`

Convert a PNG image to JPEG.

## Usage

```sh
agent-utils png2jpeg [-quality n] [input [output]]
```

## Flags

| Flag               | Description                       |
| ------------------ | --------------------------------- |
| `-quality <1-100>` | JPEG quality (default 85).        |
| `-q <1-100>`       | Alias for `-quality`.             |

Takes an optional input file and output file, like [`jpeg2png`](jpeg2png.md). Transparent and semi-transparent pixels are composited onto a white background, since JPEG has no transparency.

## Examples

```sh
agent-utils png2jpeg in.png out.jpg
agent-utils png2jpeg -quality 95 in.png out.jpg
agent-utils png2jpeg < in.png > out.jpg
```

---

[All commands](../../README.md#commands)
