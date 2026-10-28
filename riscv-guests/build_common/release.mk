.PHONY: release-asset

RELEASE_DIR ?= $(MAKEFILE_DIR)../release-assets
RELEASE_GUEST := $(notdir $(patsubst %/,%,$(MAKEFILE_DIR)))

release-asset:
	@set -eu; \
	: "$${RELEASE_VERSION:?RELEASE_VERSION is required for release-asset}"; \
	$(ZIG) build program-id $(ZIG_BUILD_FLAGS) -Drelease-dir="$(abspath $(RELEASE_DIR))" -Dprogram-id-file="$(MAKEFILE_DIR)zig-out/program-id" -Drelease-guest="$(RELEASE_GUEST)" -Drelease-version="$(RELEASE_VERSION)" --summary none; \
	id=$$(cat "$(MAKEFILE_DIR)zig-out/program-id"); \
	filename=$$(cat "$(MAKEFILE_DIR)zig-out/program-id.asset"); \
	echo "Program ID: 0x$$id"; \
	if [ -n "$${GITHUB_OUTPUT:-}" ]; then \
		printf 'program_id=%s\nasset=%s/%s\nfilename=%s\n' "$$id" "$(abspath $(RELEASE_DIR))" "$$filename" "$$filename" >> "$$GITHUB_OUTPUT"; \
	fi
