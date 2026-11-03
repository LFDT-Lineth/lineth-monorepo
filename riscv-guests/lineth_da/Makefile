# lineth_da package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library;
# `compile` links its freestanding rv64im smoke guests, the proof that the package module and the
# crypto and C libraries behind it build and link on the guest target. `exec`/`debug` run the smoke
# guest SMOKE (zstd, blob, truncation or decompress) under the ZKC interpreter. The default INPUT is
# generated for zstd, blob and decompress; truncation needs INPUT=path/to/input.ssz.

.PHONY: fetch fetch-host test clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines the ZKC run (elf-to-json and zkc exec).
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# The native tests and tools: they fetch the reference vectors and build the host crypto providers.
ZIG_HOST_FLAGS ?= -Dhost-tools=true
ZIG_FIXTURES_FLAGS = -Dzkevm-fixtures="$(ZKEVM_FIXTURES_DIR)"
# NOTE: `zig build --fetch` fetches the dependency tree and exits without building or running, so
# pre-fetching is the dedicated `fetch` and `fetch-host` targets alone; `fetch-host` adds the lazy
# dependencies of the native tests and tools.
ZIG_FETCH_FLAGS ?=

SMOKES := zstd blob truncation decompress
SMOKE  ?= zstd
NAME   := $(SMOKE)_smoke
BIN    := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

# Default exec/debug input of the zstd, blob and decompress smokes, generated into a `.ssz` file (so
# the ELF->JSON conversion frames it with the 8-byte length prefix `read_input` expects):
#   zstd        the canonical payload of a fixed sample conflation, compressed by compress_frame;
#   blob        a case of the official EIP-4844 reference vectors, written by blob_sample;
#   decompress  the same payload published as a one-blob DA stream by da_sample.
# SMOKE_PAYLOAD=path/to/file uses the bytes of a file as the payload instead. Pass
# INPUT=path/to/input.ssz to use something else; the truncation smoke needs it.
SMOKE_PAYLOAD   ?=
GENERATED_INPUT := $(MAKEFILE_DIR)zig-out/sample.$(SMOKE).ssz
GENERATOR       := $(strip $(if $(filter decompress,$(SMOKE)),da_sample,$(if $(filter blob,$(SMOKE)),blob_sample,compress_frame)))
GENERATOR_ARGS  := $(if $(filter decompress,$(SMOKE)),blob )$(if $(filter zstd decompress,$(SMOKE)),$(if $(SMOKE_PAYLOAD),"$(SMOKE_PAYLOAD)"))
INPUT           ?= $(if $(filter zstd blob decompress,$(SMOKE)),$(GENERATED_INPUT))

# Flags forwarded to `zkc exec` by exec.
ZKC_EXEC_FLAGS ?=

# If true, generates an objdump of the generated ELF for debugging purposes.
OBJDUMP ?= false

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

fetch-host:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) --fetch=all

include $(MAKEFILE_DIR)../build_common/zkevm_fixtures.mk

test: fetch-host fetch-zkevm-fixtures
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FIXTURES_FLAGS) $(ZIG_FETCH_FLAGS)

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

# Regenerates the default INPUT (a no-op for a user-supplied INPUT= override), then fails early on a
# still-missing/empty INPUT instead of erroring in the converter.
require-input: compile
	@if [ "$(INPUT)" = "$(GENERATED_INPUT)" ]; then \
		$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS) && \
		$(MAKEFILE_DIR)zig-out/bin/$(GENERATOR) "$(GENERATED_INPUT)" $(GENERATOR_ARGS); \
	fi
	@[ -n "$(INPUT)" ] && [ -s "$(INPUT)" ] || { echo "INPUT missing or empty: '$(INPUT)' (set INPUT=path/to/input.ssz)"; exit 1; }

exec: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-exec BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"

debug: require-input
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)"
