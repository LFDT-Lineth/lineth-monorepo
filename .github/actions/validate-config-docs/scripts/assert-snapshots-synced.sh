#!/usr/bin/env bash
# Asserts that every committed snapshot path is byte-identical to what
# `gradle-generate-task` just produced. Exits non-zero with an actionable error
# message if any path drifted.
#
# Env:
#   COMMITTED_SNAPSHOT_PATHS  whitespace-separated repo-relative paths
#   GRADLE_GENERATE_TASK      gradle task the caller should re-run to fix drift
#   COMPONENT_NAME            human-readable component name for the error message
set -euo pipefail

if [[ -z "${COMMITTED_SNAPSHOT_PATHS:-}" ]]; then
  echo "::error::COMMITTED_SNAPSHOT_PATHS is empty; nothing to assert."
  exit 1
fi

# shellcheck disable=SC2086  # intentional word-splitting on the path list
if ! git diff --exit-code -- $COMMITTED_SNAPSHOT_PATHS; then
  echo "::error::${COMPONENT_NAME:-component} config docs are out of date. Run './gradlew ${GRADLE_GENERATE_TASK:-<generateConfigDocs>}' and commit the changes."
  exit 1
fi

echo "Committed snapshots synchronized ($COMMITTED_SNAPSHOT_PATHS)."
