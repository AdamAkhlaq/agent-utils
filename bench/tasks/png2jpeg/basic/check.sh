#!/bin/sh
# Pass when /out/photo.jpg decodes as a JPEG with the source's dimensions.
# JPEG is lossy, so pixels are not compared.
exec python3 -I - <<'PY'
import sys
from PIL import Image, UnidentifiedImageError
try:
    with Image.open("/out/photo.jpg") as im:
        fmt, size = im.format, im.size
except FileNotFoundError:
    sys.exit("/out/photo.jpg was not written")
except UnidentifiedImageError:
    sys.exit("/out/photo.jpg is not an image")
if fmt != "JPEG":
    sys.exit(f"not a JPEG: {fmt}")
with Image.open("/task/fixtures/photo.png") as src:
    want = src.size
if size != want:
    sys.exit(f"wrong dimensions: want {want[0]}x{want[1]}, got {size[0]}x{size[1]}")
PY
