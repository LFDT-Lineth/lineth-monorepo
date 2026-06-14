# l2-execution guest — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `gp-compile`/`test`/`spec-test`/`fetch`/`clean` out to this file for every guest;
# guest-specific targets (the ZKC fixture-exec/fixture-debug below) live here, not in the orchestrator.
#
# Scope split: building + host-testing is proving-system-AGNOSTIC and lives here (`compile` links the
# statically-linked rv64im ELF — the zkvm-standards artifact — via build_common). Only the ZKC run
# itself (ELF->JSON via elf_to_json_gen, then zkc) is owned by
# arithmetization/src/test/Makefile; fixture-exec/fixture-debug build that ELF and hand it over.

# guest-program = gp

.PHONY: fetch test spec-test clean gp-linker-script gp-compile gp-exec gp-debug

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# NOTE: `zig build --fetch` fetches the dependency tree and EXITS without building or running, so it
# must NOT be passed to build/test targets. Pre-fetching is the dedicated `fetch` target.
ZIG_FETCH_FLAGS ?=

# Makefile containaining the logic for generating a linker scritpt, executing and debugging generic programs
ARITHMETIZATION_TESTING_MAKEFILE := $(abspath $(MAKEFILE_DIR)../../arithmetization/src/test)/Makefile

# Name and ELF generated of the guest program (@TODO: check if we need that)
GP_NAME     := evm_execution_guest
GP_BIN      := $(MAKEFILE_DIR)zig-out/bin/$(GP_NAME)

# Sample fixture input used by gp-exec/gp-debug
GP_INPUT    ?= $(MAKEFILE_DIR)test/testdata/stateless_input.ssz

# If true, generates an objdump of the generated ELF for debugging purposes
OBJDUMP     ?= false

# ── Proving-system-agnostic guest lifecycle ─────────────────────────────────
fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

test: gp-linker-script
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)

# Run the guest against the full EF zkevm stateless fixture set on the host. Fixtures come from the
# lazy execution_spec_tests_zkevm dependency. Narrow/triage with SPEC_ARGS, e.g.
# `make spec-test SPEC_ARGS="--fork Amsterdam -x"` or `SPEC_ARGS="--report-only"`.
SPEC_ARGS ?=
spec-test: gp-linker-script
	$(ZIG) build spec-tests $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) $(if $(strip $(SPEC_ARGS)),-- $(SPEC_ARGS))

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

# Note: gp-compile uses custom logic to this Makefile.
# gp-linker-scritpt, gp-exec, gp-debug delegate logic
# to arithmetization/src/test/Makefile.

# The following variables 
# STACK_ORIGIN   
# SP             
# PROGRAM_ORIGIN 
# IN_ORIGIN      
# HEAP_ORIGIN
# in arithmetization/src/test/Makefile have default values but can be overriden.

gp-linker-script:
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) linker-script LINKER_SCRIPT=$(MAKEFILE_DIR)../build_common/linker_script.ld

gp-compile: gp-linker-script
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)
	@if command -v riscv64-unknown-elf-objdump >/dev/null 2>&1; then \
		if [ "$(OBJDUMP)" = "true" ]; then \
			echo "\nDisassembling $(GP_BIN)"; \
			LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(GP_BIN) > $(GP_BIN).objdump; \
		fi \
	else \
		echo "\nSkipping objdump: riscv64-unknown-elf-objdump is not on PATH"; \
	fi

gp-exec: gp-compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(GP_BIN)" IN_BYTES="@$(GP_INPUT)" ZKC_EXEC_FLAGS=-q

gp-debug: gp-compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(GP_BIN)" IN_BYTES="@$(GP_INPUT)"
