# Runs the Poseidon2 permutation unit test.
#
# zkc execute consumes a single JSON object per run (with a .json extension),
# while the fixtures hold one trace per line (with `;;` comments). This target
# splits each fixture line into a one-line JSON and runs it through the VM:
#   - .accepts lines must run to completion (exit 0)
#   - .rejects lines must hit `fail` (nonzero exit)
#
# TODO: `zkc execute -c` (constraint checking) is not used because register
# splitting is not implemented yet.

SHELL := /bin/bash

ZKC_EXEC_FLAGS ?= --fast

SRC     := permutation.zkc
ACCEPTS := permutation.accepts
REJECTS := permutation.rejects

.PHONY: test
test:
	@tmp=$$(mktemp --suffix=.json); rc=0; \
	readarray -t lines < <(grep '^{' $(ACCEPTS)); \
	for i in "$${!lines[@]}"; do \
		printf '%s\n' "$${lines[$$i]}" > $$tmp; \
		if zkc execute $(ZKC_EXEC_FLAGS) --field KOALABEAR_16 $$tmp $(SRC) >/dev/null 2>&1; then \
			echo "accept[$$i] PASS"; \
		else \
			echo "accept[$$i] FAIL (expected to be accepted)"; rc=1; \
		fi; \
	done; \
	readarray -t lines < <(grep '^{' $(REJECTS)); \
	for i in "$${!lines[@]}"; do \
		printf '%s\n' "$${lines[$$i]}" > $$tmp; \
		if zkc execute $(ZKC_EXEC_FLAGS) --field KOALABEAR_16 $$tmp $(SRC) >/dev/null 2>&1; then \
			echo "reject[$$i] FAIL (expected to be rejected)"; rc=1; \
		else \
			echo "reject[$$i] PASS"; \
		fi; \
	done; \
	rm -f $$tmp; \
	if [[ $$rc -eq 0 ]]; then echo "ALL OK"; else echo "FAILURES"; fi; \
	exit $$rc
