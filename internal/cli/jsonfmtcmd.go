package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// JSONFmtCommand builds the json-fmt command around the format package's pure
// functions, keeping this package independent of it; main wires the layers
// together.
func JSONFmtCommand(
	pretty func(w io.Writer, r io.Reader, indent string) error,
	compact func(w io.Writer, r io.Reader) error,
	valid func(r io.Reader) error,
) Command {
	return Command{
		Name:    "json-fmt",
		Summary: "pretty-print, minify (-c), or validate (-check) JSON",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("json-fmt", flag.ContinueOnError)
			fs.SetOutput(stderr)
			indent := fs.Int("indent", 2, "spaces per indentation level")
			minify := fs.Bool("c", false, "compact (minify) instead of pretty-print")
			check := fs.Bool("check", false, "validate only: no output, exit 0 if valid, 1 if not")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if *minify && *check {
				return &UsageError{Err: fmt.Errorf("json-fmt: -c and -check are mutually exclusive")}
			}
			// An explicitly passed -indent (even at its default value) is
			// silently dead alongside -c or -check; reject it instead.
			indentSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "indent" {
					indentSet = true
				}
			})
			if indentSet && (*minify || *check) {
				return &UsageError{Err: fmt.Errorf("json-fmt: -indent only applies when pretty-printing")}
			}
			if *indent < 0 || *indent > 8 {
				return &UsageError{Err: fmt.Errorf("json-fmt: indent must be between 0 and 8, got %d", *indent)}
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()

			if *check {
				return valid(in)
			}
			if *minify {
				err = compact(stdout, in)
			} else {
				err = pretty(stdout, in, strings.Repeat(" ", *indent))
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout)
			return err
		},
	}
}
