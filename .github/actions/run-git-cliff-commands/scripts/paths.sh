#!/usr/bin/env bash
# Normalize INCLUDE_PATH (newline-separated paths) into one trimmed, non-empty
# path per line so downstream steps can reconstruct a bash array safely.
#
# Inputs (env):  INCLUDE_PATH
# Output:        include_paths  (multi-line)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

: "${INCLUDE_PATH:?INCLUDE_PATH is required}"

printf '%s\n' "$INCLUDE_PATH" \
  | awk '{ sub(/^[[:space:]]+/,""); sub(/[[:space:]]+$/,""); if (length) print }' \
  | emit_block include_paths
