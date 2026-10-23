#!/bin/sh
# Pass when /out/links.json holds the parts of every link in file order.
# Key order and whitespace do not matter; query values must keep their order.
exec python3 -I - <<'PY'
import json, sys

KEYS = ["fragment", "host", "path", "port", "query", "scheme"]
try:
    with open("/out/links.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/links.json was not written")
except (ValueError, UnicodeDecodeError) as e:
    sys.exit(f"/out/links.json is not valid JSON: {e}")
with open("/task/expected/links.json", encoding="utf-8") as f:
    want = json.load(f)

if not isinstance(got, list):
    sys.exit(f"expected a JSON array, got {type(got).__name__}")
if len(got) != len(want):
    sys.exit(f"expected {len(want)} links, got {len(got)}")
for i, (g, w) in enumerate(zip(got, want)):
    if not isinstance(g, dict) or sorted(g) != KEYS:
        sys.exit(f"[{i}]: expected an object with keys {KEYS}, got {g!r}")
    for k in KEYS:
        if g[k] != w[k]:
            sys.exit(f"[{i}].{k}: expected {w[k]!r}, got {g[k]!r}")
PY
