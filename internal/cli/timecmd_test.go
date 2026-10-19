package cli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	errTestParse    = errors.New("unrecognized timestamp")
	errTestZone     = errors.New("unknown timezone")
	errTestFormat   = errors.New("unknown format")
	errTestDuration = errors.New("invalid duration")
	errTestDiffJSON = errors.New("encoding difference")
)

// timeCalls records what the stubbed core received, so these tests exercise
// only the CLI layer's argument handling and error mapping.
type timeCalls struct {
	inputs   []string
	nows     []time.Time
	parseLoc string
	t        time.Time
	name     string
	layout   string
	zone     string
	json     bool
	loc      string
	duration string
	from, to time.Time
	diffJSON bool
}

// newTestTimeCommand returns the time command over stubs: "bad" fails to
// parse, Mars/Olympus is an unknown zone, -f bogus an unknown format, the
// duration "bad" fails, and a diff from "jsonfail" fails to encode.
func newTestTimeCommand(c *timeCalls, now func() time.Time) Command {
	parsed := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)
	return TimeCommand(
		now,
		func(s string, now time.Time, loc *time.Location) (time.Time, error) {
			c.inputs = append(c.inputs, s)
			c.nows = append(c.nows, now)
			c.parseLoc = loc.String()
			if s == "bad" {
				return time.Time{}, errTestParse
			}
			if s == "jsonfail" {
				return time.Time{}, nil
			}
			return parsed, nil
		},
		func(name string) (*time.Location, error) {
			if name == "Mars/Olympus" {
				return nil, errTestZone
			}
			return time.FixedZone(name, 0), nil
		},
		func(tm time.Time, name, layout, zone string) (string, error) {
			if name == "bogus" {
				return "", errTestFormat
			}
			// The CLI formats the epoch once up front to validate -f, before
			// it reads any argument; only the real call is recorded.
			if !tm.Equal(time.Unix(0, 0)) {
				c.t, c.name, c.layout, c.zone = tm, name, layout, zone
			}
			return "formatted", nil
		},
		func(tm time.Time, zone string) (string, error) {
			c.t, c.zone, c.json = tm, zone, true
			return `{"all"}`, nil
		},
		func(tm time.Time, duration string, loc *time.Location) (time.Time, error) {
			c.duration, c.loc = duration, loc.String()
			if duration == "bad" {
				return time.Time{}, errTestDuration
			}
			return tm.Add(time.Hour), nil
		},
		func(from, to time.Time, loc *time.Location) string {
			c.from, c.to, c.loc = from, to, loc.String()
			return "PT1H"
		},
		func(from, to time.Time, loc *time.Location) (string, error) {
			c.from, c.to, c.loc, c.diffJSON = from, to, loc.String(), true
			if from.IsZero() {
				return "", errTestDiffJSON
			}
			return `{"diff"}`, nil
		},
	)
}

