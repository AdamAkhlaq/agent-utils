#!/bin/sh
set -eu
agent-utils json2toml /task/fixtures/service.json > /out/service.toml
