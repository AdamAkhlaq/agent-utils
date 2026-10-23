#!/bin/sh
# Pass when Python's tomllib reads /out/service.toml as exactly the JSON
# fixture's data, with integers and floats kept distinct as the instruction
# asks. Keys with a dot or a space must be quoted to survive.
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
import tomllib
try:
    with open("/out/service.toml", "rb") as f:
        got = tomllib.load(f)
except FileNotFoundError:
    sys.exit("/out/service.toml was not written")
except tomllib.TOMLDecodeError as e:
    sys.exit(f"/out/service.toml is not valid TOML: {e}")
with open("/task/fixtures/service.json", encoding="utf-8") as f:
    want = json.load(f)
d = first_difference(got, want, strict_numbers=True)
if d:
    sys.exit(f"documents differ at {d}")
PY
