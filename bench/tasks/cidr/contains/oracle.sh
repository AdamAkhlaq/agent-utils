#!/bin/sh
set -eu
# contains exits 3 for a true "false" answer; only exit 1 (bad input) is a failure here.
agent-utils cidr contains 172.29.141.77/21 172.29.145.6 > /out/result.txt || [ $? -eq 3 ]