func TestTimeCommand(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	parsed := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)
	added := parsed.Add(time.Hour)

	tests := []struct {
		name string
		args []string
		want string
		// wantCalls lists the fields the core must have received; nows and
		// zero-valued fields of calls that did not happen are not compared.
		wantCalls timeCalls
	}{
		{name: "no argument formats now", want: "formatted\n", wantCalls: timeCalls{t: fixedNow, name: "rfc3339", zone: "UTC"}},
		{name: "argument is parsed", args: []string{"1000000000"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"1000000000"}, parseLoc: "UTC", t: parsed, name: "rfc3339", zone: "UTC"}},
		{name: "zone and format plumbed", args: []string{"-z", "Asia/Tokyo", "-f", "unix", "x"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"x"}, parseLoc: "UTC", t: parsed, name: "unix", zone: "Asia/Tokyo"}},
		{name: "layout plumbed", args: []string{"-layout", "2006", "x"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"x"}, parseLoc: "UTC", t: parsed, name: "rfc3339", layout: "2006", zone: "UTC"}},
		{name: "json mode", args: []string{"-json"}, want: "{\"all\"}\n", wantCalls: timeCalls{t: fixedNow, zone: "UTC", json: true}},
		{name: "add formats the result", args: []string{"add", "x", "P1D"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"x"}, parseLoc: "UTC", duration: "P1D", loc: "UTC", t: added, name: "rfc3339", zone: "UTC"}},
		{name: "add takes every output flag", args: []string{"-z", "Asia/Tokyo", "-layout", "2006", "add", "x", "P1D"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"x"}, parseLoc: "Asia/Tokyo", duration: "P1D", loc: "Asia/Tokyo", t: added, name: "rfc3339", layout: "2006", zone: "Asia/Tokyo"}},
		{name: "add with json", args: []string{"-json", "add", "now", "1h"}, want: "{\"all\"}\n", wantCalls: timeCalls{inputs: []string{"now"}, parseLoc: "UTC", duration: "1h", loc: "UTC", t: added, zone: "UTC", json: true}},
		{name: "add takes a negative duration literally", args: []string{"add", "x", "-90s"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"x"}, parseLoc: "UTC", duration: "-90s", loc: "UTC", t: added, name: "rfc3339", zone: "UTC"}},
		{name: "add takes a negative epoch literally", args: []string{"add", "-1", "-P1D"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"-1"}, parseLoc: "UTC", duration: "-P1D", loc: "UTC", t: added, name: "rfc3339", zone: "UTC"}},
		{name: "diff prints a duration", args: []string{"diff", "a", "b"}, want: "PT1H\n", wantCalls: timeCalls{inputs: []string{"a", "b"}, parseLoc: "UTC", loc: "UTC", from: parsed, to: parsed}},
		{name: "diff ignores a leading --", args: []string{"diff", "--", "a", "b"}, want: "PT1H\n", wantCalls: timeCalls{inputs: []string{"a", "b"}, parseLoc: "UTC", loc: "UTC", from: parsed, to: parsed}},
		{name: "add ignores a leading --", args: []string{"add", "--", "-1", "-90s"}, want: "formatted\n", wantCalls: timeCalls{inputs: []string{"-1"}, parseLoc: "UTC", duration: "-90s", loc: "UTC", t: added, name: "rfc3339", zone: "UTC"}},
		{name: "diff with json and zone", args: []string{"-json", "-z", "Europe/London", "diff", "a", "b"}, want: "{\"diff\"}\n", wantCalls: timeCalls{inputs: []string{"a", "b"}, parseLoc: "Europe/London", loc: "Europe/London", from: parsed, to: parsed, diffJSON: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls timeCalls
			cmd := newTestTimeCommand(&calls, func() time.Time { return fixedNow })
			var stdout, stderr bytes.Buffer
			if err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
			calls.nows = nil
			if !reflect.DeepEqual(calls, tt.wantCalls) {
				t.Errorf("Run() called core with\n %+v\nwant\n %+v", calls, tt.wantCalls)
			}
		})
	}
}

