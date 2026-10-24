#!/bin/sh
# Pass when /out/tests.log is the fixture with its escape sequences removed,
# byte for byte, ignoring surrounding whitespace. The expected text is derived
# from the fixture with the ECMA-48 grammar: OSC, DCS, SOS, PM, and APC strings
# ended by BEL or ST, CSI sequences, and other ESC sequences.
exec python3 -I - tests.log <<'PY'
import re, sys

name = sys.argv[1]
SEQ = re.compile(rb"\x1b[\]PX^_].*?(?:\x07|\x1b\\)|\x1b\[[\x30-\x3f]*[\x20-\x2f]*[\x40-\x7e]|\x1b[\x20-\x2f]*[\x30-\x7e]", re.S)
with open(f"/task/fixtures/{name}", "rb") as f:
    want = SEQ.sub(b"", f.read()).strip()
try:
    with open(f"/out/{name}", "rb") as f:
        got = f.read().strip()
except FileNotFoundError:
    sys.exit(f"/out/{name} was not written")
if got == want:
    sys.exit(0)
if b"\x1b" in got:
    i = got.index(b"\x1b")
    sys.exit(f"an escape sequence is left at byte {i}: {got[i:i + 16]!r}")
if b"\r" in want and (got.replace(b"\r", b"") == want.replace(b"\r", b"") or got == want.replace(b"\r", b"\n")):
    sys.exit("the carriage returns were removed or turned into line breaks; they are not escape sequences and must stay")
i = next((i for i, (a, b) in enumerate(zip(got, want)) if a != b), min(len(got), len(want)))
line = want[:i].count(b"\n") + 1
sys.exit(f"differs from the expected text at byte {i} (line {line}): expected {want[i:i + 40]!r}, got {got[i:i + 40]!r} ({len(want)} bytes expected, {len(got)} written)")
PY
