package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxScale bounds -scale: far beyond any real use, and small enough that a
// typo cannot ask for gigabytes of zeros.
const maxScale = 1000

// CalcCommand builds the calc command around the arith package's evaluator,
// keeping this package independent of it; main wires the layers together.
// calc and calcJSON receive a negative scale when -scale is not given. The
// expression is a literal positional argument (like color's value) or stdin,
// never a filename.
func CalcCommand(calc, calcJSON func(expr string, scale int) (string, error)) Command {
	return Command{
		Name:    "calc",
		Summary: "evaluate an arithmetic expression exactly (-scale places, -json with the fraction)",
		Usage:   "agent-utils calc [-scale n] [-json] [expression]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("calc")
			var scale optionalInt
			fs.Var(&scale, "scale", "round the result to exactly `n` decimal places, padded with zeros (default: exact, or 20 places if the decimal does not terminate)")
			jsonOut := fs.Bool("json", false, "print the result, the exact fraction, and whether the result is exact as JSON")
			if err := parseFlags(fs, endFlagsAtExpression(fs, args)); err != nil {
				return err
			}
			if fs.NArg() > 1 {
				return &UsageError{Err: fmt.Errorf("calc: expected one expression argument, got %d (quote the whole expression: agent-utils calc '2 * (3 + 4)')", fs.NArg())}
			}
			places := -1
			if scale.set {
				if scale.value < 0 || scale.value > maxScale {
					return &UsageError{Err: fmt.Errorf("calc: -scale must be between 0 and %d, got %d", maxScale, scale.value)}
				}
				places = scale.value
			}
			expr := fs.Arg(0)
			if fs.NArg() == 0 {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				expr = strings.TrimSuffix(string(data), "\n")
			}
			eval := calc
			if *jsonOut {
				eval = calcJSON
			}
			out, err := eval(expr, places)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}

// endFlagsAtExpression inserts "--" before an expression that starts with a
// minus sign (-2*3, --5, - 2), which the flag package would otherwise reject
// as an unknown flag. A flag name starts with a letter and an expression
// cannot, so the first character after the dashes tells them apart.
func endFlagsAtExpression(fs *flag.FlagSet, args []string) []string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" || len(a) < 2 || a[0] != '-' {
			return args
		}
		name := strings.TrimPrefix(a[1:], "-")
		if r, _ := utf8.DecodeRuneInString(name); !unicode.IsLetter(r) {
			return slices.Concat(args[:i], []string{"--"}, args[i:])
		}
		name, _, inline := strings.Cut(name, "=")
		if f := fs.Lookup(name); f != nil && !inline && !isBoolFlag(f) {
			i++ // its value is the next argument
		}
	}
	return args
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
