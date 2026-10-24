#!/bin/sh
# Pass when /out/env-names.txt holds each setting's words in uppercase joined
# by underscores, in order, split by the rule the instruction states:
# non-alphanumerics separate words, and a word starts at an uppercase letter
# after a lowercase letter or digit, or at the last uppercase letter of a run
# that a lowercase letter follows. Surrounding whitespace and trailing
# whitespace on a line are ignored.
exec python3 -I - <<'PY'
import re, sys

def words(name):
    out = []
    for run in re.findall(r"[A-Za-z0-9]+", name):
        start = 0
        for i in range(1, len(run)):
            if run[i].isupper() and (run[i - 1].islower() or run[i - 1].isdigit()):
                cut = True
            else:
                cut = run[i].isupper() and run[i - 1].isupper() and i + 1 < len(run) and run[i + 1].islower()
            if cut:
                out.append(run[start:i])
                start = i
        out.append(run[start:])
    return out

with open("/task/fixtures/settings.txt", encoding="utf-8") as f:
    names = f.read().splitlines()
try:
    with open("/out/env-names.txt", encoding="utf-8") as f:
        got = [line.rstrip() for line in f.read().strip().split("\n")]
except FileNotFoundError:
    sys.exit("/out/env-names.txt was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/env-names.txt is not UTF-8: {e}")
if len(got) != len(names):
    sys.exit(f"expected {len(names)} lines, one per setting, got {len(got)}")
for n, (g, name) in enumerate(zip(got, names), 1):
    w = "_".join(words(name)).upper()
    if g != w:
        sys.exit(f"line {n}: {name!r} should become {w!r}, got {g!r}")
PY
