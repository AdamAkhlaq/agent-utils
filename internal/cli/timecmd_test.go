package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// The stubs record their arguments and return canned output, so these tests
// exercise only the CLI layer's flag handling and error mapping.
func TestTimeCommand(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	parsed := time.Date(2001, 9, 9, 1, 46, 40, 0, time.UTC)

	tests := []struct {
		name       string
		args       []string
		want       string
		wantInput  string
		wantT      time.Time
		wantName   string
		wantLayout string
		wantZone   string
		wantJSON   bool
		wantErr    bool
		wantUsage  bool
	}{
		{name: "no argument formats now", args: nil, want: "formatted\n", wantT: fixedNow, wantName: "rfc3339", wantZone: "UTC"},
		{name: "argument is parsed", args: []string{"1000000000"}, want: "formatted\n", wantInput: "1000000000", wantT: parsed, wantName: "rfc3339", wantZone: "UTC"},
		{name: "zone and format plumbed", args: []string{"-z", "Asia/Tokyo", "-f", "unix", "x"}, want: "formatted\n", wantInput: "x", wantT: parsed, wantName: "unix", wantZone: "Asia/Tokyo"},
		{name: "layout plumbed", args: []string{"-layout", "2006", "x"}, want: "formatted\n", wantInput: "x", wantT: parsed, wantName: "rfc3339", wantLayout: "2006", wantZone: "UTC"},
		{name: "json mode", args: []string{"-json"}, want: "{\"all\"}\n", wantT: fixedNow, wantJSON: true, wantZone: "UTC"},
		{name: "f and layout conflict", args: []string{"-f", "unix", "-layout", "2006"}, wantErr: true, wantUsage: true},
		{name: "f and json conflict", args: []string{"-f", "unix", "-json"}, wantErr: true, wantUsage: true},
		{name: "layout and json conflict", args: []string{"-layout", "2006", "-json"}, wantErr: true, wantUsage: true},
		{name: "two arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotInput, gotName, gotLayout, gotZone string
			var gotT time.Time
			gotJSON := false
			cmd := TimeCommand(
				func() time.Time { return fixedNow },
				func(s string, now time.Time) (time.Time, error) {
					gotInput = s
					return parsed, nil
				},
				func(tm time.Time, name, layout, zone string) (string, error) {
					gotT, gotName, gotLayout, gotZone = tm, name, layout, zone
					return "formatted", nil
				},
				func(tm time.Time, zone string) (string, error) {
					gotT, gotZone, gotJSON = tm, zone, true
					return `{"all"}`, nil
				},
			)
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
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
			if gotInput != tt.wantInput || !gotT.Equal(tt.wantT) || gotName != tt.wantName ||
				gotLayout != tt.wantLayout || gotZone != tt.wantZone || gotJSON != tt.wantJSON {
				t.Errorf("Run() called core with (input=%q, t=%v, name=%q, layout=%q, zone=%q, json=%v), want (%q, %v, %q, %q, %q, %v)",
					gotInput, gotT, gotName, gotLayout, gotZone, gotJSON,
					tt.wantInput, tt.wantT, tt.wantName, tt.wantLayout, tt.wantZone, tt.wantJSON)
			}
		})
	}
}

func TestTimeCommandErrorMapping(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	parseErr := errors.New("unrecognized timestamp")
	formatErr := errors.New("unknown timezone")

	t.Run("parse failure is a runtime error", func(t *testing.T) {
		cmd := TimeCommand(
			func() time.Time { return fixedNow },
			func(s string, now time.Time) (time.Time, error) { return time.Time{}, parseErr },
			func(tm time.Time, name, layout, zone string) (string, error) { return "", nil },
			func(tm time.Time, zone string) (string, error) { return "", nil },
		)
		var stdout, stderr bytes.Buffer
		err := cmd.Run([]string{"garbage"}, strings.NewReader(""), &stdout, &stderr)
		if !errors.Is(err, parseErr) {
			t.Fatalf("Run() error = %v, want %v", err, parseErr)
		}
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			t.Error("Run() returned a usage error; bad input data must exit 1, not 2")
		}
	})

	t.Run("format failure is a usage error", func(t *testing.T) {
		cmd := TimeCommand(
			func() time.Time { return fixedNow },
			func(s string, now time.Time) (time.Time, error) { return fixedNow, nil },
			func(tm time.Time, name, layout, zone string) (string, error) { return "", formatErr },
			func(tm time.Time, zone string) (string, error) { return "", nil },
		)
		var stdout, stderr bytes.Buffer
		err := cmd.Run([]string{"-z", "Mars/Olympus"}, strings.NewReader(""), &stdout, &stderr)
		if !errors.Is(err, formatErr) {
			t.Fatalf("Run() error = %v, want %v", err, formatErr)
		}
		var usageErr *UsageError
		if !errors.As(err, &usageErr) {
			t.Error("Run() error is not a usage error; a bad flag value must exit 2")
		}
	})
}
