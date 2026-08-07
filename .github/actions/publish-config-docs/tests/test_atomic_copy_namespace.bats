#!/usr/bin/env bats

# Tests for scripts/atomic-copy-namespace.sh.

setup() {
  WORK="$(mktemp -d)"
  export WORK
  ARTIFACT_DIR="$WORK/artifact"
  DOCS_CHECKOUT="$WORK/doc.linea"
  mkdir -p "$ARTIFACT_DIR/_generated/coordinator" "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator"
  # Pre-existing destination content that should disappear after the swap.
  printf 'stale\n' > "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/old.mdx"
  SCRIPT_DIR="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)/scripts"
}

teardown() {
  [[ -n "${WORK:-}" ]] && rm -rf "$WORK"
}

@test "happy path: swaps the namespace cleanly" {
  printf 'new\n' > "$ARTIFACT_DIR/_generated/coordinator/reference.mdx"
  printf 'prov\n' > "$ARTIFACT_DIR/_generated/coordinator/provenance.mdx"
  ARTIFACT_DIR="$ARTIFACT_DIR" \
  ARTIFACT_NAMESPACE="_generated/coordinator" \
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/atomic-copy-namespace.sh"
  [ "$status" -eq 0 ]
  [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/reference.mdx" ]
  [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/provenance.mdx" ]
  ! [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/old.mdx" ]
  # Staging dir must be gone after the swap.
  ! [ -d "$DOCS_CHECKOUT/.coordinator-config-docs-staging" ]
}

@test "refuses when the artifact namespace is missing" {
  ARTIFACT_DIR="$ARTIFACT_DIR" \
  ARTIFACT_NAMESPACE="_generated/does-not-exist" \
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/atomic-copy-namespace.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"artifact namespace"*"does not exist"* ]]
  # Destination untouched.
  [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/old.mdx" ]
}

@test "refuses when the artifact namespace has no MDX partials" {
  mkdir -p "$ARTIFACT_DIR/_generated/empty"
  ARTIFACT_DIR="$ARTIFACT_DIR" \
  ARTIFACT_NAMESPACE="_generated/empty" \
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/atomic-copy-namespace.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"no MDX partials"* ]]
  # Destination untouched.
  [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/old.mdx" ]
}

@test "defaults ARTIFACT_NAMESPACE to basename of DOCS_NAMESPACE" {
  # When ARTIFACT_NAMESPACE is unset, the script defaults it to the basename of
  # DOCS_NAMESPACE ("coordinator"), so the source must live at $ARTIFACT_DIR/coordinator/.
  mkdir -p "$ARTIFACT_DIR/coordinator"
  printf 'new\n' > "$ARTIFACT_DIR/coordinator/reference.mdx"
  ARTIFACT_DIR="$ARTIFACT_DIR" \
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/atomic-copy-namespace.sh"
  [ "$status" -eq 0 ]
  [ -f "$DOCS_CHECKOUT/docs/stack/reference/_generated/coordinator/reference.mdx" ]
}

@test "refuses when required env vars are missing" {
  ARTIFACT_DIR="" \
  DOCS_CHECKOUT="$DOCS_CHECKOUT" \
  DOCS_NAMESPACE="docs/stack/reference/_generated/coordinator" \
  run bash "$SCRIPT_DIR/atomic-copy-namespace.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"ARTIFACT_DIR, DOCS_CHECKOUT and DOCS_NAMESPACE must all be set"* ]]
}
