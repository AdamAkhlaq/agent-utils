package clock

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Cron expressions follow Vixie cron and cronie: five fields, lists, ranges,
// steps, three-letter month and weekday names, and the @ macros. Anything
// cronie rejects or warns about is an error here, as is anything older
// cronie or Debian's Vixie cron reads differently and the Quartz, Jenkins,
// and cronie-random extensions, so an accepted expression means the same
// thing to every standard cron daemon.

type cronField struct {
	name     string
	min, max int
	names    []string
}

var cronFields = [5]cronField{
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day-of-month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}},
	// 7 is also Sunday, as in cronie.
	{name: "day-of-week", min: 0, max: 7, names: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}},
}

const (
	fMinute = iota
	fHour
	fDOM
	fMonth
	fDOW
)

var cronMacros = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

const cronMacroList = "@yearly, @annually, @monthly, @weekly, @daily, @midnight, @hourly"

var quartzLastOrWeekday = regexp.MustCompile(`^(\d*L|LW|\d+W)$`)

// cronSchedule is a parsed expression: one bit per allowed value of each
// field, plus the flags cronie derives from a field's leading "*".
type cronSchedule struct {
	expr string
	sets [5]uint64
	// When either day field starts with "*", a day must match both of them;
	// otherwise it may match either (the Vixie cron day rule).
	domStar, dowStar bool
	// A "*" leading the minute or hour field makes the job a wildcard job,
	// which cronie treats differently across a DST change.
	wildcard bool
}

func parseCron(expr string) (cronSchedule, error) {
	if i := strings.IndexFunc(expr, func(r rune) bool { return r != '\t' && unicode.IsControl(r) }); i >= 0 {
		return cronSchedule{}, fmt.Errorf("unexpected control character %q: pass one expression on one line", []rune(expr[i:])[0])
	}
	fields := strings.FieldsFunc(expr, func(r rune) bool { return r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return cronSchedule{}, errors.New("empty cron expression")
	}
	if strings.HasPrefix(fields[0], "@") {
		return parseCronMacro(fields)
	}
	switch {
	case len(fields) == 6 || len(fields) == 7:
		return cronSchedule{}, fmt.Errorf("expected 5 fields (minute hour day-of-month month day-of-week), got %d: seconds and year fields (Quartz, Spring) are not standard cron, and a crontab line's user and command must be left out", len(fields))
	case len(fields) != 5:
		return cronSchedule{}, fmt.Errorf("expected 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	s := cronSchedule{expr: strings.Join(fields, " ")}
	for i, f := range fields {
		set, err := parseCronField(f, cronFields[i])
		if err != nil {
			return cronSchedule{}, fmt.Errorf("%s field %q: %w", cronFields[i].name, f, err)
		}
		s.sets[i] = set
	}
	s.domStar = fields[fDOM][0] == '*'
	s.dowStar = fields[fDOW][0] == '*'
	s.wildcard = fields[fMinute][0] == '*' || fields[fHour][0] == '*'
	return s, nil
}

func parseCronMacro(fields []string) (cronSchedule, error) {
	macro := fields[0]
	if macro == "@reboot" {
		return cronSchedule{}, errors.New("@reboot runs once when the cron daemon starts, so it has no schedule to list")
	}
	equivalent, ok := cronMacros[macro]
	if !ok {
		if _, lower := cronMacros[strings.ToLower(macro)]; lower {
			return cronSchedule{}, fmt.Errorf("unknown macro %q: macros are lowercase (%s)", macro, strings.ToLower(macro))
		}
		return cronSchedule{}, fmt.Errorf("unknown macro %q (valid: %s)", macro, cronMacroList)
	}
	if len(fields) > 1 {
		return cronSchedule{}, fmt.Errorf("%s replaces all five fields, so nothing may follow it", macro)
	}
	s, err := parseCron(equivalent)
	if err != nil {
		return cronSchedule{}, err
	}
	s.expr = macro
	return s, nil
}

func parseCronField(field string, f cronField) (uint64, error) {
	if err := cronExtension(field); err != nil {
		return 0, err
	}
	var set uint64
	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return 0, errors.New("empty list element")
		}
		bits, err := parseCronRange(part, f)
		if err != nil {
			return 0, err
		}
		set |= bits
	}
	return set, nil
}

// cronExtension names the non-standard syntax an expression copied from
// another scheduler is most likely to contain, so the error says why it is
// rejected instead of just calling it invalid.
func cronExtension(field string) error {
	switch {
	case strings.Contains(field, "?"):
		return errors.New(`"?" (no specific value) is a Quartz extension, not standard cron: use *`)
	case strings.Contains(field, "#"):
		return errors.New(`"#" (nth weekday of the month) is a Quartz extension, not standard cron`)
	case strings.Contains(field, "~"):
		return errors.New(`"~" (a random value) is a cronie extension; only deterministic schedules are accepted`)
	}
	for _, tok := range strings.FieldsFunc(field, func(r rune) bool { return r == ',' || r == '-' || r == '/' }) {
		upper := strings.ToUpper(tok)
		switch {
		case quartzLastOrWeekday.MatchString(upper):
			return fmt.Errorf("%q (last day or nearest weekday) is a Quartz extension, not standard cron", tok)
		case upper == "H" || strings.HasPrefix(upper, "H("):
			return fmt.Errorf("%q (a hashed value) is a Jenkins extension, not standard cron", tok)
		}
	}
	return nil
}

func parseCronRange(part string, f cronField) (uint64, error) {
	span, stepText, hasStep := strings.Cut(part, "/")
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepText)
		switch {
		case stepText == "":
			return 0, fmt.Errorf("%q has no step after the /", part)
		case strings.Trim(stepText, "0123456789") != "":
			return 0, fmt.Errorf("step %q is not a number", stepText)
		case err != nil:
			return 0, fmt.Errorf("step %s is too large", stepText)
		case n == 0:
			return 0, errors.New("step 0 never advances; use 1 or more")
		}
		step = n
	}
	var lo, hi int
	if span == "*" {
		lo, hi = f.min, f.max
	} else {
		first, last, isRange := strings.Cut(span, "-")
		var err error
		if lo, err = cronValue(first, f); err != nil {
			return 0, err
		}
		hi = lo
		if isRange {
			if hi, err = cronValue(last, f); err != nil {
				return 0, err
			}
			if lo > hi {
				return 0, reversedRangeError(span, lo, hi, f)
			}
		} else if hasStep {
			return 0, fmt.Errorf("a step needs a range or *, as in %d-%d/%d", lo, f.max, step)
		}
	}
	var set uint64
	for v := lo; v <= hi; v += step {
		set |= 1 << v
	}
	if f.name == "day-of-week" && set&(1<<7) != 0 {
		set = set&^(1<<7) | 1
	}
	if step > 1 && bits.OnesCount64(set) == 1 {
		return 0, fmt.Errorf("step %d over %d-%d would only select %d", step, lo, hi, lo)
	}
	return set, nil
}

