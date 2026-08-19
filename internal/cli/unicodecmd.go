package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// UnicodeCommand builds the unicode command around the chars package's pure
// functions, keeping this package independent of it; main wires the layers
// together. Input is inspected exactly as received: no trailing newline is
// stripped, because the whole point of the command is byte-exact diagnosis.
func UnicodeCommand(
	inspectJSON func(data []byte) (string, error),
	checkJSON func(data []byte) (string, error),
	nfc func(data []byte) ([]byte, error),
	nfd func(data []byte) ([]byte, error),
) Command {
	return Command{
		Name:    "unicode",
		Summary: "inspect text codepoints as JSON (-check suspicious chars, -nfc/-nfd normalize)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("unicode", flag.ContinueOnError)
			fs.SetOutput(stderr)
			nfcMode := fs.Bool("nfc", false, "write the input normalized to NFC (composed) instead of inspecting")
			nfdMode := fs.Bool("nfd", false, "write the input normalized to NFD (decomposed) instead of inspecting")
			checkMode := fs.Bool("check", false, "report only suspicious characters and NFC status as JSON")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			modes := 0
			for _, set := range []bool{*nfcMode, *nfdMode, *checkMode} {
				if set {
					modes++
				}
			}
			if modes > 1 {
				return &UsageError{Err: fmt.Errorf("unicode: -nfc, -nfd, and -check are mutually exclusive")}
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

			switch {
			case *nfcMode, *nfdMode:
				normalize := nfc
				if *nfdMode {
					normalize = nfd
				}
				out, err := normalize(data)
				if err != nil {
					return fmt.Errorf("unicode: %w", err)
				}
				// The output is the normalized text itself, byte for byte;
				// appending a newline would corrupt it.
				_, err = stdout.Write(out)
				return err
			case *checkMode:
				out, err := checkJSON(data)
				if err != nil {
					return fmt.Errorf("unicode: %w", err)
				}
				_, err = fmt.Fprintln(stdout, out)
				return err
			default:
				out, err := inspectJSON(data)
				if err != nil {
					return fmt.Errorf("unicode: %w", err)
				}
				_, err = fmt.Fprintln(stdout, out)
				return err
			}
		},
	}
}
