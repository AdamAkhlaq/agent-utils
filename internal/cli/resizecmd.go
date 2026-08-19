package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// ImgResizeCommand builds the img-resize command around a pure resize
// transform, keeping this package independent of internal/img; main wires the
// layers together. Zero width/height/max mean "not requested".
func ImgResizeCommand(resize func(w io.Writer, r io.Reader, width, height, max, quality int) error) Command {
	return Command{
		Name:    "img-resize",
		Summary: "resize a PNG or JPEG (-width/-height exact, or -max to fit; -quality for JPEG)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("img-resize", flag.ContinueOnError)
			fs.SetOutput(stderr)
			width := fs.Int("width", 0, "target width in pixels (alone, height follows the aspect ratio)")
			height := fs.Int("height", 0, "target height in pixels (alone, width follows the aspect ratio)")
			max := fs.Int("max", 0, "scale down so the longest side fits within this many pixels (never upscales)")
			quality := fs.Int("quality", 75, "JPEG quality, 1-100 (applies when the output is JPEG)")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			// An explicit zero (e.g. -width 0) must be rejected, not read as
			// "unset", so validation looks at which flags were passed.
			passed := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
			for name, value := range map[string]int{"width": *width, "height": *height, "max": *max} {
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
			return runConvert(fs, stdin, stdout, func(w io.Writer, r io.Reader) error {
				return resize(w, r, *width, *height, *max, *quality)
			})
		},
	}
}
