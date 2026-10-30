#!/bin/sh
set -eu
agent-utils semver /task/fixtures/tags.txt > /out/sorted.txt
