# FRI + PCS Verifier — Implementation Plan (verifier-ray)

Branch: `verifier-ray/fri-pcs`

Port the **verifier halves** of prover-ray's `crypto/koalabear/fri` package
(`fri.go`, `pcs.go`, `tree.go`, `multi_size_table.go`) into `verifier-ray`.
Prover code (Commit / RSEncoder / ProverState / Fold / Open, FFT encode) is
**out of scope** — the Zig side only ever verifies.

## Guiding constraints (from the existing codebase)

1. **Allocation-free.** Nothing in `src/` allocates or takes an `Allocator`;
   the R5 zkVM target builds `ReleaseSmall`. All scratch is stack buffers whose
   sizes are bounded by comptime constants in a compiled `System` (the exact
   pattern `vanishing.zig` / `verifier.zig` already use).
2. **Comptime System + runtime slices.** Shape/soundness parameters that are
   fixed at protocol-compile time live in a comptime `System`; per-proof data
   (roots, claimed values, openings, challenges) arrive as runtime slices.
3. **No FFT.** The Go verifier IFFTs `FinalPoly` into a codeword then indexes it.
   We instead Horner-evaluate `FinalPoly` at each query's folded domain point
   (`canonical.evaluateExtAtExt`). Identical result, no FFT port. **Only
   intentional deviation from the Go verifier.**
4. **Reuse existing primitives.** `field.Element`, `ext.Ext`, `poseidon2`
   (`compress`, `MDHasher`), `commitment.Commitment` ([8]Element = Octuplet).

## Module map (new files under `src/pcs/`)

