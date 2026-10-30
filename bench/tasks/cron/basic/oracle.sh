#!/bin/sh
set -eu
grep -vE '^[[:space:]]*(#|$|[A-Za-z_][A-Za-z0-9_]*=)' /task/fixtures/crontab |
  while read -r minute hour dom month dow command; do
    agent-utils cron -json -n 3 -from 2026-11-27T16:50:00Z "$minute $hour $dom $month $dow" |
      jq --arg command "$command" '{($command): .next}'
  done |
  jq -s add > /out/next-runs.json
