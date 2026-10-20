// Package clock parses, formats, and does date arithmetic on timestamps for
// the time command. Nothing
// here calls time.Now(): the reference time is always passed in, so every
// caller and test is deterministic.
package clock

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Integers at or above this are epoch milliseconds, below it epoch seconds.
// 1e11 seconds is the year 5138 and 1e11 milliseconds is March 1973, so each
// side of the line covers every plausible timestamp of the other unit.
const msThreshold = 100_000_000_000

// layouts are tried in order; longer variants come before their prefixes so
// inputs with seconds aren't cut short.
var layouts = []struct {
	layout string
	zoned  bool
}{
	{time.RFC3339Nano, true},
	{"2006-01-02T15:04:05", false},
	{"2006-01-02 15:04:05", false},
	{"2006-01-02T15:04", false},
	{"2006-01-02 15:04", false},
	{"2006-01-02", false},
	{time.RFC1123Z, true},
	{time.RFC1123, true},
}

// ParseIn turns s into a time: "now" (the passed reference time), epoch
// seconds or milliseconds, RFC 3339, RFC 1123, or a date with optional time.
// A timestamp without a zone or offset is read as a wall clock time in loc,
// resolving DST gaps and repeats as Add does. Epoch values, "now", and
// timestamps with an offset are absolute and ignore loc.
func ParseIn(s string, now time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if s == "now" {
		return now, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n >= msThreshold || n <= -msThreshold {
			return time.UnixMilli(n).UTC(), nil
		}
		return time.Unix(n, 0).UTC(), nil
	}
	for _, l := range layouts {
		t, err := time.Parse(l.layout, s)
		if err != nil {
			continue
		}
		if !l.zoned {
			y, m, d := t.Date()
			t = resolveWall(time.Date(y, m, d, 0, 0, 0, 0, time.UTC), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf(
		"unrecognized timestamp %q (accepted: \"now\", epoch seconds or milliseconds, RFC 3339, RFC 1123, \"2006-01-02\" with optional \"15:04[:05]\")", s)
}

// Zone resolves a timezone name: "" and "UTC" mean UTC, "local" the system
// zone, anything else an IANA name like America/New_York.
func Zone(name string) (*time.Location, error) {
	switch name {
	case "", "UTC", "utc":
		return time.UTC, nil
	case "local":
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q (use an IANA name like America/New_York, \"UTC\", or \"local\")", name)
	}
	return loc, nil
}

// Format renders t in the given zone, using the custom Go layout when one is
// given and a named format otherwise.
func Format(t time.Time, name, layout, zone string) (string, error) {
	loc, err := Zone(zone)
	if err != nil {
		return "", err
	}
	t = t.In(loc)
	if layout != "" {
		return t.Format(layout), nil
	}
	switch name {
	case "rfc3339":
		return t.Format(time.RFC3339), nil
	case "unix":
		return strconv.FormatInt(t.Unix(), 10), nil
	case "unix-ms":
		return strconv.FormatInt(t.UnixMilli(), 10), nil
	case "date":
		return t.Format("2006-01-02"), nil
	case "time":
		return t.Format("15:04:05"), nil
	default:
		return "", fmt.Errorf("unknown format %q (valid: rfc3339, unix, unix-ms, date, time)", name)
	}
}

// JSON renders every representation of t at once, so one call gives an agent
// whatever form it turns out to need. Zone-dependent fields use the given
// zone; unix and utc are zone-independent.
func JSON(t time.Time, zone string) (string, error) {
	loc, err := Zone(zone)
	if err != nil {
		return "", err
	}
	in := t.In(loc)
	repr := struct {
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
	}{
		Unix:    t.Unix(),
		UnixMs:  t.UnixMilli(),
		RFC3339: in.Format(time.RFC3339),
		UTC:     t.UTC().Format(time.RFC3339),
		Date:    in.Format("2006-01-02"),
		Time:    in.Format("15:04:05"),
		Weekday: in.Weekday().String(),
		Zone:    loc.String(),
	}
	repr.ISOYear, repr.ISOWeek = in.ISOWeek()
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding representations: %w", err)
	}
	return string(out), nil
}

// minTime and maxTime bound every result: the int64 epoch-millisecond range,
// the widest form Parse accepts, so unix_ms never overflows.
var (
	minTime = time.UnixMilli(math.MinInt64)
	maxTime = time.UnixMilli(math.MaxInt64)
)

var errOutOfRange = fmt.Errorf("result is out of range (timestamps are limited to 64-bit epoch milliseconds, years %d to %d)",
	minTime.UTC().Year(), maxTime.UTC().Year())

var errDurationTooLarge = errors.New("too large to represent")

// Calendar parts beyond these cannot land inside [minTime, maxTime] from any
// start inside it; rejecting them first keeps the date math from overflowing.
const (
	maxMonths = 12 * 600_000_000
	maxDays   = 366 * 600_000_000
)

// Duration is a signed span in the shape of an ISO 8601 duration: calendar
// months and days, applied to a zone's wall clock, then an exact elapsed part.
// All non-zero fields share one sign.
type Duration struct {
	Months  int64 // years count as 12 months each
	Days    int64 // weeks count as 7 days each
	Seconds int64
	Nanos   int64 // |Nanos| < 1e9
}

const durationForms = "want ISO 8601 like P1Y2M10DT2H30M, P2W, or PT1.5S, or Go-style like 1h30m or -90s, either with an optional leading minus"

// ParseDuration reads an ISO 8601 duration (P1Y2M10DT2H30M, P2W, PT1.5S) or a
// Go-style one (1h30m), either optionally signed. Go-style durations are exact
// elapsed time and have no day unit, because a calendar day is not always 24
// hours.
func ParseDuration(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Duration{}, errors.New("empty duration")
	}
	body, negative := s, false
	switch body[0] {
	case '-':
		body, negative = body[1:], true
	case '+':
		body = body[1:]
	}
	var d Duration
	var err error
	switch {
	case body == "":
		err = errors.New("no value after the sign")
	case body[0] == '-' || body[0] == '+':
		err = errors.New("more than one sign")
	case body[0] == 'P' || body[0] == 'p':
		d, err = parseISODuration(strings.ToUpper(body[1:]))
	default:
		d, err = parseGoDuration(body, negative)
	}
	if err != nil {
		return Duration{}, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if negative {
		d = d.neg()
	}
	return d, nil
}

func parseISODuration(body string) (Duration, error) {
	datePart, timePart, hasT := strings.Cut(body, "T")
	switch {
	case datePart == "" && !hasT:
		return Duration{}, fmt.Errorf("no components after P (%s)", durationForms)
	case hasT && timePart == "":
		return Duration{}, fmt.Errorf("no hours, minutes, or seconds after T (%s)", durationForms)
	}
	var years, months, weeks, days, hours, minutes, seconds, nanos int64
	err := scanISO(datePart, "YMWD", "date", func(unit byte, n int64, frac string) error {
		if frac != "" {
			return fmt.Errorf("only seconds may have a fraction, not %c", unit)
		}
		switch unit {
		case 'Y':
			years = n
		case 'M':
			months = n
		case 'W':
			weeks = n
		case 'D':
			days = n
		}
		return nil
	})
	if err != nil {
		return Duration{}, err
	}
	err = scanISO(timePart, "HMS", "time", func(unit byte, n int64, frac string) error {
		switch unit {
		case 'H':
			hours = n
		case 'M':
			minutes = n
		case 'S':
			seconds = n
		}
		if frac == "" {
			return nil
		}
		if unit != 'S' {
			return fmt.Errorf("only seconds may have a fraction, not %c (for example PT1H30M, not PT1.5H)", unit)
		}
		if len(frac) > 9 {
			return fmt.Errorf("fraction %q is finer than nanoseconds", frac)
		}
		nanos, _ = strconv.ParseInt(frac+strings.Repeat("0", 9-len(frac)), 10, 64)
		return nil
	})
	if err != nil {
		return Duration{}, err
	}
	d := Duration{Nanos: nanos}
	var ok1, ok2, ok3, ok4 bool
	d.Months, ok1 = mulAdd(years, 12, months)
	d.Days, ok2 = mulAdd(weeks, 7, days)
	d.Seconds, ok3 = mulAdd(hours, 3600, 0)
	if ok3 {
		d.Seconds, ok3 = mulAdd(minutes, 60, d.Seconds)
	}
	if ok3 {
		d.Seconds, ok4 = mulAdd(seconds, 1, d.Seconds)
	}
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Duration{}, errDurationTooLarge
	}
	return d, nil
}

// scanISO walks one part of an ISO 8601 duration (the date part or the time
// part), calling fn for each number-and-designator pair. Designators must
// appear at most once each, in the order given.
func scanISO(part, order, name string, fn func(unit byte, n int64, frac string) error) error {
	last := -1
	for part != "" {
		i := 0
		for i < len(part) && part[i] >= '0' && part[i] <= '9' {
			i++
		}
		whole, frac := part[:i], ""
		if i < len(part) && (part[i] == '.' || part[i] == ',') {
			j := i + 1
			for j < len(part) && part[j] >= '0' && part[j] <= '9' {
				j++
			}
			frac, i = part[i+1:j], j
			if frac == "" {
				return fmt.Errorf("no digits after the decimal separator in %q", part[:i])
			}
		}
		if whole == "" {
			if i < len(part) {
				return fmt.Errorf("%q has no number before it (%s)", part[i], durationForms)
			}
			return fmt.Errorf("no number in the %s part (%s)", name, durationForms)
		}
		if i == len(part) {
			return fmt.Errorf("number %q has no designator (%s)", part, durationForms)
		}
		unit := part[i]
		pos := strings.IndexByte(order, unit)
		if pos < 0 {
			return fmt.Errorf("%q is not a %s designator (date part before T: Y, M, W, D; time part after T: H, M, S)", unit, name)
		}
		if pos <= last {
			return fmt.Errorf("%c is repeated or out of order (the %s part's order is %s)", unit, name, order)
		}
		last = pos
		n, err := strconv.ParseInt(whole, 10, 64)
		if err != nil {
			return errDurationTooLarge
		}
		if err := fn(unit, n, frac); err != nil {
			return err
		}
		part = part[i+1:]
	}
	return nil
}

// goTerm matches one number-and-unit term of a Go-style duration, and
// goSyntax a whole one that time.ParseDuration accepts unless it is too long.
var (
	goTerm   = regexp.MustCompile(`([0-9.]*)([^0-9.]+)`)
	goSyntax = regexp.MustCompile(`^(?:(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:ns|us|µs|μs|ms|s|m|h))+$`)
)

var goUnitNanos = map[string]int64{
	"ns": 1, "us": 1e3, "µs": 1e3, "μs": 1e3, "ms": 1e6, "s": 1e9, "m": 60e9, "h": 3600e9,
}

func parseGoDuration(body string, negative bool) (Duration, error) {
	terms := goTerm.FindAllStringSubmatch(body, -1)
	for _, term := range terms {
		number := term[1]
		if len(terms) > 1 {
			number = ""
		}
		if err := calendarUnitError(number, term[2], negative); err != nil {
			return Duration{}, err
		}
	}
	d, err := time.ParseDuration(body)
	switch {
	case err != nil && goSyntax.MatchString(body):
		return Duration{}, errors.New("a Go-style duration is limited to about 292 years; use ISO 8601 for longer spans, like PT3000000H")
	case err != nil:
		return Duration{}, fmt.Errorf("%s (%s)", strings.TrimPrefix(err.Error(), "time: "), durationForms)
	}
	// time.ParseDuration silently drops anything finer than a nanosecond.
	for _, term := range terms {
		r, ok := new(big.Rat).SetString(term[1])
		if ok && !r.Mul(r, big.NewRat(goUnitNanos[term[2]], 1)).IsInt() {
			return Duration{}, fmt.Errorf("%s%s is finer than nanoseconds", term[1], term[2])
		}
	}
	return Duration{Seconds: int64(d / time.Second), Nanos: int64(d % time.Second)}, nil
}

// calendarUnitError rejects the calendar units Go-style durations lack,
// rather than guessing whether a day means 24 hours or a calendar day.
func calendarUnitError(number, unit string, negative bool) error {
	var designator, noun string
	var hours int64
	switch strings.ToLower(unit) {
	case "d":
		designator, noun, hours = "D", "days", 24
	case "w":
		designator, noun, hours = "W", "weeks", 7*24
	case "mo":
		designator, noun = "M", "months"
	case "y":
		designator, noun = "Y", "years"
	default:
		return nil
	}
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil || n > math.MaxInt64/(7*24) {
		number, n = "n", 0
	}
	sign := ""
	if negative {
		sign = "-"
	}
	msg := fmt.Sprintf("a Go-style duration has no %q unit: use %sP%s%s for calendar %s, keeping the wall-clock time", unit, sign, number, designator, noun)
	switch {
	case hours > 0 && n > 0:
		msg += fmt.Sprintf(", or %s%dh for exactly %d hours", sign, n*hours, n*hours)
	case hours > 0:
		msg += fmt.Sprintf(", or hours (%d per %s) for exact elapsed time", hours, strings.TrimSuffix(noun, "s"))
	}
	return errors.New(msg)
}

// mulAdd returns a*b + c for non-negative operands, reporting overflow.
func mulAdd(a, b, c int64) (int64, bool) {
	if b != 0 && a > (math.MaxInt64-c)/b {
		return 0, false
	}
	return a*b + c, true
}

func (d Duration) neg() Duration {
	return Duration{Months: -d.Months, Days: -d.Days, Seconds: -d.Seconds, Nanos: -d.Nanos}
}

// String renders d in ISO 8601 (P1Y2M3DT4H5M6.5S, -P1D, PT0S), the form
// ParseDuration reads back. The exact part is never folded into days, since a
// day is not always 24 hours.
func (d Duration) String() string {
	var b strings.Builder
	if d.Months < 0 || d.Days < 0 || d.Seconds < 0 || d.Nanos < 0 {
		b.WriteByte('-')
		d = d.neg()
	}
	b.WriteByte('P')
	writePart := func(n int64, designator byte) {
		if n != 0 {
			b.WriteString(strconv.FormatInt(n, 10))
			b.WriteByte(designator)
		}
	}
	writePart(d.Months/12, 'Y')
	writePart(d.Months%12, 'M')
	writePart(d.Days, 'D')
	if d.Seconds != 0 || d.Nanos != 0 {
		b.WriteByte('T')
		writePart(d.Seconds/3600, 'H')
		writePart(d.Seconds%3600/60, 'M')
		if s := d.Seconds % 60; s != 0 || d.Nanos != 0 {
			b.WriteString(decimalSeconds(s, d.Nanos))
			b.WriteByte('S')
		}
	}
	if s := b.String(); s != "P" {
		return s
	}
	return "PT0S"
}

// decimalSeconds renders seconds and nanoseconds of the same sign as an exact
// decimal with no trailing zeros: 90, -1.5, 0.000000001.
func decimalSeconds(sec, nanos int64) string {
	sign := ""
	if sec < 0 || nanos < 0 {
		sign, sec, nanos = "-", -sec, -nanos
	}
	s := sign + strconv.FormatInt(sec, 10)
	if nanos != 0 {
		s += "." + strings.TrimRight(fmt.Sprintf("%09d", nanos), "0")
	}
	return s
}

// Add returns t moved by d: years and months together, then days, on loc's
// wall clock, then the exact part as elapsed time. A day past the end of the
// target month clamps to its last day, so 2024-01-31 plus P1M is 2024-02-29
// (time.AddDate would overflow into March). Calendar steps keep the wall-clock
// time, so P1D across a DST change is 23 or 25 hours.
func Add(t time.Time, d Duration, loc *time.Location) (time.Time, error) {
	if d.Months > maxMonths || d.Months < -maxMonths || d.Days > maxDays || d.Days < -maxDays {
		return time.Time{}, errOutOfRange
	}
	r := shift(t, d.Months, d.Days, loc)
	sec := r.Unix()
	if (d.Seconds > 0 && sec > maxTime.Unix()-d.Seconds) || (d.Seconds < 0 && sec < minTime.Unix()-d.Seconds) {
		return time.Time{}, errOutOfRange
	}
	r = time.Unix(sec+d.Seconds, int64(r.Nanosecond())+d.Nanos)
	if r.Before(minTime) || r.After(maxTime) {
		return time.Time{}, errOutOfRange
	}
	return r.In(loc), nil
}

// shift moves t by calendar months and then days on loc's wall clock,
// clamping the day of the month. Callers bound months and days so the
// arithmetic cannot overflow.
func shift(t time.Time, months, days int64, loc *time.Location) time.Time {
	if months == 0 && days == 0 {
		return t
	}
	w := t.In(loc)
	y, m, d := w.Date()
	total := int64(y)*12 + int64(m-1) + months
	year, month := int(floorDiv(total, 12)), time.Month(total-floorDiv(total, 12)*12+1)
	if last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day(); d > last {
		d = last
	}
	date := time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
	if days != 0 {
		date = time.Unix(date.Unix()+days*86400, 0).UTC()
	}
	hour, minute, second := w.Clock()
	return resolveWall(date, hour, minute, second, w.Nanosecond(), loc)
}

// resolveWall finds the instant showing the given date and time on loc's wall
// clock. time.Date leaves the two hard cases unspecified (and in practice
// moves a skipped New York time backward but a skipped London time forward),
// so they are defined here as RFC 5545 and JavaScript's Temporal do: a time
// skipped by a forward transition moves forward by the gap, and a time that
// occurs twice resolves to the earlier instant.
func resolveWall(date time.Time, hour, minute, second, nanos int, loc *time.Location) time.Time {
	wall := date.Unix() + int64(hour)*3600 + int64(minute)*60 + int64(second)
	_, before := time.Unix(wall-86400, 0).In(loc).Zone()
	_, after := time.Unix(wall+86400, 0).In(loc).Zone()
	u := wall - int64(before)
	if _, off := time.Unix(u, 0).In(loc).Zone(); off != before {
		if _, off := time.Unix(wall-int64(after), 0).In(loc).Zone(); off == after {
			u = wall - int64(after)
		}
	}
	return time.Unix(u, int64(nanos)).In(loc)
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// Diff returns the Duration from one instant to another such that
// Add(from, Diff(from, to, loc), loc) is to: the most whole months that do
// not pass to, then the most whole days, then the exact remainder. Each part
// carries the sign of to minus from. It is exact over the whole range Parse
// accepts; time.Time.Sub would saturate at about 292 years.
func Diff(from, to time.Time, loc *time.Location) Duration {
	sign := int64(to.Compare(from))
	if sign == 0 {
		return Duration{}
	}
	months := fit(monthsBetween(from.In(loc), to.In(loc)), sign, func(n int64) bool {
		return !passes(shift(from, n, 0, loc), to, sign)
	})
	days := wholeDays(from, to, months, loc)
	sec, nanos := elapsed(shift(from, months, days, loc), to)
	return Duration{Months: months, Days: days, Seconds: sec, Nanos: nanos}
}

// wholeDays is the most whole calendar days that, after the given months,
// take from toward to without passing it.
func wholeDays(from, to time.Time, months int64, loc *time.Location) int64 {
	sign := int64(to.Compare(from))
	if sign == 0 {
		return 0
	}
	base := shift(from, months, 0, loc)
	return fit(civilDay(to.In(loc))-civilDay(base.In(loc)), sign, func(n int64) bool {
		return !passes(shift(from, months, n, loc), to, sign)
	})
}

// fit corrects an estimated step count to the largest one in the direction
// of sign for which ok holds. ok must hold for 0 and be monotonic, and the
// estimate must be close, as the wall-clock calendar differences are.
func fit(n, sign int64, ok func(int64) bool) int64 {
	if n*sign < 0 {
		n = 0
	}
	for n != 0 && !ok(n) {
		n -= sign
	}
	for ok(n + sign) {
		n += sign
	}
	return n
}

// passes reports whether t has gone beyond target in the direction of sign.
func passes(t, target time.Time, sign int64) bool {
	return int64(t.Compare(target))*sign > 0
}

func monthsBetween(a, b time.Time) int64 {
	return (int64(b.Year())-int64(a.Year()))*12 + int64(b.Month()-a.Month())
}

// civilDay numbers t's wall-clock date as days since 1970-01-01.
func civilDay(t time.Time) int64 {
	y, m, d := t.Date()
	return floorDiv(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix(), 86400)
}

// elapsed returns to minus from as seconds and nanoseconds of the same sign.
func elapsed(from, to time.Time) (sec, nanos int64) {
	sec = to.Unix() - from.Unix()
	nanos = int64(to.Nanosecond() - from.Nanosecond())
	switch {
	case sec > 0 && nanos < 0:
		sec, nanos = sec-1, nanos+1e9
	case sec < 0 && nanos > 0:
		sec, nanos = sec+1, nanos-1e9
	}
	return sec, nanos
}

// DiffJSON describes to minus from in every form an agent might need: the
// ISO 8601 duration that Add turns from back into to, its calendar parts, the
// exact total seconds, and the whole calendar days. Calendar parts use the
// wall clock of loc.
func DiffJSON(from, to time.Time, loc *time.Location) (string, error) {
	d := Diff(from, to, loc)
	sec, nanos := elapsed(from, to)
	type calendar struct {
		Years   int64       `json:"years"`
		Months  int64       `json:"months"`
		Days    int64       `json:"days"`
		Hours   int64       `json:"hours"`
		Minutes int64       `json:"minutes"`
		Seconds json.Number `json:"seconds"`
	}
	repr := struct {
		From         string      `json:"from"`
		To           string      `json:"to"`
		Zone         string      `json:"zone"`
		Duration     string      `json:"duration"`
		TotalSeconds json.Number `json:"total_seconds"`
		TotalDays    int64       `json:"total_days"`
		Calendar     calendar    `json:"calendar"`
	}{
		From:         from.In(loc).Format(time.RFC3339Nano),
		To:           to.In(loc).Format(time.RFC3339Nano),
		Zone:         loc.String(),
		Duration:     d.String(),
		TotalSeconds: json.Number(decimalSeconds(sec, nanos)),
		TotalDays:    wholeDays(from, to, 0, loc),
		Calendar: calendar{
			Years:   d.Months / 12,
			Months:  d.Months % 12,
			Days:    d.Days,
			Hours:   d.Seconds / 3600,
			Minutes: d.Seconds % 3600 / 60,
			Seconds: json.Number(decimalSeconds(d.Seconds%60, d.Nanos)),
		},
	}
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding difference: %w", err)
	}
	return string(out), nil
}
