# Not itself a guest program (no ELF to link) — a plain Zig package the riscv-guests/ guests
# consume as a path dependency. Still gets its own Makefile with the common
# compile/test/fetch/clean targets so the top-level Makefile's fan-out (GUESTS) can run its unit
# tests alongside every guest's own.

.PHONY: fetch test compile clean

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all

fetch:
	$(ZIG) build $(ZIG_BUILD_FLAGS) --fetch

test:
	$(ZIG) build test $(ZIG_BUILD_FLAGS)

# No ELF to link here — nothing for the default `zig build` to install. Kept only so the
# top-level fan-out (which calls compile/test/fetch/clean uniformly across GUESTS) has a target.
compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)
