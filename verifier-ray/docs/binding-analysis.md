# Value Binding Analysis — verifier-ray

This document catalogues every value the `verifier-ray` verifier consumes, states
whether it is **trusted** (accepted as-is) or **bound** (provably tied to a trusted
anchor), names the binding relation, and gives the `file:line` where it is enforced.
It also records which value-computations are **confirmed to match prover-ray** versus
merely claimed by comments.

Scope: the `verify` entry point ([`src/verifier.zig`](../src/verifier.zig)) and the
sub-verifiers it dispatches — protocol replay, PCS/FRI, vanishing, logderivativesum.

---

## 1. The trust model

Every value is either an **anchor of trust** or must **bind back to one**. There are
exactly three anchors:

1. **The compiled `System`** — comptime constants emitted by the Go codegen and
   compiled into the verifier (a verification key): layout, shapes, shift schedules,
   claim maps, precomputed roots, coin routing (`protocol.Spec`), and batch-root
   provenance (`batch_roots`). Trusted by construction.
2. **Fiat-Shamir coins** — squeezed from the Poseidon2 Merkle-Damgård transcript.
   A prover cannot choose them, so any value absorbed *before* a squeeze is committed
   to by the coins that follow.
3. **Public columns** — trusted by the protocol's visibility model
   ([`types.zig` `Visibility.public`](../src/protocol/types.zig)); public input by
   definition.

Everything else is only sound if it binds to one of these.

### The verify flow

