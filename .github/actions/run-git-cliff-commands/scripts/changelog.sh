#!/usr/bin/env bash
# Generate the unreleased (or next-tag) changelog section for a component.
#
# Inputs (env):  COMPONENT, CLIFF_CONFIG, INCLUDE_PATHS (multi-line),
#                GENERATE_WITH_NEXT_TAG (true|false, default false),
#                NEXT_VERSION (required when GENERATE_WITH_NEXT_TAG=true)
# Output:        content  (the changelog markdown, also printed to stdout)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

: "${COMPONENT:?COMPONENT is required}"
: "${CLIFF_CONFIG:?CLIFF_CONFIG is required}"
GENERATE_WITH_NEXT_TAG="${GENERATE_WITH_NEXT_TAG:-false}"
NEXT_VERSION="${NEXT_VERSION:-}"

# Read INCLUDE_PATHS (one path per line) into an array; portable to bash 3.2.
paths=()
while IFS= read -r p; do
  [ -n "$p" ] && paths+=("$p")
done <<< "${INCLUDE_PATHS:-}"
tag_args=(--unreleased)
if [ "${GENERATE_WITH_NEXT_TAG}" = "true" ]; then
  if [ -z "${NEXT_VERSION}" ]; then
    log "GENERATE_WITH_NEXT_TAG=true but NEXT_VERSION is empty"
    exit 1
  fi
  tag_args+=(--tag "${NEXT_VERSION}")
fi
cliff_args=(--config "${CLIFF_CONFIG}" "${tag_args[@]}")
for p in "${paths[@]}"; do
  [ -n "$p" ] || continue
  cliff_args+=(--include-path "${p}")
done
cliff_args+=(--tag-pattern "releases/${COMPONENT}/v[0-9]+\.[0-9]+\.[0-9]+$")

git cliff "${cliff_args[@]}" | emit_block content
