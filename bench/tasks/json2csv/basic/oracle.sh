#!/bin/sh
set -eu
agent-utils json2csv /task/fixtures/regions.json > /out/regions.csv
