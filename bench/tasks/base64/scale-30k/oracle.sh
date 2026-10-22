#!/bin/sh
set -eu
agent-utils base64 /task/fixtures/events.log > /out/events.b64
