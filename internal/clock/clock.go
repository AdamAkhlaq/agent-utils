// Package clock parses and formats timestamps for the time command. Nothing
// here calls time.Now(): the reference time is always passed in, so every
// caller and test is deterministic.
package clock

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Integers at or above this are epoch milliseconds, below it epoch seconds.
// 1e11 seconds is the year 5138 and 1e11 milliseconds is March 1973, so each
// side of the line covers every plausible timestamp of the other unit.
const msThreshold = 100_000_000_000

// layouts are tried in order; longer variants come before their prefixes so
// inputs with seconds aren't cut short. Zone-less layouts parse as UTC.
var layouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
	time.RFC1123Z,
	time.RFC1123,
}

// Parse turns s into a time: "now" (the passed reference time), epoch seconds
// or milliseconds, RFC 3339, RFC 1123, or a date with optional time.
func Parse(s string, now time.Time) (time.Time, error) {
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
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
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
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding representations: %w", err)
	}
	return string(out), nil
}
