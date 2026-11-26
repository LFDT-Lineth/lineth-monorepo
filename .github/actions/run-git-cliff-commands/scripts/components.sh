#!/usr/bin/env bash
# Single source of truth for each component's git-cliff SCOPES, INCLUDE_PATH
# and the folder hosting its CHANGELOG.md. Sourced by component-info.sh and
# update-unreleased-changelogs.sh.
#
# To add/change a component's scopes, include-path or changelog folder, edit
# this file only — every workflow that passes `component-name` to the
# run-git-cliff-commands action (or a reusable workflow wrapping it) picks the
# values up automatically. New components must also be added to
# .github/path-filters/changelog.yml so their changes are detected.

# component_scopes NAME — regex alternation of conventional-commit scopes.
component_scopes() {
  case "$1" in
    coordinator)         echo "coordinator|deps|misc" ;;
    maru)                echo "maru|deps|misc" ;;
    prover)               echo "prover|deps|misc" ;;
    prover-ray)           echo "prover-ray|deps|misc" ;;
    riscv-l2-execution)  echo "riscv-guest|deps|misc" ;;
    riscv-rollup)        echo "riscv-guest|deps|misc" ;;
    riscv-lineth-da)     echo "riscv-guest|deps|misc" ;;
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

# component_names — every known component, one per line. Used as the default
# component set by update-unreleased-changelogs.sh; keep in sync with the
# case statements below and .github/path-filters/changelog.yml.
component_names() {
  printf '%s\n' \
    "coordinator" \
    "maru" \
    "prover" \
    "prover-ray" \
    "riscv-l2-execution" \
    "riscv-rollup" \
    "riscv-lineth-da" \
    "postman" \
    "tx-exclusion-api" \
    "linea-besu-package" \
    "linea"
}

# component_folder NAME — folder hosting the component's CHANGELOG.md.
component_folder() {
  case "$1" in
    coordinator)        echo "coordinator" ;;
    maru)               echo "maru" ;;
    prover)             echo "prover" ;;
    prover-ray)         echo "prover-ray" ;;
    riscv-l2-execution) echo "riscv-guests/l2_execution" ;;
    riscv-rollup)       echo "riscv-guests/rollup" ;;
    riscv-lineth-da)    echo "riscv-guests/lineth_da" ;;
    postman)            echo "postman" ;;
    tx-exclusion-api)   echo "transaction-exclusion-api" ;;
    linea-besu-package) echo "linea-besu/package" ;;
    linea)              echo "." ;;
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
    prover-ray)        printf '%s\n' "prover-ray/**" ;;
    riscv-l2-execution)
      printf '%s\n' \
        "riscv-guests/l2_execution/**" \
        "riscv-guests/guest_common/**" \
        "riscv-guests/build_common/**" \
        "riscv-guests/release/**" \
        "riscv-guests/lineth_accelerators/**" \
        "riscv-guests/guest_crypto_ctt/**" \
        "riscv-guests/.zigversion" \
        "riscv-guests/Makefile" \
        "riscv-guests/Dockerfile" \
        "riscv-guests/.dockerignore"
      ;;
    riscv-rollup)
      printf '%s\n' \
        "riscv-guests/rollup/**" \
        "riscv-guests/lineth_da/**" \
        "riscv-guests/guest_common/**" \
        "riscv-guests/build_common/**" \
        "riscv-guests/release/**" \
        "riscv-guests/lineth_accelerators/**" \
        "riscv-guests/.zigversion" \
        "riscv-guests/Makefile" \
        "riscv-guests/Dockerfile" \
        "riscv-guests/.dockerignore"
      ;;
    riscv-lineth-da)
      printf '%s\n' \
        "riscv-guests/lineth_da/**" \
        "riscv-guests/guest_common/**" \
        "riscv-guests/build_common/**" \
        "riscv-guests/lineth_accelerators/**" \
        "riscv-guests/guest_crypto_ctt/**" \
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
        "linea-besu/plugins/sequencer-interfaces/**" \
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
        "linea-besu/plugins/sequencer-interfaces/**" \
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
