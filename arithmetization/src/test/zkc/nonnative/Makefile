# Runs the u384 non-native modular arithmetic unit tests.
#
# zkc execute consumes a single JSON object per run (with a .json extension),
# while the fixtures hold one trace per line (with `;;` comments). This target
# splits each fixture line into a one-line JSON and runs it through the VM:
#   - .accepts lines must run to completion (exit 0)
#   - .rejects lines must hit `fail` (nonzero exit)
#
# Unlike the poseidon2 unit test this runs in TRACING mode with `-c`
# (constraint checking) rather than `--fast`: the fast interpreter panics with
# "unsupported instruction form (6 v 7 limbs)" on the u385 carry type used by
# addmod_384/submod_384. Tracing mode handles it, and `-c` is the check that
# actually matters here -- it verifies the 48-limb mulmod_384 native circuit
# against its constraints, not merely that it executes.

SHELL := /bin/sh

HARNESSES := u384_arith u384_inv
FIELD     := KOALABEAR_16
ZKC       ?= zkc

.PHONY: test
test:
	@dir=$$(mktemp -d); tmp=$$dir/case.json; rc=0; \
	for h in $(HARNESSES); do \
		echo "=== $$h ==="; \
		i=0; \
		grep '^{' $$h.accepts | while IFS= read -r l; do \
			printf '%s\n' "$$l" > $$tmp; \
			if $(ZKC) execute -c --field $(FIELD) $$tmp $$h.zkc >/dev/null 2>&1; then \
				echo "  accept[$$i] PASS"; \
			else \
				echo "  accept[$$i] FAIL (expected to be accepted)"; echo fail >> $$dir/rc; \
			fi; \
			i=$$((i+1)); \
		done; \
		i=0; \
		grep '^{' $$h.rejects | while IFS= read -r l; do \
			printf '%s\n' "$$l" > $$tmp; \
			if $(ZKC) execute -c --field $(FIELD) $$tmp $$h.zkc >/dev/null 2>&1; then \
				echo "  reject[$$i] FAIL (expected to be rejected)"; echo fail >> $$dir/rc; \
			else \
				echo "  reject[$$i] PASS"; \
			fi; \
			i=$$((i+1)); \
		done; \
	done; \
	if [ -f $$dir/rc ]; then rc=1; fi; \
	rm -rf $$dir; \
	if [ $$rc -eq 0 ]; then echo "ALL OK"; else echo "FAILURES"; fi; \
	exit $$rc
