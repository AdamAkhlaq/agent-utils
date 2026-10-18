package cli

import (
	"fmt"
	"io"
	"strconv"
)

// CIDRCommand builds the cidr command around the netcalc package's pure
// functions, keeping this package independent of it; main wires the layers
// together. A malformed prefix or address is bad data and exits 1, like a bad
// version in semver; split marks a bad requested length as a *UsageError
// itself. contains and overlaps mirror semver -check by printing true/false
// and exiting 0/3.
func CIDRCommand(
	infoJSON func(prefix string) (string, error),
	contains func(prefix, ip string) (bool, error),
	overlaps func(a, b string) (bool, error),
	split func(prefix string, newLen int) ([]string, error),
) Command {
	return Command{
		Name:    "cidr",
		Summary: "CIDR subnet math: info <prefix>, contains <prefix> <ip>, overlaps <a> <b>, split <prefix> <len>",
		Usage: `agent-utils cidr info <prefix>
agent-utils cidr contains <prefix> <ip>
agent-utils cidr overlaps <prefix> <prefix>
agent-utils cidr split <prefix> <new-length>`,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("cidr")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if fs.NArg() == 0 {
				return &UsageError{Err: fmt.Errorf("cidr: expected a mode: info, contains, overlaps, or split")}
			}
			mode, rest := fs.Arg(0), fs.Args()[1:]
			switch mode {
			case "info":
				if len(rest) != 1 {
					return &UsageError{Err: fmt.Errorf("cidr info: takes exactly one prefix argument, got %d", len(rest))}
				}
				out, err := infoJSON(rest[0])
				if err != nil {
					return fmt.Errorf("cidr info: %w", err)
				}
				_, err = fmt.Fprintln(stdout, out)
				return err
			case "contains":
				if len(rest) != 2 {
					return &UsageError{Err: fmt.Errorf("cidr contains: takes a prefix and an IP argument, got %d argument(s)", len(rest))}
				}
				ok, err := contains(rest[0], rest[1])
				if err != nil {
					return fmt.Errorf("cidr contains: %w", err)
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "false"); err != nil {
						return err
					}
					return &FalseError{Err: fmt.Errorf("cidr: %s does not contain %s", rest[0], rest[1])}
				}
				_, err = fmt.Fprintln(stdout, "true")
				return err
			case "overlaps":
				if len(rest) != 2 {
					return &UsageError{Err: fmt.Errorf("cidr overlaps: takes exactly two prefix arguments, got %d", len(rest))}
				}
				ok, err := overlaps(rest[0], rest[1])
				if err != nil {
					return fmt.Errorf("cidr overlaps: %w", err)
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "false"); err != nil {
						return err
					}
					return &FalseError{Err: fmt.Errorf("cidr: %s and %s do not overlap", rest[0], rest[1])}
				}
				_, err = fmt.Fprintln(stdout, "true")
				return err
			case "split":
				if len(rest) != 2 {
					return &UsageError{Err: fmt.Errorf("cidr split: takes a prefix and a new prefix length, got %d argument(s)", len(rest))}
				}
				newLen, err := strconv.Atoi(rest[1])
				if err != nil {
					return &UsageError{Err: fmt.Errorf("cidr split: new prefix length %q is not an integer", rest[1])}
				}
				subnets, err := split(rest[0], newLen)
				if err != nil {
					return fmt.Errorf("cidr split: %w", err)
				}
				for _, s := range subnets {
					if _, err := fmt.Fprintln(stdout, s); err != nil {
						return err
					}
				}
				return nil
			default:
				return &UsageError{Err: fmt.Errorf("cidr: unknown mode %q: expected info, contains, overlaps, or split", mode)}
			}
		},
	}
}
