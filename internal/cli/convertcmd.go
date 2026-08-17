package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// ConvertCommand builds a file-conversion command around a pure Reader ->
// Writer transform: input from an optional first positional or stdin, output
// to an optional second positional or stdout.
func ConvertCommand(name, summary string, convert func(w io.Writer, r io.Reader) error) Command {
	return Command{
		Name:    name,
		Summary: summary,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet(name, flag.ContinueOnError)
			fs.SetOutput(stderr)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			return runConvert(fs, stdin, stdout, convert)
		},
	}
}

// PNGToJPEGCommand is ConvertCommand's shape plus the JPEG-only quality flag.
func PNGToJPEGCommand(convert func(w io.Writer, r io.Reader, quality int) error) Command {
	return Command{
		Name:    "png2jpeg",
		Summary: "convert a PNG image to JPEG (-q quality)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("png2jpeg", flag.ContinueOnError)
			fs.SetOutput(stderr)
			quality := fs.Int("q", 85, "JPEG quality, 1-100")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if *quality < 1 || *quality > 100 {
				return &UsageError{Err: fmt.Errorf("png2jpeg: quality must be between 1 and 100, got %d", *quality)}
			}
			return runConvert(fs, stdin, stdout, func(w io.Writer, r io.Reader) error {
				return convert(w, r, *quality)
			})
		},
	}
}

func runConvert(fs *flag.FlagSet, stdin io.Reader, stdout io.Writer, convert func(w io.Writer, r io.Reader) error) (err error) {
	in, out, err := openInOut(fs, stdin, stdout)
	if err != nil {
		return err
	}
	defer in.Close()
	// A write error can surface at Close (buffered file data is flushed then),
	// so a plain deferred Close would swallow it. The named return lets this
	// defer report it.
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing output: %w", cerr)
		}
	}()
	return convert(out, in)
}
