#!/bin/sh
# Pass when /out/deploys.log is the fixture with each leading epoch replaced by
# an RFC 3339 timestamp for the same instant, written with Adelaide's offset at
# that instant. The log crosses the end of daylight saving on 2026-04-05, when
# the offset drops from +10:30 to +09:30 and 02:00 to 03:00 happens twice, so
# the right instant with the wrong offset fails. Trailing whitespace is ignored.
exec python3 -I - <<'EOF'
import re, sys
from datetime import datetime
from zoneinfo import ZoneInfo

zone = ZoneInfo("Australia/Adelaide")
with open("/task/fixtures/deploys.log") as f:
    want = [line.split(" ", 1) for line in f.read().splitlines()]
try:
    with open("/out/deploys.log", encoding="utf-8") as f:
        got = [line.rstrip() for line in f.read().strip().splitlines()]
except FileNotFoundError:
    sys.exit("/out/deploys.log was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/deploys.log is not UTF-8: {e}")
if got == []:
    sys.exit("/out/deploys.log is empty")
if len(got) != len(want):
    sys.exit(f"expected {len(want)} lines, got {len(got)}")
for n, (g, (epoch, message)) in enumerate(zip(got, want), 1):
    stamp, _, rest = g.partition(" ")
    if rest != message.rstrip():
        sys.exit(f"line {n}: expected the message {message!r} after the timestamp, got {rest!r}")
    try:
        t = datetime.fromisoformat(stamp)
    except ValueError:
        t = None
    if t is None or t.tzinfo is None or not re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?([+-]\d\d:\d\d|Z)", stamp):
        sys.exit(f"line {n}: {stamp!r} is not an RFC 3339 timestamp with an offset")
    w = datetime.fromtimestamp(int(epoch), zone)
    # Compared as epoch seconds: PEP 495 makes == between zones false for any
    # datetime in a repeated hour, even when both name the same instant.
    if t.timestamp() != int(epoch):
        sys.exit(f"line {n}: wrong instant: expected {w.isoformat()}, got {stamp}")
    if t.utcoffset() != w.utcoffset():
        sys.exit(f"line {n}: wrong offset: expected {w.isoformat()}, got {stamp}")
EOF
