#!/bin/sh
set -eu
agent-utils yaml2json /task/fixtures/worker.yaml > /out/worker.json
