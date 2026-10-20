package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// ImgResizeCommand builds the img-resize command around a pure resize
// transform, keeping this package independent of internal/img; main wires the
// layers together. Zero width/height/max mean "not requested". maxPixels caps
// the output image; flags that exceed it whatever the input are usage errors.
func ImgResizeCommand(resize func(w io.Writer, r io.Reader, width, height, max, quality int) error, maxPixels int) Command {
	return Command{
		Name:    "img-resize",
		Summary: "resize a PNG or JPEG (-width/-height exact, or -max to fit; -quality for JPEG)",
		Usage: `agent-utils img-resize [-width n] [-height n] [-quality n] [input [output]]
agent-utils img-resize -max n [-quality n] [input [output]]`,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("img-resize")
			width := fs.Int("width", 0, fmt.Sprintf("target width in pixels (alone, height follows the aspect ratio); width x height at most %d", maxPixels))
			height := fs.Int("height", 0, fmt.Sprintf("target height in pixels (alone, width follows the aspect ratio); width x height at most %d", maxPixels))
			maxSide := fs.Int("max", 0, "scale down so the longest side fits within this many pixels (never upscales)")
			quality := fs.Int("quality", 85, "JPEG quality, 1-100 (applies when the output is JPEG)")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			// An explicit zero (e.g. -width 0) must be rejected, not read as
			// "unset", so validation looks at which flags were passed.
			passed := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
			for name, value := range map[string]int{"width": *width, "height": *height, "max": *maxSide} {
				if passed[name] && value < 1 {
					return &UsageError{Err: fmt.Errorf("img-resize: -%s must be at least 1, got %d", name, value)}
				}
			}
			hasDims := passed["width"] || passed["height"]
			if hasDims && passed["max"] {
				return &UsageError{Err: fmt.Errorf("img-resize: -max cannot be combined with -width or -height")}
			}
			if !hasDims && !passed["max"] {
				return &UsageError{Err: fmt.Errorf("img-resize: one sizing mode required: -width and/or -height, or -max")}
			}
			if *quality < 1 || *quality > 100 {
				return &UsageError{Err: fmt.Errorf("img-resize: quality must be between 1 and 100, got %d", *quality)}
			}
			// A side given alone pairs with a derived side of at least 1, so
			// it is over the limit on its own if it exceeds it.
			if hasDims && max(*width, 1) > maxPixels/max(*height, 1) {
				var requested []string
				if passed["width"] {
					requested = append(requested, fmt.Sprintf("-width %d", *width))
				}
				if passed["height"] {
					requested = append(requested, fmt.Sprintf("-height %d", *height))
				}
				return &UsageError{Err: fmt.Errorf("img-resize: %s is over the limit of %d pixels (width x height)", strings.Join(requested, " "), maxPixels)}
			}
			return runConvert(fs, stdin, stdout, func(w io.Writer, r io.Reader) error {
				return resize(w, r, *width, *height, *maxSide, *quality)
			})
		},
	}
}
