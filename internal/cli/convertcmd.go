package cli

import (
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
		Usage:   "agent-utils " + name + " [input [output]]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet(name)
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			return runConvert(fs, stdin, stdout, convert)
		},
	}
}

// PNGToJPEGCommand is ConvertCommand's shape plus the JPEG-only quality flag.
func PNGToJPEGCommand(convert func(w io.Writer, r io.Reader, quality int) error) Command {
	return Command{
		Name:    "png2jpeg",
		Summary: "convert a PNG image to JPEG (-quality)",
		Usage:   "agent-utils png2jpeg [-quality n] [input [output]]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("png2jpeg")
			quality := fs.Int("quality", 85, "JPEG quality, 1-100")
			fs.IntVar(quality, "q", 85, "alias for -quality")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			passed := 0
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "q" || f.Name == "quality" {
					passed++
				}
			})
			if passed > 1 {
				return &UsageError{Err: fmt.Errorf("png2jpeg: -q is an alias for -quality; give only one")}
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

func runConvert(fs *flag.FlagSet, stdin io.Reader, stdout io.Writer, convert func(w io.Writer, r io.Reader) error) error {
	in, out, err := openInOut(fs, stdin, stdout)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := convert(out, in); err != nil {
		out.Abort()
		return err
	}
	return out.Commit()
}
