# ZKC toolchain for RISC-V arithmetization.
# Usage from lineth-monorepo root:
#   make -C arithmetization check
# Or from this directory:
#   make check

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
REPO_ROOT := $(abspath $(MAKEFILE_DIR)/..)

# Used to install zkc from go-corset repository (while zkc is not released yet).
GO_CORSET ?= $(REPO_ROOT)/go-corset
GO_CORSET_REPO ?= https://github.com/LFDT-Lineth/zkc.git
# Optional: pin a commit or branch (e.g. 763f30878e5fc260f0d88863c9883c904f8fc056). Leave empty for default branch.
GO_CORSET_REF_FOR_ZKC ?=414e7b73971eaa77afcb6a4797b44d0fdf8c99db

# Used to install go-corset
GO_CORSET_VERSION ?= v1.2.17

# Used to install Sail for ACT4 host builds.
ACT4_RISCV_DIR ?= $(HOME)/riscv
SAIL_RISCV_VERSION ?= 0.11

ZKC_FILES ?= $(shell find $(MAKEFILE_DIR)src/main/riscv/* -name "*.zkc")
ZKC_MAIN ?= $(MAKEFILE_DIR)src/main/riscv/main.zkc
ZKC_BINDIR ?= $(MAKEFILE_DIR)bin
ZKC_BINARY ?= riscv.bin
ZKC_INPUT ?= input.json

.PHONY: checkout-go-corset install-go-corset install-zkc install-sail zkc-compile zkc-check zkc-compile-and-check check

# Fetch go-corset if missing (no-op when CI already checked it out into GO_CORSET).
checkout-go-corset:
	@if [ -d "$(GO_CORSET)/.git" ]; then \
		echo "go-corset already present at $(GO_CORSET)"; \
	else \
		echo "Cloning $(GO_CORSET_REPO) -> $(GO_CORSET)"; \
		git clone "$(GO_CORSET_REPO)" "$(GO_CORSET)"; \
	fi
	@if [ -n "$(GO_CORSET_REF_FOR_ZKC)" ]; then \
		git -C "$(GO_CORSET)" fetch origin "$(GO_CORSET_REF_FOR_ZKC)" || true; \
		git -C "$(GO_CORSET)" checkout "$(GO_CORSET_REF_FOR_ZKC)"; \
		git -C "$(GO_CORSET)" pull --ff-only origin "$(GO_CORSET_REF_FOR_ZKC)" 2>/dev/null || true; \
	fi

# Install the zkc CLI from the go-corset repository.
# - When GO_CORSET_REF_FOR_ZKC is set: always checkout the pinned ref and reinstall.
# - When GO_CORSET_REF_FOR_ZKC is empty: skip if zkc is already on PATH.
# To update zkc version with latest, make install-zkc GO_CORSET_REF_FOR_ZKC=main
install-zkc:
	@set -eu; \
	if [ -z "$(GO_CORSET_REF_FOR_ZKC)" ] && command -v zkc >/dev/null 2>&1; then \
		exit 0; \
	fi; \
	$(MAKE) checkout-go-corset; \
	echo "Installing zkc from $(GO_CORSET)..."; \
	cd "$(GO_CORSET)" && go install ./cmd/zkc

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

# Install the go-corset CLI from a released version (skips if already at the target version).
install-go-corset:
	@if command -v go-corset >/dev/null 2>&1; then \
		installed=$$(go-corset --version | sed 's/go-corset//;s/^ *//'); \
		if [ "$$installed" = "$(GO_CORSET_VERSION)" ]; then \
			echo "go-corset $(GO_CORSET_VERSION) already installed"; \
		else \
			echo "go-corset version mismatch: installed=$$installed expected=$(GO_CORSET_VERSION)"; \
			echo "Installing go-corset@$(GO_CORSET_VERSION)..."; \
			go install github.com/consensys/go-corset/cmd/go-corset@$(GO_CORSET_VERSION); \
		fi; \
	else \
		echo "go-corset not found in PATH"; \
		echo "Installing go-corset@$(GO_CORSET_VERSION)..."; \
		go install github.com/consensys/go-corset/cmd/go-corset@$(GO_CORSET_VERSION); \
	fi

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
