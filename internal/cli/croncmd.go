package cli

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const maxCronRuns = 1000

// CronCommand builds the cron command around the clock package's schedule
// functions, keeping this package independent of it; main wires the layers
// together and supplies the real clock via now. The expression is a literal
// positional argument (like color's value) or stdin minus one line ending,
// never a filename.
func CronCommand(
	now func() time.Time,
	zone func(name string) (*time.Location, error),
	parseFrom func(s string, now time.Time, loc *time.Location) (time.Time, error),
	next func(expr string, from time.Time, loc *time.Location, n int) (string, error),
	nextJSON func(expr string, from time.Time, loc *time.Location, n int) (string, error),
) Command {
	return Command{
		Name:    "cron",
		Summary: "validate a cron expression and list its next run times (-n count, -z zone, -json)",
		Usage:   "agent-utils cron [-n count] [-z zone] [-from timestamp] [-json] [expression]",
		Run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
			fs := newFlagSet("cron")
			count := fs.Int("n", 5, fmt.Sprintf("number of run times to list, 1-%d", maxCronRuns))
			zoneName := fs.String("z", "UTC", "timezone the schedule runs in: IANA name, UTC, or local")
			fromText := fs.String("from", "now", "list runs strictly after this timestamp, in any form time accepts; one without an offset is wall-clock time in the -z zone")
			jsonOut := fs.Bool("json", false, "print the expression, zone, expanded fields, and run times as JSON")
			if err := parseFlags(fs, args); err != nil {
				return err
			}
			if *count < 1 || *count > maxCronRuns {
				return &UsageError{Err: fmt.Errorf("cron: -n must be between 1 and %d, got %d", maxCronRuns, *count)}
			}
			loc, err := zone(*zoneName)
			if err != nil {
				return &UsageError{Err: fmt.Errorf("cron: %w", err)}
			}
			from, err := parseFrom(*fromText, now(), loc)
			if err != nil {
				return &UsageError{Err: fmt.Errorf("cron: -from: %w", err)}
			}
			if fs.NArg() > 1 {
				for _, arg := range fs.Args()[1:] {
					if len(arg) > 1 && arg[0] == '-' {
						return &UsageError{Err: fmt.Errorf("cron: flags must come before the expression, got %q after it", arg)}
					}
				}
				return &UsageError{Err: fmt.Errorf("cron: expected the expression as one quoted argument, got %d arguments (quote it: agent-utils cron '*/15 9-17 * * 1-5')", fs.NArg())}
			}
			expr := fs.Arg(0)
			if fs.NArg() == 0 {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return fmt.Errorf("reading input: %w", err)
				}
				expr = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
			}
			render := next
			if *jsonOut {
				render = nextJSON
			}
			out, err := render(expr, from, loc, *count)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(stdout, out)
			return err
		},
	}
}
