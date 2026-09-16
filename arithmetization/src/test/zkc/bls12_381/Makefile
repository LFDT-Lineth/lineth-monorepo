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
# Cost notes:
#   - every affine result costs one Fermat inversion, ~613 mulmod_384 on its
#     own, which dominates the group law it follows;
#   - g2_subgroup runs the real 64-bit seed ladder, ~20s per fixture;
#   - miller, final_exp, pairing_check and pairing_ram are the expensive ones:
#     a single pairing is ~17k mulmod_384, about a minute per fixture under -c.
# Harnesses are ordered cheapest-first so a regression surfaces early.
#
# Two targets:
#   test     -- full suite (>1 hour, developer / scheduled)
#   test-ci  -- bounded CI target (<15 min): the cheap harnesses and pairing_ram
#               run their full fixture set, the slow curve-math ones sample one
#               accept + one reject

SHELL := /bin/sh

HARNESSES := fp2_arith fp2_inv fp12_arith fp12_ops g1_add g1_mul g1_subgroup \
             g2_add g2_mul g2_subgroup miller final_exp pairing_check pairing_ram
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

# Bounded CI target.  All harnesses are covered; the slow pure-curve-math ones
# (miller, final_exp, pairing_check -- each fixture ~1 min under -c) are sampled
# to one accept and one reject so the whole target fits in ~10 minutes. The full
# suite is `make test` (>1 hour).
#
# pairing_ram is NOT sampled. It is the only harness that reaches guest RAM and
# the only one that can stage more than one chunk, so it owns both the ABI byte
# layout and the >64-pair chunked path -- sampling it would take the one-pair
# fixture and silently skip every chunk-boundary case. All 11 of its fixtures
# run in ~2 minutes, which the budget affords.
#
# The n_h list encodes the per-harness fixture cap as "CAP:NAME": 9999 is
# effectively unlimited (no harness has anywhere near that many fixtures).
.PHONY: test-ci
test-ci:
	@dir=$$(mktemp -d); tmp=$$dir/case.json; err=$$dir/err; \
	for n_h in \
	    9999:fp2_arith 9999:fp2_inv 9999:fp12_arith 9999:fp12_ops \
	    9999:g1_add 9999:g1_mul 9999:g1_subgroup \
	    9999:g2_add 9999:g2_mul 9999:g2_subgroup \
	    1:miller 1:final_exp 1:pairing_check 9999:pairing_ram; do \
		n=$${n_h%%:*}; h=$${n_h##*:}; \
		echo "=== $$h ==="; \
		for kind in accepts rejects; do \
			[ -f $$h.$$kind ] || continue; \
			i=0; \
			grep '^{' $$h.$$kind | head -$$n | while IFS= read -r l; do \
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
