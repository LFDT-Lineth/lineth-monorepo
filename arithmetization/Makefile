# ZKC toolchain for RISC-V arithmetization.
# Usage from lineth-monorepo root:
#   make -C arithmetization check
# Or from this directory:
#   make check

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Use the zkc tool version pinned in go.mod by default.
ZKC ?= go tool zkc
# Optional: override the pinned zkc module version with a commit, branch, or tag.
ZKC_REF ?=

# Used to install Sail for ACT4 host builds.
ACT4_RISCV_DIR ?= $(HOME)/riscv
SAIL_RISCV_VERSION ?= 0.11

ZKC_RISCV_INTERPRETER ?= $(shell find $(MAKEFILE_DIR)src/main/riscv/* -name "*.zkc")
ZKC_ZKVM_LIBRARY ?= $(shell find $(MAKEFILE_DIR)src/main/lib/* -name "*.zkc")
ZKC_FILES ?= $(ZKC_RISCV_INTERPRETER) $(ZKC_ZKVM_LIBRARY)
ZKC_MAIN ?= $(MAKEFILE_DIR)src/main/riscv/main.zkc
ZKC_BINDIR ?= $(MAKEFILE_DIR)bin
ZKC_BINARY ?= riscv.bin
ZKC_INPUT ?= input.json

.PHONY: prepare-zkc install-sail zkc-compile zkc-check zkc-compile-and-check check

# Use go.mod's pinned tool unless ZKC_REF requests a temporary module override.
prepare-zkc:
	@set -eu; \
	if [ -n "$(ZKC_REF)" ]; then \
		echo "Using zkc override $(ZKC_REF)..."; \
		cd "$(MAKEFILE_DIR)" && go get "github.com/LFDT-Lineth/zkc@$(ZKC_REF)"; \
	fi; \
	:

# Compile zkc program in a binary file
zkc-compile: prepare-zkc $(ZKC_FILES)
	mkdir -p $(ZKC_BINDIR)
	$(ZKC) compile -o $(ZKC_BINDIR)/$(ZKC_BINARY) $(ZKC_MAIN)

# Check execution of the compiled zkc binary against an input JSON.
# Only practical for small zkc programs: constraint checking, not tracing, is the expensive half.
# Measured weekly for the l2-execution guest — see arithmetization/src/test/scripts/README-weekly-metrics.md.
zkc-check: prepare-zkc
	$(ZKC) trace --check $(ZKC_INPUT) $(ZKC_BINDIR)/$(ZKC_BINARY)

zkc-compile-and-check: zkc-compile zkc-check

zkc-lint: prepare-zkc
	$(ZKC) format --check $(ZKC_FILES)

zkc-lint-apply: prepare-zkc
	$(ZKC) format $(ZKC_FILES)

# Compile the main RISC-V zkc program in a binary file
riscv-check-compilation: zkc-compile

riscv-check-lint: zkc-lint

# Install Sail for ACT4 host builds. Other host tools are prerequisites.
install-sail:
	@mkdir -p "$(ACT4_RISCV_DIR)/bin" || exit $$?; \
	if ! "$(ACT4_RISCV_DIR)/bin/sail_riscv_sim" --version 2>/dev/null | grep -qx "$(SAIL_RISCV_VERSION)"; then \
		echo "Installing sail_riscv_sim $(SAIL_RISCV_VERSION)..."; \
		sail_os=$$(uname -s); \
		if [ "$$sail_os" = "Darwin" ]; then sail_os=Mac; fi; \
		sail_archive="$$(mktemp)" || exit $$?; \
		curl --fail --location --output "$$sail_archive" "https://github.com/riscv/sail-riscv/releases/download/$(SAIL_RISCV_VERSION)/sail-riscv-$$sail_os-$$(uname -m).tar.gz" || exit $$?; \
		tar xzf "$$sail_archive" -C "$(ACT4_RISCV_DIR)" --strip-components=1 || exit $$?; \
		rm -f "$$sail_archive"; \
	fi; \
	echo "sail_riscv_sim is ready at $(ACT4_RISCV_DIR)/bin/sail_riscv_sim"
