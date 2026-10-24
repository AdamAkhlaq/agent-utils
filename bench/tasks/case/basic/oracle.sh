#!/bin/sh
set -eu
while IFS= read -r name; do
  printf '%s' "$name" | agent-utils case -to screaming
done < /task/fixtures/settings.txt > /out/env-names.txt
