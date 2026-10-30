# Shared Zig package consumed by the other riscv-guests/ guests as a path dependency. Implements
# the common compile/test/fetch/clean targets so the top-level Makefile's fan-out (GUESTS) covers
# it uniformly.

.PHONY: fetch test compile clean

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS)

# Dependency check + host-test compilation; the fan-out calls this uniformly on every GUESTS entry.
compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)
