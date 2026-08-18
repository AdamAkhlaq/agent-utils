package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// ColorCommand builds the color command around the hue package's pure
// converters, keeping this package independent of it; main wires the layers
// together. The color value is a literal positional argument (like time's
// timestamp) or stdin, never a filename.
func ColorCommand(convert func(input, to string) (string, error), jsonRepr func(input string) (string, error)) Command {
	return Command{
		Name:    "color",
		Summary: "convert a color between hex, rgb, and hsl (-to form, -json all forms)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("color", flag.ContinueOnError)
			fs.SetOutput(stderr)
			to := fs.String("to", "", "output form: hex, rgb, or hsl")
			jsonOut := fs.Bool("json", false, "print all three forms as JSON")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if *jsonOut && *to != "" {
				return &UsageError{Err: fmt.Errorf("color: -to and -json are mutually exclusive")}
			}
			if !*jsonOut {
				switch *to {
				case "hex", "rgb", "hsl":
				case "":
					return &UsageError{Err: fmt.Errorf("color: -to <form> is required unless -json is given (hex, rgb, or hsl)")}
				default:
					return &UsageError{Err: fmt.Errorf("color: unknown output form %q (valid: hex, rgb, hsl)", *to)}
				}
			}
			if fs.NArg() > 1 {
				return &UsageError{Err: fmt.Errorf("color: expected at most one color argument, got %d", fs.NArg())}
			}
			input := fs.Arg(0)
			if fs.NArg() == 0 {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				input = strings.TrimSuffix(string(data), "\n")
			}
			var out string
			var err error
			if *jsonOut {
				out, err = jsonRepr(input)
			} else {
				out, err = convert(input, *to)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
