#!/bin/sh
set -eu
agent-utils csv2json /task/fixtures/contacts.csv > /out/contacts.json
