#!/usr/bin/env bash
# Regenerate the [Unreleased] section of each changed component's CHANGELOG.md
# in parallel — one background subshell per component — on a single machine.
#
# This is the body of the "Generate changelogs for changed components
# (parallel)" step in .github/workflows/component-changelog-update.yml,
# extracted so the exact same logic can be reproduced locally.
#
# Usage (from the repository root; git-cliff include-path patterns and the
# template path are relative to it):
#
#   # every known component (the default — no args, no env needed):
#   .github/actions/run-git-cliff-commands/scripts/update-unreleased-changelogs.sh
#
#   # selected components as arguments (no jq needed):
#   .github/actions/run-git-cliff-commands/scripts/update-unreleased-changelogs.sh maru coordinator
#
#   # or as a JSON array, the shape dorny/paths-filter produces in CI:
#   CHANGED_COMPONENTS='["maru","coordinator"]' \
#     .github/actions/run-git-cliff-commands/scripts/update-unreleased-changelogs.sh
#
#   # or via the Makefile (comma-separated COMPONENTS; default: all):
#   make -f .github/actions/run-git-cliff-commands/Makefile update-unreleased-changelogs \
#     COMPONENTS=coordinator,maru,postman
#
# To preview changes WITHOUT writing files, use the Makefile's *-dry-run
# targets per component instead (e.g. prepend-dry-run).
#
# Inputs (env):  CHANGED_COMPONENTS (JSON array; default when unset/empty and no
#                args are given: all components from component_names() in
#                components.sh; an explicit '[]' means "none"),
#                MAX_PARALLEL (default 4) — cap on concurrent git-cliff walks,
#                RUNNER_TEMP — scratch dir for rendered configs and per-component
#                logs (defaults to TMPDIR or /tmp).
# Output:        updated (true|false) — whether any CHANGELOG.md changed on disk.
# Exit:          non-zero if any component failed (all are still attempted).
#
# Kept bash 3.2-compatible (like the sibling scripts) so it runs on macOS
# machines without a newer bash: no mapfile, no `wait -n`, no `&>` redirect.
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
. "${SCRIPT_DIR}/lib.sh"
# shellcheck source=components.sh
. "${SCRIPT_DIR}/components.sh"

MAX_PARALLEL="${MAX_PARALLEL:-4}"
WORK_DIR="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"

if [ "$#" -gt 0 ]; then
  changed=("$@")
elif [ -n "${CHANGED_COMPONENTS:-}" ]; then
  changed=()
  while IFS= read -r line; do
    [ -n "${line}" ] && changed+=("${line}")
  done <<< "$(jq -r '.[]' <<< "${CHANGED_COMPONENTS}")"
else
  # No selection given: default to every known component (component_names() is
  # the single source of truth in components.sh).
  changed=()
  while IFS= read -r line; do
    [ -n "${line}" ] && changed+=("${line}")
  done <<< "$(component_names)"
fi

log "Changed components: ${changed[*]:-none}"
if ((${#changed[@]} == 0)); then
  emit_kv updated false
  exit 0
fi

# One component per subshell, following the same plumbing as the Makefile
# *-dry-run targets: component-info -> paths -> render -> version -> prepend.
# Scopes, include-paths and the changelog folder come from components.sh (the
# single source of truth).
run_component() {
  local c="$1"
  (
    # The scripts' step outputs are unused here; keep the parallel subshells
    # from appending to the shared $GITHUB_OUTPUT file.
    unset GITHUB_OUTPUT
    set -euo pipefail
    folder="$(component_folder "$c")"
    info="$(COMPONENT="$c" "${SCRIPT_DIR}/component-info.sh")"
    scopes="$(sed -n 's/^scopes=//p' <<< "${info}")"
    include_path="$(sed '1d' <<< "${info}")"
    include_paths="$(INCLUDE_PATH="${include_path}" "${SCRIPT_DIR}/paths.sh")"
    # render.sh's default RENDERED_CONFIG is a single fixed path which parallel
    # components would clobber — give each its own.
    config="$(SCOPES="${scopes}" RENDERED_CONFIG="${WORK_DIR}/cliff.${c}.toml" \
      "${SCRIPT_DIR}/render.sh" | sed -n 's/^config=//p')"
    vout="$(COMPONENT="$c" INCLUDE_PATHS="${include_paths}" CLIFF_CONFIG="${config}" \
      "${SCRIPT_DIR}/version.sh")"
    if [ "$(sed -n 's/^changed=//p' <<< "${vout}")" != "true" ]; then
      log "[${c}] no release-relevant commits; nothing to prepend"
      exit 0
    fi
    COMPONENT="$c" INCLUDE_PATHS="${include_paths}" CLIFF_CONFIG="${config}" \
      "${SCRIPT_DIR}/prepend.sh"
    log "[${c}] updated ${folder}/CHANGELOG.md"
  ) > "${WORK_DIR}/changelog-gen.${c}.log" 2>&1
  echo "$?" > "${WORK_DIR}/changelog-gen.${c}.exit"
}

for c in "${changed[@]}"; do
  run_component "$c" &
  # Throttle: git-cliff full-history walks are CPU-heavy, so don't start every
  # component at once on a small runner (or a laptop).
  while (( $(jobs -rp | wc -l) >= MAX_PARALLEL )); do sleep 1; done
done
wait

failed=0
for c in "${changed[@]}"; do
  code="$(cat "${WORK_DIR}/changelog-gen.${c}.exit" 2>/dev/null || echo 1)"
  if [ "${code}" = "0" ]; then
    log "== ${c}: ok"
  else
    log "== ${c}: FAILED (exit ${code})"
    failed=1
  fi
  sed 's/^/    /' "${WORK_DIR}/changelog-gen.${c}.log" >&2 || true
done

# The workspace only ever receives CHANGELOG.md edits (rendered configs and
# logs live under WORK_DIR outside the repo), so a plain diff tells the caller
# whether there is anything to commit.
if git diff --quiet; then
  emit_kv updated false
else
  emit_kv updated true
fi
exit "${failed}"
