#!/usr/bin/env bash
# Regression coverage for first releases and subsequent component version bumps.
# Run from any directory with git-cliff on PATH.
set -euo pipefail

ACTION_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$ACTION_DIR/../../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export COMPONENT=prover-ray
export INCLUDE_PATHS='prover-ray/**'
export CLIFF_CONFIG="$WORK/cliff.toml"
SCOPES='prover-ray|deps|misc' TEMPLATE="$REPO_ROOT/cliff.template.toml" \
  RENDERED_CONFIG="$CLIFF_CONFIG" "$ACTION_DIR/scripts/render.sh" >/dev/null

cd "$WORK"
git init -q
git config user.name 'Release Test'
git config user.email 'release-test@example.invalid'
mkdir prover-ray
printf 'first\n' > prover-ray/source.txt
git add .
git commit -q -s -m 'feat(prover-ray): initial component'

assert_output() {
  if ! grep -Fxq "$2" <<< "$1"; then
    printf 'Expected %s in output:\n%s\n' "$2" "$1" >&2
    exit 1
  fi
}

output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'tag=releases/prover-ray/v0.1.0'
assert_output "$output" 'version=0.1.0'
assert_output "$output" 'changed=true'

output=$(RELEASE_TAG_SUFFIX=dev-mock "$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'tag=releases/prover-ray/v0.1.0-dev-mock'
assert_output "$output" 'version=0.1.0-dev-mock'

git tag releases/prover-ray/v0.1.0
printf 'fix\n' >> prover-ray/source.txt
git add prover-ray/source.txt
git commit -q -s -m 'fix(prover-ray): subsequent fix'
output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'tag=releases/prover-ray/v0.1.1'
assert_output "$output" 'version=0.1.1'
assert_output "$output" 'changed=true'

git tag releases/prover-ray/v0.1.1
output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'changed=false'
echo 'Component version regression tests passed'
