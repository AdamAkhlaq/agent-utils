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

The input and the output are each limited to 128 megapixels (width x height up to 128,000,000), so memory use stays bounded whatever size a file declares or a flag requests. Within the limit, memory use follows the pixel count, not the file size: up to about 3 GB for a progressive CMYK JPEG of 128 megapixels. An input over the limit fails with exit `1` before it is decoded; `-width` and `-height` values that exceed it on their own fail with exit `2`; a size that exceeds it only once the other side follows the input's aspect ratio fails with exit `1`.

When the fastest resampler would need more than 1 GB of working memory (when the output width times the input height is over about 33 million, or one side is extremely long), resizing switches to one that needs far less: it gives the same result, but resamples 3 to 6 times more slowly (shrinking a 108-megapixel photo to 8000 pixels wide takes about twice as long overall). That resampler's work is the wider of the two widths times the taller of the two heights, and this working area is held to the same 128-megapixel limit, so a resize that widens one side while shrinking the other (a 1000x100000 image to 2000x1000) fails with exit `1` before decoding; a plain downscale or upscale never hits it. A change in size so extreme that even the second resampler would need more than 1 GB (shrinking a 128,000,000-pixel-wide line to a single pixel) also fails with exit `1` before decoding.

## Examples

```sh
agent-utils img-resize -width 800 photo.jpg thumb.jpg
agent-utils img-resize -max 512 icon.png > icon-small.png
agent-utils img-resize -width 100 -height 100 -quality 90 < in.jpg > avatar.jpg
```

---

[All commands](../../README.md#commands)
