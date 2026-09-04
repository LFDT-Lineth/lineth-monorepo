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

.PHONY: fetch test reference-test prep-execution-specs-json-fixtures fetch-mainnet-fixtures mainnet-test clean compile exec debug require-input

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# The arithmetization test Makefile defines ZKC run (elf-to-json and zkc exec)
ARITHMETIZATION_TESTING_MAKEFILE ?= $(MAKEFILE_DIR)../../arithmetization/src/test/Makefile

ZIG ?= zig
GO  ?= go

# GNU tar (Linux) warns about macOS LIBARCHIVE.xattr PAX extension headers in archives
# created on macOS; --warning=no-unknown-keyword silences them. BSD tar (macOS) handles
# these natively and does not support that flag, so we only pass it on Linux.
TAR_EXTRA_FLAGS := $(if $(filter Linux,$(shell uname -s)),--warning=no-unknown-keyword,)

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

# Parameters for Linea mainnet zkevm fixture archives.
# Archives (JSON) are cached under MAINNET_FIXTURES_ARCHIVE_DIR and only re-downloaded when absent.
# Converted SSZ inputs live under MAINNET_FIXTURES_SSZ_DIR, produced by zesu-convert.
# Default is /var/tmp (persists across reboots, on the real disk — both archives are ~1.5 GB each,
# which exceeds the typical 3–4 GB tmpfs on /tmp). Override MAINNET_FIXTURES_DIR to change location.
MAINNET_FIXTURES_DIR              ?= /var/tmp/zkevm-fixtures-mainnet
MAINNET_FIXTURES_ARCHIVE_DIR      := $(MAINNET_FIXTURES_DIR)/archives
MAINNET_FIXTURES_JSON_DIR         := $(MAINNET_FIXTURES_DIR)/json
MAINNET_FIXTURES_SSZ_DIR          := $(MAINNET_FIXTURES_DIR)/ssz
MAINNET_FIXTURES_EXTRACTED        := $(MAINNET_FIXTURES_SSZ_DIR)/.extracted
MAINNET_FIXTURES_CONVERTER_REPO   := https://github.com/Gabriel-Trintinalia/stateless-executor
MAINNET_FIXTURES_CONVERTER_DIR    := $(MAINNET_FIXTURES_DIR)/stateless-executor
MAINNET_FIXTURES_CONVERTER_BIN    := $(MAINNET_FIXTURES_CONVERTER_DIR)/bin/zesu-convert
MAINNET_FIXTURES_URLS             := \
	https://github.com/LFDT-Lineth/lineth-monorepo/releases/download/zkevm-fixtures/zkevm-fixtures-input-mainnet-500-Q12026-part-1.tar.zst \
	https://github.com/LFDT-Lineth/lineth-monorepo/releases/download/zkevm-fixtures/zkevm-fixtures-input-mainnet-500-Q12026-part-2.tar.zst

# Extra args forwarded to mainnet-runner, e.g. --limit 10 or -x.
MAINNET_ARGS ?=

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

# Re-uses logic from build.zig to download execution-spec JSON fixtures under $(EXECUTION_SPECS_FIXTURES_DIR).
prep-execution-specs-json-fixtures:
	$(ZIG) build prep-execution-specs-json-fixtures $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -Dexecution-specs-fixtures-link="$(EXECUTION_SPECS_FIXTURES_DIR)"

# Downloads archives → extracts JSON under MAINNET_FIXTURES_JSON_DIR → clones+builds zesu-convert
# → converts rpc_block_*.json to SSZ under MAINNET_FIXTURES_SSZ_DIR. Archives are cached so only the
# first run downloads. Delete $(MAINNET_FIXTURES_EXTRACTED) (or the whole SSZ_DIR) to re-convert.
fetch-mainnet-fixtures: $(MAINNET_FIXTURES_EXTRACTED)

$(MAINNET_FIXTURES_EXTRACTED):
	@set -eu; \
	mkdir -p "$(MAINNET_FIXTURES_ARCHIVE_DIR)"; \
	rm -rf "$(MAINNET_FIXTURES_JSON_DIR)"; \
	mkdir -p "$(MAINNET_FIXTURES_JSON_DIR)"; \
	for url in $(MAINNET_FIXTURES_URLS); do \
		archive="$(MAINNET_FIXTURES_ARCHIVE_DIR)/$$(basename "$$url")"; \
		if [ ! -f "$$archive" ]; then \
			echo "downloading $$url"; \
			rm -f "$$archive.tmp"; \
			curl -L --fail -o "$$archive.tmp" "$$url"; \
			mv "$$archive.tmp" "$$archive"; \
		fi; \
		echo "extracting $$archive"; \
		tar --zstd $(TAR_EXTRA_FLAGS) -xf "$$archive" -C "$(MAINNET_FIXTURES_JSON_DIR)"; \
	done; \
	if [ ! -d "$(MAINNET_FIXTURES_CONVERTER_DIR)/.git" ]; then \
		rm -rf "$(MAINNET_FIXTURES_CONVERTER_DIR)"; \
		git clone "$(MAINNET_FIXTURES_CONVERTER_REPO)" "$(MAINNET_FIXTURES_CONVERTER_DIR)"; \
	fi; \
	mkdir -p "$(MAINNET_FIXTURES_DIR)/go-tmp"; \
	cd "$(MAINNET_FIXTURES_CONVERTER_DIR)" && TMPDIR="$(MAINNET_FIXTURES_DIR)/go-tmp" $(GO) build -o bin/zesu-convert ./cmd/zesu-convert; \
	rm -rf "$(MAINNET_FIXTURES_SSZ_DIR)"; \
	mkdir -p "$(MAINNET_FIXTURES_SSZ_DIR)"; \
	find "$(MAINNET_FIXTURES_JSON_DIR)" -name "rpc_block_*.json" | while read json; do \
		stem=$$(basename "$$json" .json); \
		subdir=$$(dirname "$$json"); \
		rel="$${subdir#$(MAINNET_FIXTURES_JSON_DIR)/}"; \
		mkdir -p "$(MAINNET_FIXTURES_SSZ_DIR)/$$rel"; \
		"$(MAINNET_FIXTURES_CONVERTER_BIN)" "$$json" "$(MAINNET_FIXTURES_SSZ_DIR)/$$rel/$$stem.ssz"; \
	done; \
	touch "$@"

# Runs Linea mainnet zkevm SSZ fixtures through the extended l2-execution guest on the host.
# Downloads the fixture archives first if not already cached. Same approach as reference-test:
# the guest runs as native code — no riscv64 compilation and no ZkC required.
# Narrow/triage with MAINNET_ARGS, e.g. `make mainnet-test MAINNET_ARGS="--limit 10"`.
mainnet-test: fetch-mainnet-fixtures
	$(ZIG) build mainnet-test $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS) -- --fixtures "$(MAINNET_FIXTURES_SSZ_DIR)" $(MAINNET_ARGS)

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
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) elf-debug BIN_EXT="$(BIN)" IN_BYTES="@$(INPUT)" ZKC_EXEC_FLAGS="$(ZKC_EXEC_FLAGS)"
