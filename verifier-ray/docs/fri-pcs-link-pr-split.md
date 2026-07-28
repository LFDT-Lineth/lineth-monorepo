# PR-split plan: vanishing↔PCS claim link + FRI-challenge FS derivation

Planning doc only — no code changes here. Splits the current working-tree change
into independently-reviewable, each-green PRs.

## ⚠️ Current tree state (must fix before slicing)

The working tree does **not build** right now:

- `build.zig` is **unmodified vs HEAD** — the `test_frip` / `test_coexist`
  module wiring is missing, but `test/all.zig` imports `pcs_frip_test.zig` and
  `pcs_coexist_test.zig`, which `@import("test_frip")` / `@import("test_coexist")`.
  → 2 compile errors: `no module named 'test_frip' / 'test_coexist'`.
- `testdata/generated/verify.zig` and `vanishing.zig` carry **prover-ray
  version-drift churn** (~682 non-structural lines in verify.zig, 376 in
  vanishing.zig) from a stray default-generator run, on top of the intended
  `.claims` restructure. These must be restored to HEAD values and only the
  `.claims` structural edit re-applied to verify.zig (vanishing.zig should be
  HEAD-clean; its `VanishingProofView` is unrelated to the union change).

**Reconciliation step (pre-req for all PRs):** restore both generated fixtures
to HEAD, re-apply the surgical `.claims` restructure to verify.zig only, and add
the `test_frip`/`test_coexist` modules to build.zig. Only then is the tree green
and sliceable.

## What's in the combined change (inventory)

Source (behavior):
- `src/crypto/fiat_shamir.zig` — `randomManyIntegers`, `setState`/`getState`.
- `src/field/koalabear_ext.zig` — `halve` (+ `inv2`).
- `src/protocol/root.zig` — `replayWithTranscript(&t, spec, rounds)` + `replay` wrapper.
- `src/pcs/**` — the whole FRI/PCS verifier (params, tree, paired_leaf, layout,
  fold, reconstruct, verify, root). `verify.zig` has: System (+claim maps +
  zeta_coin_index), Challenges, replayWithTranscript, verify, routeClaims.
- `src/verifier.zig` — ClaimSource union, transcript orchestration, routeClaims,
  PCS dispatch.
- `src/lib.zig` — pcs barrel export.

Tests:
- `test/pcs_{foundations,paired_leaf,layout_fold,reconstruct,verify,frip,coexist}_test.zig`
- edits to `test/{all,golden,verifier,transcript,vanishing}_test.zig`.

Generators / fixtures:
- `testdata/generate/{frip.go,frip_emit.go,coexist.go}` + `main.go` edits + go.mod replace.
- `testdata/generated/{frip.zig,coexist.zig}` (new) + `verify.zig` (.claims restructure).

## Proposed PRs (ordered; each builds + tests green on its own)

### PR 0 — Reconciliation / no-op-behavior prep
Restore the churned fixtures to HEAD; nothing else. Establishes a clean, green
baseline so later PRs diff cleanly. (If the branch history already has these
clean, fold PR 0 into PR 1.)

### PR 1 — Field + FS primitives (pure additions, no wiring)
- `koalabear_ext.zig`: `halve` + `inv2`.
- `fiat_shamir.zig`: `randomManyIntegers`, `setState`, `getState`.
- Unit tests for each (byte-vector tests; `randomManyIntegers` vs a known Go
  squeeze, `halve` round-trip, setState/getState round-trip).
- **Why first:** zero dependencies, zero behavior change to existing verify.
  Reviewable in isolation; these are the soundness-critical primitives so they
  deserve focused review.

