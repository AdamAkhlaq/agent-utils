#!/bin/sh
# Pass when /out/landing.html is the mockup with each data-filler-words
# paragraph holding that many words of the sample passage, cycled from its
# first word. Words compare without case and trailing punctuation, and the
# rest of the page compares with whitespace runs collapsed.
exec python3 -I - <<'PY'
import re, sys

SLOT = re.compile(r'(<p\b[^>]*\bdata-filler-words="(\d+)"[^>]*>)(.*?)(</p>)', re.S)
SAMPLE = re.compile(r'<p class="sample">(.*?)</p>', re.S)

def skeleton(page):
    return " ".join(SLOT.sub(r"\1\4", page).split())

def norm(word):
    return word.lower().rstrip(".,;:")

with open("/task/fixtures/landing.html") as f:
    fixture = f.read()
passage = SAMPLE.search(fixture).group(1).split()
try:
    with open("/out/landing.html") as f:
        page = f.read()
except FileNotFoundError:
    sys.exit("/out/landing.html was not written")
except UnicodeDecodeError as e:
    sys.exit(f"/out/landing.html is not UTF-8: {e}")
if skeleton(page) != skeleton(fixture):
    sys.exit("the page changed outside the four filler paragraphs")
slots = SLOT.findall(page)
if len(slots) != 4:
    sys.exit(f"expected 4 filler paragraphs, found {len(slots)}")
for _, count, body, _ in slots:
    n = int(count)
    got = body.split()
    if len(got) != n:
        sys.exit(f"data-filler-words=\"{n}\": expected {n} words, got {len(got)}")
    want = [passage[i % len(passage)] for i in range(n)]
    for i, (g, w) in enumerate(zip(got, want), 1):
        if norm(g) != norm(w):
            sys.exit(f"data-filler-words=\"{n}\": word {i} should be {w!r}, got {g!r}")
PY
