#!/bin/sh
set -eu
tail -n +2 /task/fixtures/contracts.csv | {
  echo contract_id,end_date
  while IFS=, read -r id _ start months; do
    echo "$id,$(agent-utils time -f date add "$start" "P${months}M")"
  done
} > /out/end-dates.csv