func reversedRangeError(span string, lo, hi int, f cronField) error {
	top := f.max
	if f.name == "day-of-week" {
		switch {
		case lo == 7:
			return fmt.Errorf("range %q runs backwards: write Sunday as 0 at the start of a range, as in %s", span, cronSpan(0, hi))
		case hi == 0:
			return fmt.Errorf("range %q runs backwards: write Sunday as 7 at the end of a range, as in %d-7", span, lo)
		}
		top = 6
	}
	return fmt.Errorf("range %q runs backwards and ranges do not wrap around: use a list, as in %s,%s", span, cronSpan(lo, top), cronSpan(f.min, hi))
}

func cronSpan(lo, hi int) string {
	if lo == hi {
		return strconv.Itoa(lo)
	}
	return fmt.Sprintf("%d-%d", lo, hi)
}

func cronValue(tok string, f cronField) (int, error) {
	switch tok {
	case "":
		return 0, errors.New("missing a number or name")
	case "*":
		return 0, errors.New("* must stand alone or take a step (*/n), not be part of a range")
	}
	if strings.Trim(tok, "0123456789") == "" {
		n, err := strconv.Atoi(tok)
		if err != nil || n < f.min || n > f.max {
			return 0, fmt.Errorf("%s is out of range %d-%d", tok, f.min, f.max)
		}
		return n, nil
	}
	if f.names == nil {
		return 0, fmt.Errorf("%q is not a number", tok)
	}
	if i := slices.Index(f.names, strings.ToUpper(tok)); i >= 0 {
		return f.min + i, nil
	}
	return 0, fmt.Errorf("unknown %s name %q (use %s-%s or %d-%d)", f.name, tok, f.names[0], f.names[len(f.names)-1], f.min, f.max)
}

func (s cronSchedule) values(field int) []int {
	vals := []int{}
	for set := s.sets[field]; set != 0; set &= set - 1 {
		vals = append(vals, bits.TrailingZeros64(set))
	}
	return vals
}

