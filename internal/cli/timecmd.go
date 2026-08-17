package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"
)

// TimeCommand builds the time command around the clock package's pure
// functions, keeping this package independent of it; main wires the layers
// together and supplies the real clock via now.
func TimeCommand(
	now func() time.Time,
	parse func(s string, now time.Time) (time.Time, error),
	format func(t time.Time, name, layout, zone string) (string, error),
	jsonRepr func(t time.Time, zone string) (string, error),
) Command {
	return Command{
		Name:    "time",
		Summary: "print or convert a timestamp (-z zone, -f format, -json all forms)",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := flag.NewFlagSet("time", flag.ContinueOnError)
			fs.SetOutput(stderr)
			zone := fs.String("z", "UTC", "output timezone: IANA name, UTC, or local")
			name := fs.String("f", "rfc3339", "output format: rfc3339, unix, unix-ms, date, time")
			layout := fs.String("layout", "", "custom Go time layout, e.g. 2006-01-02 15:04")
			jsonOut := fs.Bool("json", false, "print every representation as JSON")
			if err := fs.Parse(args); err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return nil
				}
				return &UsageError{Err: err}
			}
			if fs.NArg() > 1 {
				return &UsageError{Err: fmt.Errorf("time: expected at most one timestamp argument, got %d", fs.NArg())}
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

			t := now()
			if fs.NArg() == 1 {
				var err error
				if t, err = parse(fs.Arg(0), now()); err != nil {
					return err
				}
			}
			var out string
			var err error
			if *jsonOut {
				out, err = jsonRepr(t, *zone)
			} else {
				out, err = format(t, *name, *layout, *zone)
			}
			if err != nil {
				// The only failure modes here are bad flag values (unknown
				// format name or timezone), so they exit 2, not 1.
				return &UsageError{Err: fmt.Errorf("time: %w", err)}
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
