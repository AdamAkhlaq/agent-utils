#!/bin/sh
set -eu
agent-utils json-fmt -c /task/fixtures/events.json > /out/events.min.json
