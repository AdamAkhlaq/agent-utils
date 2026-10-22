#!/bin/sh
set -eu
agent-utils base64 -d /task/fixtures/secret.b64 > /out/config.txt
