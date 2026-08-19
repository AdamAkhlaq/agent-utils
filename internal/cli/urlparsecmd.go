package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// URLParseCommand builds the urlparse command around the inspect package's
// URL renderer, keeping this package independent of it; main wires the layers
// together. The URL is a literal positional argument (like color's value) or
// the first line of stdin, never a filename.
func URLParseCommand(parseJSON func(raw string) (string, error)) Command {
	return Command{
		Name:    "urlparse",
		Summary: "parse a URL into its components as JSON",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("urlparse", flag.ContinueOnError)
			fs.SetOutput(stderr)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 1 {
				return &UsageError{Err: fmt.Errorf("urlparse: expected at most one URL argument, got %d", fs.NArg())}
			}
			raw := fs.Arg(0)
			if fs.NArg() == 0 {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				raw, _, _ = strings.Cut(string(data), "\n")
				raw = strings.TrimSpace(raw)
			}
			out, err := parseJSON(raw)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
