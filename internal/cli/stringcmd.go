package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// StringTransformCommand builds a command around a pure string -> string
// transform, for cores whose natural input is a small piece of text rather
// than a stream. Input comes from an optional file argument or stdin; one
// trailing newline is stripped first, because it belongs to the shell
// pipeline, not the text.
func StringTransformCommand(name, summary string, fn func(string) string) Command {
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
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			data, err := io.ReadAll(in)
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			_, err = fmt.Fprintln(stdout, fn(strings.TrimSuffix(string(data), "\n")))
			return err
		},
	}
}
