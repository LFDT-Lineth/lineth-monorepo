# Runs the BLS12-381 unit tests.
#
# Same fixture convention as ../nonnative: one JSON object per line in
# .accepts/.rejects with `;;` comments, split into a temp file per run.
#   - .accepts lines must run to completion (exit 0)
#   - .rejects lines must hit `fail` (nonzero exit)
#
# Runs in TRACING mode with `-c`; `--fast` panics on u384 ("unsupported
# instruction form (6 v 7 limbs)"). See ../nonnative/Makefile.
#
# THREE OUTCOMES, NOT TWO. `zkc execute -c` reports a violated AIR constraint on
# stderr but still exits 0, so exit status alone cannot tell "the program ran and
# its constraints hold" from "the program ran and its constraints do not". A
# harness that only tested the exit code would pass a circuit that cannot be
# proved. So:
#   exit 0, no error output   -> accepted
#   exit 0, error output      -> BROKEN, reported whichever file it came from
#   nonzero exit              -> rejected (it reached `fail`)
# A `fail` also prints an AIR error, which is why the exit code, not the error
# output, is what separates rejected from broken.
#
# Cost notes (these are dev targets, not CI -- the whole run is over an hour):
#   - every affine result costs one Fermat inversion, ~613 mulmod_384 on its
#     own, which dominates the group law it follows;
#   - g2_subgroup runs the real 64-bit seed ladder, ~20s per fixture;
#   - miller, final_exp and pairing_check are the expensive ones: a single
#     pairing is ~17k mulmod_384, about a minute per fixture under -c.
# Harnesses are ordered cheapest-first so a regression surfaces early.

SHELL := /bin/sh

HARNESSES := fp2_arith fp2_inv fp12_arith fp12_ops g1_add g1_mul g1_subgroup \
             g2_add g2_mul g2_subgroup miller final_exp pairing_check
FIELD     := KOALABEAR_16

# Default to the zkc version pinned in arithmetization/go.mod, as the poseidon2
# harness does. A bare `zkc` off PATH is whatever the developer last installed
# globally, which silently tests against the wrong compiler. Override with
# ZKC=path/to/zkc to try an unreleased build.
ZKC       ?= go tool zkc

.PHONY: test
test:
	@dir=$$(mktemp -d); tmp=$$dir/case.json; err=$$dir/err; \
	for h in $(HARNESSES); do \
		echo "=== $$h ==="; \
		for kind in accepts rejects; do \
			[ -f $$h.$$kind ] || continue; \
			i=0; \
			grep '^{' $$h.$$kind | while IFS= read -r l; do \
				printf '%s\n' "$$l" > $$tmp; \
				$(ZKC) execute -c --field $(FIELD) $$tmp $$h.zkc >/dev/null 2>$$err; \
				rc=$$?; \
				if [ $$rc -eq 0 ] && grep -q 'level=error' $$err; then \
					echo "  $$kind[$$i] BROKEN (constraints violated, exit 0)"; \
					echo fail >> $$dir/rc; \
				elif [ $$rc -eq 0 ]; then \
					if [ $$kind = accepts ]; then echo "  $$kind[$$i] PASS"; \
					else echo "  $$kind[$$i] FAIL (expected to be rejected)"; echo fail >> $$dir/rc; fi; \
				else \
					if [ $$kind = rejects ]; then echo "  $$kind[$$i] PASS"; \
					else echo "  $$kind[$$i] FAIL (expected to be accepted)"; echo fail >> $$dir/rc; fi; \
				fi; \
				i=$$((i+1)); \
			done; \
		done; \
	done; \
	rc=0; if [ -f $$dir/rc ]; then rc=1; fi; \
	rm -rf $$dir; \
	if [ $$rc -eq 0 ]; then echo "ALL OK"; else echo "FAILURES"; fi; \
	exit $$rc
