package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// EncodeCommand builds a Command for an encode/decode pair. Every encode
// command shares the same CLI shape: a -d flag, input from an optional file
// argument or stdin, encoded text output with a trailing newline, decoded
// output as raw bytes.
func EncodeCommand(name, summary string, enc, dec func(w io.Writer, r io.Reader) error) Command {
	return Command{
		Name:    name,
		Summary: summary,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet(name, flag.ContinueOnError)
			fs.SetOutput(stderr)
			decode := fs.Bool("d", false, "decode instead of encode")
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
			if *decode {
				return dec(stdout, in)
			}
			if err := enc(stdout, in); err != nil {
				return err
			}
			// Trailing newline is presentation, so it lives here, not in the
			// core; decoded output is raw bytes and gets none.
			_, err = fmt.Fprintln(stdout)
			return err
		},
	}
}
