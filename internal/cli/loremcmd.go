package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// LoremCommand builds the lorem command around the two generator functions,
// keeping this package independent of the generate package; main wires the
// layers together.
func LoremCommand(words, paragraphs func(n int) string) Command {
	return Command{
		Name:    "lorem",
		Summary: "generate lorem ipsum filler text (-w words or -p paragraphs)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("lorem", flag.ContinueOnError)
			fs.SetOutput(stderr)
			w := fs.Int("w", 0, "number of words")
			p := fs.Int("p", 1, "number of paragraphs")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 0 {
				return &UsageError{Err: fmt.Errorf("lorem: unexpected argument %q", fs.Arg(0))}
			}
			wSet, pSet := false, false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "w":
					wSet = true
				case "p":
					pSet = true
				}
			})
			if wSet && pSet {
				return &UsageError{Err: fmt.Errorf("lorem: -w and -p are mutually exclusive")}
			}
			var out string
			switch {
			case wSet:
				if *w < 1 {
					return &UsageError{Err: fmt.Errorf("lorem: word count must be at least 1, got %d", *w)}
				}
				out = words(*w)
			default:
				if *p < 1 {
					return &UsageError{Err: fmt.Errorf("lorem: paragraph count must be at least 1, got %d", *p)}
				}
				out = paragraphs(*p)
			}
			_, err := fmt.Fprintln(stdout, out)
			return err
		},
	}
}
