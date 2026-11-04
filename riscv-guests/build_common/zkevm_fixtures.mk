.PHONY: fetch-zkevm-fixtures

# The EF zkevm corpus: the blockchain_tests/ tree of the pinned release, unpacked once per machine into
# ZKEVM_FIXTURES_DIR and read by every guest that includes this file, from every checkout. The
# directory carries a stamp holding the sha256 of the tarball it was extracted from; a stamp equal to
# the pinned sha256 makes the target a no-op.
include $(dir $(lastword $(MAKEFILE_LIST)))zkevm_fixtures_pin.mk
ZKEVM_FIXTURES_DIR ?= $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/lineth/zkevm-fixtures

# The copy is assembled in a temp directory beside the destination and renamed into place, so the
# destination never holds a partial copy; a run that finds the destination installed meanwhile drops
# its own temp directory. Replacing a stale copy takes two renames, between which the destination is
# briefly absent.
fetch-zkevm-fixtures:
	@set -eu; \
	dir="$(ZKEVM_FIXTURES_DIR)"; sha="$(ZKEVM_FIXTURES_SHA256)"; \
	current() { [ "$$(cat "$$dir/.sha256" 2>/dev/null)" = "$$sha" ]; }; \
	if current; then exit 0; fi; \
	if [ -e "$$dir" ] && [ ! -f "$$dir/.sha256" ]; then \
		echo "$$dir exists and is not a zkevm fixtures directory" >&2; exit 1; \
	fi; \
	parent=$$(dirname "$$dir"); mkdir -p "$$parent"; \
	tmp=$$(mktemp -d "$$parent/.zkevm-fixtures.XXXXXX"); \
	trap 'rm -rf "$$tmp" "$$tmp.old"' EXIT; trap 'exit 1' HUP INT TERM; \
	echo "Fetching zkevm fixtures $(ZKEVM_FIXTURES_VERSION) into $$dir"; \
	if [ -t 2 ]; then progress=--progress-bar; else progress=--silent; fi; \
	curl --fail --location $$progress --show-error --connect-timeout 30 --retry 3 --output "$$tmp/fixtures.tar.gz" "$(ZKEVM_FIXTURES_URL)"; \
	if command -v sha256sum >/dev/null 2>&1; then got=$$(sha256sum "$$tmp/fixtures.tar.gz"); \
	else got=$$(shasum -a 256 "$$tmp/fixtures.tar.gz"); fi; \
	got=$${got%% *}; \
	if [ "$$got" != "$$sha" ]; then \
		echo "sha256 mismatch for $(ZKEVM_FIXTURES_URL): expected $$sha, got $$got" >&2; exit 1; \
	fi; \
	tar -xzf "$$tmp/fixtures.tar.gz" -C "$$tmp" --strip-components=1 fixtures/blockchain_tests; \
	rm "$$tmp/fixtures.tar.gz"; \
	echo "$$sha" > "$$tmp/.sha256"; \
	chmod 755 "$$tmp"; \
	if current; then exit 0; fi; \
	if [ -e "$$dir" ]; then mv "$$dir" "$$tmp.old"; fi; \
	mv "$$tmp" "$$dir"
