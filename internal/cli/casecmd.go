package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// CaseCommand builds the case command around the text package's pure casing
// converter, keeping this package independent of it; main wires the layers
// together.
func CaseCommand(convert func(s, target string) (string, error)) Command {
	return Command{
		Name:    "case",
		Summary: "convert text between snake, camel, pascal, kebab, screaming case (-to target)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("case", flag.ContinueOnError)
			fs.SetOutput(stderr)
			target := fs.String("to", "", "target casing: snake, camel, pascal, kebab, or screaming")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if *target == "" {
				return &UsageError{Err: fmt.Errorf("case: -to <target> is required (snake, camel, pascal, kebab, or screaming)")}
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
			out, err := convert(strings.TrimSuffix(string(data), "\n"), *target)
			if err != nil {
				// The only failure mode here is a bad -to value, so it
				// exits 2, not 1.
				return &UsageError{Err: fmt.Errorf("case: %w", err)}
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
