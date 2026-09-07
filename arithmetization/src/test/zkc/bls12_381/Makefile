# Runs the BLS12-381 G1 unit tests.
#
# Same fixture convention as ../nonnative: one JSON object per line in
# .accepts/.rejects with `;;` comments, split into a temp file per run.
#   - .accepts lines must run to completion (exit 0)
#   - .rejects lines must hit `fail` (nonzero exit)
#
# Runs in TRACING mode with `-c`; `--fast` panics on u384 ("unsupported
# instruction form (6 v 7 limbs)"). See ../nonnative/Makefile.
#
# Cost notes (these are dev targets, not CI -- the whole run is ~20 minutes):
#   - every affine result costs one Fermat inversion, ~613 mulmod_384 on its
#     own, which dominates the group law it follows;
#   - g2_subgroup runs the real 64-bit seed ladder and is the slowest at ~20s
#     per fixture, against ~11s for everything else.
# Harnesses are ordered cheapest-first so a regression surfaces early.

SHELL := /bin/sh

HARNESSES := fp2_arith fp2_inv g1_add g1_mul g2_add g2_mul g2_subgroup
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
