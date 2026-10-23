#!/bin/sh
# Pass when /out/report.json maps every upload to its real media type and, for
# images only, its dimensions. Key order and whitespace do not matter; media
# types compare case-insensitively, without parameters, and accept the common
# alias for ZIP.
exec python3 -I - <<'PY'
import json, sys

ALIASES = {"application/x-zip-compressed": "application/zip"}

def media_type(v):
    if not isinstance(v, str):
        return None
    t = v.split(";")[0].strip().lower()
    return ALIASES.get(t, t)

try:
    with open("/out/report.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/report.json was not written")
except (ValueError, UnicodeDecodeError) as e:
    sys.exit(f"/out/report.json is not valid JSON: {e}")
with open("/task/expected/report.json", encoding="utf-8") as f:
    want = json.load(f)

if not isinstance(got, dict):
    sys.exit(f"expected a JSON object, got {type(got).__name__}")
if sorted(got) != sorted(want):
    sys.exit(f"file names are {sorted(got)}, want {sorted(want)}")
for name, w in want.items():
    g = got[name]
    if not isinstance(g, dict):
        sys.exit(f"{name}: expected an object, got {g!r}")
    if sorted(g) != sorted(w):
        sys.exit(f"{name}: keys are {sorted(g)}, want {sorted(w)}")
    if media_type(g["mime"]) != w["mime"]:
        sys.exit(f"{name}: mime is {g['mime']!r}, want {w['mime']!r}")
    for k in ("width", "height"):
        if k in w and (type(g[k]) is not int or g[k] != w[k]):
            sys.exit(f"{name}: {k} is {g[k]!r}, want {w[k]}")
PY
