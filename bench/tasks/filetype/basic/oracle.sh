#!/bin/sh
set -eu
cd /task/fixtures/uploads
for f in *; do
  agent-utils filetype -json "$f" | jq --arg name "$f" '{($name): del(.bytes)}'
done | jq -s 'add' > /out/report.json
