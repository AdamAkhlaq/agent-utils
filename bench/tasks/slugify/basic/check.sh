#!/bin/sh
# Pass when /out/slugs.txt holds the slug of each title, in order: letters
# (Unicode category L) and decimal digits (Nd) lowercased and kept, every other
# run of characters one hyphen, none at either end. Surrounding whitespace and
# trailing whitespace on a line (a CR from CRLF line endings) are ignored.
exec python3 -I - <<'PY'
import sys, unicodedata

def slug(title):
    out, pending = [], False
    for c in title:
        cat = unicodedata.category(c)
        if cat[0] != "L" and cat != "Nd":
            pending = True
            continue
        if pending and out:
            out.append("-")
        pending = False
        out.append(c.lower())
    return "".join(out)

with open("/task/fixtures/titles.txt", encoding="utf-8") as f:
    titles = f.read().splitlines()
try:
    with open("/out/slugs.txt", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/slugs.txt was not written")
try:
    got = [line.rstrip() for line in raw.decode("utf-8").strip().split("\n")]
except UnicodeDecodeError as e:
    sys.exit(f"/out/slugs.txt is not UTF-8: {e}")
if len(got) != len(titles):
    sys.exit(f"expected {len(titles)} lines, one per title, got {len(got)}")
for n, (g, t) in enumerate(zip(got, titles), 1):
    w = slug(t)
    if g != w:
        if unicodedata.normalize("NFC", g) == w:
            sys.exit(f"line {n} has the right slug but not in composed form (NFC): {g!r}")
        sys.exit(f"line {n}: the slug of {t!r} is {w!r}, got {g!r}")
PY
