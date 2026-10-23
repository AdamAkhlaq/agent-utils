#!/bin/sh
set -eu
agent-utils password -l 28 -n 5 -no-symbols | paste -d= /task/fixtures/services.txt - | sed 's/=/_SECRET=/' > /out/secrets.env
