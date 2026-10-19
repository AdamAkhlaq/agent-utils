package clock

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// cronFrom is a Thursday, the start of every UTC case unless one says
// otherwise.
var cronFrom = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func cronZone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParseCron(t *testing.T) {
	all := func(lo, hi int) []int {
		var vals []int
		for v := lo; v <= hi; v++ {
			vals = append(vals, v)
		}
		return vals
	}
	tests := []struct {
		name    string
		expr    string
		want    [5][]int
		domStar bool
		dowStar bool
		wild    bool
		wantErr string
	}{
		{name: "all stars", expr: "* * * * *", want: [5][]int{all(0, 59), all(0, 23), all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true, wild: true},
		{name: "values lists and ranges", expr: "0,30 9-17 1,15 6 1-5", want: [5][]int{{0, 30}, all(9, 17), {1, 15}, {6}, all(1, 5)}},
		{name: "star step", expr: "*/15 */6 */10 */3 */2", want: [5][]int{{0, 15, 30, 45}, {0, 6, 12, 18}, {1, 11, 21, 31}, {1, 4, 7, 10}, {0, 2, 4, 6}}, domStar: true, dowStar: true, wild: true},
		{name: "range step", expr: "1-30/5 0 1 1 0", want: [5][]int{{1, 6, 11, 16, 21, 26}, {0}, {1}, {1}, {0}}},
		{name: "step equal to the span", expr: "*/59 0 1 1 0", want: [5][]int{{0, 59}, {0}, {1}, {1}, {0}}, wild: true},
		{name: "names any case", expr: "0 0 * jan,Jul,DEC Mon-fri", want: [5][]int{{0}, {0}, all(1, 31), {1, 7, 12}, all(1, 5)}, domStar: true},
		{name: "name range with step", expr: "0 0 * JAN-DEC/3 SUN-SAT/2", want: [5][]int{{0}, {0}, all(1, 31), {1, 4, 7, 10}, {0, 2, 4, 6}}, domStar: true},
		{name: "seven is sunday", expr: "0 0 * * 7", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), {0}}, domStar: true},
		{name: "range ending at seven", expr: "0 0 * * 5-7", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), {0, 5, 6}}, domStar: true},
		{name: "odd days to seven include sunday", expr: "0 0 * * 1-7/2", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), {0, 1, 3, 5}}, domStar: true},
		{name: "overlapping list", expr: "*,5 1,1 * * *", want: [5][]int{all(0, 59), {1}, all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true, wild: true},
		{name: "leading zeros", expr: "05 09 01 01 00", want: [5][]int{{5}, {9}, {1}, {1}, {0}}},
		{name: "tabs and extra spaces", expr: " \t0  0\t* * *  ", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true},
		{name: "hour star makes a wildcard job", expr: "30 * * * *", want: [5][]int{{30}, all(0, 23), all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true, wild: true},
		{name: "yearly", expr: "@yearly", want: [5][]int{{0}, {0}, {1}, {1}, all(0, 6)}, dowStar: true},
		{name: "annually", expr: "@annually", want: [5][]int{{0}, {0}, {1}, {1}, all(0, 6)}, dowStar: true},
		{name: "monthly", expr: "@monthly", want: [5][]int{{0}, {0}, {1}, all(1, 12), all(0, 6)}, dowStar: true},
		{name: "weekly", expr: "@weekly", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), {0}}, domStar: true},
		{name: "daily", expr: "@daily", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true},
		{name: "midnight", expr: "@midnight", want: [5][]int{{0}, {0}, all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true},
		{name: "hourly", expr: "@hourly", want: [5][]int{{0}, all(0, 23), all(1, 31), all(1, 12), all(0, 6)}, domStar: true, dowStar: true, wild: true},

		{name: "empty", expr: "", wantErr: "empty cron expression"},
		{name: "only whitespace", expr: " \t ", wantErr: "empty cron expression"},
		{name: "too few fields", expr: "* * *", wantErr: "expected 5 fields (minute hour day-of-month month day-of-week), got 3"},
		{name: "seconds field", expr: "0 0 12 * * *", wantErr: "got 6: seconds and year fields (Quartz, Spring)"},
		{name: "seconds and year fields", expr: "0 0 12 * * ? 2026", wantErr: "got 7: seconds and year fields"},
		{name: "crontab line with command", expr: "0 5 * * * /usr/bin/backup", wantErr: "command must be left out"},
		{name: "eight fields", expr: "0 5 * * * root run job", wantErr: "got 8"},
		{name: "reboot", expr: "@reboot", wantErr: "@reboot runs once when the cron daemon starts"},
		{name: "uppercase macro", expr: "@DAILY", wantErr: `macros are lowercase (@daily)`},
		{name: "unknown macro", expr: "@every 5m", wantErr: `unknown macro "@every" (valid: @yearly, @annually, @monthly, @weekly, @daily, @midnight, @hourly)`},
		{name: "macro with fields", expr: "@daily *", wantErr: "@daily replaces all five fields"},
		{name: "minute out of range", expr: "60 * * * *", wantErr: `minute field "60": 60 is out of range 0-59`},
		{name: "hour out of range", expr: "0 24 * * *", wantErr: `hour field "24": 24 is out of range 0-23`},
		{name: "day zero", expr: "0 0 0 * *", wantErr: `day-of-month field "0": 0 is out of range 1-31`},
		{name: "month thirteen", expr: "0 0 1 13 *", wantErr: `month field "13": 13 is out of range 1-12`},
		{name: "weekday eight", expr: "0 0 * * 8", wantErr: `day-of-week field "8": 8 is out of range 0-7`},
		{name: "huge number", expr: "99999999999999999999 * * * *", wantErr: "99999999999999999999 is out of range 0-59"},
		{name: "range end out of range", expr: "1-234/5678 * * * *", wantErr: "234 is out of range 0-59"},
		{name: "reversed range", expr: "0 17-9 * * *", wantErr: `range "17-9" runs backwards and ranges do not wrap around: use a list, as in 17-23,0-9`},
		{name: "reversed range at the field ends", expr: "0 23-0 * * *", wantErr: "use a list, as in 23,0"},
		{name: "reversed weekday range", expr: "0 0 * * 5-2", wantErr: "as in 5-6,0-2"},
		{name: "reversed weekday range from saturday", expr: "0 0 * * SAT-MON", wantErr: "as in 6,0-1"},
		{name: "weekday range ending at sunday zero", expr: "0 0 * * FRI-SUN", wantErr: "write Sunday as 7 at the end of a range, as in 5-7"},
		{name: "weekday range starting at sunday seven", expr: "0 0 * * 7-3", wantErr: "write Sunday as 0 at the start of a range, as in 0-3"},
		{name: "weekday range from seven to zero", expr: "0 0 * * 7-0", wantErr: "as in 0"},
		{name: "zero step", expr: "*/0 * * * *", wantErr: "step 0 never advances"},
		{name: "step larger than the field", expr: "*/90 * * * *", wantErr: "step 90 over 0-59 would only select 0"},
		{name: "step larger than the range", expr: "0 0 1-3/5 * *", wantErr: "step 5 over 1-3 would only select 1"},
		{name: "weekday step reaching only sunday twice", expr: "0 0 * * */7", wantErr: "step 7 over 0-7 would only select 0"},
		{name: "weekday step from zero to seven", expr: "0 0 * * 0-7/7", wantErr: "step 7 over 0-7 would only select 0"},
		{name: "huge step", expr: "*/99999999999999999999 * * * *", wantErr: "step 99999999999999999999 is too large"},
		{name: "step on a single value", expr: "5/15 * * * *", wantErr: "a step needs a range or *, as in 5-59/15"},
		{name: "missing step", expr: "*/ * * * *", wantErr: "has no step after the /"},
		{name: "named step", expr: "0 0 * * */MON", wantErr: `step "MON" is not a number`},
		{name: "double step", expr: "*/2/3 * * * *", wantErr: `step "2/3" is not a number`},
		{name: "empty list element", expr: "1,,2 * * * *", wantErr: "empty list element"},
		{name: "trailing comma", expr: "1, * * * *", wantErr: "empty list element"},
		{name: "open range", expr: "5- * * * *", wantErr: "missing a number or name"},
		{name: "leading dash", expr: "-5 * * * *", wantErr: "missing a number or name"},
		{name: "star in a range", expr: "*-5 * * * *", wantErr: "* must stand alone or take a step"},
		{name: "negative number", expr: "0 0 * * -1", wantErr: "missing a number or name"},
		{name: "letters in a numeric field", expr: "abc * * * *", wantErr: `"abc" is not a number`},
		{name: "full month name", expr: "0 0 1 January *", wantErr: `unknown month name "January" (use JAN-DEC or 1-12)`},
		{name: "full weekday name", expr: "0 0 * * Monday", wantErr: `unknown day-of-week name "Monday" (use SUN-SAT or 0-7)`},
		{name: "carriage return", expr: "0 0 * * *\r", wantErr: `unexpected control character '\r': pass one expression on one line`},
		{name: "two lines", expr: "0 9 * * 1\n0 10 * * 1", wantErr: `unexpected control character '\n'`},
		{name: "quartz question mark", expr: "0 0 ? * MON", wantErr: `"?" (no specific value) is a Quartz extension`},
		{name: "quartz last day", expr: "0 0 L * *", wantErr: `"L" (last day or nearest weekday) is a Quartz extension`},
		{name: "quartz last weekday", expr: "0 0 * * 5L", wantErr: `"5L" (last day or nearest weekday)`},
		{name: "quartz nearest weekday", expr: "0 0 15W * *", wantErr: `"15W" (last day or nearest weekday)`},
		{name: "quartz last weekday of month", expr: "0 0 LW * *", wantErr: "Quartz extension"},
		{name: "quartz last minus offset", expr: "0 0 L-3 * *", wantErr: "Quartz extension"},
		{name: "quartz nth weekday", expr: "0 0 * * 1#2", wantErr: `"#" (nth weekday of the month) is a Quartz extension`},
		{name: "cronie random", expr: "0~30 * * * *", wantErr: `"~" (a random value) is a cronie extension`},
		{name: "jenkins hash", expr: "H * * * *", wantErr: `"H" (a hashed value) is a Jenkins extension`},
		{name: "jenkins hash range", expr: "H(0-29)/10 * * * *", wantErr: "Jenkins extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := parseCron(tt.expr)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseCron(%q) expected an error, got nil", tt.expr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseCron(%q) error = %q, want it to contain %q", tt.expr, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCron(%q) error = %v", tt.expr, err)
			}
			for i, want := range tt.want {
				if got := s.values(i); !slices.Equal(got, want) {
					t.Errorf("%s values = %v, want %v", cronFields[i].name, got, want)
				}
			}
			if s.domStar != tt.domStar || s.dowStar != tt.dowStar || s.wildcard != tt.wild {
				t.Errorf("flags (domStar, dowStar, wildcard) = (%v, %v, %v), want (%v, %v, %v)",
					s.domStar, s.dowStar, s.wildcard, tt.domStar, tt.dowStar, tt.wild)
			}
		})
	}
}

