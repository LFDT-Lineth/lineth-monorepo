# lineth_recursion package — self-contained lifecycle.
#
# This folder is its own Zig package (build.zig + build.zig.zon). The riscv-guests/ top-level
# Makefile fans `compile`/`test`/`fetch`/`clean` out to this file. The package is a library that
# the guests build for their target; `test` runs its native tests, which build and run
# verifier-ray's devmode prover with `go`.

.PHONY: fetch test clean compile devmode-system

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

ZIG ?= zig

BUILD_CACHE_DIR ?= $(MAKEFILE_DIR).zig-cache
ZIG_BUILD_FLAGS ?= --cache-dir $(BUILD_CACHE_DIR) --summary all
# The native tests: they build the devmode prover.
ZIG_HOST_FLAGS ?= -Dhost-tools=true
ZIG_FETCH_FLAGS ?=

# The system of the recursive verifier: verifier-ray's devmode system, a Go-generated source the
# build imports.
VERIFIER_RAY_DIR := $(MAKEFILE_DIR)../../verifier-ray
DEVMODE_SYSTEM := $(VERIFIER_RAY_DIR)/testdata/generated/devmode_system.zig
DEVMODE_SYSTEM_INPUTS := $(filter-out %_test.go,$(wildcard $(VERIFIER_RAY_DIR)/codegen/*.go $(VERIFIER_RAY_DIR)/codegen/generate-devmode-system/*.go)) \
	$(VERIFIER_RAY_DIR)/go.mod $(VERIFIER_RAY_DIR)/go.sum

$(DEVMODE_SYSTEM): $(DEVMODE_SYSTEM_INPUTS)
	$(MAKE) -C $(VERIFIER_RAY_DIR) generate-devmode-system ZIG="$(ZIG)"

# Generates the devmode system when it is missing or older than its generator, the `codegen`
# package the generator calls or verifier-ray's `go.mod` and `go.sum`.
devmode-system: $(DEVMODE_SYSTEM)

fetch: devmode-system
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) --fetch=all

test: devmode-system
	$(ZIG) build test $(ZIG_BUILD_FLAGS) $(ZIG_HOST_FLAGS) $(ZIG_FETCH_FLAGS)

clean:
	rm -rf $(MAKEFILE_DIR)zig-out/
	rm -rf $(BUILD_CACHE_DIR)

# Dependency check of the package build; the fan-out calls this uniformly on every GUESTS entry.
compile: devmode-system
	$(ZIG) build $(ZIG_BUILD_FLAGS) $(ZIG_FETCH_FLAGS)
