#!/bin/sh
set -eu
agent-utils hex -d /task/fixtures/payload.txt > /out/message.txt
