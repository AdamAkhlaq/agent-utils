#!/bin/sh
# Pass when /out/claims.json parses as JSON and equals the token's payload,
# whatever the key order or whitespace.
exec python3 -I - <<'PY'
import json, sys
try:
    with open("/out/claims.json") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/claims.json was not written")
except ValueError as e:
    sys.exit(f"/out/claims.json is not valid JSON: {e}")
with open("/task/expected/claims.json") as f:
    want = json.load(f)
if got != want:
    sys.exit(f"claims differ\nwant: {json.dumps(want, sort_keys=True)}\ngot:  {json.dumps(got, sort_keys=True)}")
PY
