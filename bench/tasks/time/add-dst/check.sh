#!/bin/sh
# Pass when /out/next.txt is one RFC 3339 timestamp for 2025-10-05 09:15 on
# Lord Howe Island, written with that day's local offset. Lord Howe's daylight
# saving shifts by 30 minutes, not 60: standard time is +10:30 and daylight
# time, which began overnight, is +11:00. Assuming a one-hour shift (+11:30)
# or the old offset (+10:30) is wrong.
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
want = datetime(2025, 10, 5, 9, 15, 0, tzinfo=timezone(timedelta(hours=11)))
if got != want:
    sys.exit(f"wrong instant: want {want.isoformat()}, got {got.isoformat()}")
if got.utcoffset() != timedelta(hours=11):
    sys.exit(f"wrong offset: want +11:00 (Lord Howe daylight time), got {got.isoformat()[-6:]}")
PY
