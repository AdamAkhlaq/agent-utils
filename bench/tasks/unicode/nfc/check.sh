#!/bin/sh
# Pass when /out/catalog.txt is the NFC form of the fixture, compared after
# trimming surrounding whitespace.
exec python3 -I - <<'PY'
import sys, unicodedata

with open("/task/fixtures/catalog.txt", encoding="utf-8") as f:
    want = unicodedata.normalize("NFC", f.read()).strip()
try:
    with open("/out/catalog.txt", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/catalog.txt was not written")
try:
    got = raw.decode("utf-8").strip()
except UnicodeDecodeError as e:
    sys.exit(f"/out/catalog.txt is not UTF-8: {e}")
if got == want:
    sys.exit(0)
got_lines, want_lines = got.split("\n"), want.split("\n")
if len(got_lines) != len(want_lines):
    sys.exit(f"expected {len(want_lines)} lines, got {len(got_lines)}")
for n, (g, w) in enumerate(zip(got_lines, want_lines), 1):
    if g != w:
        if unicodedata.normalize("NFC", g) == w:
            sys.exit(f"line {n} has the right text but is not in NFC: {g!r}")
        sys.exit(f"line {n}: expected {w!r}, got {g!r}")
sys.exit("files differ")
PY
