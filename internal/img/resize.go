package img

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
)

// ResizeOptions selects the target size for Resize. Exactly one sizing mode
// must be set: Width and/or Height, or Max.
type ResizeOptions struct {
	// Width and Height are the target dimensions in pixels. Either alone
	// derives the other from the source's aspect ratio; both together force
	// exact dimensions. Zero means unset.
	Width, Height int
	// Max scales the image down so its longest side fits within Max pixels;
	// images that already fit are re-encoded unscaled. Zero means unset.
	Max int
	// Quality is the JPEG encode quality (1-100), ignored for PNG output.
	Quality int
}

// Resize reads a PNG or JPEG from r, scales it per opts using the CatmullRom
// resampler, and writes it to w in the same format as the input.
func Resize(w io.Writer, r io.Reader, opts ResizeOptions) error {
	if err := opts.validate(); err != nil {
		return err
	}
	src, format, err := image.Decode(r)
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}
	// Other decoders (e.g. GIF) may be registered elsewhere in the binary, so
	// image.Decode succeeding does not imply the input was PNG or JPEG.
	if format != "png" && format != "jpeg" {
		return fmt.Errorf("unsupported input format %q: want PNG or JPEG", format)
	}

	srcSize := src.Bounds().Size()
	width, height := targetSize(srcSize.X, srcSize.Y, opts)
	out := src
	if width != srcSize.X || height != srcSize.Y {
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		out = dst
	}

	switch format {
	case "png":
		if err := png.Encode(w, out); err != nil {
			return fmt.Errorf("encoding PNG: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(w, out, &jpeg.Options{Quality: opts.Quality}); err != nil {
			return fmt.Errorf("encoding JPEG: %w", err)
		}
	}
	return nil
}

func (opts ResizeOptions) validate() error {
	hasDims := opts.Width > 0 || opts.Height > 0
	hasMax := opts.Max > 0
	if hasDims == hasMax {
		return fmt.Errorf("exactly one sizing mode required: width/height or max")
	}
	if opts.Width < 0 || opts.Height < 0 || opts.Max < 0 {
		return fmt.Errorf("dimensions must be positive")
	}
	if opts.Quality < 1 || opts.Quality > 100 {
		return fmt.Errorf("quality must be between 1 and 100, got %d", opts.Quality)
	}
	return nil
}

func targetSize(srcW, srcH int, opts ResizeOptions) (width, height int) {
	switch {
	case opts.Max > 0:
		if srcW <= opts.Max && srcH <= opts.Max {
			return srcW, srcH
		}
		if srcW >= srcH {
			return opts.Max, scaled(srcH, opts.Max, srcW)
		}
		return scaled(srcW, opts.Max, srcH), opts.Max
	case opts.Width > 0 && opts.Height > 0:
		return opts.Width, opts.Height
	case opts.Width > 0:
		return opts.Width, scaled(srcH, opts.Width, srcW)
	default:
		return scaled(srcW, opts.Height, srcH), opts.Height
	}
}

// scaled returns dim*num/den rounded half up, and at least 1 so extreme
// aspect ratios never produce a zero-pixel side.
func scaled(dim, num, den int) int {
	v := (dim*num + den/2) / den
	if v < 1 {
		return 1
	}
	return v
}
