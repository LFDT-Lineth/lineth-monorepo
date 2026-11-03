.PHONY: zkevm-fixtures

# The EF zkevm corpus: the blockchain_tests/ tree of one pinned execution-specs release, unpacked once
# per machine into ZKEVM_FIXTURES_DIR and read by every guest and every checkout. The directory carries
# a stamp holding the sha256 of the tarball it was extracted from; a stamp equal to the pinned sha256
# makes the target a no-op.
ZKEVM_FIXTURES_VERSION := v21.0.5
ZKEVM_FIXTURES_URL     := https://github.com/ethereum/execution-specs/releases/download/tests-zkevm%40$(ZKEVM_FIXTURES_VERSION)/fixtures_zkevm.tar.gz
ZKEVM_FIXTURES_SHA256  := 01c067c0d8ff0c34022f85a6adf4509bb40e62131e07221e81da1c535b544759
ZKEVM_FIXTURES_DIR     ?= $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/lineth/zkevm-fixtures

# The copy is assembled in a temp directory beside the destination and renamed into place, so the
# destination never holds a partial copy; a run that finds the destination installed meanwhile drops
# its own temp directory.
zkevm-fixtures:
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
	curl --fail --location --silent --show-error --retry 3 --output "$$tmp/fixtures.tar.gz" "$(ZKEVM_FIXTURES_URL)"; \
	if command -v sha256sum >/dev/null 2>&1; then got=$$(sha256sum "$$tmp/fixtures.tar.gz"); \
	else got=$$(shasum -a 256 "$$tmp/fixtures.tar.gz"); fi; \
	got=$${got%% *}; \
	if [ "$$got" != "$$sha" ]; then \
		echo "sha256 mismatch for $(ZKEVM_FIXTURES_URL): expected $$sha, got $$got" >&2; exit 1; \
	fi; \
	tar -xzf "$$tmp/fixtures.tar.gz" -C "$$tmp" --strip-components=1 fixtures/blockchain_tests; \
	rm "$$tmp/fixtures.tar.gz"; \
	echo "$$sha" > "$$tmp/.sha256"; \
	if current; then exit 0; fi; \
	if [ -e "$$dir" ]; then mv "$$dir" "$$tmp.old"; fi; \
	mv "$$tmp" "$$dir"
