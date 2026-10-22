#!/bin/sh
# Pass when /out/harbor.png is a PNG of the source's size whose pixels match
# Pillow's decoding of the JPEG to within a mean of 1.5 per channel. JPEG
# decoders differ slightly (agent-utils is 0.44 from Pillow, ImageMagick 0),
# so an exact comparison would be unfair.
exec python3 -I - <<'PY'
import os, sys
from PIL import Image, ImageChops, ImageStat, UnidentifiedImageError

def open_out(path, fmt):
    try:
        im = Image.open(path)
        im.load()
    except FileNotFoundError:
        sys.exit(f"{path} was not written")
    except (UnidentifiedImageError, OSError) as e:
        sys.exit(f"{path} is not a readable image: {e}")
    if im.format != fmt:
        sys.exit(f"{path} is {im.format}, not {fmt}")
    return im

def same_size(got, want):
    if got.size != want.size:
        sys.exit(f"wrong dimensions: want {want.size[0]}x{want.size[1]}, got {got.size[0]}x{got.size[1]}")

def mean_diff(a, b):
    st = ImageStat.Stat(ImageChops.difference(a.convert("RGB"), b.convert("RGB")))
    return sum(st.mean) / 3

def rgba_pixels(im):
    # Fully transparent pixels compare equal whatever color they hide.
    im = im.convert("RGBA")
    return [(0, 0, 0, 0) if p[3] == 0 else p for p in im.getdata()]

def exact(got, want):
    same_size(got, want)
    g, w = rgba_pixels(got), rgba_pixels(want)
    for i, (a, b) in enumerate(zip(g, w)):
        if a != b:
            x, y = i % want.width, i // want.width
            n = sum(1 for p, q in zip(g, w) if p != q)
            sys.exit(f"{n} pixels differ; first at ({x}, {y}): want RGBA {b}, got {a}")

def only_file(name):
    extra = sorted(f for f in os.listdir("/out") if f != name)
    if extra:
        sys.exit(f"/out holds other files besides {name}: {extra}")
got = open_out("/out/harbor.png", "PNG")
want = Image.open("/task/fixtures/harbor.jpg")
same_size(got, want)
d = mean_diff(got, want)
if d > 1.5:
    sys.exit(f"pixels differ from the photo by a mean of {d:.2f} per channel (limit 1.5)")
PY
