#!/bin/sh
# Pass when /out/results.txt holds, line by line, the exact value of each
# expression in the worksheet, evaluated with Fraction arithmetic (the
# worksheet's grammar matches Python's once ^ becomes **, including -2**2 and
# right-grouping powers). Any plain decimal spelling of the exact value is
# accepted: trailing zeros, a leading + or a bare leading point (bc's .5).
exec python3 -I - <<'EOF'
import re, sys
from decimal import Decimal, localcontext
from fractions import Fraction

def exact(expr):
    py = re.sub(r"\d+(?:\.\d+)?", lambda m: f"Fraction('{m.group(0)}')", expr.replace("^", "**"))
    return eval(py, {"__builtins__": {}, "Fraction": Fraction})

def decimal(x):
    with localcontext() as ctx:
        ctx.prec = 1000
        return format(Decimal(x.numerator) / Decimal(x.denominator), "f")

with open("/task/fixtures/worksheet.txt") as f:
    want = [exact(line) for line in f.read().splitlines()]
try:
    with open("/out/results.txt", encoding="utf-8") as f:
        got = f.read().strip().splitlines()
except FileNotFoundError:
    sys.exit("/out/results.txt was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/results.txt is not UTF-8: {e}")
if got == []:
    sys.exit("/out/results.txt is empty")
if len(got) != len(want):
    sys.exit(f"expected {len(want)} lines, got {len(got)}")
plain = re.compile(r"[+-]?(?:\d+(?:\.\d*)?|\.\d+)")
for n, (g, w) in enumerate(zip(got, want), 1):
    g = g.strip()
    if not plain.fullmatch(g):
        sys.exit(f"line {n}: {g!r} is not a plain decimal number")
    if Fraction(g) != w:
        sys.exit(f"line {n}: expected {decimal(w)}, got {g}")
EOF
