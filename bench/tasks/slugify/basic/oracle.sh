#!/bin/sh
set -eu
while IFS= read -r title; do
  printf '%s' "$title" | agent-utils slugify
done < /task/fixtures/titles.txt > /out/slugs.txt
