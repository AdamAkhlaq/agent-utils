#!/bin/sh
set -eu
agent-utils unicode /task/fixtures/handles.txt | jq '
  reduce .[] as $c ({line: 1, found: []};
    if $c.codepoint == "U+000A" then .line += 1
    elif $c.category == "Cf" then .found += [{line, codepoint: $c.codepoint}]
    else . end)
  | .found' > /out/findings.json
