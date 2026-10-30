#!/bin/sh
set -eu
while read -r epoch message; do
  printf '%s %s\n' "$(agent-utils time -z Australia/Adelaide "$epoch")" "$message"
done < /task/fixtures/deploys.log > /out/deploys.log
