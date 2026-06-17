#!/bin/sh
set -eu

if [ -f /initialization/genesis-besu.json ] \
  && [ -f /initialization/genesis-maru.json ] \
  && [ -f /initialization/fork-timestamp.txt ]; then
  echo "[l2-genesis-init] L2 genesis already initialized - skipping."
  exit 0
fi

echo "[l2-genesis-init] Rendering fresh L2 genesis."
sh /scripts/services/render-l2-genesis.sh