func (s cronSchedule) matchesDay(day time.Time) bool {
	if s.sets[fMonth]&(1<<int(day.Month())) == 0 {
		return false
	}
	dom := s.sets[fDOM]&(1<<day.Day()) != 0
	dow := s.sets[fDOW]&(1<<int(day.Weekday())) != 0
	if s.domStar || s.dowStar {
		return dom && dow
	}
	return dom || dow
}

// neverRuns reports whether no calendar date matches. Only one shape can do
// that: an unrestricted day of week with days of the month that none of the
// months have (February 30). A field starting with "*" always includes the
// 1st, and a restricted day of week matches some day of every month.
func (s cronSchedule) neverRuns() bool {
	if !s.dowStar || s.domStar {
		return false
	}
	longest := [13]int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for _, m := range s.values(fMonth) {
		if bits.TrailingZeros64(s.sets[fDOM]) <= longest[m] {
			return false
		}
	}
	return true
}

const (
	// The Gregorian calendar, weekdays included, repeats every 400 years, so
	// a schedule with no run in that span after a run never runs again.
	gregorianCycleDays = 146097
	// UTC offsets in the tz database stay well within ±26 hours, so a
	// wall-clock time maps to an instant within that distance of the same
	// reading taken as UTC.
	maxZoneOffset   = 26 * 60 * 60
	lastRFC3339Year = 9999
)

