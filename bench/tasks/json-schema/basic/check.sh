#!/bin/sh
# Pass when /out/invalid.json is a JSON array of exactly the invalid orders'
# positions. The traps are spec details a hand-written validator tends to get
# wrong: 2026-02-29 is not a date (2028-02-29 is), 2.0 is an integer, and
# 19.99 and 0.07 are multiples of 0.01 (exactly, not by float remainder).
# The order of the positions is not enforced beyond being a set.
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
got = load_json_out("/out/invalid.json")
with open("/task/expected/invalid.json") as f:
    want = json.load(f)
if not isinstance(got, list) or not all(isinstance(x, int) and not isinstance(x, bool) for x in got):
    sys.exit(f"expected a JSON array of integers, got {got!r}")
if sorted(got) != want:
    extra = sorted(set(got) - set(want))
    missing = sorted(set(want) - set(got))
    sys.exit(f"wrong positions: want {want}, got {sorted(got)} (wrongly listed {extra}, missed {missing})")
PY
