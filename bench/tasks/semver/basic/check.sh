#!/bin/sh
# Pass when /out/sorted.txt lists the fixture's tags in ascending Semantic
# Versioning 2.0.0 precedence: numeric fields compared as numbers (v1.11.0 is
# above v1.3.0), a pre-release below its release, numeric pre-release
# identifiers compared as numbers and below alphanumeric ones, a longer
# identifier list above its prefix, and build metadata ignored. GNU sort -V
# puts v2.0.0 below v2.0.0-rc.1 and alpha.beta below alpha.1. Surrounding
# whitespace on each line and CRLF line endings are ignored.
exec python3 -I - <<'EOF'
import functools, sys

def parts(tag):
    core, _, pre = tag[1:].split("+")[0].partition("-")
    return [int(x) for x in core.split(".")], pre.split(".") if pre else None

def compare(a, b):
    (ca, pa), (cb, pb) = parts(a), parts(b)
    if ca != cb:
        return -1 if ca < cb else 1
    if pa is None or pb is None:
        return (pa is None) - (pb is None)
    for x, y in zip(pa, pb):
        if x == y:
            continue
        if x.isdigit() and y.isdigit():
            return -1 if int(x) < int(y) else 1
        if x.isdigit() != y.isdigit():
            return -1 if x.isdigit() else 1
        return -1 if x < y else 1
    return (len(pa) > len(pb)) - (len(pa) < len(pb))

with open("/task/fixtures/tags.txt") as f:
    want = sorted(f.read().split(), key=functools.cmp_to_key(compare))
try:
    with open("/out/sorted.txt", encoding="utf-8") as f:
        got = [line.strip() for line in f.read().strip().splitlines()]
except FileNotFoundError:
    sys.exit("/out/sorted.txt was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/sorted.txt is not UTF-8: {e}")
if got == []:
    sys.exit("/out/sorted.txt is empty")
if sorted(got) != sorted(want):
    missing = sorted(set(want) - set(got))
    extra = sorted(set(got) - set(want))
    sys.exit(f"not the fixture's tags: missing {missing[:3]}, unexpected {extra[:3]}" if missing or extra else "a tag is repeated")
for n, (g, w) in enumerate(zip(got, want), 1):
    if g != w:
        sys.exit(f"line {n}: expected {w}, got {g}")
EOF
