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
// layers together. The trailing newline is appended here because the core
// emits a bare JSON document.
func CSVToJSONCommand(convert func(w io.Writer, r io.Reader, sep rune) error) Command {
	return sepConvertCommand("csv2json", "convert CSV with a header row to a JSON array (-sep delimiter)",
		func(w io.Writer, r io.Reader, sep rune) error {
			if err := convert(w, r, sep); err != nil {
				return err
			}
			_, err := fmt.Fprintln(w)
			return err
		})
}

// JSONToCSVCommand builds the json2csv command, csv2json's inverse. No extra
// newline: the CSV writer already terminates every record.
func JSONToCSVCommand(convert func(w io.Writer, r io.Reader, sep rune) error) Command {
	return sepConvertCommand("json2csv", "convert a JSON array of objects to CSV (-sep delimiter)", convert)
}

// sepConvertCommand is the shared shape of the CSV conversion commands: a
// -sep delimiter flag, input from an optional file argument or stdin, output
// to stdout.
func sepConvertCommand(name, summary string, convert func(w io.Writer, r io.Reader, sep rune) error) Command {
	return Command{
		Name:    name,
		Summary: summary,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet(name, flag.ContinueOnError)
			fs.SetOutput(stderr)
			sepFlag := fs.String("sep", ",", `field separator: one character, or \t for tab`)
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			sep, err := parseSep(name, *sepFlag)
			if err != nil {
				return &UsageError{Err: err}
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()
			return convert(stdout, in, sep)
		},
	}
}

func parseSep(name, s string) (rune, error) {
	if s == `\t` {
		return '\t', nil
	}
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || size != len(s) || (r == utf8.RuneError && size == 1) {
		return 0, fmt.Errorf(`%s: -sep must be a single character or \t, got %q`, name, s)
	}
	return r, nil
}
