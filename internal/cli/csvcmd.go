package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"unicode/utf8"
)

// CSVToJSONCommand builds the csv2json command around the format package's
// pure function, keeping this package independent of it; main wires the
// layers together.
func CSVToJSONCommand(convert func(w io.Writer, r io.Reader, sep rune) error) Command {
	return Command{
		Name:    "csv2json",
		Summary: "convert CSV with a header row to a JSON array (-sep delimiter)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("csv2json", flag.ContinueOnError)
			fs.SetOutput(stderr)
			sepFlag := fs.String("sep", ",", `field separator: one character, or \t for tab`)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			sep, err := parseSep(*sepFlag)
			if err != nil {
				return &UsageError{Err: err}
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			if err := convert(stdout, in, sep); err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout)
			return err
		},
	}
}

func parseSep(s string) (rune, error) {
	if s == `\t` {
		return '\t', nil
	}
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || size != len(s) || (r == utf8.RuneError && size == 1) {
		return 0, fmt.Errorf(`csv2json: -sep must be a single character or \t, got %q`, s)
	}
	return r, nil
}
