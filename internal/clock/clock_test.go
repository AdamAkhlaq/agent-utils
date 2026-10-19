package clock

import (
	"encoding/json"
	"math"
	"math/rand/v2"
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

func TestParseIn(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		input   string
		zone    string
		want    string
		wantErr string
	}{
		{name: "bare date is midnight in the zone", input: "2024-01-31", zone: "America/New_York", want: "2024-01-31T05:00:00Z"},
		{name: "date and time in the zone", input: "2024-07-01 12:30", zone: "Europe/London", want: "2024-07-01T11:30:00Z"},
		{name: "skipped wall time moves forward", input: "2024-03-10T02:30:00", zone: "America/New_York", want: "2024-03-10T07:30:00Z"},
		{name: "repeated wall time takes the earlier", input: "2024-11-03 01:30:00", zone: "America/New_York", want: "2024-11-03T05:30:00Z"},
		{name: "utc zone matches Parse", input: "2024-01-31", zone: "UTC", want: "2024-01-31T00:00:00Z"},
		{name: "offset wins over the zone", input: "2024-01-31T00:00:00+09:00", zone: "America/New_York", want: "2024-01-30T15:00:00Z"},
		{name: "epoch ignores the zone", input: "1000000000", zone: "Asia/Tokyo", want: "2001-09-09T01:46:40Z"},
		{name: "now ignores the zone", input: "now", zone: "Asia/Tokyo", want: "2026-08-17T12:00:00Z"},
		{name: "garbage", input: "soon", zone: "Asia/Tokyo", wantErr: "unrecognized timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIn(tt.input, now, mustZone(t, tt.zone))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseIn() = %v, %v; want an error containing %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseIn() error = %v", err)
			}
			if s := got.UTC().Format(time.RFC3339Nano); s != tt.want {
				t.Errorf("ParseIn(%q, %s) = %s, want %s", tt.input, tt.zone, s, tt.want)
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
		ISOYear int    `json:"iso_year"`
		ISOWeek int    `json:"iso_week"`
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
	if got.ISOYear != 2001 || got.ISOWeek != 36 {
		t.Errorf("JSON() iso_year = %d, iso_week = %d; want 2001, 36", got.ISOYear, got.ISOWeek)
	}

	// 2021-01-03 is a Sunday that ISO 8601 counts in the last week of 2020.
	out, err = JSON(time.Date(2021, 1, 3, 12, 0, 0, 0, time.UTC), "UTC")
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON() output is not valid JSON: %v\n%s", err, out)
	}
	if got.ISOYear != 2020 || got.ISOWeek != 53 || got.Weekday != "Sunday" {
		t.Errorf("JSON() iso_year = %d, iso_week = %d, weekday = %q; want 2020, 53, Sunday", got.ISOYear, got.ISOWeek, got.Weekday)
	}

	if _, err := JSON(ref, "Nowhere/Void"); err == nil {
		t.Error("JSON() expected an error for an unknown zone")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Duration
		wantErr string
	}{
		{name: "full iso", input: "P1Y2M10DT2H30M", want: Duration{Months: 14, Days: 10, Seconds: 9000}},
		{name: "weeks", input: "P2W", want: Duration{Days: 14}},
		{name: "weeks with other parts", input: "P1Y1W2D", want: Duration{Months: 12, Days: 9}},
		{name: "fractional seconds", input: "PT1.5S", want: Duration{Seconds: 1, Nanos: 500_000_000}},
		{name: "comma decimal separator", input: "PT0,25S", want: Duration{Nanos: 250_000_000}},
		{name: "nanosecond fraction", input: "PT0.000000001S", want: Duration{Nanos: 1}},
		{name: "negative iso", input: "-P1DT1H", want: Duration{Days: -1, Seconds: -3600}},
		{name: "explicit plus", input: "+P1D", want: Duration{Days: 1}},
		{name: "lowercase designators", input: "p1dt2h", want: Duration{Days: 1, Seconds: 7200}},
		{name: "hours beyond a day stay exact", input: "PT36H", want: Duration{Seconds: 129_600}},
		{name: "zero", input: "P0D", want: Duration{}},
		{name: "longer than go durations allow", input: "PT3000000H", want: Duration{Seconds: 10_800_000_000}},
		{name: "surrounding whitespace", input: " PT1H\n", want: Duration{Seconds: 3600}},
		{name: "go style", input: "1h30m", want: Duration{Seconds: 5400}},
		{name: "go style negative", input: "-90s", want: Duration{Seconds: -90}},
		{name: "go style negative fraction", input: "-1.5s", want: Duration{Seconds: -1, Nanos: -500_000_000}},
		{name: "go style milliseconds", input: "250ms", want: Duration{Nanos: 250_000_000}},
		{name: "go style zero", input: "0", want: Duration{}},
		{name: "empty", input: "", wantErr: "empty duration"},
		{name: "blank", input: "  ", wantErr: "empty duration"},
		{name: "sign alone", input: "-", wantErr: "no value after the sign"},
		{name: "double sign", input: "--1h", wantErr: "more than one sign"},
		{name: "P alone", input: "P", wantErr: "no components after P"},
		{name: "T without time parts", input: "P1DT", wantErr: "no hours, minutes, or seconds after T"},
		{name: "PT alone", input: "PT", wantErr: "no hours, minutes, or seconds after T"},
		{name: "hours in date part", input: "P2H", wantErr: `'H' is not a date designator`},
		{name: "days in time part", input: "PT1D", wantErr: `'D' is not a time designator`},
		{name: "out of order", input: "P1D1Y", wantErr: "Y is repeated or out of order"},
		{name: "repeated", input: "P1M1M", wantErr: "M is repeated or out of order"},
		{name: "fractional days", input: "P1.5D", wantErr: "only seconds may have a fraction, not D"},
		{name: "fractional hours", input: "PT1.5H", wantErr: "for example PT1H30M, not PT1.5H"},
		{name: "finer than nanoseconds", input: "PT0.0000000001S", wantErr: "finer than nanoseconds"},
		{name: "number without designator", input: "P1", wantErr: `number "1" has no designator`},
		{name: "designator without number", input: "PD", wantErr: "'D' has no number before it"},
		{name: "per-component sign", input: "P-1D", wantErr: "'-' has no number before it"},
		{name: "no digits after separator", input: "PT1.S", wantErr: "no digits after the decimal separator"},
		{name: "component overflows int64", input: "P99999999999999999999Y", wantErr: "too large to represent"},
		{name: "years overflow as months", input: "P768614336404564651Y", wantErr: "too large to represent"},
		{name: "hours overflow as seconds", input: "PT9223372036854775807H", wantErr: "too large to represent"},
		{name: "go style day unit", input: "3d", wantErr: `no "d" unit: use P3D for calendar days, keeping the wall-clock time, or 72h for exactly 72 hours`},
		{name: "go style negative day unit", input: "-3d", wantErr: "use -P3D for calendar days, keeping the wall-clock time, or -72h for exactly 72 hours"},
		{name: "go style finer than nanoseconds", input: "0.0000000005s", wantErr: "0.0000000005s is finer than nanoseconds"},
		{name: "go style fraction of a nanosecond", input: "1.5ns", wantErr: "1.5ns is finer than nanoseconds"},
		{name: "go style long fraction that is exact", input: "1.000000001s", want: Duration{Seconds: 1, Nanos: 1}},
		{name: "go style fractional hours", input: "1.5h", want: Duration{Seconds: 5400}},
		{name: "go style uppercase day unit", input: "3D", wantErr: `no "D" unit: use P3D`},
		{name: "go style week unit", input: "2w", wantErr: "use P2W for calendar weeks, keeping the wall-clock time, or 336h"},
		{name: "go style mixed day unit", input: "1d12h", wantErr: "use PnD for calendar days, keeping the wall-clock time, or hours (24 per day)"},
		{name: "go style fractional day unit", input: "1.5d", wantErr: "use PnD"},
		{name: "go style month unit", input: "1mo", wantErr: "use P1M for calendar months"},
		{name: "go style year unit", input: "1y", wantErr: "use P1Y for calendar years"},
		{name: "go style missing unit", input: "1h30", wantErr: "missing unit"},
		{name: "go style too long", input: "3000000h", wantErr: "limited to about 292 years"},
		{name: "garbage", input: "soon", wantErr: "want ISO 8601"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseDuration(%q) = %+v, want an error", tt.input, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseDuration(%q) error = %q, want it to contain %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDuration(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseDuration(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDurationString(t *testing.T) {
	tests := []struct {
		name string
		d    Duration
		want string
	}{
		{name: "zero", d: Duration{}, want: "PT0S"},
		{name: "all parts", d: Duration{Months: 14, Days: 10, Seconds: 9006, Nanos: 500_000_000}, want: "P1Y2M10DT2H30M6.5S"},
		{name: "whole years", d: Duration{Months: 24}, want: "P2Y"},
		{name: "negative", d: Duration{Months: -1, Days: -1, Seconds: -43_200}, want: "-P1M1DT12H"},
		{name: "exact hours are not folded into days", d: Duration{Seconds: 2 * 86_400}, want: "PT48H"},
		{name: "go style negative", d: Duration{Seconds: -90}, want: "-PT1M30S"},
		{name: "one nanosecond", d: Duration{Nanos: 1}, want: "PT0.000000001S"},
		{name: "negative fraction", d: Duration{Seconds: -1, Nanos: -500_000_000}, want: "-PT1.5S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.d.String()
			if got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
			back, err := ParseDuration(got)
			if err != nil {
				t.Fatalf("ParseDuration(%q) error = %v", got, err)
			}
			if back != tt.d {
				t.Errorf("ParseDuration(%q) = %+v, want the original %+v", got, back, tt.d)
			}
		})
	}
}

func mustZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := Zone(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		start    string
		duration string
		want     string
		wantErr  string
	}{
		{name: "month end clamps in a leap year", start: "2024-01-31T00:00:00Z", duration: "P1M", want: "2024-02-29T00:00:00Z"},
		{name: "month end clamps in a common year", start: "2023-01-31T00:00:00Z", duration: "P1M", want: "2023-02-28T00:00:00Z"},
		{name: "leap day plus a year clamps", start: "2024-02-29T00:00:00Z", duration: "P1Y", want: "2025-02-28T00:00:00Z"},
		{name: "leap day plus four years", start: "2024-02-29T00:00:00Z", duration: "P4Y", want: "2028-02-29T00:00:00Z"},
		{name: "years and months apply together", start: "2024-02-29T00:00:00Z", duration: "P1Y1M", want: "2025-03-29T00:00:00Z"},
		{name: "days apply after the month clamps", start: "2024-01-31T00:00:00Z", duration: "P1M1D", want: "2024-03-01T00:00:00Z"},
		{name: "negative month clamps", start: "2024-03-31T00:00:00Z", duration: "-P1M", want: "2024-02-29T00:00:00Z"},
		{name: "negative crosses a year", start: "2024-01-15T00:00:00Z", duration: "-P1M", want: "2023-12-15T00:00:00Z"},
		{name: "weeks", start: "2024-01-01T00:00:00Z", duration: "P2W", want: "2024-01-15T00:00:00Z"},
		{name: "forty-five days", start: "2026-10-19T00:00:00Z", duration: "P45D", want: "2026-12-03T00:00:00Z"},
		{name: "time part crosses midnight", start: "2024-12-31T23:00:00Z", duration: "PT2H", want: "2025-01-01T01:00:00Z"},
		{name: "fractional seconds", start: "2024-01-01T00:00:00Z", duration: "PT1.5S", want: "2024-01-01T00:00:01.5Z"},
		{name: "go style", start: "2024-01-01T00:00:00Z", duration: "1h30m", want: "2024-01-01T01:30:00Z"},
		{name: "go style negative", start: "2024-01-01T00:00:00Z", duration: "-90s", want: "2023-12-31T23:58:30Z"},
		{name: "beyond go duration range", start: "2000-01-01T00:00:00Z", duration: "PT3000000H", want: "2342-03-29T00:00:00Z"},
		{name: "into year zero", start: "0001-01-01T00:00:00Z", duration: "-P1Y", want: "0000-01-01T00:00:00Z"},
		{name: "calendar day across spring forward is 23h", zone: "America/New_York", start: "2024-03-09T12:00:00-05:00", duration: "P1D", want: "2024-03-10T12:00:00-04:00"},
		{name: "exact day across spring forward", zone: "America/New_York", start: "2024-03-09T12:00:00-05:00", duration: "PT24H", want: "2024-03-10T13:00:00-04:00"},
		{name: "go style day across spring forward", zone: "America/New_York", start: "2024-03-09T12:00:00-05:00", duration: "24h", want: "2024-03-10T13:00:00-04:00"},
		{name: "calendar day across fall back is 25h", zone: "America/New_York", start: "2024-11-02T12:00:00-04:00", duration: "P1D", want: "2024-11-03T12:00:00-05:00"},
		{name: "skipped time moves forward", zone: "America/New_York", start: "2024-03-09T02:30:00-05:00", duration: "P1D", want: "2024-03-10T03:30:00-04:00"},
		{name: "repeated time takes the earlier", zone: "America/New_York", start: "2024-11-02T01:30:00-04:00", duration: "P1D", want: "2024-11-03T01:30:00-04:00"},
		{name: "london skipped time moves forward", zone: "Europe/London", start: "2024-03-30T01:30:00Z", duration: "P1D", want: "2024-03-31T02:30:00+01:00"},
		{name: "london repeated time takes the earlier", zone: "Europe/London", start: "2024-10-26T01:30:00+01:00", duration: "P1D", want: "2024-10-27T01:30:00+01:00"},
		{name: "calendar parts use the zone's wall clock", zone: "Asia/Tokyo", start: "2024-01-31T15:00:00Z", duration: "P1M", want: "2024-03-01T00:00:00+09:00"},
		{name: "before the first representable instant", start: "1970-01-01T00:00:00Z", duration: "-P292300000Y", wantErr: "out of range"},
		{name: "months beyond any range", start: "2024-01-01T00:00:00Z", duration: "P9000000000Y", wantErr: "out of range"},
		{name: "days beyond any range", start: "2024-01-01T00:00:00Z", duration: "P300000000000D", wantErr: "out of range"},
		{name: "seconds overflow", start: "2024-01-01T00:00:00Z", duration: "PT9223372036854775807S", wantErr: "out of range"},
		{name: "negative seconds overflow", start: "2024-01-01T00:00:00Z", duration: "-PT9223372036854775807S", wantErr: "out of range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := mustZone(t, tt.zone)
			start := mustTime(t, tt.start)
			d, err := ParseDuration(tt.duration)
			if err != nil {
				t.Fatalf("ParseDuration(%q) error = %v", tt.duration, err)
			}
			got, err := Add(start, d, loc)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Add() = %v, want an error", got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Add() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if s := got.Format(time.RFC3339Nano); s != tt.want {
				t.Errorf("Add(%s, %s) = %s, want %s", tt.start, tt.duration, s, tt.want)
			}
		})
	}
}

