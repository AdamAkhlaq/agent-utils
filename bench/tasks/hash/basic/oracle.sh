#!/bin/sh
set -eu
cd /task/fixtures/release
for f in install.sh manifest.json notes.md; do
  printf '%s  %s\n' "$(agent-utils hash "$f")" "$f"
done > /out/SHA256SUMS
