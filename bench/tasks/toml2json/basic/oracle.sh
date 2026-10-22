#!/bin/sh
set -eu
agent-utils toml2json /task/fixtures/pipeline.toml > /out/pipeline.json
