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

.PHONY: compile test fetch clean fmt fmt-check help

# Each common target fans out to every guest's own Makefile.
compile test fetch clean:
	@for g in $(GUESTS); do \
		echo "==> make $@ ($$g)"; \
		$(MAKE) -C $$g $@ ZIG="$(ZIG)" || exit $$?; \
	done

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
	@echo ""
	@echo "Guest-specific targets run directly, e.g.:"
	@echo "  make -C l2-execution spec-test   # EF zkevm stateless fixtures (SPEC_ARGS=…)"
	@echo "  make -C l2-execution exec        # run inside the ZKC interpreter"
