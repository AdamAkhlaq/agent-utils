package clock

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// 1e9 seconds after the epoch: 2001-09-09T01:46:40Z, a Sunday.
var ref = time.Unix(1_000_000_000, 0).UTC()

func TestParse(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr string
	}{
		{name: "now keyword", input: "now", want: now},
		{name: "epoch seconds", input: "1000000000", want: ref},
		{name: "epoch milliseconds", input: "1000000000000", want: ref},
		{name: "negative epoch", input: "-1", want: time.Unix(-1, 0).UTC()},
		{name: "just below ms threshold is seconds", input: "99999999999", want: time.Unix(99_999_999_999, 0).UTC()},
		{name: "at ms threshold is milliseconds", input: "100000000000", want: time.UnixMilli(100_000_000_000).UTC()},
		{name: "rfc3339", input: "2001-09-09T01:46:40Z", want: ref},
		{name: "rfc3339 with offset", input: "2001-09-08T21:46:40-04:00", want: ref},
		{name: "date and time", input: "2001-09-09 01:46:40", want: ref},
		{name: "date and time with T", input: "2001-09-09T01:46:40", want: ref},
		{name: "date and minutes", input: "2001-09-09 01:46", want: ref.Truncate(time.Minute)},
		{name: "bare date", input: "2001-09-09", want: time.Date(2001, 9, 9, 0, 0, 0, 0, time.UTC)},
		{name: "rfc1123 with numeric zone", input: "Sun, 09 Sep 2001 01:46:40 +0000", want: ref},
		{name: "surrounding whitespace", input: " 1000000000\n", want: ref},
		{name: "empty input", input: "", wantErr: "empty timestamp"},
		{name: "garbage", input: "yesterday-ish", wantErr: "unrecognized timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input, now)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Parse() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		layout  string
		zone    string
		want    string
		wantErr string
	}{
		{name: "rfc3339 utc", format: "rfc3339", want: "2001-09-09T01:46:40Z"},
		{name: "unix", format: "unix", want: "1000000000"},
		{name: "unix-ms", format: "unix-ms", want: "1000000000000"},
		{name: "date", format: "date", want: "2001-09-09"},
		{name: "time", format: "time", want: "01:46:40"},
		{name: "rfc3339 in new york", format: "rfc3339", zone: "America/New_York", want: "2001-09-08T21:46:40-04:00"},
		{name: "unix ignores zone", format: "unix", zone: "America/New_York", want: "1000000000"},
		{name: "custom layout", layout: "2006/01/02 15:04", want: "2001/09/09 01:46"},
		{name: "unknown format", format: "iso", wantErr: "unknown format"},
		{name: "unknown zone", format: "rfc3339", zone: "Mars/Olympus", wantErr: "unknown timezone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format(ref, tt.format, tt.layout, tt.zone)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Format() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Format() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestZone(t *testing.T) {
	for _, name := range []string{"", "UTC", "utc"} {
		loc, err := Zone(name)
		if err != nil || loc != time.UTC {
			t.Errorf("Zone(%q) = %v, %v; want UTC, nil", name, loc, err)
		}
	}
	if loc, err := Zone("local"); err != nil || loc != time.Local {
		t.Errorf("Zone(local) = %v, %v; want Local, nil", loc, err)
	}
}

func TestJSON(t *testing.T) {
	out, err := JSON(ref, "America/New_York")
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	var got struct {
		Unix    int64  `json:"unix"`
		UnixMs  int64  `json:"unix_ms"`
		RFC3339 string `json:"rfc3339"`
		UTC     string `json:"utc"`
		Date    string `json:"date"`
		Time    string `json:"time"`
		Weekday string `json:"weekday"`
		Zone    string `json:"zone"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON() output is not valid JSON: %v\n%s", err, out)
	}
	if got.Unix != 1_000_000_000 || got.UnixMs != 1_000_000_000_000 {
		t.Errorf("JSON() unix = %d, unix_ms = %d", got.Unix, got.UnixMs)
	}
	if got.RFC3339 != "2001-09-08T21:46:40-04:00" {
		t.Errorf("JSON() rfc3339 = %q", got.RFC3339)
	}
	if got.UTC != "2001-09-09T01:46:40Z" {
		t.Errorf("JSON() utc = %q", got.UTC)
	}
	if got.Date != "2001-09-08" || got.Weekday != "Saturday" {
		t.Errorf("JSON() date = %q, weekday = %q; want the New York date and weekday", got.Date, got.Weekday)
	}
	if got.Zone != "America/New_York" {
		t.Errorf("JSON() zone = %q", got.Zone)
	}

	if _, err := JSON(ref, "Nowhere/Void"); err == nil {
		t.Error("JSON() expected an error for an unknown zone")
	}
}
