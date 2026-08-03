package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// QRCommand builds the qr command around a generator and a decoder, keeping
// this package independent of the img package; main wires the layers together.
func QRCommand(enc func(w io.Writer, text string, size int) error, dec func(r io.Reader) (string, error)) Command {
	return Command{
		Name:    "qr",
		Summary: "generate a QR code PNG from text, or decode one (-d)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
			fs := flag.NewFlagSet("qr", flag.ContinueOnError)
			fs.SetOutput(stderr)
			decode := fs.Bool("d", false, "decode a QR code image to its text")
			size := fs.Int("s", 256, "image size in pixels when encoding")
			outPath := fs.String("o", "", "write output to a file instead of stdout")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			out, err := openOutput(*outPath, stdout)
			if err != nil {
				return err
			}
			// A write error can surface at Close (buffered file data is
			// flushed then), so a plain deferred Close would swallow it.
			// The named return lets this defer report it.
			defer func() {
				if cerr := out.Close(); cerr != nil && err == nil {
					err = fmt.Errorf("closing output: %w", cerr)
				}
			}()

			if *decode {
				in, err := openInput(fs, stdin)
				if err != nil {
					return err
				}
				defer in.Close()
				text, err := dec(in)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(out, text)
				return err
			}

			if fs.NArg() != 1 {
				return &UsageError{Err: fmt.Errorf("qr: expected exactly one text argument")}
			}
			if *size <= 0 {
				return &UsageError{Err: fmt.Errorf("qr: size must be positive, got %d", *size)}
			}
			return enc(out, fs.Arg(0), *size)
		},
	}
}