### PR 2 — FRI/PCS verifier modules (self-contained, not yet wired)
- All of `src/pcs/**` EXCEPT the `verifier.zig` dispatch.
- `src/lib.zig` pcs barrel export.
- `test/pcs_{foundations,paired_leaf,layout_fold,reconstruct}_test.zig` (unit
  tests that don't need the top-level verifier).
- **Why second:** depends on PR 1 (`halve`, transcript). Large but cohesive —
  it's the ported crypto. `pcs.verify` exists and is unit-tested via
  `pcs_verify_test.zig` (D=1) but `Systems.pcs` still defaults to `disabled`, so
  `verifier.verify` behavior is unchanged. No protocol sees PCS yet.
- Depends on: the `Challenges` + `replayWithTranscript` split lives here.

### PR 3 — protocol.replay transcript-by-pointer refactor
- `src/protocol/root.zig`: `replayWithTranscript(&t, ...)` + `replay` wrapper.
- Update coin-only callers: `transcript_test.zig`, `vanishing_test.zig`,
  `golden_test.zig`.
- **Why separate:** a mechanical, behavior-preserving signature change that
  touches the protocol layer + several tests. Keeping it apart from the PCS
  dispatch keeps each diff small. Could merge before PR 2 (independent), but
  ordering it here lets PR 4 assume it.
- Alternative: fold into PR 4 (it's small); flagged because it touches
  non-PCS tests.

### PR 4 — Wire PCS into verifier + the ClaimSource union
- `src/verifier.zig`: `ClaimSource`/`DirectClaims` union, transcript
  orchestration, `pcs.replayWithTranscript` + `pcs.verify` dispatch,
  `routeClaims`, zeta from `all_coins[zeta_coin_index]`.
- `verifier.Proof` field change → ripples to generated `verify.zig` (.claims
  restructure) + `verifier_test.zig`.
- **Why fourth:** this is the actual behavior change (PCS now runs, vanishing
  claims re-sliced). Depends on PR 2 (pcs modules) + PR 3 (transcript). Still no
  *protocol* enables PCS, so `Systems.pcs` stays disabled for all shipped
  fixtures — the union's `.direct` arm is what everything uses. Add negative
  tests here: `MissingPcsClaims` / `UnexpectedPcsClaims`.

### PR 5 — Go generators: frip fixture (isolated PCS byte-gate)
- `testdata/generate/{frip.go,frip_emit.go}` + `main.go` `-frip` flag + go.mod
  replace (local prover-ray).
- `testdata/generated/frip.zig` + `test/pcs_frip_test.zig` +
  build.zig `test_frip` module + all.zig import.
- **Why fifth:** depends on PR 2/4 (the verifier it exercises). Self-contained
  authoritative byte-gate for the FRI/PCS path. The go.mod `replace` +
  generator-churn hazard is contained to this PR.

### PR 6 — Go generator: coexisting fixture (the end-to-end link proof)
- `testdata/generate/coexist.go` + `main.go` `-coexist` flag.
- `testdata/generated/coexist.zig` + `test/pcs_coexist_test.zig` +
  build.zig `test_coexist` module + all.zig import.
- **Why last:** the capstone — proves witness/quotient_claims == entry_claims
  re-slicing end-to-end (accept + entry_claim-mutation + mis-routed-map
  rejection). Depends on everything.

## Cross-cutting notes for whoever splits

- **The generator-churn hazard** (default `go run .` rewrites verify.zig +
  vanishing.zig with prover-ray version drift) must be documented in PR 5/6 and
  in `AGENTS.md`/README for the testdata dir. Only `-frip` / `-coexist` should be
  run; verify.zig's `.claims` edit is surgical (perl on the field-pair), never a
  full regen. See memory `vanishing-pcs-claim-link`.
- **`docs/fri-pcs-plan.md`** (untracked) can land in PR 2 as the design record.
- **`testdata/generate/generate`** (untracked binary) is a build artifact — add
  to `.gitignore`, do not commit.
- **go.sum change** rides with PR 5 (the go.mod replace).
- If reviewers prefer fewer PRs: merge {1,3} (primitives + protocol refactor)
  and {5,6} (both generators). Minimum coherent split is 3: primitives+modules,
  verifier-wiring, generators+fixtures.

## Dependency graph
```
PR1 (field/FS primitives)
  └─> PR2 (pcs modules) ──┐
PR3 (protocol replay) ────┼─> PR4 (verifier wiring) ─> PR5 (frip) ─> PR6 (coexist)
```
