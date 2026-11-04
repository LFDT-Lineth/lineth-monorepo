# lineth_da package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library that
# the guests build for their target; `test` runs its native unit and reference tests.

.PHONY: fetch fetch-host test clean compile

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# The native tests: they fetch the reference vectors and build the host crypto providers.
ZIG_HOST_FLAGS ?= -Dhost-tools=true
ZIG_FIXTURES_FLAGS ?= -Dzkevm-fixtures="$(ZKEVM_FIXTURES_DIR)"
# NOTE: `zig build --fetch` fetches the dependency tree and exits without building or running, so
# pre-fetching is the dedicated `fetch` and `fetch-host` targets alone; `fetch-host` adds the lazy
# dependencies of the native tests.
ZIG_FETCH_FLAGS ?=

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

# Dependency check of the package build; the fan-out calls this uniformly on every GUESTS entry.
compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)
