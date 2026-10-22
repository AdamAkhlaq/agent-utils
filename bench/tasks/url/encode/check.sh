#!/bin/sh
# Pass when /out/value.txt is a percent-encoded form of the phrase that a
# query-string decoder (application/x-www-form-urlencoded, so + and %20 are
# both a space) turns back into the exact phrase, using only characters that
# are legal inside a query value. Encoders differ on which reserved
# characters they escape (/, ?, !, *, ', parentheses), so any choice that
# decodes correctly passes; a raw space, &, =, #, or non-ASCII byte fails,
# because it would break the parameter or the request.
exec python3 -I - <<'PY'
import re, sys
from urllib.parse import unquote_plus
try:
    with open("/out/value.txt", "rb") as f:
        raw = f.read()
except FileNotFoundError:
    sys.exit("/out/value.txt was not written")
got = raw.decode("latin-1").strip()
with open("/task/fixtures/query.txt", encoding="utf-8") as f:
    want = f.read().rstrip("\n")
if not got:
    sys.exit("/out/value.txt is empty")
bad = re.search(r"[^A-Za-z0-9\-._~!$'()*,;:@/?%+]", got)
if bad:
    sys.exit(f"the value contains {bad.group()!r}, which is not allowed raw in a query value")
if re.search(r"%(?![0-9A-Fa-f]{2})", got):
    sys.exit("the value has a % that is not followed by two hex digits")
try:
    decoded = unquote_plus(got, encoding="utf-8", errors="strict")
except UnicodeDecodeError as e:
    sys.exit(f"the percent-escapes do not decode as UTF-8: {e}")
if decoded != want:
    sys.exit(f"the value does not decode to the phrase\nwant: {want!r}\ngot:  {decoded!r}\nvalue: {got!r}")
PY
