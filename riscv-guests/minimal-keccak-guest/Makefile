# minimal-keccak-guest — self-contained lifecycle.
#
# Purpose-built witness for verifier-ray/codegen's main.zkc bootstrap fixture (see src/main.zig
# for the full rationale): calls keccak twice and write_output once, small enough to stay well
# under main.zkc's current static column caps while clearing the dynamic-module minimum-safe-size
# floor those two opcodes' modules would otherwise fail to reach.
#
# Not yet registered in the top-level riscv-guests/Makefile's GUESTS list — this is bootstrap
# tooling for one codegen driver, not a production zkVM deliverable like l2-execution.

.PHONY: fetch test clean compile

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all

NAME := minimal_keccak_guest
BIN  := $(MAKEFILE_DIR)zig-out/bin/$(NAME)

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

# No host-only tests: this guest has no native (non-riscv64) logic to unit-test.
test:
	@true

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS)
