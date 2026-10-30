# lineth_da package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library;
# `compile` links its freestanding rv64im smoke guests, the proof that the package module and the
# crypto and C libraries behind it build and link on the guest target. `exec`/`debug` run the smoke
# guest SMOKE under the ZKC interpreter:
#   zstd        decompresses INPUT, a zstd frame compressed against the package dictionary, and
#               publishes the decompressed bytes (a default INPUT is generated);
#   blob        verifies the EIP-4844 proof of INPUT (48-byte commitment, 48-byte proof, 131072-byte
#               blob) and publishes the versioned hash;
#   truncation  truncates INPUT (the chain ID as 8 big-endian bytes, then a full block's RLP) and
#               publishes the block hash, the parent hash and the canonical payload.

.PHONY: fetch test clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines the ZKC run (elf-to-json and zkc exec).
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# The native tests and tools: they fetch the reference vectors and build the host crypto providers.
ZIG_HOST_FLAGS ?= -Dhost-tools=true
# NOTE: `zig build --fetch=all` fetches the dependency tree, lazy dependencies included, and exits
# without building or running, so pre-fetching is the dedicated `fetch` target alone.
ZIG_FETCH_FLAGS ?=

SMOKES := zstd blob truncation
SMOKE  ?= zstd
NAME   := $(SMOKE)_smoke
BIN    := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

# Default exec/debug input of the zstd smoke: SMOKE_PAYLOAD compressed by the compress_frame tool
# (named `.ssz`, so the ELF->JSON conversion frames it with the 8-byte length prefix `read_input`
# expects). Pass INPUT=path/to/input.ssz to use something else; the other smokes need it.
SMOKE_PAYLOAD   ?= $(MAKEFILE_DIR)README.md
GENERATED_INPUT := $(MAKEFILE_DIR)zig-out/sample.frame.ssz
INPUT           ?= $(if $(filter zstd,$(SMOKE)),$(GENERATED_INPUT))

# Flags forwarded to `zkc exec` by exec.
ZKC_EXEC_FLAGS ?=

# If true, generates an objdump of the generated ELF for debugging purposes.
OBJDUMP ?= false

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) --fetch=all

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

compile:
	@for smoke in $(SMOKES); do \
		echo "==> zig build -Dsmoke=$$smoke"; \
		$(ZIG) build -Dsmoke=$$smoke $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) || exit $$?; \
	done
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
		$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS) && \
		$(MAKEFILE_DIR)zig-out/bin/compress_frame "$(SMOKE_PAYLOAD)" "$(GENERATED_INPUT)"; \
	fi
	@[ -n "$(INPUT)" ] && [ -s "$(INPUT)" ] || { echo "INPUT missing or empty: '$(INPUT)' (set INPUT=path/to/input.ssz)"; exit 1; }

exec: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

debug: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"
