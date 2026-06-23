#!/bin/sh
set -eu

log() { printf '[render-maru-config] %s\n' "$*"; }

src="${MARU_TEMPLATE:-/templates/maru-config.toml.template}"
dst="${MARU_CONFIG:-/rendered/maru/config.toml}"
tmp="${dst}.tmp"

mkdir -p "$(dirname "$dst")"
sed \
  -e "s|__L1_RPC_URL__|$L1_RPC_URL|g" \
  -e "s|__LINEA_ROLLUP_ADDRESS__|$LINEA_ROLLUP_ADDR|g" \
  "$src" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[render-maru-config] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$dst"
log "rendered $src -> $dst"
