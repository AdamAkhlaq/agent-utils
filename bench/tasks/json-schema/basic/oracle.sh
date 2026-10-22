#!/bin/sh
set -eu
# json-schema exits 3 for a document that does not match; any other
# non-zero exit is a real failure and stops the oracle.
n=$(jq length /task/fixtures/orders.json)
i=0
bad=""
while [ "$i" -lt "$n" ]; do
  rc=0
  jq ".[$i]" /task/fixtures/orders.json | agent-utils json-schema /task/fixtures/order.schema.json > /dev/null 2>&1 || rc=$?
  case $rc in
    0) ;;
    3) bad="$bad${bad:+,}$i" ;;
    *) echo "json-schema failed on order $i with exit $rc" >&2; exit 1 ;;
  esac
  i=$((i + 1))
done
echo "[$bad]" > /out/invalid.json
