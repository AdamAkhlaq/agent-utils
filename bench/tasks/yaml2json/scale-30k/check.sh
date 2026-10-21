#!/bin/sh
# Pass when /out/config.json parses as JSON and equals the expected document
# after parsing, so key order, whitespace, and number spelling do not matter.
exec python3 -I - <<'PY'
import json, sys
try:
    with open("/out/config.json") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/config.json was not written")
except ValueError as e:
    sys.exit(f"/out/config.json is not valid JSON: {e}")
with open("/task/expected/config.json") as f:
    want = json.load(f)
def first_difference(a, b, path="$"):
    if type(a) is not type(b) and not (isinstance(a, (int, float)) and isinstance(b, (int, float))):
        return f"{path}: expected {type(b).__name__}, got {type(a).__name__}"
    if isinstance(a, dict):
        for k in b:
            if k not in a:
                return f"{path}.{k}: missing"
        for k in a:
            if k not in b:
                return f"{path}.{k}: unexpected key"
        for k in b:
            d = first_difference(a[k], b[k], f"{path}.{k}")
            if d:
                return d
    elif isinstance(a, list):
        if len(a) != len(b):
            return f"{path}: expected {len(b)} items, got {len(a)}"
        for i, (x, y) in enumerate(zip(a, b)):
            d = first_difference(x, y, f"{path}[{i}]")
            if d:
                return d
    elif a != b:
        return f"{path}: expected {b!r}, got {a!r}"
    return None

if got != want:
    sys.exit(f"documents differ at {first_difference(got, want)}")
PY
