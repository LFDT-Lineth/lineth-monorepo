#!/bin/sh
set -eu

log() { printf '[render-sequencer-config] %s\n' "$*"; }

src="${SEQUENCER_TEMPLATE:-/templates/sequencer.config.toml.template}"
dst="${SEQUENCER_CONFIG:-/rendered/sequencer/sequencer.config.toml}"
tmp="${dst}.tmp"

mkdir -p "$(dirname "$dst")"
sed \
  -e "s|__L2_MESSAGE_SERVICE_ADDRESS__|$L2_MS_ADDR|g" \
  "$src" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[render-sequencer-config] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$dst"
log "rendered $src -> $dst"
