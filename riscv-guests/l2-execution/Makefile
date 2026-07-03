# l2-execution guest — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`spec-test`/`fetch`/`clean` out to this file for every guest;
# guest-specific targets (the ZKC fixture-exec/fixture-debug below) live here, not in the orchestrator.
#
# Scope split: building + host-testing is proving-system-AGNOSTIC and lives here (`compile` links the
# statically-linked rv64im ELF — the zkvm-standards artifact — via build_common). Only the ZKC run
# itself (ELF->JSON via elf_to_json_gen, then zkc) is owned by
# arithmetization/src/test/Makefile; fixture-exec/fixture-debug build that ELF and hand it over.

.PHONY: fetch test spec-test prep-execution-specs-json-fixtures run-execution-specs-ssz-fixtures clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines ZKC run (elf-to-json and zkc exec)
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig
GO ?= go

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# NOTE: `zig build --fetch` fetches the dependency tree and EXITS without building or running, so it
# must NOT be passed to build/test targets. Pre-fetching is the dedicated `fetch` target.
ZIG_FETCH_FLAGS ?=


# Name and ELF generated of the guest program (@TODO: check if we need that)
NAME     := evm_execution_guest
BIN      := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

# Sample fixture input used by exec/debug
INPUT    ?= $(MAKEFILE_DIR)test/testdata/stateless_input.ssz

# Flags forwarded to `zkc exec` by exec. Default -q (quiet: suppress the guest's per-instruction
# printf). Override to pick the execution backend — e.g. "--quiet --gogen --fast" runs the guest via
# zkc's generated-Go backend in fast mode, the runnability path used while the interpreter does not
# implement tracing yet.
ZKC_EXEC_FLAGS ?=--gogen --fast -q 
# ZKC_EXEC_FLAGS ?=-q --check
# TODO: remove unnecessary flags before merging

# Parameters for execution-spec zkevm fixtures and selected temporary SSZ inputs.
EXECUTION_SPECS_JSON_FIXTURES_DIR   := /tmp/execution-specs-json-fixtures
EXECUTION_SPECS_SSZ_FIXTURES_DIR    := /tmp/execution-specs-ssz-fixtures
EXECUTION_SPECS_FIXTURES_DIR        := $(EXECUTION_SPECS_JSON_FIXTURES_DIR)/fixtures
EXECUTION_SPECS_RUN_FIXTURE_PATHS   ?= blockchain_tests/for_amsterdam/amsterdam,blockchain_tests/for_amsterdam/osaka
EXECUTION_SPECS_RUN_SSZ_LIMIT       ?= 100

# If true, generates an objdump of the generated ELF for debugging purposes
OBJDUMP     ?= false

# If true, build with the arithmetization keccak wrapper (prover-accelerated
# custom op) instead of the standard zig keccak.
KECCAK_ACCEL ?= false

# ── Proving-system-agnostic guest lifecycle ─────────────────────────────────
fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)

# Run the guest against the full EF zkevm stateless fixture set on the host. Fixtures come from the
# lazy execution_spec_tests_zkevm dependency. Narrow/triage with SPEC_ARGS, e.g.
# `make spec-test SPEC_ARGS="--fork Amsterdam -x"` or `SPEC_ARGS="--report-only"`.
SPEC_ARGS ?=
spec-test:
	$(ZIG) build spec-tests $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) $(if $(strip $(SPEC_ARGS)),-- $(SPEC_ARGS))

## Exposes the downloaded execution-spec JSON fixtures under $(EXECUTION_SPECS_FIXTURES_DIR).
prep-execution-specs-json-fixtures:
	$(ZIG) build prep-execution-specs-json-fixtures $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -Dexecution-specs-fixtures-link="$(EXECUTION_SPECS_FIXTURES_DIR)"

## Runs selected execution-spec fixtures through zkc; generated SSZ inputs can be inspected in $(EXECUTION_SPECS_SSZ_FIXTURES_DIR).
run-execution-specs-ssz-fixtures: prep-execution-specs-json-fixtures
	GOCACHE=/tmp/go-build $(GO) run $(MAKEFILE_DIR)scripts/run_execution_specs_ssz_fixtures.go --fixtures-dir "$(EXECUTION_SPECS_FIXTURES_DIR)" --ssz-dir "$(EXECUTION_SPECS_SSZ_FIXTURES_DIR)" --fixture-paths "$(EXECUTION_SPECS_RUN_FIXTURE_PATHS)" --ssz-limit "$(EXECUTION_SPECS_RUN_SSZ_LIMIT)"

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) $(if $(filter true,$(KECCAK_ACCEL)),-Dkeccak-accel=true)
	@if command -v riscv64-unknown-elf-objdump >/dev/null 2>&1; then \
		if [ "$(OBJDUMP)" = "true" ]; then \
			echo "\nDisassembling $(BIN)"; \
			LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(BIN) > $(BIN).objdump; \
		fi \
	else \
		echo "\nSkipping objdump: riscv64-unknown-elf-objdump is not on PATH"; \
	fi

# Fail early on a missing/empty INPUT instead of building then erroring in the converter.
require-input:
	@[ -n "$(INPUT)" ] && [ -f "$(INPUT)" ] || { echo "INPUT not found: '$(INPUT)' (set INPUT=path/to/input.ssz)"; exit 1; }

exec: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

debug: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"
