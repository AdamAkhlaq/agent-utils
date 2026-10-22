#!/bin/sh
# Pass when /out/pipeline.json holds the data Python's tomllib reads from the
# fixture. Integers must be JSON integers. A TOML date, time, or date-time
# must be a string that parses back to the same value, and an offset
# date-time must keep its offset; RFC 3339 allows a space instead of the T.
# The traps are hex and octal integers, underscores in numbers, a literal
# string full of backslashes, and a line-ending backslash in a multi-line
# string.
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
import datetime, tomllib
got = load_json_out("/out/pipeline.json")
with open("/task/fixtures/pipeline.toml", "rb") as f:
    want = tomllib.load(f)

def check(g, w, path):
    if isinstance(w, datetime.datetime):
        if not isinstance(g, str):
            return f"{path}: expected a date-time string, got {g!r}"
        try:
            v = datetime.datetime.fromisoformat(g)
        except ValueError:
            return f"{path}: {g!r} is not an RFC 3339 date-time"
        if v != w or v.utcoffset() != w.utcoffset():
            return f"{path}: expected {w.isoformat()}, got {g!r}"
        return None
    for kind in (datetime.date, datetime.time):
        if type(w) is kind:
            if not isinstance(g, str):
                return f"{path}: expected a {kind.__name__} string, got {g!r}"
            try:
                v = kind.fromisoformat(g)
            except ValueError:
                return f"{path}: {g!r} is not a {kind.__name__}"
            return None if v == w else f"{path}: expected {w.isoformat()}, got {g!r}"
    if isinstance(w, dict) and isinstance(g, dict):
        if set(g) != set(w):
            return f"{path}: keys differ: missing {sorted(set(w) - set(g))}, unexpected {sorted(set(g) - set(w))}"
        for k in w:
            d = check(g[k], w[k], f"{path}.{k}")
            if d:
                return d
        return None
    if isinstance(w, list) and isinstance(g, list):
        if len(g) != len(w):
            return f"{path}: expected {len(w)} items, got {len(g)}"
        for i, (a, b) in enumerate(zip(g, w)):
            d = check(a, b, f"{path}[{i}]")
            if d:
                return d
        return None
    if isinstance(w, int) and not isinstance(w, bool) and not (isinstance(g, int) and not isinstance(g, bool)):
        return f"{path}: expected the integer {w}, got {g!r}"
    return first_difference(g, w, path)

d = check(got, want, "$")
if d:
    sys.exit(f"documents differ at {d}")
PY
