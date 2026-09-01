# l2-execution guest — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file for every guest; guest-specific
# targets (reference-test, the ZKC fixture-exec/fixture-debug below) live here, not in the
# orchestrator.
#
# Scope split: building + host-testing is proving-system-AGNOSTIC and lives here (`compile` links the
# statically-linked rv64im ELF — the zkvm-standards artifact — via build_common). Only the ZKC run
# itself (ELF->JSON via elf_to_json_gen, then zkc) is owned by
# arithmetization/src/test/Makefile; fixture-exec/fixture-debug build that ELF and hand it over.

.PHONY: fetch test reference-test reference-test-zkc prep-execution-specs-json-fixtures clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines ZKC run (elf-to-json and zkc exec)
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

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

# Flags forwarded to `zkc exec` by exec. Quiet (no per-instruction printf) is zkc's default unless
# -v/-vv/-vvv raise verbosity. Override to pick the execution backend — e.g. "--gogen --fast" runs
# the guest via zkc's generated-Go backend in fast mode, the runnability path used while the
# interpreter does not implement tracing yet. "--check" runs the guest via the interpreter with
# tracing enabled.
ZKC_EXEC_FLAGS ?=

# Parameters for execution-spec zkevm fixtures.
EXECUTION_SPECS_JSON_FIXTURES_DIR   := /tmp/execution-specs-json-fixtures
EXECUTION_SPECS_FIXTURES_DIR        := $(EXECUTION_SPECS_JSON_FIXTURES_DIR)/fixtures

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

# Permanent reference-test guard for the extended guest, run against the EF zkevm corpus on the
# host (see extended_vanilla_runner.zig): asserts the dummy-wrapped extended guest (runL2Execution,
# delegating per-block execution to execution.executeStatelessInputWithLogs) agrees with the EF
# fixture's own expected validity verdict, cheaply on the host instead of compiling to riscv64 and
# executing the real guest ELF via ZkC. Scoped to --fork Amsterdam to keep runtime bounded; narrow
# further with REFERENCE_ARGS.
REFERENCE_ARGS ?= --fork Amsterdam
reference-test:
	$(ZIG) build extended-vanilla $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -- $(REFERENCE_ARGS)

# ZkC twin of reference-test: the corpus logic lives in test/zkc_reference_runner.zig, which runs
# each block through the guest ELF under zkc (ELF->JSON->zkc exec) instead of in-process. Needs zkc
# and go on PATH (like exec/debug). ZKC_EXEC_FLAGS picks the `zkc exec` backend (default --gogen
# --fast). Far slower than reference-test; use it to prove the real ELF agrees with the corpus.
reference-test-zkc:
	$(ZIG) build reference-test-zkc $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -- $(REFERENCE_ARGS) $(if $(ZKC_EXEC_FLAGS),--zkc-flags "$(ZKC_EXEC_FLAGS)")

# Re-uses logic from build.zig to download execution-spec JSON fixtures under $(EXECUTION_SPECS_FIXTURES_DIR).
prep-execution-specs-json-fixtures:
	$(ZIG) build prep-execution-specs-json-fixtures $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -Dexecution-specs-fixtures-link="$(EXECUTION_SPECS_FIXTURES_DIR)"

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

trace: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-trace BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_TRACE_FLAGS="$(ZKC_TRACE_FLAGS)"

debug: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"
