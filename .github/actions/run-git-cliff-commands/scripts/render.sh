#!/usr/bin/env bash
# Render the shared cliff template into a temp config with the caller's scopes
# injected into the @@SCOPES@@ placeholder. git-cliff can't inject scopes into
# commit_parsers itself, so we substitute at run time and point every git cliff
# invocation at the rendered file.
#
# Inputs (env):  SCOPES, TEMPLATE (default cliff.template.toml),
#                RENDERED_CONFIG (default ${RUNNER_TEMP:-/tmp}/cliff.rendered.toml)
# Output:        config  (path to the rendered toml)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

: "${SCOPES:?scopes input must be non-empty}"
TEMPLATE="${TEMPLATE:-cliff.template.toml}"
rendered="${RENDERED_CONFIG:-${RUNNER_TEMP:-/tmp}/cliff.rendered.toml}"

if [ ! -f "$TEMPLATE" ]; then
  log "template not found: ${TEMPLATE} (run from the repo root)"
  exit 1
fi

# '#' delimiter is safe: neither the placeholder nor scope tokens (which may
# contain '|') contain '#'.
sed "s#@@SCOPES@@#${SCOPES}#g" "$TEMPLATE" > "$rendered"
log "rendered git-cliff config: ${rendered}"
emit_kv config "$rendered"
