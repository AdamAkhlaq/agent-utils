package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// TimeCommand builds the time command around the clock package's pure
// functions, keeping this package independent of it; main wires the layers
// together and supplies the real clock via now. The flags are validated
// before any argument is read, so a bad flag value is always a usage error
// and every later failure is about the timestamps or duration given.
func TimeCommand(
	now func() time.Time,
	parse func(s string, now time.Time, loc *time.Location) (time.Time, error),
	zone func(name string) (*time.Location, error),
	format func(t time.Time, name, layout, zone string) (string, error),
	jsonRepr func(t time.Time, zone string) (string, error),
	add func(t time.Time, duration string, loc *time.Location) (time.Time, error),
	diff func(from, to time.Time, loc *time.Location) string,
	diffJSON func(from, to time.Time, loc *time.Location) (string, error),
) Command {
	return Command{
		Name:    "time",
		Summary: "print or convert a timestamp, add a duration to one, or diff two (-z zone, -f format, -json)",
		Usage: `agent-utils time [-z zone] [-f format | -layout layout | -json] [timestamp]
agent-utils time [-z zone] [-f format | -layout layout | -json] add <timestamp> <duration>
agent-utils time [-z zone] [-json] diff <from> <to>`,
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("time")
			zoneName := fs.String("z", "UTC", "timezone for output and calendar arithmetic: IANA name, UTC, or local")
			name := fs.String("f", "rfc3339", "output format: rfc3339, unix, unix-ms, date, time")
			layout := fs.String("layout", "", "custom Go time layout, e.g. 2006-01-02 15:04")
			jsonOut := fs.Bool("json", false, "print every representation as JSON")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			set := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
			exclusive := 0
			for _, name := range []string{"f", "layout", "json"} {
				if set[name] {
					exclusive++
				}
			}
			if exclusive > 1 {
				return &UsageError{Err: fmt.Errorf("time: -f, -layout, and -json are mutually exclusive")}
			}
			loc, err := zone(*zoneName)
			if err != nil {
				return &UsageError{Err: fmt.Errorf("time: %w", err)}
			}
			if _, err := format(time.Unix(0, 0), *name, *layout, *zoneName); err != nil {
				return &UsageError{Err: fmt.Errorf("time: %w", err)}
			}

			mode, rest := "", fs.Args()
			if len(rest) > 0 && (rest[0] == "add" || rest[0] == "diff") {
				mode, rest = rest[0], rest[1:]
				if len(rest) > 0 && rest[0] == "--" {
					rest = rest[1:]
				}
			}
			// Arguments after the flags are taken literally, so negative
			// durations and epochs (-90s, -P1D, -1) need no "--"; a flag
			// placed there is a mistake to point out, not a value.
			if mode != "" || len(rest) > 1 {
				where := "the timestamp: agent-utils time [flags] <timestamp>"
				if mode != "" {
					where = fmt.Sprintf("the mode: agent-utils time [flags] %s ...", mode)
				}
				for _, arg := range rest {
					if arg == "-h" || arg == "-help" || arg == "--help" {
						return &helpRequest{flags: fs}
					}
					if isFlag(fs, arg) {
						return &UsageError{Err: fmt.Errorf("time: flag %s must come before %s", arg, where)}
					}
				}
			}
			if mode == "" && len(rest) > 1 {
				return &UsageError{Err: fmt.Errorf("time: %q is not a mode (add or diff), and a plain timestamp takes no other arguments; got %d", rest[0], len(rest))}
			}

			printTime := func(t time.Time) error {
				var out string
				var err error
				if *jsonOut {
					out, err = jsonRepr(t, *zoneName)
				} else {
					out, err = format(t, *name, *layout, *zoneName)
				}
				if err != nil {
					return fmt.Errorf("time: %w", err)
				}
				_, err = fmt.Fprintln(stdout, out)
				return err
			}

			current := now()
			switch mode {
			case "add":
				if len(rest) != 2 {
					return &UsageError{Err: fmt.Errorf("time add: takes a timestamp and a duration, got %d argument(s)", len(rest))}
				}
				t, err := parse(rest[0], current, loc)
				if err != nil {
					return fmt.Errorf("time add: %w", err)
				}
				if t, err = add(t, rest[1], loc); err != nil {
					return fmt.Errorf("time add: %w", err)
				}
				return printTime(t)
			case "diff":
				if set["f"] || set["layout"] {
					return &UsageError{Err: fmt.Errorf("time diff: -f and -layout format timestamps; diff prints an ISO 8601 duration, or JSON with -json")}
				}
				if len(rest) != 2 {
					return &UsageError{Err: fmt.Errorf("time diff: takes two timestamps (from and to), got %d argument(s)", len(rest))}
				}
				from, err := parse(rest[0], current, loc)
				if err != nil {
					return fmt.Errorf("time diff: %w", err)
				}
				to, err := parse(rest[1], current, loc)
				if err != nil {
					return fmt.Errorf("time diff: %w", err)
				}
				if !*jsonOut {
					_, err = fmt.Fprintln(stdout, diff(from, to, loc))
					return err
				}
				out, err := diffJSON(from, to, loc)
				if err != nil {
					return fmt.Errorf("time diff: %w", err)
				}
				_, err = fmt.Fprintln(stdout, out)
				return err
			default:
				t := current
				if len(rest) == 1 {
					// Unlike add and diff, the plain form has always read a
					// timestamp without an offset as UTC and converted it to -z.
					if t, err = parse(rest[0], current, time.UTC); err != nil {
						return err
					}
				}
				return printTime(t)
			}
		},
	}
}

// isFlag reports whether arg names one of fs's flags, as -name, --name, or
// -name=value.
func isFlag(fs *flag.FlagSet, arg string) bool {
	name, ok := strings.CutPrefix(arg, "-")
	if !ok {
		return false
	}
	name = strings.TrimPrefix(name, "-")
	name, _, _ = strings.Cut(name, "=")
	return fs.Lookup(name) != nil
}
