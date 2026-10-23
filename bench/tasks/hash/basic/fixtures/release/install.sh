#!/bin/sh
# Installs ledger-sync into $PREFIX (default /opt/tarnwick).
set -eu
PREFIX=${PREFIX:-/opt/tarnwick}
install -d "$PREFIX/bin" "$PREFIX/share/ledger-sync"
install -m 0755 bin/ledger-sync "$PREFIX/bin/"
install -m 0644 share/ledger-sync/rates.csv "$PREFIX/share/ledger-sync/"
echo "ledger-sync 4.13.2 installed in $PREFIX ✓; run ledger-sync --check"
