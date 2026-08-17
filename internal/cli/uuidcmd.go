package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// UUIDCommand builds the uuid command around a generator function, keeping
// this package independent of the generate package; main wires the layers
// together. The command reads no input: it only writes.
func UUIDCommand(gen func() (string, error)) Command {
	return Command{
		Name:    "uuid",
		Summary: "generate random (v4) UUIDs, one per line (-n for count)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("uuid", flag.ContinueOnError)
			fs.SetOutput(stderr)
			n := fs.Int("n", 1, "how many UUIDs to generate")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("uuid: unexpected argument %q", fs.Arg(0))}
			}
			if *n < 1 {
				return &UsageError{Err: fmt.Errorf("uuid: count must be at least 1, got %d", *n)}
			}
			for range *n {
				id, err := gen()
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(stdout, id); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