// Both timestamps of a diff must see the same "now", or "diff now now"
// would not be zero.
func TestTimeCommandReadsTheClockOnce(t *testing.T) {
	var calls timeCalls
	tick := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	cmd := newTestTimeCommand(&calls, func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	})
	var stdout, stderr bytes.Buffer
	if err := cmd.Run([]string{"diff", "now", "now"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(calls.nows) != 2 || !calls.nows[0].Equal(calls.nows[1]) {
		t.Errorf("parse received now = %v, want the same instant twice", calls.nows)
	}
}

func TestTimeCommandErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantErr   error
		wantMsg   string
		wantUsage bool
	}{
		{name: "f and layout conflict", args: []string{"-f", "unix", "-layout", "2006"}, wantUsage: true},
		{name: "f and json conflict", args: []string{"-f", "unix", "-json"}, wantUsage: true},
		{name: "layout and json conflict", args: []string{"-layout", "2006", "-json"}, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantUsage: true},
		{name: "unknown zone", args: []string{"-z", "Mars/Olympus"}, wantErr: errTestZone, wantUsage: true},
		{name: "unknown zone in add", args: []string{"-z", "Mars/Olympus", "add", "x", "P1D"}, wantErr: errTestZone, wantUsage: true},
		{name: "unknown format", args: []string{"-f", "bogus"}, wantErr: errTestFormat, wantUsage: true},
		{name: "unknown format in add", args: []string{"-f", "bogus", "add", "x", "P1D"}, wantErr: errTestFormat, wantUsage: true},
		{name: "unknown format beats a bad timestamp", args: []string{"-f", "bogus", "add", "bad", "P1D"}, wantErr: errTestFormat, wantUsage: true},
		{name: "unknown format with diff", args: []string{"-f", "bogus", "diff", "a", "b"}, wantErr: errTestFormat, wantUsage: true},
		{name: "two arguments without a mode", args: []string{"a", "b"}, wantMsg: `"a" is not a mode (add or diff), and a plain timestamp takes no other arguments; got 2`, wantUsage: true},
		{name: "flag after a plain timestamp", args: []string{"1755459000", "-z", "UTC"}, wantMsg: "flag -z must come before the timestamp", wantUsage: true},
		{name: "unknown mode", args: []string{"sub", "x", "P1D"}, wantMsg: `"sub" is not a mode (add or diff)`, wantUsage: true},
		{name: "add without arguments", args: []string{"add"}, wantMsg: "takes a timestamp and a duration, got 0", wantUsage: true},
		{name: "add with one argument", args: []string{"add", "x"}, wantMsg: "got 1", wantUsage: true},
		{name: "add with three arguments", args: []string{"add", "x", "P1D", "P2D"}, wantMsg: "got 3", wantUsage: true},
		{name: "flag after add", args: []string{"add", "x", "P1D", "-json"}, wantMsg: "flag -json must come before the mode: agent-utils time [flags] add ...", wantUsage: true},
		{name: "flag with value after add", args: []string{"add", "x", "-z=UTC"}, wantMsg: "flag -z=UTC must come before the mode", wantUsage: true},
		{name: "double-dash flag after diff", args: []string{"diff", "--json", "a", "b"}, wantMsg: "flag --json must come before the mode", wantUsage: true},
		{name: "diff with one argument", args: []string{"diff", "a"}, wantMsg: "takes two timestamps (from and to), got 1", wantUsage: true},
		{name: "diff with f", args: []string{"-f", "unix", "diff", "a", "b"}, wantMsg: "-f and -layout format timestamps", wantUsage: true},
		{name: "diff with layout", args: []string{"-layout", "2006", "diff", "a", "b"}, wantMsg: "-f and -layout format timestamps", wantUsage: true},
		{name: "malformed timestamp", args: []string{"bad"}, wantErr: errTestParse},
		{name: "malformed timestamp in add", args: []string{"add", "bad", "P1D"}, wantErr: errTestParse},
		{name: "malformed duration", args: []string{"add", "x", "bad"}, wantErr: errTestDuration},
		{name: "malformed from", args: []string{"diff", "bad", "b"}, wantErr: errTestParse},
		{name: "malformed to", args: []string{"diff", "a", "bad"}, wantErr: errTestParse},
		{name: "diff json failure", args: []string{"-json", "diff", "jsonfail", "b"}, wantErr: errTestDiffJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls timeCalls
			cmd := newTestTimeCommand(&calls, time.Now)
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("Run() expected an error, got nil (stdout %q)", stdout.String())
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Run() error = %v, want it to wrap %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Run() error = %q, want it to contain %q", err, tt.wantMsg)
			}
			var usageErr *UsageError
			if got := errors.As(err, &usageErr); got != tt.wantUsage {
				t.Errorf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("Run() wrote %q to stdout on failure", stdout.String())
			}
		})
	}
}

func TestTimeCommandHelpAfterMode(t *testing.T) {
	for _, args := range [][]string{{"add", "-h"}, {"diff", "a", "b", "--help"}} {
		var calls timeCalls
		err := newTestTimeCommand(&calls, time.Now).Run(args, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
		var help *helpRequest
		if !errors.As(err, &help) {
			t.Errorf("Run(%q) error = %v, want a help request", args, err)
		}
	}
}
