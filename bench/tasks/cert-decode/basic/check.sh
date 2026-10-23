#!/bin/sh
# Pass when /out/certs.json lists the chain's certificates in file order with
# exactly the requested fields. dns_names may come in any order, and not_after
# may be any RFC 3339 spelling of the same UTC instant.
exec python3 -I - <<'PY'
import json, sys
from datetime import datetime, timezone

KEYS = ["subject_cn", "issuer_cn", "serial", "not_after", "dns_names", "sha256_fingerprint"]

def instant(s):
    if not isinstance(s, str):
        return None
    try:
        t = datetime.fromisoformat(s.replace("Z", "+00:00").replace("z", "+00:00"))
    except ValueError:
        return None
    if t.tzinfo is None or t.utcoffset().total_seconds() != 0:
        return None
    return t.astimezone(timezone.utc)

try:
    with open("/out/certs.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/certs.json was not written")
except (ValueError, UnicodeDecodeError) as e:
    sys.exit(f"/out/certs.json is not valid JSON: {e}")
with open("/task/expected/certs.json", encoding="utf-8") as f:
    want = json.load(f)

if not isinstance(got, list):
    sys.exit(f"expected a JSON array, got {type(got).__name__}")
if len(got) != len(want):
    sys.exit(f"expected {len(want)} certificates, got {len(got)}")
for i, (g, w) in enumerate(zip(got, want)):
    if not isinstance(g, dict):
        sys.exit(f"[{i}]: expected an object, got {type(g).__name__}")
    if sorted(g) != sorted(KEYS):
        sys.exit(f"[{i}]: keys are {sorted(g)}, want {sorted(KEYS)}")
    for k in ["subject_cn", "issuer_cn", "serial", "sha256_fingerprint"]:
        if g[k] != w[k]:
            sys.exit(f"[{i}].{k}: expected {w[k]!r}, got {g[k]!r}")
    if instant(g["not_after"]) is None:
        sys.exit(f"[{i}].not_after: {g['not_after']!r} is not an RFC 3339 time in UTC")
    if instant(g["not_after"]) != instant(w["not_after"]):
        sys.exit(f"[{i}].not_after: expected {w['not_after']!r}, got {g['not_after']!r}")
    names = g["dns_names"]
    if not isinstance(names, list) or not all(isinstance(n, str) for n in names):
        sys.exit(f"[{i}].dns_names: expected an array of strings, got {names!r}")
    if sorted(names) != sorted(w["dns_names"]):
        sys.exit(f"[{i}].dns_names: expected {w['dns_names']!r}, got {names!r}")
PY
