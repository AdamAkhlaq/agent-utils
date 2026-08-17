package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// PasswordCommand builds the password command around a generator function,
// keeping this package independent of the generate package; main wires the
// layers together.
func PasswordCommand(gen func(length int, symbols bool) (string, error)) Command {
	return Command{
		Name:    "password",
		Summary: "generate random passwords (-l length, -n count, -no-symbols)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("password", flag.ContinueOnError)
			fs.SetOutput(stderr)
			length := fs.Int("l", 20, "password length")
			n := fs.Int("n", 1, "how many passwords to generate")
			noSymbols := fs.Bool("no-symbols", false, "letters and digits only")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("password: unexpected argument %q", fs.Arg(0))}
			}
			if *length < 1 {
				return &UsageError{Err: fmt.Errorf("password: length must be at least 1, got %d", *length)}
			}
			if *n < 1 {
				return &UsageError{Err: fmt.Errorf("password: count must be at least 1, got %d", *n)}
			}
			for range *n {
				pw, err := gen(*length, !*noSymbols)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(stdout, pw); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
