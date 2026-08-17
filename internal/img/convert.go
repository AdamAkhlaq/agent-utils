package img

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
)

// PNGToJPEG converts the PNG in r to a JPEG in w at the given quality (1-100).
// Transparency is composited onto white first: JPEG has no alpha channel, and
// Go's premultiplied color model would otherwise render transparent pixels
// black.
func PNGToJPEG(w io.Writer, r io.Reader, quality int) error {
	src, err := png.Decode(r)
	if err != nil {
		return fmt.Errorf("decoding PNG: %w", err)
	}
	if err := jpeg.Encode(w, flattenOntoWhite(src), &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encoding JPEG: %w", err)
	}
	return nil
}

// JPEGToPNG converts the JPEG in r to a PNG in w, losslessly.
func JPEGToPNG(w io.Writer, r io.Reader) error {
	src, err := jpeg.Decode(r)
	if err != nil {
		return fmt.Errorf("decoding JPEG: %w", err)
	}
	// jpeg.Decode returns YCbCr (or CMYK), which png.Encode's generic path
	// widens to 16-bit RGB: double the file for 8-bit source data. Converting
	// to RGBA first keeps the PNG 8-bit. Grayscale already encodes as 8-bit.
	if _, ok := src.(*image.Gray); !ok {
		dst := image.NewRGBA(src.Bounds())
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
		src = dst
	}
	if err := png.Encode(w, src); err != nil {
		return fmt.Errorf("encoding PNG: %w", err)
	}
	return nil
}

func flattenOntoWhite(src image.Image) image.Image {
	// Every stdlib decoder returns a type with Opaque(); skipping the copy for
	// opaque images makes the common case allocation-free.
	if op, ok := src.(interface{ Opaque() bool }); ok && op.Opaque() {
		return src
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}
