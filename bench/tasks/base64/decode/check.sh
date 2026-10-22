#!/bin/sh
# Pass when /out/config.txt holds the expected text, ignoring surrounding whitespace.
exec python3 -I - <<'PY'
import sys
try:
    with open("/out/config.txt", "rb") as f:
        got = f.read()
except FileNotFoundError:
    sys.exit("/out/config.txt was not written")
with open("/task/expected/config.txt", "rb") as f:
    want = f.read()
try:
    got_text = got.decode("utf-8")
except UnicodeDecodeError as e:
    sys.exit(f"/out/config.txt is not valid UTF-8: {e}")
if got_text.strip() != want.decode("utf-8").strip():
    sys.exit(f"text differs\nwant: {want.decode('utf-8').strip()!r}\ngot:  {got_text.strip()!r}")
PY
