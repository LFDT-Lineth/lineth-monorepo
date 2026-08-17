# rollup guest — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file for every guest.
#
# Scope split: building + host-testing is proving-system-AGNOSTIC and lives here (`compile` links
# the statically-linked rv64im ELF — the zkvm-standards artifact — via build_common). Only the ZKC
# run itself (ELF->JSON via elf_to_json_gen, then zkc) is owned by
# arithmetization/src/test/Makefile; `exec`/`debug` build that ELF and hand it over.

.PHONY: fetch test clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines the ZKC run (elf-to-json and zkc exec).
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# NOTE: `zig build --fetch` fetches the dependency tree and EXITS without building or running, so
# it must NOT be passed to build/test targets. Pre-fetching is the dedicated `fetch` target.
ZIG_FETCH_FLAGS ?=

NAME := rollup_guest
BIN  := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

# Sample fixture input used by exec/debug — the checked-in golden rollup-input vector.
INPUT ?= $(MAKEFILE_DIR)test/testdata/10-14-getZkRollupProofV1.request.ssz

# Flags forwarded to `zkc exec` by exec. Quiet (no per-instruction printf) is zkc's default unless
# -v/-vv/-vvv raise verbosity.
ZKC_EXEC_FLAGS ?=

# If true, generates an objdump of the generated ELF for debugging purposes.
OBJDUMP ?= false

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)
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
