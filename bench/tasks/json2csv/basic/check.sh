#!/bin/sh
# Pass when Python's csv module reads /out/regions.csv as the expected header
# and rows. Strings must match exactly (a line break inside a field
# included), numbers may be spelled any way that has the same value, booleans
# must be lowercase true or false, and null or missing gives an empty field.
exec python3 -I - <<'PY'
import json, sys

def is_num(x):
    return isinstance(x, (int, float)) and not isinstance(x, bool)

def first_difference(got, want, path="$", strict_numbers=False):
    if is_num(got) and is_num(want):
        if strict_numbers and type(got) is not type(want):
            return f"{path}: expected {type(want).__name__} {want!r}, got {type(got).__name__} {got!r}"
        return None if got == want else f"{path}: expected {want!r}, got {got!r}"
    if type(got) is not type(want):
        return f"{path}: expected {type(want).__name__} {want!r}, got {type(got).__name__} {got!r}"
    if isinstance(want, dict):
        for k in want:
            if k not in got:
                return f"{path}.{k}: missing"
        for k in got:
            if k not in want:
                return f"{path}.{k}: unexpected key"
        for k in want:
            d = first_difference(got[k], want[k], f"{path}.{k}", strict_numbers)
            if d:
                return d
        return None
    if isinstance(want, list):
        if len(got) != len(want):
            return f"{path}: expected {len(want)} items, got {len(got)}"
        for i, (g, w) in enumerate(zip(got, want)):
            d = first_difference(g, w, f"{path}[{i}]", strict_numbers)
            if d:
                return d
        return None
    return None if got == want else f"{path}: expected {want!r}, got {got!r}"

def load_json_out(path, **kw):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f, **kw)
    except FileNotFoundError:
        sys.exit(f"{path} was not written")
    except (ValueError, UnicodeDecodeError) as e:
        sys.exit(f"{path} is not valid JSON: {e}")
import csv
try:
    with open("/out/regions.csv", encoding="utf-8", newline="") as f:
        rows = list(csv.reader(f))
except FileNotFoundError:
    sys.exit("/out/regions.csv was not written")
except (csv.Error, UnicodeDecodeError) as e:
    sys.exit(f"/out/regions.csv is not readable CSV: {e}")
with open("/task/fixtures/regions.json", encoding="utf-8") as f:
    records = json.load(f)
header = []
for r in records:
    for k in r:
        if k not in header:
            header.append(k)
if not rows:
    sys.exit("/out/regions.csv is empty")
if rows[0] != header:
    sys.exit(f"wrong header: want {header}, got {rows[0]}")
if len(rows) - 1 != len(records):
    sys.exit(f"expected {len(records)} data rows, got {len(rows) - 1}")
for n, (row, rec) in enumerate(zip(rows[1:], records), 2):
    if len(row) != len(header):
        sys.exit(f"row {n} has {len(row)} fields, want {len(header)}: {row}")
    for col, cell in zip(header, row):
        v = rec.get(col)
        where = f"row {n}, column {col!r}"
        if v is None:
            ok = cell == ""
        elif isinstance(v, bool):
            ok = cell == ("true" if v else "false")
        elif is_num(v):
            try:
                ok = float(cell) == v
            except ValueError:
                ok = False
        else:
            ok = cell == v
        if not ok:
            sys.exit(f"{where}: want {v!r}, got {cell!r}")
PY
