#!/bin/sh
# Pass when /out/events.b64, with whitespace removed, decodes as standard
# base-64 to the fixture byte for byte. Line wrapping (as coreutils emits) is
# allowed, and so is missing padding, since the instruction asks for the
# content, not a layout.
exec python3 -I - <<'PY'
import base64, binascii, re, sys
try:
    with open("/out/events.b64", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/events.b64 was not written")
text = re.sub(rb"\s+", b"", raw)
if not text:
    sys.exit("/out/events.b64 is empty")
if re.search(rb"[^A-Za-z0-9+/=]", text):
    bad = re.search(rb"[^A-Za-z0-9+/=]", text).group().decode("latin-1")
    sys.exit(f"not standard base-64: contains {bad!r}")
text = text.rstrip(b"=")
text += b"=" * (-len(text) % 4)
try:
    got = base64.b64decode(text, validate=True)
except binascii.Error as e:
    sys.exit(f"/out/events.b64 does not decode: {e}")
with open("/task/fixtures/events.log", "rb") as f:
    want = f.read()
if got != want:
    n = next((i for i, (a, b) in enumerate(zip(got, want)) if a != b), min(len(got), len(want)))
    sys.exit(f"decoded content differs from the fixture at byte {n} (decoded {len(got)} bytes, fixture {len(want)} bytes)")
PY
