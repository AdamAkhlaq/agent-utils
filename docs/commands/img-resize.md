# `img-resize`

Resize a PNG or JPEG image. The input format is auto-detected and the output keeps the same format.

## Usage

```sh
agent-utils img-resize [-width n] [-height n] [-quality n] [input [output]]
agent-utils img-resize -max n [-quality n] [input [output]]
```

## Flags

| Flag           | Description                                                                        |
| -------------- | ---------------------------------------------------------------------------------- |
| `-width <n>`   | Target width in pixels. Alone, the height follows the aspect ratio.                |
| `-height <n>`  | Target height in pixels. Alone, the width follows the aspect ratio.                |
| `-max <n>`     | Scale down so the longest side fits within `n` pixels. Never upscales.             |
| `-quality <n>` | JPEG quality, 1-100 (default 85, as for `png2jpeg`). Applies when the output is JPEG; ignored for PNG. |

Exactly one sizing mode is required: `-width` and/or `-height`, or `-max`. Giving both `-width` and `-height` forces those exact dimensions (the aspect ratio may change). `-max` only shrinks: an image that already fits is re-encoded at its original size. Scaling uses the Catmull-Rom resampler for high-quality results, and PNG transparency is preserved. Takes an optional input file and output file, like [`jpeg2png`](jpeg2png.md). The output may be the input file, which resizes in place (`agent-utils img-resize -max 512 photo.jpg photo.jpg`); the result replaces it only once resizing has succeeded.

## Examples

```sh
agent-utils img-resize -width 800 photo.jpg thumb.jpg
agent-utils img-resize -max 512 icon.png > icon-small.png
agent-utils img-resize -width 100 -height 100 -quality 90 < in.jpg > avatar.jpg
```

---

[All commands](../../README.md#commands)
