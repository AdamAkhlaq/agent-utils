#!/bin/sh
set -eu
set --
for v in $(grep -oE '#[0-9A-Fa-f]{6}|#[0-9A-Fa-f]{3}' /task/fixtures/theme.css | sort -u); do
  set -- "$@" -e "s/$v\([^0-9A-Fa-f]\)/$(agent-utils color -to hsl "$v")\1/g"
done
sed "$@" /task/fixtures/theme.css > /out/theme.css
