#!/bin/sh
# Pass when /out/secrets.env has one NAME_SECRET=value line per service, in
# order, each value 28 letters or digits and all distinct. A random value
# misses upper or lower case with odds of 2.5 in 10 million and has fewer than
# 12 distinct characters with odds below 1 in 100 million, so either one
# marks a value that was not drawn from all 62 characters.
exec python3 -I - <<'PY'
import re, sys

with open("/task/fixtures/services.txt") as f:
    services = f.read().split()
try:
    with open("/out/secrets.env", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/secrets.env was not written")
try:
    text = raw.decode("utf-8")
except UnicodeDecodeError as e:
    sys.exit(f"/out/secrets.env is not UTF-8: {e}")
lines = [line.rstrip("\r") for line in text.strip().split("\n")]
if len(lines) != len(services):
    sys.exit(f"expected {len(services)} lines, got {len(lines)}: {lines!r}")
values = []
for i, (line, name) in enumerate(zip(lines, services), 1):
    prefix = f"{name}_SECRET="
    if not line.startswith(prefix):
        sys.exit(f"line {i}: expected it to start with {prefix!r}, got {line!r}")
    v = line[len(prefix):]
    if not re.fullmatch(r"[A-Za-z0-9]{28}", v):
        sys.exit(f"line {i}: expected 28 ASCII letters or digits, got {v!r} ({len(v)} characters)")
    if not re.search(r"[A-Z]", v) or not re.search(r"[a-z]", v):
        sys.exit(f"line {i}: {v!r} lacks upper or lower case letters, so it was not drawn from all 62 characters")
    if len(set(v)) < 12:
        sys.exit(f"line {i}: {v!r} has only {len(set(v))} distinct characters, which looks like a pattern, not random")
    values.append(v)
if len(set(values)) != len(values):
    sys.exit(f"the values are not all distinct: {values!r}")
PY
