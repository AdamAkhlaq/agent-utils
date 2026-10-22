#!/bin/sh
set -eu
agent-utils csv2json /task/fixtures/orders.csv > /out/orders.json
