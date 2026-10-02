#!/usr/bin/env bash
# Single source of truth for each component's git-cliff SCOPES and
# INCLUDE_PATH. Sourced by component-info.sh.
#
# To add/change a component's scopes or include-path, edit this file only —
# every workflow that passes `component-name` to the run-git-cliff-commands
# action (or a reusable workflow wrapping it) picks the values up automatically.

# component_scopes NAME — regex alternation of conventional-commit scopes.
component_scopes() {
  case "$1" in
    coordinator)         echo "coordinator|deps|misc" ;;
    maru)                echo "maru|deps|misc" ;;
    prover)               echo "prover|deps|misc" ;;
    riscv-l2-execution)  echo "riscv-guest|deps|misc" ;;
    riscv-rollup)        echo "riscv-guest|deps|misc" ;;
    postman)              echo "postman|deps|misc" ;;
    tx-exclusion-api)     echo "tx-exclusion-api|deps|misc" ;;
    linea-besu-package)   echo "linea-besu|tracer|sequencer|deps|misc" ;;
    linea)                echo "coordinator|linea-besu|tracer|sequencer|maru|prover|postman|tx-exclusion-api|deps|misc" ;;
    *)
      echo "components.sh: unknown component '$1'" >&2
      return 1
      ;;
  esac
}

# component_include_path NAME — git-cliff --include-path filter, one path per line.
component_include_path() {
  case "$1" in
    coordinator)       printf '%s\n' "coordinator/**" ;;
    maru)              printf '%s\n' "maru/**" ;;
    prover)            printf '%s\n' "prover/**" ;;
    riscv-l2-execution)
      printf '%s\n' \
        "riscv-guests/l2-execution/**" \
        "riscv-guests/guest-common/**" \
        "riscv-guests/build_common/**" \
        "riscv-guests/release/**" \
        "riscv-guests/lineth-accelerators/**" \
        "riscv-guests/guest-crypto-ctt/**" \
        "riscv-guests/.zigversion" \
        "riscv-guests/Makefile"
      ;;
    riscv-rollup)
      printf '%s\n' \
        "riscv-guests/rollup/**" \
        "riscv-guests/guest-common/**" \
        "riscv-guests/build_common/**" \
        "riscv-guests/release/**" \
        "riscv-guests/lineth-accelerators/**" \
        "riscv-guests/.zigversion" \
        "riscv-guests/Makefile"
      ;;
    postman)           printf '%s\n' "postman/**" ;;
    tx-exclusion-api)  printf '%s\n' "transaction-exclusion-api/**" ;;
    linea-besu-package)
      printf '%s\n' \
        "tracer/**" \
        "tracer-constraints/**" \
        "linea-besu/plugins/linea-sequencer/**" \
        "linea-besu/besu/**" \
        "linea-besu/package/**" \
        "gradle/libs.versions.toml"
      ;;
    linea)
      printf '%s\n' \
        "coordinator/**" \
        "maru/**" \
        "postman/**" \
        "prover/**" \
        "transaction-exclusion-api/**" \
        "tracer/**" \
        "tracer-constraints/**" \
        "linea-besu/plugins/linea-sequencer/**" \
        "linea-besu/besu/**" \
        "linea-besu/package/**" \
        "gradle/libs.versions.toml"
      ;;
    *)
      echo "components.sh: unknown component '$1'" >&2
      return 1
      ;;
  esac
}
