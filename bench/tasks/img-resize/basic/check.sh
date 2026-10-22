#!/bin/sh
# Pass when /out/panorama-600.jpg is a 600x400 JPEG (the source is 1800x1200)
# within a mean of 4 per channel of Pillow's Lanczos downscale. Any ordinary
# resampler passes (agent-utils and ImageMagick are 1.5 away, nearest
# neighbor 1.9); a crop instead of a scale is 50 away.
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
got = open_out("/out/panorama-600.jpg", "JPEG")
src = Image.open("/task/fixtures/panorama.jpg")
want = src.resize((600, 400), Image.LANCZOS)
same_size(got, want)
d = mean_diff(got, want)
if d > 4:
    sys.exit(f"the image differs from a scaled copy of the photo by a mean of {d:.2f} per channel (limit 4)")
PY
