#!/bin/sh
set -eu

log() { printf '[render-l2-node-besu-config] %s\n' "$*"; }

src="${L2_NODE_BESU_TEMPLATE:-/templates/l2-node-besu.config.toml.template}"
dst="${L2_NODE_BESU_CONFIG:-/rendered/l2-node-besu/l2-node-besu.config.toml}"
tmp="${dst}.tmp"

mkdir -p "$(dirname "$dst")"
sed \
  -e "s|__L2_MESSAGE_SERVICE_ADDRESS__|$L2_MS_ADDR|g" \
  "$src" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[render-l2-node-besu-config] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$dst"
log "rendered $src -> $dst"