// next returns the first n runs strictly after from, evaluated against wall
// clock time in loc and resolving DST changes the way cronie does for its
// own time zone.
func (s cronSchedule) next(from time.Time, loc *time.Location, n int) ([]time.Time, error) {
	if n < 1 {
		return nil, fmt.Errorf("run count must be at least 1, got %d", n)
	}
	if s.neverRuns() {
		return nil, fmt.Errorf("%q never runs: day-of-month %s does not occur in %s",
			s.expr, joinInts(s.values(fDOM)), monthNames(s.values(fMonth)))
	}
	after := from.Unix()
	wall := from.In(loc)
	// Starting two days early covers wall-clock times whose instants land
	// after from despite an earlier reading (a DST overlap).
	day := time.Date(wall.Year(), wall.Month(), wall.Day()-2, 0, 0, 0, 0, time.UTC)
	if day.Year() < 0 {
		day = time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	lastRun := day
	hours, minutes := s.values(fHour), s.values(fMinute)
	var runs []int64
	for len(runs) < n || day.Unix()-maxZoneOffset <= runs[n-1] {
		if day.Year() > lastRFC3339Year {
			if len(runs) >= n {
				break
			}
			return nil, fmt.Errorf("%q has only %d run(s) after %s up to the year %d, the last one RFC 3339 can represent",
				s.expr, len(runs), wall.Format(time.RFC3339), lastRFC3339Year)
		}
		if day.Unix()-lastRun.Unix() > gregorianCycleDays*24*60*60 {
			if len(runs) == 0 {
				return nil, fmt.Errorf("%q never runs after %s: it has no run in 400 years, a full calendar cycle",
					s.expr, wall.Format(time.RFC3339))
			}
			return nil, fmt.Errorf("%q has only %d run(s) after %s: none in the 400 years after the last",
				s.expr, len(runs), wall.Format(time.RFC3339))
		}
		if s.sets[fMonth]&(1<<int(day.Month())) == 0 {
			day = time.Date(day.Year(), day.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if s.matchesDay(day) {
			found := len(runs)
			runs = s.appendRuns(runs, day, hours, minutes, loc, after)
			slices.Sort(runs)
			runs = slices.Compact(runs)
			if len(runs) > found {
				lastRun = day
			}
			if len(runs) > n {
				runs = runs[:n]
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	out := make([]time.Time, len(runs))
	for i, r := range runs {
		out[i] = time.Unix(r, 0).In(loc)
	}
	return out, nil
}

// appendRuns appends the instants after the given unix second at which the
// schedule runs on the wall-clock day (a date at midnight UTC) in loc.
func (s cronSchedule) appendRuns(runs []int64, day time.Time, hours, minutes []int, loc *time.Location, after int64) []int64 {
	midnight := day.Unix()
	periods := zonePeriods(loc, midnight-maxZoneOffset, midnight+24*60*60+maxZoneOffset)
	for _, h := range hours {
		for _, m := range minutes {
			for _, t := range s.instants(midnight+int64(h)*3600+int64(m)*60, periods) {
				if t > after {
					runs = append(runs, t)
				}
			}
		}
	}
	return runs
}

// zonePeriod is a span of instants [start, end) with one UTC offset.
type zonePeriod struct {
	start, end, offset int64
}

func zonePeriods(loc *time.Location, from, to int64) []zonePeriod {
	var periods []zonePeriod
	t := time.Unix(from, 0).In(loc)
	for {
		_, offset := t.Zone()
		start, end := t.ZoneBounds()
		p := zonePeriod{start: math.MinInt64, end: math.MaxInt64, offset: int64(offset)}
		if !start.IsZero() {
			p.start = start.Unix()
		}
		if !end.IsZero() {
			p.end = end.Unix()
		}
		periods = append(periods, p)
		if end.IsZero() || p.end > to {
			return periods
		}
		t = end
	}
}

// instants resolves a wall-clock reading (in seconds, as if it were UTC) to
// the instants cronie runs a matching job at, following its DST rule: a
// fixed-time job whose time is skipped runs at the first instant after the
// gap, and one whose time repeats runs only the first time, while a
// wildcard job is skipped in a gap and runs both times in a repeat.
func (s cronSchedule) instants(reading int64, periods []zonePeriod) []int64 {
	var runs []int64
	for _, p := range periods {
		if t := reading - p.offset; t >= p.start && t < p.end && (len(runs) == 0 || s.wildcard) {
			runs = append(runs, t)
		}
	}
	if len(runs) > 0 || s.wildcard {
		return runs
	}
	for i := 1; i < len(periods); i++ {
		if p := periods[i]; reading >= p.start+periods[i-1].offset && reading < p.start+p.offset {
			return []int64{p.start}
		}
	}
	return nil
}

func joinInts(vals []int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

func monthNames(months []int) string {
	parts := make([]string, len(months))
	for i, m := range months {
		parts[i] = time.Month(m).String()
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " or " + parts[len(parts)-1]
}

// Cron validates a cron expression and returns its next n run times strictly
// after from, one RFC 3339 timestamp per line in loc.
func Cron(expr string, from time.Time, loc *time.Location, n int) (string, error) {
	_, runs, err := cronRuns(expr, from, loc, n)
	if err != nil {
		return "", err
	}
	return strings.Join(runs, "\n"), nil
}

// CronJSON is Cron as a JSON object that also shows how the expression was
// understood: each field's expanded values and how the two day fields
// combine, so an agent can confirm the schedule matches its intent.
func CronJSON(expr string, from time.Time, loc *time.Location, n int) (string, error) {
	s, runs, err := cronRuns(expr, from, loc, n)
	if err != nil {
		return "", err
	}
	type fields struct {
		Minute     []int `json:"minute"`
		Hour       []int `json:"hour"`
		DayOfMonth []int `json:"day_of_month"`
		Month      []int `json:"month"`
		DayOfWeek  []int `json:"day_of_week"`
	}
	repr := struct {
		Expression string   `json:"expression"`
		Zone       string   `json:"zone"`
		Fields     fields   `json:"fields"`
		DayMatch   string   `json:"day_match"`
		Next       []string `json:"next"`
	}{
		Expression: s.expr,
		Zone:       loc.String(),
		Fields: fields{
			Minute:     s.values(fMinute),
			Hour:       s.values(fHour),
			DayOfMonth: s.values(fDOM),
			Month:      s.values(fMonth),
			DayOfWeek:  s.values(fDOW),
		},
		DayMatch: "either",
		Next:     runs,
	}
	if s.domStar || s.dowStar {
		repr.DayMatch = "both"
	}
	out, err := json.MarshalIndent(repr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding schedule: %w", err)
	}
	return string(out), nil
}

func cronRuns(expr string, from time.Time, loc *time.Location, n int) (cronSchedule, []string, error) {
	s, err := parseCron(expr)
	if err != nil {
		return cronSchedule{}, nil, err
	}
	runs, err := s.next(from, loc, n)
	if err != nil {
		return cronSchedule{}, nil, err
	}
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.Format(time.RFC3339)
	}
	return s, out, nil
}

// CronFrom parses a -from timestamp like Parse, except that one without a
// UTC offset is wall-clock time in loc, the zone the schedule runs in.
func CronFrom(s string, now time.Time, loc *time.Location) (time.Time, error) {
	t, err := Parse(s, now)
	if err != nil {
		return t, err
	}
	trimmed := strings.TrimSpace(s)
	for _, layout := range layouts {
		if strings.Contains(layout, "Z07") || strings.Contains(layout, "-07") || strings.Contains(layout, "MST") {
			continue
		}
		if local, err := time.ParseInLocation(layout, trimmed, loc); err == nil {
			return local, nil
		}
	}
	return t, nil
}
