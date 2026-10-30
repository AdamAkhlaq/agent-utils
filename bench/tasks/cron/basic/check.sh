#!/bin/sh
# Pass when /out/next-runs.json maps each job's command to its next three runs
# after 2026-11-27T16:50:00Z in UTC, as crontab(5) defines them: names for
# months and weekdays, ranges and steps, Sunday as 0 or 7, and a day that
# matches either day field when both are restricted. The fixture is in UTC, so
# no daylight saving rule applies. Timestamps are compared as instants, so any
# RFC 3339 offset or fraction spelling of the right instant passes.
exec python3 -I - <<'EOF'
import json, re, sys
from datetime import date, datetime, timedelta, timezone

NAMES = {
    3: "jan feb mar apr may jun jul aug sep oct nov dec".split(),
    4: "sun mon tue wed thu fri sat".split(),
}
BOUNDS = [(0, 59), (0, 23), (1, 31), (1, 12), (0, 7)]

def value(field, s):
    if s.lower() in NAMES.get(field, []):
        return NAMES[field].index(s.lower()) + (1 if field == 3 else 0)
    return int(s)

def expand(field, text):
    lo, hi = BOUNDS[field]
    out = set()
    for part in text.split(","):
        rng, _, step = part.partition("/")
        if rng == "*":
            a, b = lo, hi
        elif "-" in rng:
            a, b = (value(field, x) for x in rng.split("-"))
        else:
            a = b = value(field, rng)
        out.update(range(a, b + 1, int(step or 1)))
    if field == 4 and 7 in out:
        out.add(0)
    return out

FROM = datetime(2026, 11, 27, 16, 50, tzinfo=timezone.utc)

def next_runs(fields, count=3):
    minutes, hours, doms, months, dows = (expand(i, f) for i, f in enumerate(fields))
    either = fields[2] != "*" and fields[4] != "*"
    runs, day = [], FROM.date()
    while len(runs) < count:
        dom_ok, dow_ok = day.day in doms, (day.isoweekday() % 7) in dows
        if day.month in months and ((dom_ok or dow_ok) if either else (dom_ok and dow_ok)):
            for h in sorted(hours):
                for m in sorted(minutes):
                    t = datetime(day.year, day.month, day.day, h, m, tzinfo=timezone.utc)
                    if t > FROM and len(runs) < count:
                        runs.append(t)
        day += timedelta(days=1)
    return runs

want = {}
with open("/task/fixtures/crontab") as f:
    for line in f:
        line = line.strip()
        if not line or line.startswith("#") or re.match(r"[A-Za-z_][A-Za-z0-9_]*=", line):
            continue
        parts = line.split(None, 5)
        want[parts[5]] = next_runs(parts[:5])

try:
    with open("/out/next-runs.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/next-runs.json was not written")
except (UnicodeDecodeError, json.JSONDecodeError) as e:
    sys.exit(f"/out/next-runs.json is not valid JSON: {e}")
if not isinstance(got, dict):
    sys.exit("the top level is not a JSON object")
missing = sorted(set(want) - set(got))
extra = sorted(set(got) - set(want))
if missing:
    sys.exit(f"no entry for {missing[0]}")
if extra:
    sys.exit(f"unexpected entry {extra[0]!r}")
for command, runs in want.items():
    g = got[command]
    if not isinstance(g, list) or len(g) != 3:
        sys.exit(f"{command}: expected an array of 3 timestamps, got {g!r}")
    for i, (s, w) in enumerate(zip(g, runs)):
        try:
            t = datetime.fromisoformat(s) if isinstance(s, str) else None
        except ValueError:
            t = None
        if t is None or t.tzinfo is None or not re.match(r"\d{4}-\d\d-\d\d[Tt ]\d\d:\d\d", s):
            sys.exit(f"{command}[{i}]: {s!r} is not an RFC 3339 timestamp with an offset")
        if t != w:
            sys.exit(f"{command}[{i}]: expected {w.strftime('%Y-%m-%dT%H:%M:%SZ')}, got {s}")
EOF
