package img

import (
	"bufio"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
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
	in := bufio.NewReader(r)
	// image.DecodeConfig would read a TIFF as far as its header, which may be
	// at the end of the file, only for it to be rejected below.
	if header, _ := in.Peek(4); isTIFF(header) {
		return fmt.Errorf("unsupported input format %q: want PNG or JPEG", "tiff")
	}
	cfg, format, r, err := peekConfigAny(in)
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}
	if format != "png" && format != "jpeg" {
		return fmt.Errorf("unsupported input format %q: want PNG or JPEG", format)
	}
	if err := checkPixels("image", cfg.Width, cfg.Height); err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}
	// The JPEG decoder accepts a zero height, which targetSize would divide by.
	if cfg.Width == 0 || cfg.Height == 0 {
		return fmt.Errorf("decoding image: image is %dx%d pixels, so there is nothing to resize", cfg.Width, cfg.Height)
	}
	width, height := targetSize(cfg.Width, cfg.Height, opts)
	if err := checkPixels("resized image", width, height); err != nil {
		return err
	}
	scaling := width != cfg.Width || height != cfg.Height
	useScale := scaleMemory(cfg.Width, cfg.Height, width, height) <= maxResampleMemory
	if scaling && !useScale {
		if transformMemory(cfg.Width, cfg.Height, width, height) > maxResampleMemory {
			return fmt.Errorf("resizing %dx%d to %dx%d: the change in size is too extreme to resample within %d bytes of working memory", cfg.Width, cfg.Height, width, height, maxResampleMemory)
		}
		// Transform's work grows with the wider of the two widths times the
		// taller of the two heights, which only a resize that widens one side
		// while shrinking the other can push past either image's size.
		if err := checkPixels("working area", max(width, cfg.Width), max(height, cfg.Height)); err != nil {
			return fmt.Errorf("resizing %dx%d to %dx%d: %w", cfg.Width, cfg.Height, width, height, err)
		}
	}
	src, _, err := image.Decode(r)
	if err != nil {
		return fmt.Errorf("decoding image: %w", err)
	}

	out := src
	if scaling {
		dst := image.NewRGBA(image.Rect(0, 0, width, height))
		resample(dst, src, useScale)
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
	if opts.Width > MaxPixels || opts.Height > MaxPixels {
		return fmt.Errorf("width and height must be at most %d pixels", MaxPixels)
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

// resample scales src to fill dst with the Catmull-Rom kernel. Scale is the
// fastest but allocates in proportion to dst width x src height (see
// scaleMemory); Transform allocates far less and gives the same result (at
// most one level apart per channel) 3 to 6 times more slowly, so it is used
// when Scale would need too much.
func resample(dst *image.RGBA, src image.Image, useScale bool) {
	sb := src.Bounds()
	if useScale {
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, draw.Src, nil)
		return
	}
	sx, sy := float64(dst.Bounds().Dx())/float64(sb.Dx()), float64(dst.Bounds().Dy())/float64(sb.Dy())
	s2d := f64.Aff3{sx, 0, -float64(sb.Min.X) * sx, 0, sy, -float64(sb.Min.Y) * sy}
	draw.CatmullRom.Transform(dst, s2d, src, sb, draw.Src, nil)
}

// maxResampleMemory is the most working memory Resize lets either resampler
// allocate, beyond the source and destination images.
const maxResampleMemory = 1 << 30

// scaleMemory is an upper bound on what draw.CatmullRom.Scale allocates to
// resize sw x sh to dw x dh, from x/image/draw's scale.go: a scratch buffer of
// dw x sh entries of 32 bytes (makeTmpBuf), and for each axis a source entry
// of 24 bytes per destination column or row plus contributions of 16 bytes:
// each destination column or row reaches at most every source one, and in
// all up to 4 per source or destination column or row, whichever is more,
// because the kernel's support of 2 widens with the shrink factor
// (newDistrib). Each slice may be rounded up by a page.
func scaleMemory(sw, sh, dw, dh int) int64 {
	axis := func(s, d int64) int64 { return 16*min(d*s, 4*max(s, d)+2*d) + 24*d + 2*page }
	return 32*int64(dw)*int64(sh) + page + axis(int64(sw), int64(dw)) + axis(int64(sh), int64(dh))
}

// transformMemory is an upper bound on what draw.CatmullRom.Transform
// allocates for the same resize: for each axis, one weight of 8 bytes per
// source column or row its widened support can reach, rounded up by a page.
func transformMemory(sw, sh, dw, dh int) int64 {
	axis := func(s, d int64) int64 { return 8*(3+4*((max(s, d)+d-1)/d)) + page }
	return axis(int64(sw), int64(dw)) + axis(int64(sh), int64(dh))
}

// page is how far Go may round up a large allocation.
const page = 8 << 10
