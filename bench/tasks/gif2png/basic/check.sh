#!/bin/sh
# Pass when /out/frame1.png is the only file in /out and its pixels equal the
# GIF's first frame exactly, transparency included. Converting the whole GIF
# with ImageMagick writes one numbered file per frame instead.
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
only_file("frame1.png")
got = open_out("/out/frame1.png", "PNG")
src = Image.open("/task/fixtures/spinner.gif")
src.seek(0)
exact(got, src)
PY