// The range ends are the int64 epoch-millisecond limits, which RFC 3339
// strings cannot reach.
func TestAddRange(t *testing.T) {
	tests := []struct {
		name    string
		start   time.Time
		d       Duration
		want    time.Time
		wantErr bool
	}{
		{name: "last instant", start: maxTime, d: Duration{}, want: maxTime},
		{name: "first instant", start: minTime, d: Duration{}, want: minTime},
		{name: "past the last instant", start: maxTime, d: Duration{Nanos: 1_000_000}, wantErr: true},
		{name: "before the first instant", start: minTime, d: Duration{Nanos: -1_000_000}, wantErr: true},
		{name: "a month past the last instant", start: maxTime, d: Duration{Months: 1}, wantErr: true},
		{name: "a day before the first instant", start: minTime, d: Duration{Days: -1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Add(tt.start, tt.d, time.UTC)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "out of range") {
					t.Fatalf("Add() = %v, %v; want an out of range error", got, err)
				}
				return
			}
			if err != nil || !got.Equal(tt.want) {
				t.Errorf("Add() = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		zone string
		from string
		to   string
		want string
	}{
		{name: "same instant", from: "2024-01-01T00:00:00Z", to: "2024-01-01T00:00:00Z", want: "PT0S"},
		{name: "months then days then time", from: "2024-01-31T00:00:00Z", to: "2024-03-01T12:00:00Z", want: "P1M1DT12H"},
		{name: "clamped month end counts as a month", from: "2024-01-31T00:00:00Z", to: "2024-02-29T00:00:00Z", want: "P1M"},
		{name: "backwards", from: "2024-03-01T00:00:00Z", to: "2024-01-31T00:00:00Z", want: "-P1M1D"},
		{name: "across a leap day", from: "2023-02-28T00:00:00Z", to: "2024-02-29T00:00:00Z", want: "P1Y1D"},
		{name: "leap day to the next year", from: "2024-02-29T00:00:00Z", to: "2025-02-28T00:00:00Z", want: "P1Y"},
		{name: "to christmas", from: "2026-01-01T00:00:00Z", to: "2026-12-25T00:00:00Z", want: "P11M24D"},
		{name: "under a day", from: "2024-01-01T23:00:00Z", to: "2024-01-02T01:00:00Z", want: "PT2H"},
		{name: "fractional seconds", from: "2024-01-01T00:00:00Z", to: "2024-01-01T00:00:01.5Z", want: "PT1.5S"},
		{name: "negative fractional seconds", from: "2024-01-01T00:00:01.5Z", to: "2024-01-01T00:00:00Z", want: "-PT1.5S"},
		{name: "nanoseconds borrow", from: "2024-01-01T00:00:00.7Z", to: "2024-01-01T00:00:01.2Z", want: "PT0.5S"},
		{name: "calendar day across spring forward", zone: "America/New_York", from: "2024-03-09T12:00:00-05:00", to: "2024-03-10T12:00:00-04:00", want: "P1D"},
		{name: "same instants in utc", from: "2024-03-09T12:00:00-05:00", to: "2024-03-10T12:00:00-04:00", want: "PT23H"},
		{name: "exact time inside the dst day", zone: "America/New_York", from: "2024-03-10T00:00:00-05:00", to: "2024-03-10T05:00:00-04:00", want: "PT4H"},
		{name: "across the fold", zone: "America/New_York", from: "2024-11-03T01:30:00-04:00", to: "2024-11-03T01:10:00-05:00", want: "PT40M"},
		{name: "whole calendar range", from: "0001-01-01T00:00:00Z", to: "9999-12-31T23:59:59Z", want: "P9998Y11M30DT23H59M59S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := mustZone(t, tt.zone)
			from, to := mustTime(t, tt.from), mustTime(t, tt.to)
			d := Diff(from, to, loc)
			if got := d.String(); got != tt.want {
				t.Errorf("Diff() = %s, want %s", got, tt.want)
			}
			if back, err := Add(from, d, loc); err != nil || !back.Equal(to) {
				t.Errorf("Add(from, Diff()) = %v, %v; want %v", back, err, to)
			}
		})
	}
}

// TestDiffFullRange covers spans time.Time.Sub would saturate on.
func TestDiffFullRange(t *testing.T) {
	for _, pair := range [][2]time.Time{{minTime, maxTime}, {maxTime, minTime}} {
		from, to := pair[0], pair[1]
		d := Diff(from, to, time.UTC)
		if back, err := Add(from, d, time.UTC); err != nil || !back.Equal(to) {
			t.Errorf("Add(%v, %s) = %v, %v; want %v", from, d, back, err, to)
		}
		out, err := DiffJSON(from, to, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			TotalSeconds json.Number `json:"total_seconds"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		sec, nanos := elapsed(from, to)
		if want := decimalSeconds(sec, nanos); string(got.TotalSeconds) != want {
			t.Errorf("total_seconds = %s, want %s", got.TotalSeconds, want)
		}
	}
	if sec, _ := elapsed(minTime, maxTime); sec != 18_446_744_073_709_551 {
		t.Errorf("elapsed(minTime, maxTime) = %d s, want 18446744073709551", sec)
	}
}

// TestDiffProperties checks, over random instants in zones with DST, a
// half-hour DST, and a skipped day, that Diff inverts Add, that its months
// and days are the most that fit, and that its text form parses back.
func TestDiffProperties(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	lo := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	hi := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	random := func() time.Time {
		return time.Unix(lo+rng.Int64N(hi-lo), rng.Int64N(1e9))
	}
	for _, zone := range []string{"UTC", "America/New_York", "Europe/London", "Australia/Lord_Howe", "Pacific/Apia", "Asia/Kolkata"} {
		loc := mustZone(t, zone)
		t.Run(zone, func(t *testing.T) {
			for range 2000 {
				from := random()
				to := random()
				if rng.IntN(2) == 0 {
					// Close pairs exercise the days and time parts.
					to = from.Add(time.Duration(rng.Int64N(int64(72*time.Hour))) - 36*time.Hour)
				}
				d := Diff(from, to, loc)
				back, err := Add(from, d, loc)
				if err != nil || !back.Equal(to) {
					t.Fatalf("Add(%v, Diff() = %s) = %v, %v; want %v", from, d, back, err, to)
				}
				sign := int64(to.Compare(from))
				if !passes(shift(from, d.Months+sign, 0, loc), to, sign) && sign != 0 {
					t.Fatalf("Diff(%v, %v) = %s: one more month does not pass to", from, to, d)
				}
				if !passes(shift(from, d.Months, d.Days+sign, loc), to, sign) && sign != 0 {
					t.Fatalf("Diff(%v, %v) = %s: one more day does not pass to", from, to, d)
				}
				for _, part := range []int64{d.Months, d.Days, d.Seconds, d.Nanos} {
					if part*sign < 0 {
						t.Fatalf("Diff(%v, %v) = %+v: parts disagree in sign", from, to, d)
					}
				}
				if parsed, err := ParseDuration(d.String()); err != nil || parsed != d {
					t.Fatalf("ParseDuration(%s) = %+v, %v; want %+v", d, parsed, err, d)
				}
			}
		})
	}
}

func TestDiffJSON(t *testing.T) {
	type calendar struct {
		Years   int64       `json:"years"`
		Months  int64       `json:"months"`
		Days    int64       `json:"days"`
		Hours   int64       `json:"hours"`
		Minutes int64       `json:"minutes"`
		Seconds json.Number `json:"seconds"`
	}
	type repr struct {
		From         string      `json:"from"`
		To           string      `json:"to"`
		Zone         string      `json:"zone"`
		Duration     string      `json:"duration"`
		TotalSeconds json.Number `json:"total_seconds"`
		TotalDays    int64       `json:"total_days"`
		Calendar     calendar    `json:"calendar"`
	}
	tests := []struct {
		name string
		zone string
		from string
		to   string
		want repr
	}{
		{
			name: "month across spring forward in new york",
			zone: "America/New_York",
			from: "2024-03-01T05:00:00Z",
			to:   "2024-04-01T04:00:00Z",
			want: repr{
				From: "2024-03-01T00:00:00-05:00", To: "2024-04-01T00:00:00-04:00", Zone: "America/New_York",
				Duration: "P1M", TotalSeconds: "2674800", TotalDays: 31,
				Calendar: calendar{Months: 1, Seconds: "0"},
			},
		},
		{
			name: "same instants in utc",
			from: "2024-03-01T05:00:00Z",
			to:   "2024-04-01T04:00:00Z",
			want: repr{
				From: "2024-03-01T05:00:00Z", To: "2024-04-01T04:00:00Z", Zone: "UTC",
				Duration: "P30DT23H", TotalSeconds: "2674800", TotalDays: 30,
				Calendar: calendar{Days: 30, Hours: 23, Seconds: "0"},
			},
		},
		{
			name: "negative with every part",
			from: "2025-03-02T03:04:05.5Z",
			to:   "2024-01-01T00:00:00Z",
			want: repr{
				From: "2025-03-02T03:04:05.5Z", To: "2024-01-01T00:00:00Z", Zone: "UTC",
				Duration: "-P1Y2M1DT3H4M5.5S", TotalSeconds: "-36817445.5", TotalDays: -426,
				Calendar: calendar{Years: -1, Months: -2, Days: -1, Hours: -3, Minutes: -4, Seconds: "-5.5"},
			},
		},
		{
			name: "zero",
			from: "2024-01-01T00:00:00Z",
			to:   "2024-01-01T00:00:00Z",
			want: repr{
				From: "2024-01-01T00:00:00Z", To: "2024-01-01T00:00:00Z", Zone: "UTC",
				Duration: "PT0S", TotalSeconds: "0", Calendar: calendar{Seconds: "0"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := DiffJSON(mustTime(t, tt.from), mustTime(t, tt.to), mustZone(t, tt.zone))
			if err != nil {
				t.Fatalf("DiffJSON() error = %v", err)
			}
			var got repr
			dec := json.NewDecoder(strings.NewReader(out))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&got); err != nil {
				t.Fatalf("DiffJSON() output does not match the documented shape: %v\n%s", err, out)
			}
			if got != tt.want {
				t.Errorf("DiffJSON() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDecimalSeconds(t *testing.T) {
	tests := []struct {
		sec, nanos int64
		want       string
	}{
		{0, 0, "0"},
		{90, 0, "90"},
		{-1, -500_000_000, "-1.5"},
		{0, -1, "-0.000000001"},
		{math.MaxInt64, 999_999_999, "9223372036854775807.999999999"},
	}
	for _, tt := range tests {
		if got := decimalSeconds(tt.sec, tt.nanos); got != tt.want {
			t.Errorf("decimalSeconds(%d, %d) = %q, want %q", tt.sec, tt.nanos, got, tt.want)
		}
	}
}
