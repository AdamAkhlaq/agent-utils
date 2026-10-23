#!/bin/sh
set -eu
agent-utils json2yaml /task/fixtures/deploy.json > /out/deploy.yaml
