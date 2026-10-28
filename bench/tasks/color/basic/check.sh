#!/bin/sh
# Pass when /out/theme.css is the fixture with every #rrggbb and #rgb value
# replaced by hsl(H, S%, L%): the CSS Color formulas in exact rational
# arithmetic, each component rounded to the nearest integer with halves up,
# and a hue of 360 written as 0. Whitespace inside hsl(), trailing whitespace
# on a line, and surrounding whitespace are ignored.
exec python3 -I - <<'PY'
import re, sys
from fractions import Fraction
from math import floor

def nearest(x):
    return floor(x + Fraction(1, 2))

def hsl(match):
    v = match.group(1)
    if len(v) == 3:
        v = "".join(c * 2 for c in v)
    r, g, b = (int(v[i:i + 2], 16) for i in (0, 2, 4))
    hi, lo = max(r, g, b), min(r, g, b)
    d = hi - lo
    light = Fraction((hi + lo) * 100, 510)
    if d == 0:
        hue = sat = Fraction(0)
    else:
        sat = Fraction(d * 100, 255 - abs(hi + lo - 255))
        if hi == r:
            hue = 60 * (Fraction(g - b, d) % 6)
        elif hi == g:
            hue = 60 * (Fraction(b - r, d) + 2)
        else:
            hue = 60 * (Fraction(r - g, d) + 4)
    return f"hsl({nearest(hue) % 360}, {nearest(sat)}%, {nearest(light)}%)"

spaced = re.compile(r"hsl\(\s*(\d+)\s*,\s*(\d+)%\s*,\s*(\d+)%\s*\)")

def lines(text):
    text = spaced.sub(r"hsl(\1, \2%, \3%)", text)
    return [line.rstrip() for line in text.strip().split("\n")]

with open("/task/fixtures/theme.css", encoding="utf-8") as f:
    want = lines(re.sub(r"#([0-9A-Fa-f]{6}|[0-9A-Fa-f]{3})(?![0-9A-Fa-f])", hsl, f.read()))
try:
    with open("/out/theme.css", encoding="utf-8") as f:
        got = lines(f.read())
except FileNotFoundError:
    sys.exit("/out/theme.css was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/theme.css is not UTF-8: {e}")
if got == [""]:
    sys.exit("/out/theme.css is empty")
for n, (g, w) in enumerate(zip(got, want), 1):
    if g != w:
        sys.exit(f"line {n}: expected {w!r}, got {g!r}")
if len(got) != len(want):
    sys.exit(f"expected {len(want)} lines, got {len(got)}")
PY
