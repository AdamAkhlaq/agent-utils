# `tiff2png`

Convert a TIFF image to PNG.

## Usage

```sh
agent-utils tiff2png [input [output]]
```

For a multi-page TIFF only the first page is converted. Takes an optional input file and output file (`tiff2png in.tiff out.png`); with one argument the PNG goes to `stdout`, with none the TIFF is read from `stdin` too. An image over 128 megapixels (width x height above 128,000,000) fails with exit `1` before it is decoded, so memory use stays bounded whatever size a file declares. Within the limit, the TIFF decoder can buffer up to 8 bytes per pixel whatever the bit depth, so a large TIFF can take up to about 2.3 GB. A file argument is read in place; from `stdin`, a TIFF whose header follows its pixel data (as Go and some other writers produce) is held in memory once more while it is read.

## Examples

```sh
agent-utils tiff2png scan.tiff scan.png
agent-utils tiff2png scan.tiff > scan.png
agent-utils tiff2png < scan.tiff > scan.png
```

---

[All commands](../../README.md#commands)
