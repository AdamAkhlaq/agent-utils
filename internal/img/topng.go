package img

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"io"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

// WebPToPNG converts the WebP in r (lossy or lossless, with or without alpha)
// to a PNG in w. Animated WebP is not supported by the decoder and fails.
func WebPToPNG(w io.Writer, r io.Reader) error {
	return decodeToPNG(w, r, "WebP", webp.Decode)
}

// GIFToPNG converts the GIF in r to a PNG in w. For an animated GIF only the
// first frame is converted.
func GIFToPNG(w io.Writer, r io.Reader) error {
	return decodeToPNG(w, r, "GIF", gif.Decode)
}

// BMPToPNG converts the BMP in r to a PNG in w.
func BMPToPNG(w io.Writer, r io.Reader) error {
	return decodeToPNG(w, r, "BMP", bmp.Decode)
}

// TIFFToPNG converts the TIFF in r to a PNG in w. For a multi-page TIFF only
// the first page is converted.
func TIFFToPNG(w io.Writer, r io.Reader) error {
	return decodeToPNG(w, r, "TIFF", tiff.Decode)
}

func decodeToPNG(w io.Writer, r io.Reader, format string, decode func(io.Reader) (image.Image, error)) error {
	src, err := decode(r)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", format, err)
	}
	if err := png.Encode(w, pngNative(src)); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}
	return nil
}

// pngNative redraws images png.Encode has no direct encoding path for (e.g.
// the YCbCr a lossy WebP decodes to) into NRGBA; the generic fallback path
// would widen them to a 16-bit PNG, doubling the file for 8-bit source data.
func pngNative(src image.Image) image.Image {
	switch src.(type) {
	case *image.Gray, *image.Gray16, *image.RGBA, *image.RGBA64, *image.NRGBA, *image.NRGBA64, *image.Paletted:
		return src
	}
	dst := image.NewNRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}
