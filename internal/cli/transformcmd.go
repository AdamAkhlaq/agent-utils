package cli

import (
	"fmt"
	"io"
)

// TransformCommand builds a command around a single one-way text transform,
// with the encode family's shape: input from an optional file argument or
// stdin, text output to stdout with a trailing newline.
func TransformCommand(name, summary string, fn func(w io.Writer, r io.Reader) error) Command {
	return Command{
		Name:    name,
		Summary: summary,
		Usage:   "agent-utils " + name + " [file]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet(name)
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			if err := fn(stdout, in); err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout)
			return err
		},
	}
}
