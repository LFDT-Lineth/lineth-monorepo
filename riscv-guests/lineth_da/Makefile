# lineth_da package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library;
# `compile` links its freestanding rv64im smoke guest, the proof that the package module and the C
# library behind it build and link without a C runtime. `exec`/`debug` run that smoke guest under
# the ZKC interpreter: it decompresses INPUT, a zstd frame compressed against the package
# dictionary, and publishes the decompressed bytes.

.PHONY: fetch test clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines the ZKC run (elf-to-json and zkc exec).
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# NOTE: `zig build --fetch` fetches the dependency tree and exits without building or running, so
# pre-fetching is the dedicated `fetch` target alone.
ZIG_FETCH_FLAGS ?=

NAME := zstd_smoke
BIN  := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

# Default exec/debug input: SMOKE_PAYLOAD compressed by the compress_frame tool (named `.ssz`, so the
# ELF->JSON conversion frames it with the 8-byte length prefix `read_input` expects). Pass your own
# INPUT=path/to/frame.ssz to use something else instead.
SMOKE_PAYLOAD   ?= $(MAKEFILE_DIR)README.md
GENERATED_INPUT := $(MAKEFILE_DIR)zig-out/sample.frame.ssz
INPUT           ?= $(GENERATED_INPUT)

# Flags forwarded to `zkc exec` by exec.
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

# Regenerates the default INPUT from SMOKE_PAYLOAD (a no-op for a user-supplied INPUT= override),
# then fails early on a still-missing/empty INPUT instead of erroring in the converter.
require-input: compile
	@if [ "$(INPUT)" = "$(GENERATED_INPUT)" ]; then \
		$(MAKEFILE_DIR)zig-out/bin/compress_frame "$(SMOKE_PAYLOAD)" "$(GENERATED_INPUT)"; \
	fi
	@[ -n "$(INPUT)" ] && [ -s "$(INPUT)" ] || { echo "INPUT missing or empty: '$(INPUT)' (set INPUT=path/to/frame.ssz)"; exit 1; }

exec: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

debug: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"
