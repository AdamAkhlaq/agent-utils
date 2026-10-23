#!/bin/sh
# Pass when /out/SHA256SUMS holds one "<sha256>  <name>" line per release file,
# sorted by name, with digests of the files' exact bytes. Surrounding
# whitespace and the line ending style do not matter.
exec python3 -I - <<'PY'
import hashlib, os, sys

DIR = "/task/fixtures/release"
want = []
for name in sorted(os.listdir(DIR)):
    with open(os.path.join(DIR, name), "rb") as f:
        want.append(f"{hashlib.sha256(f.read()).hexdigest()}  {name}")
try:
    with open("/out/SHA256SUMS", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/SHA256SUMS was not written")
try:
    text = raw.decode("utf-8")
except UnicodeDecodeError as e:
    sys.exit(f"/out/SHA256SUMS is not UTF-8: {e}")
got = [line.rstrip("\r") for line in text.strip().split("\n")]
if len(got) != len(want):
    sys.exit(f"expected {len(want)} lines, got {len(got)}: {got!r}")
for i, (g, w) in enumerate(zip(got, want), 1):
    if g != w:
        sys.exit(f"line {i}: expected {w!r}, got {g!r}")
PY
