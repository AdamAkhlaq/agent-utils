#!/bin/sh
set -eu
while IFS= read -r link; do
  agent-utils urlparse "$link" | jq '{scheme, host, port, path, query, fragment}'
done < /task/fixtures/links.txt | jq -s . > /out/links.json
