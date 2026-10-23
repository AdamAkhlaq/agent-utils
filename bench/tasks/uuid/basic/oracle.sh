#!/bin/sh
set -eu
agent-utils uuid -n 12 | jq -R . | jq -s . > /tmp/ids.json
jq --slurpfile ids /tmp/ids.json '[to_entries[] | .value + {id: $ids[0][.key]}]' /task/fixtures/devices.json > /out/devices.json
