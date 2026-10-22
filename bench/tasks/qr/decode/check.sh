#!/bin/sh
# Pass when /out/text.txt holds the encoded Wi-Fi string, ignoring one
# trailing line break. The base image has no barcode decoder, so a shell-only
# agent has to write one or read the code some other way.
exec python3 -I - <<'PY'
import sys
try:
    with open("/out/text.txt", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/text.txt was not written")
try:
    got = raw.decode("utf-8")
except UnicodeDecodeError as e:
    sys.exit(f"/out/text.txt is not UTF-8: {e}")
got = got[:-1] if got.endswith("\n") else got
want = "WIFI:T:WPA;S:Lab 4F Gäste;P:k7#Qm!2v-Rx9.wN3;H:false;;"
if got != want:
    sys.exit(f"wrong text\nwant: {want!r}\ngot:  {got!r}")
PY
