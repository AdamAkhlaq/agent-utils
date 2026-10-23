#!/bin/sh
set -eu
agent-utils unicode -nfc /task/fixtures/catalog.txt > /out/catalog.txt
