# RISC-V guest programs — top-level orchestrator.
#
# Each guest under riscv-guests/ is a self-contained Zig package with its own build.zig,
# build.zig.zon (its dependencies) and Makefile (its compile/test/lint lifecycle). Shared build
# helpers live in build_common/. This Makefile is deliberately guest-agnostic: it just fans the
# common lifecycle out to every guest listed in GUESTS.
#
# Add a guest: create riscv-guests/<name>/ (build.zig + build.zig.zon + Makefile + src) and append
# <name> to GUESTS. Only targets that every guest implements are fanned out here; guest-specific
# targets (e.g. l2-execution's EF spec-test, ZKC exec/debug) live in that guest's own Makefile and
# run directly: `make -C <name> spec-test`.
#
# guest-common isn't a guest program itself (no ELF to link, just a shared Zig package other
# guests consume as a path dependency — the same role lineth-accelerators plays) but is listed
# here anyway so its own unit tests run as part of `make test`, and its build.zig/src/test get the
# same `zig fmt --check` gate every guest's own sources get.

GUESTS ?= guest-common l2-execution rollup
ZIG    ?= zig

# Sources to format: the shared build helper plus each guest's build.zig and hand-written src/ +
# test/ — NOT the whole guest dir, which would walk generated .zig under .zig-cache/ and zig-out/.
# Mirrors the CI `zig fmt --check` invocation (riscv-guests-host-tests.yml) and scales with GUESTS.
FMT_PATHS := build_common/build.zig $(foreach g,$(GUESTS),$(g)/build.zig $(g)/src $(g)/test)

.PHONY: compile test fetch clean fmt fmt-check help install-constantine-deps

# Each common target fans out to every guest's own Makefile.
compile test fetch clean:
	@for g in $(GUESTS); do \
		echo "==> make $@ ($$g)"; \
		$(MAKE) -C $$g $@ ZIG="$(ZIG)" || exit $$?; \
	done

NIM_VERSION := 2.2.10
CHOOSENIM_VERSION := 0.8.16
CHOOSENIM_LINUX_X64_SHA256 := 1034e03564b3e5b5321646f9ce290cc4d3cdff0cf0e9b523d43f0db8b9a45c74

# Toolchain for the freestanding Constantine build: Nim, nimble, and LLVM clang.
install-constantine-deps:
	@set -eu; \
	if [ "$$(uname -s)" = "Darwin" ]; then \
		command -v brew >/dev/null 2>&1 || { echo "Homebrew not found: https://brew.sh"; exit 1; }; \
		brew list nim >/dev/null 2>&1 || brew install nim; \
		brew list llvm >/dev/null 2>&1 || brew install llvm; \
		export PATH="$$(brew --prefix llvm)/bin:$$PATH"; \
	else \
		sudo=$$(command -v sudo >/dev/null 2>&1 && echo sudo || echo ""); \
		$$sudo apt-get update -qq; \
		$$sudo apt-get install -y --no-install-recommends build-essential ca-certificates clang curl git llvm xz-utils; \
		case "$$(uname -m)" in \
			x86_64|amd64) ;; \
			*) echo "unsupported architecture for choosenim: $$(uname -m)" >&2; exit 1 ;; \
		esac; \
		mkdir -p "$$HOME/.local/bin"; \
		tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
		choosenim="choosenim-$(CHOOSENIM_VERSION)_linux_amd64"; \
		curl --fail --location --silent --show-error \
			"https://github.com/nim-lang/choosenim/releases/download/v$(CHOOSENIM_VERSION)/$$choosenim" \
			-o "$$tmp/$$choosenim"; \
		printf '%s  %s\n' "$(CHOOSENIM_LINUX_X64_SHA256)" "$$tmp/$$choosenim" | sha256sum --check --status; \
		install -m 0755 "$$tmp/$$choosenim" "$$HOME/.local/bin/choosenim"; \
		"$$HOME/.local/bin/choosenim" $(NIM_VERSION) -y \
			--choosenimDir:"$$HOME/.choosenim" --nimbleDir:"$$HOME/.local"; \
		export PATH="$$HOME/.local/bin:$$PATH"; \
	fi; \
	command -v nim >/dev/null; command -v nimble >/dev/null; command -v llvm-ar >/dev/null; \
	nim_version=$$(nim --version | sed -n '1s/.*Version \([0-9][0-9.]*\).*/\1/p'); \
	if [ "$$(uname -s)" = "Darwin" ]; then \
		case "$$nim_version" in 2.2.*) ;; *) echo "Nim 2.2.x required, found $$nim_version" >&2; exit 1 ;; esac; \
	elif [ "$$nim_version" != "$(NIM_VERSION)" ]; then \
		echo "Nim $(NIM_VERSION) required, found $$nim_version" >&2; exit 1; \
	fi

# Formatting spans build_common + all guests, so it lives here (not fanned out). `fmt` rewrites in
# place; `fmt-check` only verifies — the same gate CI runs.
fmt:
	$(ZIG) fmt $(FMT_PATHS)

fmt-check:
	$(ZIG) fmt --check $(FMT_PATHS)

help:
	@echo "riscv-guests — builds every guest in GUESTS=\"$(GUESTS)\""
	@echo ""
	@echo "  make compile     # build each guest's statically-linked rv64im ELF"
	@echo "  make test        # run each guest's native host tests"
	@echo "  make fetch       # pre-fetch each guest's Zig dependencies"
	@echo "  make clean       # remove each guest's zig-out/ and .zig-cache/"
	@echo "  make fmt         # format build_common + all guest sources in place (zig fmt)"
	@echo "  make fmt-check   # verify formatting without writing (the CI gate)"
	@echo "  make install-constantine-deps    # install nim/nimble/clang for the guest-crypto-ctt build"
	@echo ""
