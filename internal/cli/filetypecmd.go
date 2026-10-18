package cli

import (
	"fmt"
	"io"
)

// FiletypeCommand builds the filetype command around two renderers from the
// inspect package, keeping this package independent of it; main wires the
// layers together.
func FiletypeCommand(plain, asJSON func(r io.Reader) (string, error)) Command {
	return Command{
		Name:    "filetype",
		Summary: "identify a file's MIME type and image dimensions (-json)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("filetype")
			jsonOut := fs.Bool("json", false, "print a JSON object with mime, bytes, and image dimensions")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			in, err := openInput(fs, stdin)
			if err != nil {
				return err
			}
			defer in.Close()

			render := plain
			if *jsonOut {
				render = asJSON
			}
			out, err := render(in)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
