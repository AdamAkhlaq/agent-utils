package cli

import (
	"flag"
	"fmt"
	"io"
)

// QRCommand builds the qr command around a generator and a decoder, keeping
// this package independent of the img package; main wires the layers together.
// maxSize caps -s.
func QRCommand(enc func(w io.Writer, text string, size int) error, dec func(r io.Reader) (string, error), maxSize int) Command {
	return Command{
		Name:    "qr",
		Summary: "generate a QR code PNG from text, or decode one (-d)",
		Usage: `agent-utils qr [-s px] [-o file] <text>
agent-utils qr -d [-o file] [image]`,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("qr")
			decode := fs.Bool("d", false, "decode a QR code image to its text")
			size := fs.Int("s", 256, fmt.Sprintf("image size in pixels when encoding, at most %d", maxSize))
			outPath := fs.String("o", "", "write output to a file instead of stdout")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			sizeSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "s" {
					sizeSet = true
				}
			})

			if *decode {
				if sizeSet {
					return &UsageError{Err: fmt.Errorf("qr: -s only applies when encoding")}
				}
				in, err := openInput(fs, stdin)
				if err != nil {
					return err
				}
				defer in.Close()
				text, err := dec(in)
				if err != nil {
					return err
				}
				return writeOutput(*outPath, stdout, func(w io.Writer) error {
					_, err := fmt.Fprintln(w, text)
					return err
				})
			}

			if fs.NArg() != 1 {
				return &UsageError{Err: fmt.Errorf("qr: expected exactly one text argument")}
			}
			if *size <= 0 {
				return &UsageError{Err: fmt.Errorf("qr: size must be positive, got %d", *size)}
			}
			if *size > maxSize {
				return &UsageError{Err: fmt.Errorf("qr: size must be at most %d, got %d", maxSize, *size)}
			}
			return writeOutput(*outPath, stdout, func(w io.Writer) error {
				return enc(w, fs.Arg(0), *size)
			})
		},
	}
}
