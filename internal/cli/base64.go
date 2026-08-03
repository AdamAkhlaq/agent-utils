package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/adamakhlaq/dev-utils/internal/encode"
)

// Base64Cmd is the CLI layer for the base64 command: flags in, streams through
// to the pure encode functions.
func Base64Cmd(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("base64", flag.ContinueOnError)
	fs.SetOutput(stderr)
	decode := fs.Bool("d", false, "decode instead of encode")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return &UsageError{Err: err}
	}
	if fs.NArg() > 0 {
		return &UsageError{Err: fmt.Errorf("base64: unexpected argument %q", fs.Arg(0))}
	}
	if *decode {
		return encode.Base64Decode(stdout, stdin)
	}
	if err := encode.Base64(stdout, stdin); err != nil {
		return err
	}
	// Trailing newline is presentation, so it lives here, not in the core;
	// decoded output is raw bytes and gets none.
	_, err := fmt.Fprintln(stdout)
	return err
}
