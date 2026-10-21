#!/bin/sh
# One trial: a fresh, empty Claude Code config directory, then Claude Code in
# print mode with the task's instruction as the prompt. The runner passes the
# arm's flags as arguments and the credential in the environment.
set -eu
rm -rf "$CLAUDE_CONFIG_DIR"
mkdir -p "$CLAUDE_CONFIG_DIR"
exec claude -p "$(cat /task/instruction.md)" "$@"