func TestCronNext(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		zone    string
		from    string
		n       int
		want    []string
		wantErr string
	}{
		// These first cases are what cronie's own cronnext prints for them,
		// built from cronie's master branch (October 2026).
		{name: "cronie: business hours", expr: "*/15 9-17 * * MON-FRI", n: 4, want: []string{"2026-01-01T09:00:00Z", "2026-01-01T09:15:00Z", "2026-01-01T09:30:00Z", "2026-01-01T09:45:00Z"}},
		{name: "cronie: both day fields restricted match either", expr: "0 0 1,15 * SUN", n: 4, want: []string{"2026-01-04T00:00:00Z", "2026-01-11T00:00:00Z", "2026-01-15T00:00:00Z", "2026-01-18T00:00:00Z"}},
		{name: "cronie: a star-led day field makes days match both", expr: "0 0 */2 * SUN", n: 4, want: []string{"2026-01-11T00:00:00Z", "2026-01-25T00:00:00Z", "2026-02-01T00:00:00Z", "2026-02-15T00:00:00Z"}},
		{name: "cronie: seven is sunday", expr: "30 4 * * 7", n: 2, want: []string{"2026-01-04T04:30:00Z", "2026-01-11T04:30:00Z"}},
		{name: "cronie: month name range with step", expr: "0 12 * JAN-MAR/2 *", n: 2, want: []string{"2026-01-01T12:00:00Z", "2026-01-02T12:00:00Z"}},
		{name: "cronie: minute range step and hour star step", expr: "5-59/20 */6 * * *", n: 4, want: []string{"2026-01-01T00:05:00Z", "2026-01-01T00:25:00Z", "2026-01-01T00:45:00Z", "2026-01-01T06:05:00Z"}},
		{name: "cronie: odd weekdays up to seven", expr: "0 0 * * 1-7/2", n: 4, want: []string{"2026-01-02T00:00:00Z", "2026-01-04T00:00:00Z", "2026-01-05T00:00:00Z", "2026-01-07T00:00:00Z"}},
		{name: "cronie: 31st skips short months", expr: "0 0 31 * *", n: 4, want: []string{"2026-01-31T00:00:00Z", "2026-03-31T00:00:00Z", "2026-05-31T00:00:00Z", "2026-07-31T00:00:00Z"}},
		{name: "cronie: friday the 13th means either", expr: "0 0 13 * fri", n: 4, want: []string{"2026-01-02T00:00:00Z", "2026-01-09T00:00:00Z", "2026-01-13T00:00:00Z", "2026-01-16T00:00:00Z"}},
		{name: "cronie: weekday list with names", expr: "0 0 1 * MON-WED,sat", n: 4, want: []string{"2026-01-03T00:00:00Z", "2026-01-05T00:00:00Z", "2026-01-06T00:00:00Z", "2026-01-07T00:00:00Z"}},
		{name: "cronie: weekly", expr: "@weekly", n: 2, want: []string{"2026-01-04T00:00:00Z", "2026-01-11T00:00:00Z"}},
		{name: "cronie: monthly", expr: "@monthly", n: 2, want: []string{"2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z"}},
		{name: "cronie: 2100 is not a leap year", expr: "0 0 29 2 *", from: "2096-03-01T00:00:00Z", n: 2, want: []string{"2104-02-29T00:00:00Z", "2108-02-29T00:00:00Z"}},

		{name: "results are strictly after from", expr: "0 0 * * *", from: "2026-01-01T00:00:00Z", n: 1, want: []string{"2026-01-02T00:00:00Z"}},
		{name: "a run within the same minute but after from", expr: "* * * * *", from: "2026-01-01T00:00:30Z", n: 2, want: []string{"2026-01-01T00:01:00Z", "2026-01-01T00:02:00Z"}},
		{name: "fractional second before a run", expr: "* * * * *", from: "2026-01-01T00:00:59.5Z", n: 1, want: []string{"2026-01-01T00:01:00Z"}},
		{name: "business hours cross a weekend", expr: "*/30 9-17 * * 1-5", from: "2026-01-02T17:30:00Z", n: 2, want: []string{"2026-01-05T09:00:00Z", "2026-01-05T09:30:00Z"}},
		{name: "rolls over the year", expr: "0 0 1 1 *", from: "2026-12-31T23:59:00Z", n: 1, want: []string{"2027-01-01T00:00:00Z"}},
		{name: "yearly", expr: "@yearly", n: 2, want: []string{"2027-01-01T00:00:00Z", "2028-01-01T00:00:00Z"}},
		{name: "hourly", expr: "@hourly", n: 2, want: []string{"2026-01-01T01:00:00Z", "2026-01-01T02:00:00Z"}},
		{name: "leap day", expr: "0 0 29 2 *", n: 2, want: []string{"2028-02-29T00:00:00Z", "2032-02-29T00:00:00Z"}},
		// A star-led weekday field still restricts: February 29 on a Saturday
		// or Sunday, 28 years apart across 2100, which is not a leap year.
		{name: "years between runs", expr: "0 0 29 2 */6", from: "2026-01-01T00:00:00Z", n: 3, want: []string{"2032-02-29T00:00:00Z", "2048-02-29T00:00:00Z", "2060-02-29T00:00:00Z"}},
		{name: "decades between runs across 2100", expr: "0 0 29 2 */6", from: "2088-03-01T00:00:00Z", n: 1, want: []string{"2116-02-29T00:00:00Z"}},
		{name: "evaluated in the zone", expr: "0 10 * * *", zone: "Asia/Tokyo", n: 1, want: []string{"2026-01-01T10:00:00+09:00"}},
		{name: "local midnight before from in utc", expr: "0 0 * * *", zone: "America/Los_Angeles", from: "2026-01-01T07:59:00Z", n: 1, want: []string{"2026-01-01T00:00:00-08:00"}},
		{name: "zone a day ahead of from", expr: "0 0 * * *", zone: "Pacific/Kiritimati", from: "2026-01-01T09:59:00Z", n: 1, want: []string{"2026-01-02T00:00:00+14:00"}},
		{name: "last representable minute", expr: "59 23 31 12 *", from: "9999-12-30T00:00:00Z", n: 1, want: []string{"9999-12-31T23:59:00Z"}},
		{name: "from before year zero", expr: "0 0 1 1 *", from: "-62198755200", n: 1, want: []string{"0000-01-01T00:00:00Z"}},

		// America/New_York springs forward at 2026-03-08 02:00 EST to 03:00
		// EDT and falls back at 2026-11-01 02:00 EDT to 01:00 EST. Each case
		// matches what crond itself did, run under libfaketime.
		{name: "spring forward: skipped fixed time runs when the gap ends", expr: "30 2 * * *", zone: "America/New_York", from: "2026-03-08T01:10:00-05:00", n: 2, want: []string{"2026-03-08T03:00:00-04:00", "2026-03-09T02:30:00-04:00"}},
		{name: "spring forward: skipped times collapse into one run", expr: "0,30 2 * * *", zone: "America/New_York", from: "2026-03-08T01:10:00-05:00", n: 2, want: []string{"2026-03-08T03:00:00-04:00", "2026-03-09T02:00:00-04:00"}},
		{name: "spring forward: catch-up and the next run coincide", expr: "0 2,3 * * *", zone: "America/New_York", from: "2026-03-08T01:10:00-05:00", n: 2, want: []string{"2026-03-08T03:00:00-04:00", "2026-03-09T02:00:00-04:00"}},
		{name: "spring forward: wildcard minute is skipped", expr: "*/15 2 * * *", zone: "America/New_York", from: "2026-03-08T01:10:00-05:00", n: 1, want: []string{"2026-03-09T02:00:00-04:00"}},
		{name: "spring forward: wildcard hour is skipped", expr: "30 * * * *", zone: "America/New_York", from: "2026-03-08T01:10:00-05:00", n: 2, want: []string{"2026-03-08T01:30:00-05:00", "2026-03-08T03:30:00-04:00"}},
		{name: "fall back: fixed time runs only the first time", expr: "15 1 * * *", zone: "America/New_York", from: "2026-11-01T00:55:00-04:00", n: 2, want: []string{"2026-11-01T01:15:00-04:00", "2026-11-02T01:15:00-05:00"}},
		{name: "fall back: wildcard minute runs both times in order", expr: "*/20 1 * * *", zone: "America/New_York", from: "2026-11-01T00:55:00-04:00", n: 7, want: []string{"2026-11-01T01:00:00-04:00", "2026-11-01T01:20:00-04:00", "2026-11-01T01:40:00-04:00", "2026-11-01T01:00:00-05:00", "2026-11-01T01:20:00-05:00", "2026-11-01T01:40:00-05:00", "2026-11-02T01:00:00-05:00"}},
		{name: "fall back: wildcard hour runs both times", expr: "30 * * * *", zone: "America/New_York", from: "2026-11-01T00:55:00-04:00", n: 3, want: []string{"2026-11-01T01:30:00-04:00", "2026-11-01T01:30:00-05:00", "2026-11-01T02:30:00-05:00"}},
		{name: "fall back: from inside the repeated hour", expr: "*/20 1 * * *", zone: "America/New_York", from: "2026-11-01T01:50:00-04:00", n: 2, want: []string{"2026-11-01T01:00:00-05:00", "2026-11-01T01:20:00-05:00"}},
		{name: "fall back: fixed time already run in the first pass", expr: "15 1 * * *", zone: "America/New_York", from: "2026-11-01T01:20:00-04:00", n: 1, want: []string{"2026-11-02T01:15:00-05:00"}},
		// Lord Howe Island moves its clocks 30 minutes, 02:00 to 02:30.
		{name: "half-hour DST gap", expr: "15 2 * * *", zone: "Australia/Lord_Howe", from: "2026-10-04T00:00:00+10:30", n: 2, want: []string{"2026-10-04T02:30:00+11:00", "2026-10-05T02:15:00+11:00"}},
		// Samoa skipped 2011-12-30 entirely, moving from UTC-10 to UTC+14.
		{name: "skipped day: fixed time runs when the gap ends", expr: "30 12 30 12 *", zone: "Pacific/Apia", from: "2011-12-29T00:00:00-10:00", n: 2, want: []string{"2011-12-31T00:00:00+14:00", "2012-12-30T12:30:00+14:00"}},
		{name: "skipped day: wildcard is skipped", expr: "* 12 30 12 *", zone: "Pacific/Apia", from: "2011-12-29T00:00:00-10:00", n: 1, want: []string{"2012-12-30T12:00:00+14:00"}},

		{name: "february 30 never runs", expr: "0 0 30 2 *", n: 1, wantErr: `"0 0 30 2 *" never runs: day-of-month 30 does not occur in February`},
		{name: "31st of short months never runs", expr: "0 0 31 4,6,9,11 *", n: 1, wantErr: "day-of-month 31 does not occur in April, June, September or November"},
		{name: "star-stepped weekday keeps both rule", expr: "0 0 30,31 2 */2", n: 1, wantErr: "day-of-month 30,31 does not occur in February"},
		{name: "restricted weekday still runs on february 30", expr: "0 0 30 2 MON", n: 1, want: []string{"2026-02-02T00:00:00Z"}},
		{name: "no run before year 10000", expr: "0 0 1 1 *", from: "9999-06-01T00:00:00Z", n: 1, wantErr: `"0 0 1 1 *" has only 0 run(s) after 9999-06-01T00:00:00Z up to the year 9999`},
		{name: "too few runs before year 10000", expr: "0 0 1 * *", from: "9999-10-15T00:00:00Z", n: 3, wantErr: "has only 2 run(s)"},
		{name: "zero count", expr: "* * * * *", n: 0, wantErr: "run count must be at least 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := time.UTC
			if tt.zone != "" {
				loc = cronZone(t, tt.zone)
			}
			from := cronFrom
			if tt.from != "" {
				var err error
				if from, err = Parse(tt.from, from); err != nil {
					t.Fatal(err)
				}
			}
			s, err := parseCron(tt.expr)
			if err != nil {
				t.Fatalf("parseCron(%q) error = %v", tt.expr, err)
			}
			runs, err := s.next(from, loc, tt.n)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("next() expected an error, got %v", runs)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("next() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("next() error = %v", err)
			}
			got := make([]string, len(runs))
			for i, r := range runs {
				got[i] = r.Format(time.RFC3339)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("next() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Outside DST changes a run is exactly a minute whose wall-clock fields
// match, so a minute-by-minute scan is an independent check of the search.
func TestCronNextMatchesScan(t *testing.T) {
	exprs := []string{
		"* * * * *", "*/7 */5 * * *", "0 0 29 2 *", "0 0 */2 * 1", "0 0 1,15 * 5",
		"59 23 31 * *", "0 12 * 2 0-3", "15,45 6-8 10-20 1-6/2 *", "0 0 1-7 * 1",
	}
	loc := cronZone(t, "Asia/Kolkata")
	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			s, err := parseCron(expr)
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			for m := cronFrom.Add(time.Minute); len(want) < 25 && m.Year() < 2040; m = m.Add(time.Minute) {
				w := m.In(loc)
				if s.matchesDay(w) && s.sets[fHour]&(1<<w.Hour()) != 0 && s.sets[fMinute]&(1<<w.Minute()) != 0 {
					want = append(want, w.Format(time.RFC3339))
				}
			}
			runs, err := s.next(cronFrom, loc, len(want))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(runs))
			for i, r := range runs {
				got[i] = r.Format(time.RFC3339)
			}
			if !slices.Equal(got, want) {
				t.Errorf("next() = %v, scan = %v", got, want)
			}
		})
	}
}

