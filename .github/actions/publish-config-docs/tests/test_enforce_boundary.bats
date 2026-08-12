#!/usr/bin/env bats

# Tests for scripts/enforce-boundary.sh.

setup() {
  WORK="$(mktemp -d)"
  export WORK
  DOCS_CHECKOUT="$WORK/doc.linea"
  mkdir -p "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator"
  cd "$DOCS_CHECKOUT" || exit 1
  git init -q
  git config user.email "t@t"
  git config user.name "t"
  git config commit.gpgsign false
  # Seed the namespace + a sibling human-owned file.
  printf 'seed\n' > docs/stack/reference/_generated/coordinator/reference.mdx
  printf 'human\n' > docs/stack/reference/linea-coordinator-options.mdx
  git add -A
  git commit -q -m init
  SCRIPT_DIR="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)/scripts"
}

teardown() {
  [[ -n "${WORK:-}" ]] && rm -rf "$WORK"
}

@test "passes when changes are confined to the namespace" {
  printf 'updated\n' > docs/stack/reference/_generated/coordinator/reference.mdx
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/enforce-boundary.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"All changes confined"* ]]
}

@test "fails when a change is outside the namespace" {
  printf 'updated\n' > docs/stack/reference/linea-coordinator-options.mdx
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/enforce-boundary.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"outside the component namespace"* ]]
  [[ "$output" == *"linea-coordinator-options.mdx"* ]]
}

@test "fails when changes span both inside and outside the namespace" {
  printf 'updated\n' > docs/stack/reference/_generated/coordinator/reference.mdx
  printf 'updated\n' > docs/stack/reference/linea-coordinator-options.mdx
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/enforce-boundary.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"outside the component namespace"* ]]
}

@test "passes when a new file is added inside the namespace" {
  printf 'new\n' > docs/stack/reference/_generated/coordinator/provenance.mdx
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/enforce-boundary.sh"
  [ "$status" -eq 0 ]
}

@test "refuses when required env vars are missing" {
  DOCS_CHECKOUT="" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/enforce-boundary.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"DOCS_CHECKOUT and DOCS_NAMESPACE must both be set"* ]]
}
