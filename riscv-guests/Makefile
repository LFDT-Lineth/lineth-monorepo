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

GUESTS ?= l2-execution
ZIG    ?= zig

# Sources to format: the shared build helper plus each guest's build.zig and hand-written src/ +
# test/ — NOT the whole guest dir, which would walk generated .zig under .zig-cache/ and zig-out/.
# Mirrors the CI `zig fmt --check` invocation (riscv-guests-host-tests.yml) and scales with GUESTS.
FMT_PATHS := build_common/build.zig $(foreach g,$(GUESTS),$(g)/build.zig $(g)/src $(g)/test)

.PHONY: compile test fetch clean fmt fmt-check help install-native-crypto-deps

# Each common target fans out to every guest's own Makefile.
compile test fetch clean:
	@for g in $(GUESTS); do \
		echo "==> make $@ ($$g)"; \
		$(MAKE) -C $$g $@ ZIG="$(ZIG)" || exit $$?; \
	done

# Native crypto libs (blst, mcl) that zesu's native host backend links — needed to build/run any
# guest's native host tools (unit tests, the wrap/runner CLIs), never for the freestanding riscv64
# guest ELF itself. Not packaged for Ubuntu, so built from pinned upstream tags; idempotent (skips
# already-installed libs), so this is the SAME command whether run in CI or on a dev machine —
# CI's job is only to cache the result, not to own a separate copy of this recipe. Bump the two SHAs
# below when updating (keep in sync with any CI cache key that also names them).
BLST_SHA := e7f90de551e8df682f3cc99067d204d8b90d27ad
MCL_SHA  := 0499298adcfad3bbcebf77f17700ebbe97166060

install-native-crypto-deps:
	@set -eu; \
	uname_s=$$(uname -s); \
	if [ "$$uname_s" = "Darwin" ]; then \
		command -v brew >/dev/null 2>&1 || { echo "Homebrew not found: https://brew.sh"; exit 1; }; \
		brew install secp256k1 openssl || true; \
		prefix=/opt/homebrew; \
		sudo=""; \
	else \
		sudo apt-get update -qq; \
		sudo apt-get install -y --no-install-recommends libsecp256k1-dev libssl-dev build-essential git; \
		prefix=/usr/local; \
		sudo="sudo"; \
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
	if [ -f "$$prefix/lib/libmcl.so" ] || [ -f "$$prefix/lib/libmcl.dylib" ]; then \
		echo "mcl already installed under $$prefix/lib"; \
	else \
		echo "Building mcl from source ($(MCL_SHA))..."; \
		rm -rf /tmp/mcl-src; \
		git clone https://github.com/herumi/mcl.git /tmp/mcl-src; \
		git -C /tmp/mcl-src checkout --detach $(MCL_SHA); \
		$(MAKE) -C /tmp/mcl-src; \
		$$sudo mkdir -p "$$prefix/lib" "$$prefix/include"; \
		if [ "$$uname_s" = "Darwin" ]; then \
			$$sudo cp /tmp/mcl-src/lib/libmcl.dylib "$$prefix/lib/" 2>/dev/null || $$sudo cp /tmp/mcl-src/lib/libmcl.a "$$prefix/lib/"; \
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
