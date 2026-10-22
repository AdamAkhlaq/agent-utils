#!/bin/sh
# Pass when PyYAML (a YAML 1.1 reader) and yq (a YAML 1.2 reader) both load
# /out/deploy.yaml as exactly the JSON fixture's data. Strings such as NO,
# 1.10, 0755, on, off, yes, and 2026-03-01 must be written so that neither
# reader turns them into a boolean, a number, or a date.
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
import subprocess, yaml
try:
    with open("/out/deploy.yaml", encoding="utf-8") as f:
        text = f.read()
except FileNotFoundError:
    sys.exit("/out/deploy.yaml was not written")
with open("/task/fixtures/deploy.json", encoding="utf-8") as f:
    want = json.load(f)
try:
    got = yaml.safe_load(text)
except yaml.YAMLError as e:
    sys.exit(f"a YAML 1.1 reader (PyYAML) cannot read it: {e}")
d = first_difference(got, want, strict_numbers=True)
if d:
    sys.exit(f"a YAML 1.1 reader (PyYAML) reads different data at {d}")
r = subprocess.run(["yq", "-p", "yaml", "-o", "json", ".", "/out/deploy.yaml"], capture_output=True, text=True)
if r.returncode != 0:
    sys.exit(f"a YAML 1.2 reader (yq) cannot read it: {r.stderr.strip()}")
d = first_difference(json.loads(r.stdout), want, strict_numbers=True)
if d:
    sys.exit(f"a YAML 1.2 reader (yq) reads different data at {d}")
PY
