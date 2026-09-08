#!/usr/bin/env bash
# Look up a component's default git-cliff SCOPES/INCLUDE_PATH from the single
# source of truth in components.sh.
#
# Inputs (env):  COMPONENT
# Output:        scopes, include_path  (multi-line)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
# shellcheck source=components.sh
. "${SCRIPT_DIR}/components.sh"

: "${COMPONENT:?COMPONENT is required}"

emit_kv scopes "$(component_scopes "$COMPONENT")"
component_include_path "$COMPONENT" | emit_block include_path
