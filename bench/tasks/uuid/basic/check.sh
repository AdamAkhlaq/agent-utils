#!/bin/sh
# Pass when /out/devices.json is the fixture with every id replaced by a
# distinct, lowercase, canonical version 4 UUID. A random UUID's 30 free hex
# digits use about 14 of the 16 digits, and fewer than 8 with odds of 1.8 in
# 10 million, so fewer than 8 marks a pattern or a counter, not chance.
exec python3 -I - <<'PY'
import json, re, sys

V4 = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}")
with open("/task/fixtures/devices.json") as f:
    want = json.load(f)
try:
    with open("/out/devices.json") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/devices.json was not written")
except ValueError as e:
    sys.exit(f"/out/devices.json is not valid JSON: {e}")
if not isinstance(got, list) or len(got) != len(want):
    sys.exit(f"expected an array of {len(want)} devices, got {type(got).__name__} of length {len(got) if isinstance(got, list) else 'n/a'}")
ids = []
for i, (g, w) in enumerate(zip(got, want)):
    if not isinstance(g, dict):
        sys.exit(f"[{i}]: expected an object, got {g!r}")
    rest = {k: v for k, v in g.items() if k != "id"}
    expected_rest = {k: v for k, v in w.items() if k != "id"}
    if rest != expected_rest:
        sys.exit(f"[{i}]: fields other than id changed: expected {expected_rest!r}, got {rest!r}")
    v = g.get("id")
    if not isinstance(v, str) or not V4.fullmatch(v):
        sys.exit(f"[{i}].id: expected a lowercase canonical version 4 UUID, got {v!r}")
    free = v.replace("-", "")
    free = free[:12] + free[13:16] + free[17:]
    if len(set(free)) < 8:
        sys.exit(f"[{i}].id: {v!r} uses only {len(set(free))} distinct hex digits, which looks like a pattern, not random")
    ids.append(v)
if len(set(ids)) != len(ids):
    sys.exit(f"the ids are not all distinct: {ids!r}")
PY
