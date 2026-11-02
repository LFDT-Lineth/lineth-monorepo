# rollup guest — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file for every guest.
#
# Scope split: building + host-testing is proving-system-AGNOSTIC and lives here (`compile` links
# the statically-linked rv64im ELF — the zkvm-standards artifact — via build_common). Only the ZKC
# run itself (ELF->JSON via elf_to_json_gen, then zkc) is owned by
# arithmetization/src/test/Makefile; `exec`/`debug` build that ELF and hand it over.

.PHONY: fetch test reference-test clean compile exec debug require-input zkc-smoke-exec zkc-smoke-external

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines the ZKC run (elf-to-json and zkc exec).
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# The native tests and tools: they fetch the DA test dependencies and build the host crypto providers.
ZIG_HOST_FLAGS ?= -Dhost-tools=true
# The reference test reads the blocks of the EF zkevm fixtures, a download of 3.4 GB.
ZIG_REFERENCE_FLAGS ?= -Dreference-tests=true
# NOTE: `zig build --fetch` fetches the dependency tree and EXITS without building or running, so
# it must NOT be passed to build/test targets. Pre-fetching is the dedicated `fetch` target; a build
# fetches the lazy dependencies of the steps it runs on demand.
ZIG_FETCH_FLAGS ?=

NAME := rollup_guest
BIN  := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

include $(MAKEFILE_DIR)../build_common/release.mk

# Sample fixture input used by exec/debug and the zkc smoke. The default is generated fresh by
# `require-input` via `zig build gen-fixture` (this package's own encodeInput over a valid request
# of one conflation in one blob), together with the output the native statement computes for it,
# so both always match the current wire format and statement. Pass your own INPUT=path/to/file to
# use something else instead.
GENERATED_INPUT  := $(MAKEFILE_DIR)zig-out/sample.request.ssz
GENERATED_OUTPUT := $(MAKEFILE_DIR)zig-out/sample.output.bin
INPUT ?= $(GENERATED_INPUT)

# The output the guest has to print for INPUT, and the guest ELF the zkc smoke runs.
EXPECTED_OUTPUT ?= $(GENERATED_OUTPUT)
ELF ?= $(BIN)

# Flags forwarded to `zkc exec` by exec. Quiet (no per-instruction printf) is zkc's default unless
# -v/-vv/-vvv raise verbosity.
ZKC_EXEC_FLAGS ?=

# If true, generates an objdump of the generated ELF for debugging purposes.
OBJDUMP ?= false

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_REFERENCE_FLAGS) --fetch=all

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS)

# The rollup guest statement over valid requests built from consecutive real blocks of the EF zkevm
# corpus, in calldata and blob chunks, and over tampered copies of them.
reference-test:
	$(ZIG) build reference-test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_REFERENCE_FLAGS) $(ZIG_FETCH_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) $(if $(KECCAK_ACCEL),-Dkeccak-accel=$(KECCAK_ACCEL)) $(if $(SECP256K1_BACKEND),-Dsecp256k1-backend=$(SECP256K1_BACKEND))
	@if command -v riscv64-unknown-elf-objdump >/dev/null 2>&1; then \
		if [ "$(OBJDUMP)" = "true" ]; then \
			echo "\nDisassembling $(BIN)"; \
			LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(BIN) > $(BIN).objdump; \
		fi \
	else \
		echo "\nSkipping objdump: riscv64-unknown-elf-objdump is not on PATH"; \
	fi

# Regenerates the default INPUT and its expected output via `zig build gen-fixture` (a no-op for a
# user-supplied INPUT= override), then fails early on a still-missing/empty INPUT instead of
# building then erroring in the converter.
require-input:
	@if [ "$(INPUT)" = "$(GENERATED_INPUT)" ]; then \
		mkdir -p $(dir $(GENERATED_INPUT)); \
		$(ZIG) build gen-fixture $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) -- $(GENERATED_INPUT) $(GENERATED_OUTPUT); \
	fi
	@[ -n "$(INPUT)" ] && [ -s "$(INPUT)" ] || { echo "INPUT missing or empty: '$(INPUT)' (set INPUT=path/to/input.ssz)"; exit 1; }

exec: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

debug: require-input compile
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"

# Compile the guest, then run it under zkc on INPUT and compare its output with the native one.
zkc-smoke-exec: require-input compile
	$(MAKE) zkc-smoke-external ELF="$(BIN)" INPUT="$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

# The same for a prebuilt ELF: the guest output printed by zkc has to equal EXPECTED_OUTPUT byte for byte.
zkc-smoke-external:
	@[ -s "$(ELF)" ] || { echo "ELF missing or empty: '$(ELF)' (set ELF=path/to/rollup_guest)"; exit 1; }
	@[ -s "$(INPUT)" ] && [ -s "$(EXPECTED_OUTPUT)" ] || { echo "INPUT or EXPECTED_OUTPUT missing or empty: '$(INPUT)', '$(EXPECTED_OUTPUT)'"; exit 1; }
	$(ZIG) build zkc-smoke $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS) -- \
		--elf "$(abspath $(ELF))" \
		--input "$(abspath $(INPUT))" \
		--expected "$(abspath $(EXPECTED_OUTPUT))" \
		--makefile "$(abspath $(ARITHMETIZATION_TESTING_MAKEFILE))" \
		$(if $(ZKC_EXEC_FLAGS),--zkc-flags "$(ZKC_EXEC_FLAGS)")
