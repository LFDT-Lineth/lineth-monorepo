#!/usr/bin/env bash
# Compute the next release version for a component using git-cliff.
#
# Inputs (env):  COMPONENT, CLIFF_CONFIG, INCLUDE_PATHS (multi-line),
#                RELEASE_TAG_SUFFIX (optional)
# Outputs:       tag, version, changed
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"

: "${COMPONENT:?COMPONENT is required}"
: "${CLIFF_CONFIG:?CLIFF_CONFIG is required}"
RELEASE_TAG_SUFFIX="${RELEASE_TAG_SUFFIX:-}"

# Get the latest tag matching the release pattern (newest by semver).
latest_tag=$(git tag --list "releases/${COMPONENT}/v*" \
  | grep -E "^releases/${COMPONENT}/v[0-9]+\.[0-9]+\.[0-9]+$" \
  | sort -V \
  | tail -n 1 \
  || true)

# Read INCLUDE_PATHS (one path per line) into an array. A plain read loop keeps
# this working on bash 3.2 (macOS) for local dry-runs, not just CI's bash 5.
paths=()
while IFS= read -r p; do
  [ -n "$p" ] && paths+=("$p")
done <<< "${INCLUDE_PATHS:-}"
# --unreleased scopes the bump computation to commits since the last matching
# tag. Without it, git-cliff walks the full visible history and can over-bump
# (e.g. counting feats from prior release windows).
cliff_args=(--config "${CLIFF_CONFIG}" --bumped-version --unreleased)
for p in "${paths[@]}"; do
  [ -n "$p" ] || continue
  cliff_args+=(--include-path "${p}")
done
cliff_args+=(--tag-pattern "releases/${COMPONENT}/v[0-9]+\.[0-9]+\.[0-9]+$")

# Capture stdout and stderr separately so we can both surface git-cliff's logs
# and detect the "nothing to bump" warning it writes to stderr.
stderr_file=$(mktemp)
trap 'rm -f "${stderr_file}"' EXIT
next_tag=$(git cliff "${cliff_args[@]}" 2> "${stderr_file}")
# Re-emit stderr so it's visible in logs
cat "${stderr_file}" >&2

changed=true
# Check if git cliff warned "nothing to bump"
if grep -q "There is nothing to bump" "${stderr_file}"; then
  changed=false
elif [ "${next_tag}" = "${latest_tag}" ]; then
  changed=false
fi
if [ -z "${next_tag}" ] || [ "${changed}" = "false" ]; then
  log "git-cliff reports nothing to bump for ${COMPONENT}; nothing to release."
  emit_kv tag ""
  emit_kv version ""
  emit_kv changed "false"
  exit 0
fi
next_semver="${next_tag#releases/${COMPONENT}/v}"
if [ -n "${RELEASE_TAG_SUFFIX}" ]; then
  next_tag="${next_tag}-${RELEASE_TAG_SUFFIX}"
  next_semver="${next_semver}-${RELEASE_TAG_SUFFIX}"
fi

if git rev-parse -q --verify "refs/tags/${next_tag}" > /dev/null; then
  log "The next tag ${next_tag} already exists; exit now."
  exit 1
fi

log "latest ${COMPONENT} tag:  ${latest_tag}"
log "next ${COMPONENT} tag:    ${next_tag}"
log "next ${COMPONENT} semver: ${next_semver}"
log "tag changed: ${changed}"
emit_kv tag "${next_tag}"
emit_kv version "${next_semver}"
emit_kv changed "${changed}"
