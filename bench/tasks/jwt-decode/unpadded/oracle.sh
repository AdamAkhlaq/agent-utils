#!/bin/sh
set -eu
agent-utils jwt-decode /task/fixtures/token.jwt | jq .payload > /out/claims.json
