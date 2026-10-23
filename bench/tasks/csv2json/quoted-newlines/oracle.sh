#!/bin/sh
set -eu
agent-utils csv2json /task/fixtures/tickets.csv > /out/tickets.json
