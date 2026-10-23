#!/bin/sh
# Pass when /out/order.json holds the source's data with keys in the source
# order, and its lines are indented exactly as a two-space, one-member-per-line
# layout indents them. Only the indentation of each line is compared, so the
# spelling of numbers and strings (1.0 or 1, "é" or "\u00e9") is free.
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
# Objects become lists of [key, value] pairs, so key order is compared too.
pairs = lambda kv: [list(p) for p in kv]
got = load_json_out("/out/order.json", object_pairs_hook=pairs)
with open("/task/fixtures/order.json", encoding="utf-8") as f:
    want = json.load(f, object_pairs_hook=pairs)
d = first_difference(got, want)
if d:
    sys.exit(f"documents differ (or keys are out of order) at {d}")
with open("/out/order.json", encoding="utf-8") as f:
    lines = f.read().rstrip("\n").split("\n")
with open("/task/fixtures/order.json", encoding="utf-8") as f:
    ref = json.dumps(json.load(f), indent=2, ensure_ascii=False).split("\n")
indent = lambda l: len(l) - len(l.lstrip(" "))
if len(lines) != len(ref):
    sys.exit(f"expected {len(ref)} lines in the two-space layout, got {len(lines)}")
for i, (g, r) in enumerate(zip(lines, ref), 1):
    if g[:1] == "\t" or g.lstrip(" ")[:1] == "\t":
        sys.exit(f"line {i} is indented with a tab")
    if indent(g) != indent(r):
        sys.exit(f"line {i} is indented {indent(g)} spaces, want {indent(r)}: {g.strip()[:60]!r}")
PY
