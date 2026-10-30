#!/bin/sh
# Pass when /out/invoice.json holds the invoice's line amounts, subtotal, tax,
# and total, each rounded to the cent with halves away from zero and written as
# a string with exactly two decimal places. The fixture has eight lines that
# land exactly on a half cent, one of them negative, so binary floats,
# half-even rounding, truncation, and floor(x + 0.5) each get some wrong.
exec python3 -I - <<'EOF'
import csv, json, re, sys
from fractions import Fraction
from math import floor

def cents(x):
    sign = -1 if x < 0 else 1
    return sign * Fraction(floor(abs(x) * 100 + Fraction(1, 2)), 100)

with open("/task/fixtures/invoice.csv", newline="") as f:
    rows = list(csv.DictReader(f))
lines = [(r["sku"], cents(Fraction(r["quantity"]) * Fraction(r["unit_price"]) * (100 - Fraction(r["discount_pct"])) / 100)) for r in rows]
subtotal = sum(a for _, a in lines)
tax = cents(subtotal * Fraction("8.875") / 100)
total = subtotal + tax

try:
    with open("/out/invoice.json", encoding="utf-8") as f:
        got = json.load(f)
except FileNotFoundError:
    sys.exit("/out/invoice.json was not written")
except (UnicodeDecodeError, json.JSONDecodeError) as e:
    sys.exit(f"/out/invoice.json is not valid JSON: {e}")

def amount(where, value, want):
    if not isinstance(value, str) or not re.fullmatch(r"-?\d+\.\d\d", value):
        sys.exit(f"{where}: {value!r} is not a string with exactly two decimal places")
    if Fraction(value) != want:
        sys.exit(f"{where}: expected {float(want):.2f}, got {value}")

if not isinstance(got, dict):
    sys.exit("the top level is not a JSON object")
for key in ("lines", "subtotal", "tax", "total"):
    if key not in got:
        sys.exit(f"missing key {key!r}")
if not isinstance(got["lines"], list) or len(got["lines"]) != len(lines):
    sys.exit(f"lines: expected an array of {len(lines)} objects")
for n, (g, (sku, want)) in enumerate(zip(got["lines"], lines), 1):
    if not isinstance(g, dict) or g.get("sku") != sku:
        sys.exit(f"lines[{n - 1}]: expected sku {sku!r}, got {g!r}")
    amount(f"lines[{n - 1}] ({sku})", g.get("amount"), want)
amount("subtotal", got["subtotal"], subtotal)
amount("tax", got["tax"], tax)
amount("total", got["total"], total)
EOF
