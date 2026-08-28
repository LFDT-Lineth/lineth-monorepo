#!/usr/bin/env bats

# Tests for scripts/assert-snapshots-synced.sh.
#
# Each test stages a tiny git repo, optionally writes drift into a tracked file,
# then runs the script and asserts the exit code.

setup() {
  REPO="$(mktemp -d)"
  export REPO
  cd "$REPO" || exit 1
  git init -q
  git config user.email "t@t"
  git config user.name "t"
  git config commit.gpgsign false
  SCRIPT_DIR="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)/scripts"
}

teardown() {
  [[ -n "${REPO:-}" ]] && rm -rf "$REPO"
}

@test "passes when committed snapshots are unchanged" {
  printf 'snapshot\n' > snapshot.md
  git add snapshot.md
  git commit -q -m init
  COMMITTED_SNAPSHOT_PATHS="snapshot.md" \
  GRADLE_GENERATE_TASK=":coordinator:app:generateConfigDocs" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/assert-snapshots-synced.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Committed snapshots synchronized"* ]]
}

@test "fails when a committed snapshot drifted" {
  printf 'snapshot\n' > snapshot.md
  git add snapshot.md
  git commit -q -m init
  printf 'drifted\n' > snapshot.md
  COMMITTED_SNAPSHOT_PATHS="snapshot.md" \
  GRADLE_GENERATE_TASK=":coordinator:app:generateConfigDocs" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/assert-snapshots-synced.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"out of date"* ]]
  [[ "$output" == *":coordinator:app:generateConfigDocs"* ]]
}

@test "fails when COMMITTED_SNAPSHOT_PATHS is empty" {
  COMMITTED_SNAPSHOT_PATHS="" \
  run bash "$SCRIPT_DIR/assert-snapshots-synced.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"COMMITTED_SNAPSHOT_PATHS is empty"* ]]
}
