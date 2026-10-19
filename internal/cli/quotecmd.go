package cli

import (
	"fmt"
	"io"
	"strings"
)

// QuoteTarget is one syntax the quote command converts raw text to. Unquote
// is nil when there is no well-defined way back to raw text.
type QuoteTarget struct {
	Name    string
	Quote   func(string) (string, error)
	Unquote func(string) (string, error)
}

// QuoteCommand builds the quote command around the encode package's quoting
// functions, keeping this package independent of them; main wires the layers
// together. Input is taken byte for byte in both directions, because a
// trailing newline may be part of the text; shell and json literals ignore
// whitespace around them by themselves.
func QuoteCommand(targets []QuoteTarget) Command {
	var all, decodable []string
	byName := make(map[string]QuoteTarget, len(targets))
	for _, t := range targets {
		all = append(all, t.Name)
		if t.Unquote != nil {
			decodable = append(decodable, t.Name)
		}
		byName[t.Name] = t
	}
	return Command{
		Name:    "quote",
		Summary: "quote or escape text for a target syntax: " + strings.Join(all, ", ") + " (-d to decode)",
		Usage: "agent-utils quote -as <" + strings.Join(all, "|") + "> [file]\n" +
			"agent-utils quote -d -as <" + strings.Join(decodable, "|") + "> [file]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("quote")
			as := fs.String("as", "", "target syntax: "+strings.Join(all, ", "))
			decode := fs.Bool("d", false, "decode a quoted literal back to raw text ("+strings.Join(decodable, ", ")+")")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			target, ok := byName[*as]
			switch {
			case *as == "":
				return &UsageError{Err: fmt.Errorf("quote: -as <target> is required (%s)", strings.Join(all, ", "))}
			case !ok:
				return &UsageError{Err: fmt.Errorf("quote: unknown target %q (valid: %s)", *as, strings.Join(all, ", "))}
			case *decode && target.Unquote == nil:
				return &UsageError{Err: fmt.Errorf("quote: -as %s cannot be decoded, because its syntax does not stand for one raw string (-d works with %s)", *as, strings.Join(decodable, ", "))}
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
			if *decode {
				raw, err := target.Unquote(string(data))
				if err != nil {
					return err
				}
				_, err = io.WriteString(stdout, raw)
				return err
			}
			quoted, err := target.Quote(string(data))
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, quoted)
			return err
		},
	}
}
