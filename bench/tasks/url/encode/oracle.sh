#!/bin/sh
set -eu
# $(...) drops the trailing newline, which is not part of the phrase.
printf '%s' "$(cat /task/fixtures/query.txt)" | agent-utils url > /out/value.txt
