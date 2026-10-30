#!/bin/sh
# Pass when /out/end-dates.csv has the header contract_id,end_date and each
# contract's start date plus its term in calendar months, with a day past the
# end of the target month clamped to its last day. GNU date's "+N months" and
# Go's AddDate roll over into the next month instead, and counting 30 days per
# month drifts. Whitespace around fields and a trailing CR are ignored.
exec python3 -I - <<'EOF'
import calendar, csv, sys
from datetime import date

with open("/task/fixtures/contracts.csv", newline="") as f:
    want = []
    for r in csv.DictReader(f):
        y, m, d = map(int, r["start_date"].split("-"))
        ty, tm = divmod(y * 12 + m - 1 + int(r["term_months"]), 12)
        tm += 1
        want.append([r["contract_id"], date(ty, tm, min(d, calendar.monthrange(ty, tm)[1])).isoformat()])
try:
    with open("/out/end-dates.csv", encoding="utf-8", newline="") as f:
        rows = [[c.strip() for c in r] for r in csv.reader(f) if any(c.strip() for c in r)]
except FileNotFoundError:
    sys.exit("/out/end-dates.csv was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/end-dates.csv is not UTF-8: {e}")
if rows == []:
    sys.exit("/out/end-dates.csv is empty")
if rows[0] != ["contract_id", "end_date"]:
    sys.exit(f"expected the header contract_id,end_date, got {','.join(rows[0])}")
rows = rows[1:]
if len(rows) != len(want):
    sys.exit(f"expected {len(want)} rows after the header, got {len(rows)}")
for n, (g, w) in enumerate(zip(rows, want), 2):
    if g != w:
        sys.exit(f"row {n}: expected {','.join(w)}, got {','.join(g)}")
EOF
