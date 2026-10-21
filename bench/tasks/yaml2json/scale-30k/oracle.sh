#!/bin/sh
set -eu
agent-utils yaml2json /task/fixtures/config.yaml > /out/config.json
