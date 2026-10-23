#!/bin/sh
set -eu
agent-utils yaml2json /task/fixtures/pipeline.yaml > /out/pipeline.json
