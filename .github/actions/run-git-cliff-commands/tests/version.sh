#!/usr/bin/env bash
# Regression coverage for component versions and dependency path filtering.
# Run from any directory with git-cliff on PATH.
set -euo pipefail

ACTION_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$ACTION_DIR/../../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
export COMPONENT=prover-ray
# shellcheck source=../scripts/components.sh
. "$ACTION_DIR/scripts/components.sh"
INCLUDE_PATHS="$(component_include_path "$COMPONENT")"
export INCLUDE_PATHS
export CLIFF_CONFIG="$WORK/cliff.toml"
SCOPES="$(component_scopes "$COMPONENT")" TEMPLATE="$REPO_ROOT/cliff.template.toml" \
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
  if ! grep -Fxq -- "$2" <<< "$1"; then
    printf 'Expected %s in output:\n%s\n' "$2" "$1" >&2
    exit 1
  fi
}

output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'tag=releases/prover-ray/v0.1.0'
assert_output "$output" 'version=0.1.0'
assert_output "$output" 'changed=true'

# An existing bump section must remain valid and the component namespace must
# override its initial tag without rewriting the config.
printf '\n[bump]\ninitial_tag = "v9.0.0"\n' >> "$CLIFF_CONFIG"
cp "$CLIFF_CONFIG" "$WORK/original.toml"
output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'tag=releases/prover-ray/v0.1.0'
cmp "$CLIFF_CONFIG" "$WORK/original.toml"

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

# A qualifying rollup-only change must not release prover-ray or appear in its
# changelog, while l2-execution and shared guest dependency fixes must do both.
mkdir -p riscv-guests/rollup
printf 'rollup\n' > riscv-guests/rollup/source.txt
git add riscv-guests/rollup/source.txt
git commit -q -s -m 'feat(riscv-guest): rollup-only change'
output=$("$ACTION_DIR/scripts/version.sh")
assert_output "$output" 'changed=false'

patch=2
for dependency in l2-execution guest-common build_common release lineth-accelerators guest-crypto-ctt; do
  mkdir -p "riscv-guests/$dependency"
  printf 'fix\n' > "riscv-guests/$dependency/source.txt"
  git add "riscv-guests/$dependency/source.txt"
  git commit -q -s -m "fix(riscv-guest): update $dependency"
  output=$("$ACTION_DIR/scripts/version.sh")
  assert_output "$output" "version=0.1.$patch"
  assert_output "$output" 'changed=true'
  changelog=$("$ACTION_DIR/scripts/changelog.sh")
  assert_output "$changelog" "- *(riscv-guest)* Update $dependency"
  if grep -qi 'rollup-only change' <<< "$changelog"; then
    echo 'Rollup-only change leaked into prover-ray changelog' >&2
    exit 1
  fi
  git tag "releases/prover-ray/v0.1.$patch"
  patch=$((patch + 1))
done
echo 'Component version regression tests passed'
