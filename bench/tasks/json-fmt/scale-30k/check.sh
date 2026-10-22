#!/bin/sh
# Pass when /out/events.min.json holds the source's data with keys in the
# source order and no whitespace outside strings, ignoring one trailing line
# break.
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
import re
# Objects become lists of [key, value] pairs, so key order is compared too.
pairs = lambda kv: [list(p) for p in kv]
got = load_json_out("/out/events.min.json", object_pairs_hook=pairs)
with open("/task/fixtures/events.json", encoding="utf-8") as f:
    want = json.load(f, object_pairs_hook=pairs)
d = first_difference(got, want)
if d:
    sys.exit(f"documents differ (or keys are out of order) at {d}")
with open("/out/events.min.json", encoding="utf-8") as f:
    text = f.read()
text = text[:-1] if text.endswith("\n") else text
outside = re.sub(r'"(?:\\.|[^"\\])*"', '""', text)
m = re.search(r"\s", outside)
if m:
    sys.exit(f"whitespace outside a string at offset {m.start()}: {outside[max(0, m.start() - 20):m.start() + 20]!r}")
PY
