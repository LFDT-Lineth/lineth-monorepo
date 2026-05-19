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
GO_CORSET_VERSION ?= v1.2.17

# Used to prepare host builds for ACT4 tests.
ACT4_REF ?= 9798a554ce4139f472c9ccd3a18c9061d0f7024d
ACT4_REPO ?= $(REPO_ROOT)/../riscv-arch-test
ACT4_RISCV_DIR ?= $(HOME)/riscv
SAIL_RISCV_VERSION ?= 0.11

ZKC_FILES ?= $(shell find $(MAKEFILE_DIR)src/main/riscv/* -name "*.zkc")
ZKC_MAIN ?= $(MAKEFILE_DIR)src/main/riscv/main.zkc
ZKC_BINDIR ?= $(MAKEFILE_DIR)bin
ZKC_BINARY ?= riscv.bin
ZKC_INPUT ?= input.json

.PHONY: checkout-go-corset install-go-corset install-zkc install-act4-host-deps zkc-compile zkc-check zkc-compile-and-check check

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
install-zkc: checkout-go-corset
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

# Install the tools needed to build ACT4 ELFs directly on the host for Linux and macOS
install-act4-host-deps:
	@set -eu; \
	if ! command -v brew >/dev/null 2>&1; then \
		echo "Installing Homebrew..."; \
		/bin/bash -c "$$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"; \
	fi; \
	brew_cmd=$$(command -v brew || true); \
	if [ -z "$$brew_cmd" ] && [ -x /home/linuxbrew/.linuxbrew/bin/brew ]; then brew_cmd=/home/linuxbrew/.linuxbrew/bin/brew; fi; \
	if [ -z "$$brew_cmd" ] && [ -x /opt/homebrew/bin/brew ]; then brew_cmd=/opt/homebrew/bin/brew; fi; \
	if [ -z "$$brew_cmd" ] && [ -x /usr/local/bin/brew ]; then brew_cmd=/usr/local/bin/brew; fi; \
	if [ -z "$$brew_cmd" ]; then echo "error: brew not found"; exit 1; fi; \
	if [ "$$(uname -s)" = "Darwin" ] && ! xcode-select -p >/dev/null 2>&1; then \
		echo "Installing Xcode Command Line Tools..."; \
		xcode-select --install || true; \
		echo "Finish the Xcode Command Line Tools installation, then rerun this target."; \
		exit 1; \
	fi; \
	brew_prefix=$$("$$brew_cmd" --prefix); \
	export PATH="$(ACT4_RISCV_DIR)/bin:$$brew_prefix/bin:$$PATH"; \
	"$$brew_cmd" install mise riscv64-elf-gcc; \
	if [ "$$(uname -s)" = "Darwin" ]; then "$$brew_cmd" install z3; fi; \
	mkdir -p "$(ACT4_RISCV_DIR)/bin"; \
	if ! "$(ACT4_RISCV_DIR)/bin/sail_riscv_sim" --version 2>/dev/null | grep -qx "$(SAIL_RISCV_VERSION)"; then \
		echo "Installing sail_riscv_sim $(SAIL_RISCV_VERSION)..."; \
		sail_os=$$(uname -s); \
		if [ "$$sail_os" = "Darwin" ]; then sail_os=Mac; fi; \
		curl --fail --location "https://github.com/riscv/sail-riscv/releases/download/$(SAIL_RISCV_VERSION)/sail-riscv-$$sail_os-$$(uname -m).tar.gz" \
			| tar xz --directory="$(ACT4_RISCV_DIR)" --strip-components=1; \
	fi; \
	ln -sf "$$brew_prefix/bin/riscv64-elf-gcc" "$(ACT4_RISCV_DIR)/bin/riscv64-unknown-elf-gcc"; \
	ln -sf "$$brew_prefix/bin/riscv64-elf-objdump" "$(ACT4_RISCV_DIR)/bin/riscv64-unknown-elf-objdump"; \
	shell_rc="$$HOME/.bashrc"; \
	if [ "$$(uname -s)" = "Darwin" ]; then shell_rc="$$HOME/.zshrc"; fi; \
	path_line='export PATH="$(ACT4_RISCV_DIR)/bin:$$PATH"'; \
	touch "$$shell_rc"; \
	if ! grep -Fqx "$$path_line" "$$shell_rc"; then echo "$$path_line" >> "$$shell_rc"; fi; \
	if [ ! -d "$(ACT4_REPO)" ]; then git clone https://github.com/riscv/riscv-arch-test.git "$(ACT4_REPO)"; fi; \
	if ! git -C "$(ACT4_REPO)" rev-parse "$(ACT4_REF)^{commit}" >/dev/null 2>&1; then git -C "$(ACT4_REPO)" fetch --tags origin; fi; \
	if [ "$$(git -C "$(ACT4_REPO)" rev-parse HEAD)" != "$$(git -C "$(ACT4_REPO)" rev-parse "$(ACT4_REF)^{commit}")" ]; then \
		git -C "$(ACT4_REPO)" checkout --detach "$(ACT4_REF)"; \
	fi; \
	cd "$(ACT4_REPO)" && mise trust .mise.toml && mise install && mise exec -- uv sync; \
	cd "$(ACT4_REPO)/framework/src/act/data" && BUNDLE_JOBS=1 mise exec -- bundle install; \
	if [ "$$(uname -s)" = "Darwin" ]; then \
		udb_cpu=$$(uname -m); \
		case "$$udb_cpu" in arm64|aarch64) udb_cpu=arm64 ;; x86_64|x64) udb_cpu=x64 ;; *) echo "error: unsupported UDB CPU $$udb_cpu"; exit 1 ;; esac; \
		udb_z3_version=$$(cd "$(ACT4_REPO)/framework/src/act/data" && mise exec -- ruby -e 'require "udb/dep_versions"; print Udb::Z3_VERSION'); \
		udb_z3_dir="$${XDG_CACHE_HOME:-$$HOME/.cache}/udb/z3/$$udb_z3_version/$$udb_cpu"; \
		mkdir -p "$$udb_z3_dir"; \
		for z3_name in libz3.so.4.8 libz3.so libz3 z3; do \
			ln -sf "$$brew_prefix/opt/z3/lib/libz3.dylib" "$$udb_z3_dir/$$z3_name"; \
		done; \
	fi; \
	echo "ACT4 host dependencies are ready"; \
	echo "ACT4 host builds will use $(ACT4_RISCV_DIR)/bin automatically"
