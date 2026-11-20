# lineth_da package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library that
# the guests build for their target; `test` runs its native unit and reference tests and builds and
# checks the shared library of its host layer.

.PHONY: fetch fetch-host test clean compile host-lib host-lib-check

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

test: fetch-host fetch-zkevm-fixtures host-lib host-lib-check
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FIXTURES_FLAGS) $(ZIG_FETCH_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

# Dependency check of the package build; the fan-out calls this uniformly on every GUESTS entry.
compile:
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)

# The shared library of the host layer, for the machine that builds it: zig-out/lib/liblineth_da.so
# (.dylib on macOS) and zig-out/include/lineth_da.h.
host-lib: fetch-host
	$(ZIG) build host-lib $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS)

# The newest glibc the Linux library may require.
HOST_LIB_GLIBC_MAX := 2.39
HOST_LIB_CHECK_DEPS := $(if $(filter Linux,$(shell uname -s)),host-lib)

# The Linux library exports the lineth_da_* functions alone and runs on the glibc of the oldest
# system that loads it; other systems have nothing to check.
host-lib-check: $(HOST_LIB_CHECK_DEPS)
	@set -eu; \
	if [ "$$(uname -s)" != "Linux" ]; then echo "host-lib-check: skipped on $$(uname -s), it checks the Linux library"; exit 0; fi; \
	lib="$(MAKEFILE_DIR)zig-out/lib/liblineth_da.so"; \
	foreign=$$(nm -D --defined-only "$$lib" | awk '{ print $$3 }' | grep -v '^lineth_da_' || true); \
	if [ -n "$$foreign" ]; then echo "host-lib-check: exported symbols outside lineth_da_*:" >&2; echo "$$foreign" >&2; exit 1; fi; \
	newest=$$(objdump -T "$$lib" | grep -o 'GLIBC_[0-9][0-9.]*' | sed 's/^GLIBC_//' | sort -V | tail -n 1); \
	if [ -n "$$newest" ] && [ "$$(printf '%s\n%s\n' "$$newest" "$(HOST_LIB_GLIBC_MAX)" | sort -V | tail -n 1)" != "$(HOST_LIB_GLIBC_MAX)" ]; then \
		echo "host-lib-check: the library needs GLIBC_$$newest, newer than GLIBC_$(HOST_LIB_GLIBC_MAX)" >&2; exit 1; \
	fi; \
	echo "host-lib-check: lineth_da_* exports only, newest glibc symbol $${newest:-none}"