func TestCronFrom(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		input   string
		zone    string
		want    string
		wantErr string
	}{
		{name: "bare date is midnight in the zone", input: "2026-03-08", zone: "America/New_York", want: "2026-03-08T00:00:00-05:00"},
		{name: "date and time in the zone", input: "2026-11-01 01:30", zone: "America/New_York", want: "2026-11-01T01:30:00-04:00"},
		{name: "date and time with T and seconds", input: " 2026-07-01T09:15:30\n", zone: "Asia/Tokyo", want: "2026-07-01T09:15:30+09:00"},
		{name: "bare date in utc", input: "2026-03-08", zone: "UTC", want: "2026-03-08T00:00:00Z"},
		{name: "explicit offset is kept", input: "2026-03-08T00:00:00Z", zone: "America/New_York", want: "2026-03-08T00:00:00Z"},
		{name: "rfc1123 zone is kept", input: "Sun, 08 Mar 2026 00:00:00 +0000", zone: "Asia/Tokyo", want: "2026-03-08T00:00:00Z"},
		{name: "epoch seconds are absolute", input: "1000000000", zone: "Asia/Tokyo", want: "2001-09-09T01:46:40Z"},
		{name: "now", input: "now", zone: "Asia/Tokyo", want: "2026-08-17T12:00:00Z"},
		{name: "garbage", input: "yesterday", zone: "UTC", wantErr: "unrecognized timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CronFrom(tt.input, now, cronZone(t, tt.zone))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CronFrom() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want, err := time.Parse(time.RFC3339, tt.want)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(want) {
				t.Errorf("CronFrom(%q) = %v, want %v", tt.input, got, want)
			}
		})
	}
}

