.PHONY: release-asset

RELEASE_DIR ?= $(MAKEFILE_DIR)../release-assets

release-asset:
	@$(ZIG) build program-id $(ZIG_BUILD_FLAGS) -Drelease-dir="$(abspath $(RELEASE_DIR))" -Dprogram-id-file="$(MAKEFILE_DIR)zig-out/program-id" --summary none || exit $$?; \
	id=$$(cat "$(MAKEFILE_DIR)zig-out/program-id") || exit $$?; \
	echo "Program ID: 0x$$id"; \
	if [ -n "$$GITHUB_OUTPUT" ]; then \
		printf 'program_id=%s\nasset=%s/%s.elf\n' "$$id" "$(abspath $(RELEASE_DIR))" "$$id" >> "$$GITHUB_OUTPUT"; \
	fi
