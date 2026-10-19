package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// The stubs record what the core receives and return canned output, so these
// tests exercise only the CLI layer's flag handling and error mapping.
func TestCronCommand(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	parsedFrom := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)

	tests := []struct {
		name       string
		args       []string
		stdin      string
		want       string
		wantExpr   string
		wantFrom   time.Time
		wantFromIn string
		wantLoc    *time.Location
		wantN      int
		wantJSON   bool
		wantErr    string
		wantUsage  bool
	}{
		{name: "argument with defaults", args: []string{"*/5 * * * *"}, want: "runs\n", wantExpr: "*/5 * * * *", wantFrom: fixedNow, wantFromIn: "now", wantLoc: time.UTC, wantN: 5},
		{name: "stdin strips one trailing newline", stdin: "0 9 * * 1\n\n", want: "runs\n", wantExpr: "0 9 * * 1\n", wantFrom: fixedNow, wantFromIn: "now", wantLoc: time.UTC, wantN: 5},
		{name: "stdin strips one windows line ending", stdin: "0 9 * * 1\r\n", want: "runs\n", wantExpr: "0 9 * * 1", wantFrom: fixedNow, wantFromIn: "now", wantLoc: time.UTC, wantN: 5},
		{name: "empty stdin reaches the core", stdin: "", want: "runs\n", wantExpr: "", wantFrom: fixedNow, wantFromIn: "now", wantLoc: time.UTC, wantN: 5},
		{name: "flags plumbed", args: []string{"-n", "3", "-z", "Asia/Tokyo", "-from", "1000000000", "@daily"}, want: "runs\n", wantExpr: "@daily", wantFrom: parsedFrom, wantFromIn: "1000000000", wantLoc: tokyo, wantN: 3},
		{name: "json mode", args: []string{"-json", "-n", "1000", "@hourly"}, want: "{\"json\"}\n", wantExpr: "@hourly", wantFrom: fixedNow, wantFromIn: "now", wantLoc: time.UTC, wantN: 1000, wantJSON: true},

		{name: "core failure is a runtime error", args: []string{"bad"}, wantErr: "cannot parse"},
		{name: "count zero", args: []string{"-n", "0", "* * * * *"}, wantErr: "-n must be between 1 and 1000, got 0", wantUsage: true},
		{name: "count too large", args: []string{"-n", "1001", "* * * * *"}, wantErr: "-n must be between 1 and 1000, got 1001", wantUsage: true},
		{name: "count not a number", args: []string{"-n", "many", "* * * * *"}, wantErr: "invalid value", wantUsage: true},
		{name: "unknown zone", args: []string{"-z", "Mars/Olympus", "* * * * *"}, wantErr: "unknown timezone", wantUsage: true},
		{name: "bad from", args: []string{"-from", "yesterday", "* * * * *"}, wantErr: "-from: unrecognized timestamp", wantUsage: true},
		{name: "unquoted expression", args: []string{"0", "9", "*", "*", "1"}, wantErr: "expected the expression as one quoted argument, got 5 arguments", wantUsage: true},
		{name: "flag after the expression", args: []string{"* * * * *", "-n", "3"}, wantErr: `flags must come before the expression, got "-n" after it`, wantUsage: true},
		{name: "unknown flag", args: []string{"-x"}, wantErr: "flag provided but not defined", wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotExpr, gotFromIn string
			var gotFrom time.Time
			var gotLoc, gotFromLoc *time.Location
			gotN := 0
			gotJSON := false
			core := func(json bool, out string) func(string, time.Time, *time.Location, int) (string, error) {
				return func(expr string, from time.Time, loc *time.Location, n int) (string, error) {
					if expr == "bad" {
						return "", errors.New("cannot parse")
					}
					gotExpr, gotFrom, gotLoc, gotN, gotJSON = expr, from, loc, n, json
					return out, nil
				}
			}
			cmd := CronCommand(
				func() time.Time { return fixedNow },
				func(name string) (*time.Location, error) {
					switch name {
					case "UTC":
						return time.UTC, nil
					case "Asia/Tokyo":
						return tokyo, nil
					}
					return nil, errors.New("unknown timezone")
				},
				func(s string, now time.Time, loc *time.Location) (time.Time, error) {
					gotFromIn, gotFromLoc = s, loc
					switch s {
					case "now":
						return now, nil
					case "1000000000":
						return parsedFrom, nil
					}
					return time.Time{}, errors.New("unrecognized timestamp")
				},
				core(false, "runs"),
				core(true, `{"json"}`),
			)
			var stdout bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, io.Discard)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Run() error = %q, want it to contain %q", err, tt.wantErr)
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
			if gotFromLoc != gotLoc {
				t.Errorf("-from parsed in %v, schedule evaluated in %v; want the same zone", gotFromLoc, gotLoc)
			}
			if gotExpr != tt.wantExpr || !gotFrom.Equal(tt.wantFrom) || gotFromIn != tt.wantFromIn || gotLoc != tt.wantLoc || gotN != tt.wantN || gotJSON != tt.wantJSON {
				t.Errorf("core called with (expr=%q, from=%v via %q, loc=%v, n=%d, json=%v), want (%q, %v via %q, %v, %d, %v)",
					gotExpr, gotFrom, gotFromIn, gotLoc, gotN, gotJSON,
					tt.wantExpr, tt.wantFrom, tt.wantFromIn, tt.wantLoc, tt.wantN, tt.wantJSON)
			}
		})
	}
}

func TestCronCommandStdinError(t *testing.T) {
	never := func(string, time.Time, *time.Location, int) (string, error) {
		t.Fatal("core called despite a stdin read error")
		return "", nil
	}
	cmd := CronCommand(
		time.Now,
		func(string) (*time.Location, error) { return time.UTC, nil },
		func(string, time.Time, *time.Location) (time.Time, error) { return time.Time{}, nil },
		never, never,
	)
	err := cmd.Run(nil, iotest.ErrReader(errors.New("boom")), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "reading input: boom") {
		t.Fatalf("Run() error = %v, want a reading input error", err)
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		t.Error("Run() returned a usage error; a failed read must exit 1, not 2")
	}
}