| New Zig file | Ports from (Go) | Contents |
|---|---|---|
| `src/pcs/params.zig` | `fri.go` Params, `domainPoint*`, `restrictTo` | `Params` (comptime log sizes, numQueries, logFinalPolySize), `domainPoint(logSize,pos)`, `numRounds()`, `restrictTo(topLog)`. Uses `field.rootOfUnityBy` for domain generators; bit-reversed exponent via `@bitReverse`/shift. |
| `src/pcs/tree.zig` | `tree.go` Branch/RecoverRoot, `hashNode` | `Octuplet = commitment.Commitment`. `hashNode(left,right,?aux)` = `poseidon2.compress` (×2 if aux). `Branch{leaf,siblings,aux_siblings}` + `recoverRoot(idx)`. Verifier needs **RecoverRoot only** — no tree building. |
| `src/pcs/paired_leaf.zig` | `pcs.go` RowOpening/RowPair/InputTreeOpening, `hashRowOpening`, `hashAuxPair`, `foldOneLevel`, `RecoverRoot`, `pairAtLevel`, `levelIndex` | Multi-size paired-leaf Merkle opening: `RowOpening{base,ext}`, `RowPair`, `InputTreeOpening{siblings,leaves}`, its `recoverRoot`, `pairAtLevel(levelSize)`, `absorbLeafHeader` + `writeRowOpeningElements` (must byte-match Go's Merkleize order). |
| `src/pcs/layout.zig` | `pcs.go` deepEntry/sizeBundle/layout, `canonicalLayout`, `validate*`, `batchOrders`, `maxSizeLog2` | The frozen canonical enumeration (size desc, batch order, base-then-ext) producing the alpha_DEEP power schedule. Comptime-buildable from a comptime `Shape`+`Shifts` **or** runtime-validated — see "Layout: comptime vs runtime" below. |
| `src/pcs/fold.zig` | `fri.go` `checkFolds`, `inputPair`, `resolvedQuery`, `octupletToExt` | Pure fold-recurrence arithmetic over already-authenticated pairs. `fold: (self+sib)/2 + alpha·(self-sib)/(2x)`. Needs `ext.Ext.halve()` (mul by inv2) — add to `koalabear_ext.zig`. |
| `src/pcs/reconstruct.zig` | `pcs.go` `reconstructQueryValueAt`, `quotientAtValue`, `rowValue`, `claimsForEntry`, `shiftedPoint` | DEEP quotient reconstruction at a query point: `Σ_i α^i · Σ_j (f_i(x)-y_ij)/(x-z_ij)`. Per-point `inverse()` (no batch invert; few points). |
| `src/pcs/verify.zig` | `pcs.go` `Verify` + helpers (`authenticateInputQuery`, `bindInputTreeOpenings`, `inputOpeningRoots`, `checkClaimPointsOutOfDomain`, `checkOpeningProofShape`, D=1 special case) | Top-level `verify(system, proof)` orchestrating: shape checks → per-query authenticate input trees + running FRI layers → reconstruct all levels → `fold.checkFolds`. |
| `src/pcs/root.zig` | — | Re-export barrel (`params`, `tree`, `verify`, public types), mirroring `protocol/root.zig`. |

Wire-up: add `pcs` import + dispatch to `src/verifier.zig` following its
documented "Adding a new sub-verifier" steps (import → `Systems` field →
`Proof` fields → dispatch call). PCS is a *protocol-level* commitment check, so
it likely runs **before** the query sub-verifiers (it authenticates the very
claims vanishing/logderivativesum consume — see
`docs/vanishing-pcs-integration-notes.md`). Exact wiring is Phase 5.

## Type mapping (Go → Zig)

- `field.Octuplet` → `commitment.Commitment` = `[8]field.Element`.
- `field.Ext` → `ext.Ext`; `field.Element` → `field.Element`.
- `map[field.Ext]int` (claim-point dedup) → **removed**; verifier does per-point
  `inverse()` so no dedup map / batch-invert needed (Go used it only to batch).
- Slices that Go `make()`s per query → fixed stack arrays sized by comptime
  `MAX_*` bounds in `System` (max rounds, max queries, max levels, max
  columns-per-level, max shifts-per-column, max base/ext width).
- Go errors → a single Zig `error{...}` set in `pcs/verify.zig`.

## Layout: comptime vs runtime (key decision, flagged for review)

The Go `Verify` builds the canonical layout at runtime from `Shapes`+`Shifts`.
In verifier-ray those are **protocol-fixed**, so the whole layout (and the
alpha_DEEP power schedule, batch orders, per-level sizes) can be a comptime
`System` — matching `vanishing.System`. This makes reconstruction fully
specialized and keeps the runtime input to just: roots, claimed values,
per-query openings, and challenges (fold alphas + query positions from the
shared transcript). Dynamic module sizes (per the integration notes) stay a
runtime concern reconciled against PCS metadata.

**Recommendation:** comptime `System`. Fallback if a protocol needs
proof-time-variable shapes: runtime layout with comptime capacity bounds.

## Additions to existing files

- `koalabear_ext.zig`: add `halve()` (multiply by precomputed inv2 in Ext) and,
  if not present, an `Ext`-at-a-point helper already covered by `canonical`.
- `koalabear.zig`: `domainPoint` needs `pow(u64)` (present) + a bit-reversed
  exponent — pure Zig, no field change.

## Test strategy (deferred per your call, options recorded)

- **A. Generate from Go** (matches existing fixtures): extend
  `testdata/generate/main.go` so the Go prover emits a real `OpeningProof` +
  `VerifyInputs` into `testdata/generated/*.zig`; Zig replays and must accept
  valid / reject mutated. Highest assurance, end-to-end.
- **B. Hand-written unit tests**: small trees + hand-computed fold vectors as
  inline Zig `test` blocks. Faster; unit-level only.
- Regardless: mutation tests (flip a sibling, a claim, a position) mirroring
  `fri_mutation_test.go` to prove rejection.

## Phasing (each phase compiles + tests green before the next)

1. **Foundations** — `params.zig` (+ domain points), `tree.zig`
   (Branch.recoverRoot), `ext.halve()`. Unit tests: recover root on a small
   hand-built branch; domain-point values vs Go.
2. **Paired-leaf openings** — `paired_leaf.zig` (row hashing byte-order must
   match Go Merkleize exactly; verify against a Go-emitted digest).
3. **Layout + fold** — `layout.zig` (canonical enumeration), `fold.zig`
   (`checkFolds` arithmetic). Unit-test the fold recurrence on known vectors.
4. **Reconstruction** — `reconstruct.zig` (DEEP quotient at a point).
5. **Top-level verify + wiring** — `verify.zig`, `pcs/root.zig`, and
   `src/verifier.zig` dispatch. End-to-end fixture (Phase 0 of chosen test
   strategy) accepts a valid proof and rejects mutations.

## Status (updated)

- **Phases 1–5 implemented and green.** `zig build` (default), `-Dr5=true`,
  `-Dr5=true -Dr5-marks=true`, and `zig build test` (65 tests) all pass;
  `test-profiling -Dverifier-profiling` passes (the no-flag failure is the
  pre-existing intentional canary).
- Modules landed: `pcs/{params,tree,paired_leaf,layout,fold,reconstruct,verify,root}.zig`.
- `verify.System` is comptime; per-query scratch is comptime-bounded stack
  arrays (`maxRounds/maxLevels/maxEntries/maxShifts`). Carries `shapes` for
  faithful row-shape checks.
- `src/verifier.zig` wired: `Systems.pcs` defaults to `System.disabled`
  (comptime no-op), dispatched BEFORE the query sub-verifiers. Verifier `Proof`
  gained an optional `pcs: ?PcsClaims`.
- End-to-end test: a **D=1** proof (numRounds=0) built from first principles
  (constant polynomial ⇒ DEEP quotient ≡ 0 ⇒ final_poly=[0]); asserts accept +
  5 mutation rejections. Exercises input-tree auth, level reconstruction, claim
  points, out-of-domain zeta, and the D=1 tie.

### Go generator cross-check — DONE
- `testdata/generate/frip.go` + `frip_emit.go` build a REAL multi-round PCS
  opening proof with the local prover-ray `fri` package (params 3/2/1, one batch
  with size-2 + size-4 ext tables, 2 fold rounds, 1 query) and serialize it to
  `testdata/generated/frip.zig`. `test/pcs_frip_test.zig` replays it through
  `pcs.verify` — **accepts** the honest proof and **rejects** a tampered claim
  (FoldMismatch) and a tampered position (InputTreeAuthFailed). This is the
  authoritative byte-level gate; the acceptance test passing proves the Zig
  verifier agrees with prover-ray on roots, DEEP quotients, folds, and final poly.
- **Regenerate with:** `cd testdata/generate && go run . -frip` (the `-frip`
  flag regenerates ONLY frip.zig).
- **go.mod:** added `replace …/prover-ray => ../../../prover-ray` because the
  FRI/PCS API lives in the local working tree, newer than the pinned version.
  That replace also upgrades `wiop` (3 mechanical `*Runtime` fixes applied to
  main.go), so the legacy vanishing/verify fixtures are NOT regenerated by the
  `-frip` path — running the default (no-flag) generator would churn them
  against the upgraded prover-ray, so only do that when intentionally bumping.

### Deferred (follow-up)
- **Coins → (zeta, fold_alphas, query_positions) mapping.** The current
  `protocol.replay` squeezes only Ext coins; deriving integer query positions is
  undefined until codegen specifies it. `verifier.PcsClaims` currently receives
  these pre-derived from the caller.
- **Input-root dedup across batches** (Go `inputOpeningRoots`): verify.zig
  authenticates one InputTreeOpening per batch (sound; skips redundant-work
  avoidance when two batches share a root).

## Open questions for you

- **Q1 Layout model:** comptime `System` (recommended) vs runtime-with-bounds?
- **Q2 Scope this session:** all 5 phases, or stop after Phase 1–2 (foundations)
  for review before building the PCS layers on top?
- **Q3 Test vectors:** A (Go generator) vs B (hand-written) — you said "decide
  later"; I'll default to B for unit checks during Phases 1–4 and propose A for
  the Phase 5 end-to-end gate.
