#!/bin/sh
# Pass when /out/next.txt is one RFC 3339 timestamp for 2024-03-10 12:00 in
# New York, written with that day's local offset (-04:00: DST began overnight).
exec python3 -I - <<'PY'
import sys
from datetime import datetime, timedelta, timezone
try:
    with open("/out/next.txt") as f:
        text = f.read()
except FileNotFoundError:
    sys.exit("/out/next.txt was not written")
lines = text.strip().splitlines()
if len(lines) != 1:
    sys.exit(f"expected one line, got {len(lines)}")
try:
    got = datetime.fromisoformat(lines[0].strip())
except ValueError as e:
    sys.exit(f"not an RFC 3339 timestamp: {e}")
if got.tzinfo is None:
    sys.exit("the timestamp has no UTC offset")
want = datetime(2024, 3, 10, 12, 0, 0, tzinfo=timezone(timedelta(hours=-4)))
if got != want:
    sys.exit(f"wrong instant: want {want.isoformat()}, got {got.isoformat()}")
if got.utcoffset() != timedelta(hours=-4):
    sys.exit(f"wrong offset: want -04:00 (New York after the DST change), got {got.isoformat()[-6:]}")
PY
