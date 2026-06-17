# ZKC toolchain for RISC-V arithmetization.
# Usage from lineth-monorepo root:
#   make -C arithmetization check
# Or from this directory:
#   make check

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
REPO_ROOT := $(abspath $(MAKEFILE_DIR)/..)

# Used to install zkc from repository (while zkc is not released yet).
ZKC ?= $(REPO_ROOT)/zkc
ZKC_REPO ?= https://github.com/LFDT-Lineth/zkc.git
# Optional: pin a commit or branch (e.g. 763f30878e5fc260f0d88863c9883c904f8fc056). Leave empty for default branch.
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

.PHONY: checkout-zkc-repo install-zkc install-sail zkc-compile zkc-check zkc-compile-and-check check

# Fetch zkc repository if missing (no-op when CI already checked it out into ZKC).
checkout-zkc-repo:
	@if [ -d "$(ZKC)/.git" ]; then \
		echo "zkc already present at $(ZKC)"; \
	else \
		echo "Cloning $(ZKC_REPO) -> $(ZKC)"; \
		git clone "$(ZKC_REPO)" "$(ZKC)"; \
	fi
	@if [ -n "$(ZKC_REF)" ]; then \
		git -C "$(ZKC)" fetch origin "$(ZKC_REF)" || true; \
		git -C "$(ZKC)" checkout "$(ZKC_REF)"; \
		git -C "$(ZKC)" pull --ff-only origin "$(ZKC_REF)" 2>/dev/null || true; \
	fi

# Install the zkc CLI from the repository.
# - When ZKC_REF is set: always checkout the pinned ref and reinstall.
# - When ZKC_REF is empty: skip if zkc is already on PATH.
# To update zkc version with latest, make install-zkc ZKC_REF=main
install-zkc:
	@set -eu; \
	if [ -z "$(ZKC_REF)" ] && command -v zkc >/dev/null 2>&1; then \
		exit 0; \
	fi; \
	$(MAKE) checkout-zkc-repo; \
	echo "Installing zkc from $(ZKC)..."; \
	cd "$(ZKC)" && go install ./cmd/zkc

# Compile zkc program in a binary file
zkc-compile: $(ZKC_FILES)
	mkdir -p $(ZKC_BINDIR)
	zkc compile -o $(ZKC_BINDIR)/$(ZKC_BINARY) --wir $(ZKC_MAIN)

# Check execution of the compiled zkc binary against an input JSON
# Only works for small zkc programs for now
zkc-check:
	zkc exec --check $(ZKC_INPUT) $(ZKC_BINDIR)/$(ZKC_BINARY)

zkc-compile-and-check: install-zkc zkc-compile zkc-check

zkc-lint:
	zkc format --check $(ZKC_FILES)

zkc-lint-apply:
	zkc format $(ZKC_FILES)

# Compile the main RISC-V zkc program in a binary file
riscv-check-compilation: install-zkc zkc-compile

riscv-check-lint: install-zkc zkc-lint

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