func TestCron(t *testing.T) {
	got, err := Cron("0 9 * * MON-FRI", cronFrom, time.UTC, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-01-01T09:00:00Z\n2026-01-02T09:00:00Z"; got != want {
		t.Errorf("Cron() = %q, want %q", got, want)
	}
	if _, err := Cron("0 0 30 2 *", cronFrom, time.UTC, 1); err == nil || !strings.Contains(err.Error(), "never runs") {
		t.Errorf("Cron() error = %v, want a never-runs error", err)
	}
	if _, err := Cron("61 * * * *", cronFrom, time.UTC, 1); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("Cron() error = %v, want a parse error", err)
	}
}

func TestCronJSON(t *testing.T) {
	type repr struct {
		Expression string `json:"expression"`
		Zone       string `json:"zone"`
		Fields     struct {
			Minute     []int `json:"minute"`
			Hour       []int `json:"hour"`
			DayOfMonth []int `json:"day_of_month"`
			Month      []int `json:"month"`
			DayOfWeek  []int `json:"day_of_week"`
		} `json:"fields"`
		DayMatch string   `json:"day_match"`
		Next     []string `json:"next"`
	}
	tests := []struct {
		name         string
		expr         string
		zone         string
		wantExpr     string
		wantDayMatch string
		wantNext     []string
	}{
		{name: "restricted day fields match either", expr: "0  0 1 * MON", zone: "UTC", wantExpr: "0 0 1 * MON", wantDayMatch: "either", wantNext: []string{"2026-01-05T00:00:00Z", "2026-01-12T00:00:00Z"}},
		{name: "star-led day field matches both", expr: "0 0 */2 * 1", zone: "UTC", wantExpr: "0 0 */2 * 1", wantDayMatch: "both", wantNext: []string{"2026-01-05T00:00:00Z", "2026-01-19T00:00:00Z"}},
		{name: "macro keeps its name", expr: "@daily", zone: "Europe/Paris", wantExpr: "@daily", wantDayMatch: "both", wantNext: []string{"2026-01-02T00:00:00+01:00", "2026-01-03T00:00:00+01:00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := CronJSON(tt.expr, cronFrom, cronZone(t, tt.zone), 2)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(out, "{\n  \"expression\"") {
				t.Errorf("CronJSON() is not pretty-printed: %q", out)
			}
			var got repr
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("CronJSON() is not valid JSON: %v\n%s", err, out)
			}
			if got.Expression != tt.wantExpr || got.Zone != tt.zone || got.DayMatch != tt.wantDayMatch || !slices.Equal(got.Next, tt.wantNext) {
				t.Errorf("CronJSON() = %+v, want expression %q, zone %q, day_match %q, next %v", got, tt.wantExpr, tt.zone, tt.wantDayMatch, tt.wantNext)
			}
			if len(got.Fields.Minute) == 0 || len(got.Fields.Hour) == 0 || len(got.Fields.DayOfMonth) == 0 || len(got.Fields.Month) == 0 || len(got.Fields.DayOfWeek) == 0 {
				t.Errorf("CronJSON() fields = %+v, want every field expanded", got.Fields)
			}
		})
	}
	t.Run("expanded fields", func(t *testing.T) {
		out, err := CronJSON("*/20 9-11 1,15 JAN-MAR 0,7", cronFrom, time.UTC, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := `"fields": {
    "minute": [
      0,
      20,
      40
    ],
    "hour": [
      9,
      10,
      11
    ],
    "day_of_month": [
      1,
      15
    ],
    "month": [
      1,
      2,
      3
    ],
    "day_of_week": [
      0
    ]
  },`
		if !strings.Contains(out, want) {
			t.Errorf("CronJSON() = %s\nwant it to contain %s", out, want)
		}
	})
	t.Run("errors pass through", func(t *testing.T) {
		if _, err := CronJSON("@reboot", cronFrom, time.UTC, 1); err == nil {
			t.Error("CronJSON(@reboot) expected an error, got nil")
		}
	})
}

func FuzzParseCron(f *testing.F) {
	for _, seed := range []string{
		"* * * * *", "*/15 9-17 * * MON-FRI", "0 0 29 2 *", "@weekly", "5/15 * * * *",
		"0 0 L * ?", "1-234/5678 * * * *", "0 0 * * FRI-SUN", "*/0 * * * *", "", "0 0 * * 7-3", "0 0 29 2 */6",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		s, err := parseCron(expr)
		if err != nil {
			return
		}
		for i, field := range cronFields {
			vals := s.values(i)
			if len(vals) == 0 || vals[0] < field.min || vals[len(vals)-1] > field.max || (i == fDOW && vals[len(vals)-1] > 6) {
				t.Fatalf("parseCron(%q) %s values = %v, outside %d-%d", expr, field.name, vals, field.min, field.max)
			}
		}
		runs, err := s.next(cronFrom, time.UTC, 1)
		if err == nil && (len(runs) != 1 || !runs[0].After(cronFrom)) {
			t.Fatalf("next(%q) = %v, want one run after %v", expr, runs, cronFrom)
		}
	})
}
