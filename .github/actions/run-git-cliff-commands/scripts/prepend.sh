#!/usr/bin/env bash
# Generate the release changelog section and prepend it to the component's
# CHANGELOG.md, dropping any stale [unreleased] block first.
#
# Inputs (env):  COMPONENT, CLIFF_CONFIG, INCLUDE_PATHS (multi-line),
#                FOLDER_NAME (dir holding CHANGELOG.md),
#                GENERATE_WITH_NEXT_TAG (true|false, default false),
#                NEXT_VERSION (required when GENERATE_WITH_NEXT_TAG=true),
#                TARGET_REF (optional; branch to fast-forward before writing, to dodge a
#                            race with a concurrent push to that same branch. Only pass
#                            this when the caller will push the result back — skipped
#                            when empty),
#                DRY_RUN (true|false, default false)
#
# With DRY_RUN=true it works on a throwaway copy and prints the resulting
# CHANGELOG.md to stdout without touching the real file or the git remote.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

: "${COMPONENT:?COMPONENT is required}"
: "${CLIFF_CONFIG:?CLIFF_CONFIG is required}"
: "${FOLDER_NAME:?FOLDER_NAME is required}"
GENERATE_WITH_NEXT_TAG="${GENERATE_WITH_NEXT_TAG:-false}"
NEXT_VERSION="${NEXT_VERSION:-}"
DRY_RUN="${DRY_RUN:-false}"

CHANGELOG_FILE="${FOLDER_NAME}/CHANGELOG.md"
if [ ! -f "${CHANGELOG_FILE}" ]; then
  log "changelog not found: ${CHANGELOG_FILE}"
  exit 1
fi

if [ "${DRY_RUN}" = "true" ]; then
  # Work on a copy so the real file and the git remote are never touched.
  target="$(mktemp)"
  trap 'rm -f "${target}"' EXIT
  cp "${CHANGELOG_FILE}" "${target}"
else
  # When the caller is about to commit-and-push this file, retrieve the latest
  # changes first to avoid rebase/merge conflicts with a concurrent push to the
  # same branch. Skipped when TARGET_REF is unset — callers that don't push the
  # result (e.g. a read-only CI preview) have nothing to race against, and must
  # not pass an event-derived ref here (see action.yml).
  if [ -n "${TARGET_REF:-}" ]; then
    git pull --ff-only origin "refs/heads/${TARGET_REF}"
  fi
  target="${CHANGELOG_FILE}"
fi

# Drop existing [unreleased] block if any
awk '
  /^## \[unreleased\]/ { skip=1; next }
  /^## \[/ && skip       { skip=0 }
  !skip                  { print }
' "${target}" > "${target}.tmp" && mv "${target}.tmp" "${target}"

# Build the git-cliff invocation for the fresh [unreleased/tag-version] section.
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
  cliff_args+=(--include-path "${p}")
done
cliff_args+=(--tag-pattern "releases/${COMPONENT}/v[0-9]+\.[0-9]+\.[0-9]+$" --prepend "${target}")
git cliff "${cliff_args[@]}"

if [ "${DRY_RUN}" = "true" ]; then
  log "===== prepended ${CHANGELOG_FILE} preview (${COMPONENT}${NEXT_VERSION:+ }${NEXT_VERSION}) ====="
  cat "${target}"
fi
