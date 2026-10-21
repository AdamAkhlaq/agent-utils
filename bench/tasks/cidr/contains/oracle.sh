#!/bin/sh
set -eu
agent-utils cidr contains 10.0.5.1/22 10.0.7.255 > /out/result.txt
