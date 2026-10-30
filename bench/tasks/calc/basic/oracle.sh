#!/bin/sh
set -eu
while IFS= read -r expr; do
  agent-utils calc -- "$expr"
done < /task/fixtures/worksheet.txt > /out/results.txt
