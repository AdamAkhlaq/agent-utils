#!/bin/sh
# Pass when /out/findings.json lists every general-category Cf character of the
# fixture, in file order, with its 1-based line and its U+XXXX codepoint.
# Whitespace does not matter.
exec python3 -I - <<'PY'
import json, sys

try:
    with open("/out/findings.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/findings.json was not written")
except (ValueError, UnicodeDecodeError) as e:
    sys.exit(f"/out/findings.json is not valid JSON: {e}")
with open("/task/expected/findings.json", encoding="utf-8") as f:
    want = json.load(f)

if not isinstance(got, list):
    sys.exit(f"expected a JSON array, got {type(got).__name__}")
for i, g in enumerate(got):
    if not isinstance(g, dict) or sorted(g) != ["codepoint", "line"]:
        sys.exit(f"[{i}]: expected an object with exactly line and codepoint, got {g!r}")
    if type(g["line"]) is not int:
        sys.exit(f"[{i}].line: expected an integer, got {g['line']!r}")
if got != want:
    missing = [w for w in want if w not in got]
    extra = [g for g in got if g not in want]
    if missing or extra:
        sys.exit(f"findings differ: missing {missing}, unexpected {extra}")
    sys.exit(f"findings are right but out of order: got {got}, want {want}")
PY
