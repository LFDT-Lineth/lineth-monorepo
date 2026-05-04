# ZKC toolchain for RISC-V arithmetization.
# Usage from linea-monorepo root:
#   make -C arithmetization check
# Or from this directory:
#   make check

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
REPO_ROOT := $(abspath $(MAKEFILE_DIR)/..)

# Used to install zkc from go-corset repository (while zkc is not released yet).
GO_CORSET ?= $(REPO_ROOT)/go-corset
GO_CORSET_REPO ?= https://github.com/consensys/go-corset.git
# Optional: pin a commit or branch (e.g. 763f30878e5fc260f0d88863c9883c904f8fc056). Leave empty for default branch.
GO_CORSET_REF_FOR_ZKC ?=

# Used to install go-corset
GO_CORSET_VERSION ?= v1.2.12

ZKC_FILES := $(shell find $(MAKEFILE_DIR)src/main/riscv/* -name "*.zkc")
ZKC_MAIN := $(MAKEFILE_DIR)src/main/riscv/main.zkc

.PHONY: checkout-go-corset install-go-corset install-zkc zkc-compile check

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
	fi

# Install the zkc CLI from the go-corset repository.
install-zkc: checkout-go-corset
	cd "$(GO_CORSET)" && go install ./cmd/zkc

# Compile the main RISC-V ZKC program.
zkc-compile: $(ZKC_FILES)
	zkc compile $(ZKC_MAIN)

zkc-lint:
	zkc format --check $(ZKC_FILES)

zkc-lint-apply:
	zkc format $(ZKC_FILES)

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
