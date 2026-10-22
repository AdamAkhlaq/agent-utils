#!/bin/sh
set -eu
# $(...) drops the trailing newline, which is not part of the name.
printf '%s' "$(cat /task/fixtures/name.txt)" | agent-utils quote -as shell > /out/word.txt
