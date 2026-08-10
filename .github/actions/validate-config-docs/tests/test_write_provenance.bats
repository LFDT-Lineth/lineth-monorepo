#!/usr/bin/env bats

# Tests for scripts/write-provenance.sh.

setup() {
  REPO="$(mktemp -d)"
  export REPO
  cd "$REPO" || exit 1
  mkdir -p _generated/coordinator
  : > _generated/coordinator/reference.mdx
  SCRIPT_DIR="$(cd "${BATS_TEST_DIRNAME}/.." && pwd)/scripts"
}

teardown() {
  [[ -n "${REPO:-}" ]] && rm -rf "$REPO"
}

@test "writes SHA-only provenance when not on a release tag" {
  SOURCE_SHA="abcdef0123456789abcdef0123456789abcdef01" \
  SOURCE_REF_NAME="main" \
  SOURCE_REF_TYPE="branch" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/coordinator/" \
  MDX_PARTIAL_PATH="_generated/coordinator/reference.mdx" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -eq 0 ]
  [ -s "_generated/coordinator/provenance.mdx" ]
  grep -q 'LFDT-Lineth/lineth-monorepo@abcdef0' _generated/coordinator/provenance.mdx
  ! grep -q 'release `releases/coordinator/' _generated/coordinator/provenance.mdx
  grep -q 'Keys may differ in other Coordinator releases.' _generated/coordinator/provenance.mdx
}

@test "writes SHA + release tag when on a matching release tag" {
  SOURCE_SHA="abcdef0123456789abcdef0123456789abcdef01" \
  SOURCE_REF_NAME="releases/coordinator/v1.2.3" \
  SOURCE_REF_TYPE="tag" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/coordinator/" \
  MDX_PARTIAL_PATH="_generated/coordinator/reference.mdx" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -eq 0 ]
  grep -q 'release `releases/coordinator/v1.2.3`' _generated/coordinator/provenance.mdx
}

@test "omits release suffix when prefix does not match" {
  SOURCE_SHA="abcdef0123456789abcdef0123456789abcdef01" \
  SOURCE_REF_NAME="releases/maru/v0.1.0" \
  SOURCE_REF_TYPE="tag" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/coordinator/" \
  MDX_PARTIAL_PATH="_generated/coordinator/reference.mdx" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -eq 0 ]
  ! grep -q 'release `releases/' _generated/coordinator/provenance.mdx
}

@test "refuses when SOURCE_SHA is empty" {
  SOURCE_SHA="" \
  SOURCE_REF_NAME="main" \
  SOURCE_REF_TYPE="branch" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/coordinator/" \
  MDX_PARTIAL_PATH="_generated/coordinator/reference.mdx" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"SOURCE_SHA is empty"* ]]
}

@test "refuses when MDX_PARTIAL_PATH is empty" {
  SOURCE_SHA="abcdef0123456789abcdef0123456789abcdef01" \
  SOURCE_REF_NAME="main" \
  SOURCE_REF_TYPE="branch" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/coordinator/" \
  MDX_PARTIAL_PATH="" \
  COMPONENT_NAME="coordinator" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"MDX_PARTIAL_PATH is empty"* ]]
}

@test "capitalizes component name in the footer" {
  SOURCE_SHA="abcdef0123456789abcdef0123456789abcdef01" \
  SOURCE_REF_NAME="main" \
  SOURCE_REF_TYPE="branch" \
  PROVENANCE_RELEASE_TAG_PREFIX="releases/maru/" \
  MDX_PARTIAL_PATH="_generated/coordinator/reference.mdx" \
  COMPONENT_NAME="maru" \
  run bash "$SCRIPT_DIR/write-provenance.sh"
  [ "$status" -eq 0 ]
  grep -q 'Keys may differ in other Maru releases.' _generated/coordinator/provenance.mdx
}
