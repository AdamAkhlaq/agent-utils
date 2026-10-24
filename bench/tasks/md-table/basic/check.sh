#!/bin/sh
# Pass when /out/jobs.md is a GFM pipe table of the fixture: header, delimiter
# row, one row per job, with the columns in first-seen key order and each
# value as written in the source (numbers keep their literal, so 0.50 is not
# 0.5). Every line must have its unescaped pipes at the same character
# positions. Leading and trailing pipes are optional, a row with fewer cells
# than the header gets empty ones (as GFM renders it), the delimiter cells may
# carry alignment colons, and a backslash before ASCII punctuation is read as
# Markdown reads it (so \| and \* count as | and *). Surrounding whitespace is
# ignored.
exec python3 -I - <<'PY'
import json, re, sys

with open("/task/fixtures/jobs.json", encoding="utf-8") as f:
    jobs = json.load(f, parse_float=str, parse_int=str)
columns = []
for job in jobs:
    for key in job:
        if key not in columns:
            columns.append(key)

def text(v):
    if v is None:
        return ""
    if v is True:
        return "true"
    if v is False:
        return "false"
    return v

want = [[text(job.get(c)) for c in columns] for job in jobs]

try:
    with open("/out/jobs.md", encoding="utf-8") as f:
        lines = [line.rstrip() for line in f.read().strip().split("\n")]
except FileNotFoundError:
    sys.exit("/out/jobs.md was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/jobs.md is not UTF-8: {e}")
if len(lines) != len(jobs) + 2:
    sys.exit(f"expected {len(jobs) + 2} lines (header, delimiter row, {len(jobs)} jobs), got {len(lines)}")

def pipes(line):
    return [i for i, c in enumerate(line) if c == "|" and (i == 0 or line[i - 1] != "\\")]

def cells(line, at):
    bounds = [-1] + at + [len(line)]
    parts = [line[a + 1:b] for a, b in zip(bounds, bounds[1:])]
    if at and not line[:at[0]].strip(" "):
        parts = parts[1:]
    if at and not line[at[-1] + 1:].strip(" "):
        parts = parts[:-1]
    return [re.sub(r"\\([!-/:-@\[-`{-~])", r"\1", p.strip(" ")) for p in parts]

positions = pipes(lines[0])
for n, line in enumerate(lines, 1):
    at = pipes(line)
    if at != positions:
        sys.exit(f"line {n}: pipes at character positions {at}, but line 1 has them at {positions}, so the columns do not line up")

header = cells(lines[0], positions)
if header != columns:
    sys.exit(f"header: expected columns {columns}, got {header}")
delim = cells(lines[1], positions)
bad = [d for d in delim if not re.fullmatch(r":?-+:?", d)]
if len(delim) != len(columns) or bad:
    sys.exit(f"line 2 is not a delimiter row: {lines[1]!r}")
for r, (line, row) in enumerate(zip(lines[2:], want), 1):
    got = cells(line, positions)
    if len(got) > len(row):
        sys.exit(f"row {r} has {len(got)} cells, more than the {len(row)} columns")
    # GFM fills a row that has fewer cells than the header with empty cells.
    got += [""] * (len(row) - len(got))
    for c, g, w in zip(columns, got, row):
        if g != w:
            sys.exit(f"row {r}, column {c!r}: expected {w!r}, got {g!r}")
PY
