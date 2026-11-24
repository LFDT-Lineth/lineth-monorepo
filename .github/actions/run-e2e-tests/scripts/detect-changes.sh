#!/usr/bin/env bash
# Usage: detect-changes.sh <zkevm-files-json> <riscv-files-json>
# Prints zkevm= and riscv= lines for $GITHUB_OUTPUT.

set -euo pipefail

is_test_or_doc() {
  # Unit tests, except the e2e tests themselves.
  [[ "$1" =~ \.(test|spec)\.ts$ && ! "$1" =~ ^e2e/src/ ]] ||
    [[ "$1" =~ _test\.go$ ]] ||
    [[ "$1" =~ ^tracer/(reference-tests|testing)/ ]] ||
    [[ "$1" =~ ^linea-besu/plugins/linea-sequencer/acceptance-tests/ ]] ||
    [[ "$1" =~ ^riscv-guests/[^/]+/test/ ]] ||
    [[ "$1" =~ \.(md|mdx)$ ]]
}
is_riscv_only() {
  [[ "$1" =~ (^|/)riscv[./] || "$1" =~ [-.]riscv[-.] || "$1" =~ ^(prover-ray|riscv-guests)/ ]]
}
is_zkevm_only() {
  [[ "$1" =~ ^e2e/src/.*\.spec\.ts$ || "$1" =~ [-.]zkevm[-.] || "$1" =~ ^docker/config/prover/v3/ ]]
}

# Usage: has_match <files-json> <keep-fn>; prints the first non-test file accepted by keep-fn.
has_match() {
  local file
  while read -r file; do
    if [ -n "$file" ] && ! is_test_or_doc "$file" && "$2" "$file"; then
      echo "$file"
      return 0
    fi
  done < <(jq -r '.[]' <<< "$1")
  return 1
}

not_riscv_only() { ! is_riscv_only "$1"; }
not_zkevm_only() { ! is_zkevm_only "$1"; }

zkevm=false riscv=false
file="$(has_match "${1:-[]}" not_riscv_only)" && zkevm=true && echo "zkEVM e2e-relevant file: $file" >&2
file="$(has_match "${2:-[]}" not_zkevm_only)" && riscv=true && echo "RISC-V e2e-relevant file: $file" >&2

echo "zkevm=$zkevm"
echo "riscv=$riscv"
