#!/bin/sh
set -eu
agent-utils time -z America/New_York add 2024-03-09T12:00:00 P1D > /out/next.txt
