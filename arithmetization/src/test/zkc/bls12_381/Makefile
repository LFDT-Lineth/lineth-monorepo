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
# g1_mul is much slower than g1_add: every affine result costs one Fermat
# inversion, which is ~613 mulmod_384 calls on its own -- far more than the
# group law it follows. Expect ~11s per fixture.

SHELL := /bin/sh

HARNESSES := g1_add g1_mul
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