`verifier.verify(spec, systems, proof)` ([`src/verifier.zig:101`](../src/verifier.zig#L101)):

1. **Replay** — `protocol.replayWithTranscript` absorbs every round message
   (oracle commitments, public columns, cells) into the shared transcript and
   squeezes all protocol coins. ([`src/protocol/root.zig:79`](../src/protocol/root.zig#L79))
2. **PCS** — `pcs.replayWithTranscript` continues the *same* transcript to derive the
   FRI fold challenges + query positions, then `pcs.verify` authenticates the
   opening. ([`src/pcs/verify.zig`](../src/pcs/verify.zig))
3. **Dispatch** — `vanishing.verify` and `logderivativesum.verify` do pure arithmetic
   on PCS-authenticated claims + FS coins.

---

## 2. Trust values and their binding relations

| Value (from proof) | Status | Binding relation | Enforced at |
|---|---|---|---|
| Oracle commitments (batch roots) | absorbed → becomes anchor #2 | FS absorb before squeeze | [`root.zig:111`](../src/protocol/root.zig#L111) |
| Public columns | trusted (public by design) | — (FS-absorbed) | [`root.zig:112`](../src/protocol/root.zig#L112) |
| Cell scalars | FS-absorbed; see below | FS absorb | [`root.zig:115`](../src/protocol/root.zig#L115) |
| `zeta` opening point | **bound** | `= all_coins[zeta_coin_index]`; never read from proof | [`verifier.zig:166`](../src/verifier.zig#L166) |
| FRI round roots / final poly | absorbed → anchor #2 | FS absorb during challenge derivation | [`verify.zig` `deriveChallenges`](../src/pcs/verify.zig) |
| Fold challenges / query positions | **bound** | FS-squeezed from round_roots + final_poly | [`verify.zig` `deriveChallenges`](../src/pcs/verify.zig) |
| PCS batch **roots** (`Inputs.roots`) | **bound** | rebuilt by `resolveRoots` from `batch_roots[b].round` → the round's own `oracle_commitment`; the proof carries no separate copy | [`verifier.zig:228`](../src/verifier.zig#L228) |
| Committed row values | **bound** | Merkle-authenticated against those roots | [`verify.zig` `authenticateInputQuery`](../src/pcs/verify.zig) |
| Running-layer self/sibling | **bound** | Merkle-authenticated against round roots | [`verify.zig` `resolveRunningLayers`](../src/pcs/verify.zig) |
| `entry_claims` (the Ys) | **bound** | DEEP-quotient reconstruction + fold recurrence that must chain to the Horner-evaluated `final_poly` | [`verify.zig` `resolveLevels` / `checkFolds`](../src/pcs/verify.zig) |
| Vanishing witness/quotient claims | **bound** | re-derived from authenticated `entry_claims` via `routeClaims`; never read from proof | [`verifier.zig:178`](../src/verifier.zig#L178) |
| Endpoint cells (`z_final`, `result`, `cell_value`) | **bound iff codegen emits a constraint** | FS-absorbed; self-authenticated only if an endpoint-binding vanishing constraint ties the cell to a PCS-authenticated column | [`logderivativesum.zig`](../src/query/logderivativesum.zig), [`pcs_endpoint_binding_test.zig`](../test/pcs_endpoint_binding_test.zig) |
| **Dynamic `module_sizes` (`n`)** | **TRUSTED** | only power-of-two + index-range gates; not tied to any commitment | [`vanishing.zig:98`](../src/query/vanishing.zig#L98), [`:129`](../src/query/vanishing.zig#L129) |
| Proof input image | trusted (structural) | raw mmap/cast; no serialization/validation | [`main.zig:99`](../src/main.zig#L99) |

### The binding pattern (how `roots` and the Ys were closed)

The sound pattern is **derive from the anchor**, not "carry a copy and equality-check
it." Codegen emits *provenance*, not the value:

```zig
// per batch:  .{ .round = N }   or   .{ .precomputed = <octuplet> }
```

`resolveRoots` then reads `rounds[N].columns[0].oracle_commitment` — the exact octuplet
absorbed into FS to derive `zeta` ([`verifier.zig:228-251`](../src/verifier.zig#L228)).
The prover has no field to choose. `PcsClaims`/`routeClaims` apply the same idea to the
Ys: vanishing's claims *are* the PCS-authenticated `entry_claims`, so "feed PCS and
vanishing different values" is unrepresentable.

---

## 3. The one open non-binding value: dynamic `module_sizes`

`n` (a dynamic module's domain size) is read from `proof.module_sizes` and gated only by
a power-of-two check and an index-range check
([`vanishing.zig:98-100,129`](../src/query/vanishing.zig#L98)). It then flows into
security-relevant arithmetic: the annihilator `rⁿ−1`, Lagrange denominators, and
roots-of-unity ([`vanishing.zig:134,264,291`](../src/query/vanishing.zig#L134)). The
quotient identity alone can be satisfied by a wrong (but power-of-two) `n` when numerator
and quotient are both zero. Tracked in
[`vanishing-pcs-integration-notes.md`](./vanishing-pcs-integration-notes.md).

**prover-ray does not fully bind `n` either.** There, `n` is also read from the proof
(`proof.DynamicSizes`), gated by power-of-two plus a consistency check against any
*visible* column — but in PCS-compiled protocols the size-determining columns are hidden,
so that check goes inert. No authenticated source of truth for `n` exists to copy.

Closing it therefore has two tiers:

- **Parity with prover-ray (smaller):** fold the dynamic sizes into the transcript so the
  challenges are bound to the claimed `n`, and add the `n == visible-column-length`
  consistency check where a module has a visible column.
- **True binding (stronger than prover-ray):** cross-check `proof.module_sizes[idx]`
  against the padded size implied by the **authenticated PCS batch shape** for that
  module's columns — i.e. `NextPowerOfTwo(RuntimeSize)`, the leaf-count the committed
  tree used. This is a derivation neither codebase has today.

---

## 4. Are these computed matching prover-ray?

The verifier's comments claim a "byte-faithful port" of prover-ray. Cross-checking the
Zig against the Go source confirms most of it — with **two divergences**, one of which is
soundness-relevant.

| # | Computation | Verdict | prover-ray reference |
|---|---|---|---|
| 1 | FS challenge schedule (fold alphas + query positions) | **MATCHES (caveat: D=1)** | `wiop/compilers/pcs/pcs.go:316-324` |
| 2 | DEEP quotient reconstruction | **MATCHES** | `crypto/koalabear/fri/pcs.go:1134-1367` |
| 3 | Ext-element absorb limb order | **MATCHES** | `crypto/koalabear/fiatshamir/poseidon2.go:37-43` |
| 4 | Merkle leaf/node hashing + domain tag | **MATCHES** | `crypto/koalabear/fri/commitment.go:14-26`, `tree.go:265-269` |
| 5 | Dynamic sizes folded into FS | **DIVERGES** | `wiop/wiop_runtime.go:140-153` |

### Confirmed matching (items 2–4)

- **DEEP quotient reconstruction** — the quotient `(f(x)−y)/(x−z)`
  ([`reconstruct.zig:53-63`](../src/pcs/reconstruct.zig#L53) vs `fri/pcs.go:1181-1197`),
  the Horner batching in reverse entry order
  ([`reconstruct.zig:86-100`](../src/pcs/reconstruct.zig#L86) vs `pcs.go:1134-1172`),
  `alpha_DEEP = fold_alpha²` with the boundary-round case
  ([`verify.zig:498-503`](../src/pcs/verify.zig#L498) vs `pcs.go:1346-1355`), and the
  conjugate-pair reconstruction at `(pos, pos^1)` all match line-for-line.
- **Ext-element absorb limb order** — Zig absorbs `B0.a0, B0.a1, B1.a0, B1.a1, B2.a0,
  B2.a1` ([`fiat_shamir.zig:22-34`](../src/crypto/fiat_shamir.zig#L22)); Go
  reinterprets `E6 = (B0,B1,B2)` of `E2 = (A0,A1)` as the same contiguous 6-limb
  sequence (`fiatshamir/poseidon2.go:37-43`).
- **Merkle hashing** — leaf domain tag `0x4c66_7269_5f6c_6631` ("Lfri_lf1"), header
  order `(tag, base_width, ext_width)`, and node hash `compress(left, right)` all match
  ([`paired_leaf.zig:37,143-147`](../src/pcs/paired_leaf.zig#L37),
  [`tree.zig:27-30`](../src/pcs/tree.zig#L27) vs `fri/commitment.go:14-26`,
  `fri/tree.go:265-269`).

### Divergence 1 — dynamic sizes not folded into FS (soundness-relevant) — **FIXED**

> **Update:** closed by folding the dynamic sizes into the transcript at the start of
> every round, byte-for-byte matching prover-ray's `AdvanceRound` order. `protocol.Spec`
> gained `dynamic_size_slots` (the absorb schedule), `replayWithTranscript` absorbs
> `module_sizes[slot]` per round before the round's columns, codegen emits the schedule
> in ascending `System.Modules` order, and the vanishing `DynamicIndex` is aligned to
> that same order (shared `DynamicModuleRanks`). The description below is the pre-fix
> state, kept for context.

**This is the same gap as §3, now confirmed as a divergence from the reference.**
prover-ray absorbs every dynamic module size into Fiat-Shamir inside `AdvanceRound`,
*before* the round commitment, on every round including round 0:

```go
// wiop/wiop_runtime.go:140-153
for k, mod := range run.System.Modules {
    if !mod.isDynamic { continue }
    size, ok := run.dynamicSizes[k]
    ...
    run.fs.Update(field.NewElement(uint64(size)))   // dynamic size → FS
}
```

So in prover-ray all coins — `zeta`, fold alphas, query positions — are bound to the
claimed `n`. `AssignColumn` even forbids a dynamic size from changing after round 0
"because it would break the fiat-shamir transcript."

verifier-ray's replay ([`root.zig:107-120`](../src/protocol/root.zig#L107)) absorbs only
oracle commitments, public columns, and cells — **it never absorbs `module_sizes`**.
`proof.module_sizes` reaches only the vanishing arithmetic, never the transcript.

**Consequence:** a prover could reuse the same commitments/openings under a different
claimed `n` and the Zig transcript (hence every coin) would be unchanged, whereas
prover-ray's would differ. This is why the parity fix in §3 ("fold dynamic sizes into the
transcript") is the minimum needed to match the reference.

### Divergence 2 — D=1 (numRounds == 0) final fold-alpha squeeze — **FIXED**

> **Update:** closed by squeezing the final fold challenge unconditionally in
> `deriveChallenges` ([`verify.zig`](../src/pcs/verify.zig)), matching prover-ray
> `pcs.go:331`. For D=1 the squeezed value is discarded (there is no fold), but the
> transcript side effect now happens before `final_poly` is absorbed and the query
> positions are drawn — so the positions match the reference. Regression:
> `"verify: D=1 challenge schedule squeezes the final alpha (matches prover-ray)"` in
> [`pcs_verify_test.zig`](../test/pcs_verify_test.zig). The description below is the
> pre-fix state.

The FS challenge schedule matches for `numRounds ≥ 1`, but the final fold-alpha squeeze
differs in the `numRounds == 0` case:

```zig
// verify.zig:356 — Zig gates the final squeeze
if (num_rounds > 0) { ch.fold_alphas[num_rounds - 1] = transcript.randomExt(); }
```
```go
// pcs.go:321 — Go squeezes it unconditionally
foldAlphas = append(foldAlphas, fs.RandomFext())
```

For `numRounds == 0` (`log_plaintext_size == log_final_poly_size`), Go squeezes one extra
`RandomFext` — which mutates FS state via the trailing safeguard update — so the two
transcripts desynchronize from the `final_poly` absorb onward. **Only a concern if a
compiled system can restrict to D=1**; otherwise this is a dead path. Worth a comment or
a compile-time assertion that `numRounds > 0` for the deployed params.

---

## 5. Findings summary

- **Bound and matching prover-ray:** roots (via `resolveRoots`/`batch_roots`), the Ys
  (via `routeClaims`), `zeta`, fold challenges, query positions, committed row values,
  DEEP reconstruction, transcript primitives, Merkle hashing.
- **Trusted by design:** public columns, the compiled `System`, the precomputed root.
- **Bound conditionally:** endpoint cells — sound iff codegen emits the binding
  constraint; the verifier does not check that it did.
- **Open / divergent:** dynamic `module_sizes` — trusted, and (unlike prover-ray) not
  folded into Fiat-Shamir. Parity fix: absorb them in `replay`. True fix: derive `n` from
  the authenticated PCS batch shape.
- **Edge case (fixed):** the D=1 final-alpha squeeze now matches prover-ray — squeezed
  unconditionally so the query positions stay in sync.
