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

.PHONY: compile test fetch clean fmt fmt-check help install-constantine-deps release-check release-asset

# Each common target fans out to every guest's own Makefile.
compile test fetch clean:
	@for g in $(GUESTS); do \
		echo "==> make $@ ($$g)"; \
		$(MAKE) -C $$g $@ ZIG="$(ZIG)" || exit $$?; \
	done

# Each guest stages its own ELF using the shared Zig build step.
RELEASE_DIR ?= $(CURDIR)/release-assets
release-check:
	$(MAKE) -C $(GUEST) test ZIG="$(ZIG)"
	$(MAKE) -C $(GUEST) compile ZIG="$(ZIG)"

release-asset:
	$(MAKE) -C $(GUEST) release-asset RELEASE_DIR="$(abspath $(RELEASE_DIR))" ZIG="$(ZIG)" RELEASE_VERSION="$(RELEASE_VERSION)"

NIM_VERSION := 2.2.10
CHOOSENIM_VERSION := 0.8.16
CHOOSENIM_LINUX_X64_SHA256 := 1034e03564b3e5b5321646f9ce290cc4d3cdff0cf0e9b523d43f0db8b9a45c74

install-native-crypto-deps:
	@set -eu; \
	uname_s=$$(uname -s); \
	if [ "$$uname_s" = "Darwin" ]; then \
		command -v brew >/dev/null 2>&1 || { echo "Homebrew not found: https://brew.sh"; exit 1; }; \
		brew install secp256k1 openssl || true; \
		prefix=$$(brew --prefix); \
		sudo=""; \
	else \
		sudo=$$(command -v sudo >/dev/null 2>&1 && echo sudo || echo ""); \
		$$sudo apt-get update -qq; \
		$$sudo apt-get install -y --no-install-recommends libsecp256k1-dev libssl-dev build-essential git; \
		prefix=/usr/local; \
	fi; \
	if [ -f "$$prefix/lib/libblst.a" ]; then \
		echo "blst already installed at $$prefix/lib/libblst.a"; \
	else \
		echo "Building blst from source ($(BLST_SHA))..."; \
		rm -rf /tmp/blst-src; \
		git clone https://github.com/supranational/blst.git /tmp/blst-src; \
		git -C /tmp/blst-src checkout --detach $(BLST_SHA); \
		(cd /tmp/blst-src && ./build.sh); \
		$$sudo mkdir -p "$$prefix/lib" "$$prefix/include"; \
		$$sudo cp /tmp/blst-src/libblst.a "$$prefix/lib/"; \
		$$sudo cp /tmp/blst-src/bindings/blst.h /tmp/blst-src/bindings/blst_aux.h "$$prefix/include/"; \
	fi; \
	if { [ -f "$$prefix/lib/libmcl.so" ] || [ -f "$$prefix/lib/libmcl.dylib" ]; } && { [ "$$uname_s" != "Darwin" ] || [ -f "$$prefix/lib/libmcl.a" ]; }; then \
		echo "mcl already installed under $$prefix/lib"; \
	else \
		echo "Building mcl from source ($(MCL_SHA))..."; \
		rm -rf /tmp/mcl-src; \
		git clone https://github.com/herumi/mcl.git /tmp/mcl-src; \
		git -C /tmp/mcl-src checkout --detach $(MCL_SHA); \
		$(MAKE) -C /tmp/mcl-src; \
		$$sudo mkdir -p "$$prefix/lib" "$$prefix/include"; \
		if [ "$$uname_s" = "Darwin" ]; then \
			$$sudo cp /tmp/mcl-src/lib/libmcl.dylib "$$prefix/lib/" 2>/dev/null || true; \
			$$sudo cp /tmp/mcl-src/lib/libmcl.a "$$prefix/lib/" 2>/dev/null || true; \
		else \
			$$sudo rm -f "$$prefix/lib/libmcl.a"; \
			$$sudo cp /tmp/mcl-src/lib/libmcl.so "$$prefix/lib/"; \
			$$sudo ldconfig; \
		fi; \
		$$sudo cp -r /tmp/mcl-src/include/mcl "$$prefix/include/"; \
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
	@echo "  make install-native-crypto-deps  # install blst/mcl for native host-tool builds"
	@echo ""
