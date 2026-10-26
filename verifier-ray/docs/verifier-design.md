# Verifier Design

## verifier.verify Architecture

```
┌──────────────────────────────────────────────┐
│  verifier.verify                             │  ← entry point
│  1. replay  — drive transcript, derive coins │
│  2. route   — wrap coins+rounds in Context   │
│  3. dispatch — call each sub-verifier        │
└────────────────┬─────────────────────────────┘
                 │ protocol.Context
                 │  all_coins  []Coin
                 │  rounds     []RoundMessage
                 ▼
        ┌────────────────┬──────────────────┐
        ▼                ▼                  ▼
  vanishing.verify  logderiv.verify   ...future...
  (constraint identity)       (constraint identity)
```

## Package Structure

```
src/
├── protocol/       RoundMessage, Coin, Scalar, Spec, Context, replay()
├── query/          sub-verifiers: vanishing, (logderiv, rangecheck, ...)
└── verifier.zig    entry point: Systems, ProofData, verify()
```

`protocol/` is internally split into two files:

- `types.zig` — wire types: `RoundMessage`, `Coin`, `Scalar`
- `root.zig` — `Spec`, `Context`, `replay()`; re-exports the public surface. `replay` takes a comptime `Spec`, absorbs each round into the transcript, and squeezes its coins inline — round ordering is fixed by the `inline for` over the spec, so no runtime ordering check is needed.

## Coin Generation

Coins are KoalaBear extension-field elements derived via the Fiat-Shamir
transform: the prover's round messages are absorbed into a Poseidon2
Merkle-Damgård hasher; after each round the hasher state is squeezed to
produce verifier challenges.

### Compile-time: counts come from the protocol spec

Coin counts are fixed at compile time in the protocol spec:

```
round_coin_counts  = [0, 2, 0, M, M]   // coins squeezed after each round
round_coin_offsets = [0, 0, 2, 2, 2+M] // start index in all_coins[]
total_round_coins  = 2 + 2*M
```

`round_coin_counts[0]` is always 0 — no coins are derived before the first
round message is absorbed. `protocol.replay` takes the `Spec` as a comptime
parameter, so coin counts are never a runtime decision.

### Runtime: protocol.replay absorbs and squeezes

```
for each round i:
    absorb columns  → oracle commitments or public column values
    absorb cells    → opened scalar values
    squeeze round_coin_counts[i+1] coins → all_coins[coin_offsets[i+1]..]
```

`protocol.replay` is the only function that touches the Fiat-Shamir
transcript. It runs once, before any sub-verifier is called.

## What the Vanishing Sub-verifier Does

`vanishing.verify` receives pre-derived coins via `ctx.all_coins` and cell
openings via `ctx.rounds[i].cells`. It never touches the transcript.

For each module it:

1. Reads `merge_coin` (α) and `eval_coin` (r) from `ctx.all_coins` at the
   per-module `merge_coin_index` / `eval_coin_index` fixed by the compiled system.
2. Computes the domain annihilator `Z_H(r) = r^n − 1`.
3. Evaluates the expression DAG, resolving `coin_value`, `cell_value`,
   `column_claim`, and `constant` leaves.
4. Aggregates vanishing numerators: `P_agg(r) = Σ αⁱ · Pᵢ(r) · Cᵢ(r)`.
5. Reconstructs the quotient: `Q(r) = Σ (r^n)^k · Qₖ(r)`.
6. Checks the PLONK identity: `P_agg(r) = Z_H(r) · Q(r)`.

## What Changed from the Previous Design

Before this design there was no `protocol/` layer and no `verifier.zig`
entry point: `vanishing.verify` was the entry point and did everything itself.

| Concern | Before (`vanishing.verify` owned everything) | After |
|---|---|---|
| Entry point | single `vanishing.verify` drove transcript replay, coin derivation, and the constraint check | `verifier.verify` orchestrates replay → route → dispatch; sub-verifiers do math only |
| Transcript & coin derivation | inside `vanishing.verify`, via a `runtime.Runtime` | `protocol.replay`, called once before any sub-verifier |
| Coin count per round | `next_round_coin_count` runtime field on `RoundMessage` | compile-time constant in `protocol.Spec` |
| Coin routing | `round_coin_counts` / `round_coin_offsets` / `max_round_coins` / `total_round_coins` fields on `vanishing.System` | extracted into the shared `protocol.Spec`; `vanishing.System` holds only modules and claim counts |
| Merge/eval coin location | positional — assumed the last two advance rounds (`len-3`, `len-2`) | explicit per-module `merge_coin_index` / `eval_coin_index` |
| Sub-verifier input | `rounds + claims` (transcript data mixed with math) | `ctx + claims` (coins pre-derived, math only) |

The sub-verifier contract is now: **given pre-derived coins and cell openings,
check the mathematical identity. Nothing else.**

`vanishing.verify` is a narrow, testable unit: give it a system and proof data,
it checks constraint identity. Adding a future sub-verifier (logderiv, rangecheck) follows the
same pattern — each verifier stays focused on its own identity check, while
protocol-level and codegen-level invariants are validated once, at the right
layer.

To add a new sub-verifier, only `verifier.zig` changes — see the how-to
comment at the top of that file.

## Validation Layers

Each layer validates only what it has the information and authority to check:

| Layer | Where | What it validates |
|---|---|---|
| Code generation | `BuildVanishingSystem` / `BuildCoinRouting` (Go) | Each module's `merge_coin_index` / `eval_coin_index` resolves to an in-range position in the flat coin array (`flatCoinIndex`); `round_coin_counts[0] == 0`. Fails at generation time — no bad Zig is ever emitted. |
| Protocol spec | `protocol.replay` comptime block (Zig) | `protocol.Spec` internal consistency: `round_coin_counts[0] == 0`, offsets are prefix sums, `total_round_coins` equals the sum. Fires at Zig compile time — zero runtime cost, and covers direct `replay` callers as well as `verifier.verify`. |
| Proof data | `replay` + `vanishing.verify` (Zig) | The runtime checks, because proof data is the only thing not known until runtime: round count matches the spec (`InvalidRoundCount`), claim slice lengths match (`InvalidClaimCount`), dynamic module sizes are present and valid (`MissingDynamicModuleSize`, `InvalidModuleSize`), and the quotient identity holds (`QuotientIdentityMismatch`). |

Coin index bounds are **not** re-checked in Zig: the codegen guarantees them, and re-validating generated data adds noise without adding safety.

## Aggregate Statement (`verifier.verifyPair`)

The two-proof aggregator (`-Daggregator=true`) is a guest whose claim must be stated as precisely as the single verifier's. The normative text is the "Aggregate statement" header in `src/verifier.zig`; this is the summary.

### Goal

For a fixed verifying key `VK`, the aggregator proves

```
∃ πA, πB :   Verify(VK, πA, X) = accept
         ∧   Verify(VK, πB, X) = accept
```

`X` is the one public statement both inner proofs share; `πA` and `πB` are private witnesses.

### Objects

- **`VK`** is the compiled inner verifier `S = (spec, systems)`: a comptime constant baked into the aggregator ELF, so the outer program commitment pins it. A different `VK` is a different aggregator program.
- **`X`** is the statement: the flat public-input vector `X ∈ F_{p^6}^n` in prover-ray registration order, every scalar lifted to the extension field (`Scalar.toExt`), with `n = systems.public_input.refs.len`.
- **`π`** is everything else in a `VerifyInput`: round messages, `module_sizes`, the PCS opening.
- **`Verify(VK, π, X)`** is `verifier.verify`. `X` is absorbed into `π`'s Fiat-Shamir transcript by `bindRoundMessages`, so a proof is a proof of exactly one `X`.

In relation form:

```
R_S(X, π)          ⇔  verify(S, π, X) accepts
R_pair(X; πA, πB)  ⇔  R_S(X, πA)  ∧  R_S(X, πB)          (what verifyPair decides)
```

### Guest procedure

The pair image carries a statement alongside each proof, `X_A` and `X_B`. `verifyPair` reduces them to the single `X`:

1. Reject unless `|X_A| = |X_B| = n` (`InvalidPublicInputCount`).
2. Reject unless `X_A = X_B` element-wise, compared as field values rather than wire encodings (`checkPublicInputConsistency`, `InconsistentPublicInputs`). Call the common value `X`.
3. `Verify(VK, πA, X)`.
4. `Verify(VK, πB, X)`.
5. Write `out = "lineth-verifier-pair-v1" || BE64(n) || Ext.toBytes(X[0]) || … || Ext.toBytes(X[n-1])`.

Step 2 runs first because it is O(n), so an inconsistent pair is rejected without paying for two full verifications. `Ext.toBytes` is the field's canonical encoding: 6 **big-endian** `u32` limbs in the order `B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1`, each `< p`. Every integer in `out` is big-endian — deliberately unlike the little-endian proof image, because `out` reuses the canonical field bytes already pinned by the golden tests and Poseidon2. The domain tag versions the encoding, `BE64(n)` makes it prefix-free, and the lift to the extension field makes `out` injective on the same equality relation `checkPublicInputConsistency` uses — one claim, one encoding. `test/verify_pair_test.zig` ("matches the documented byte layout") pins the exact bytes without going through `toBytes`.

### What binds what

| Component | Visibility | Bound by |
|---|---|---|
| `VK` | public | the outer program commitment: the ELF embeds `S` |
| `X` | public | `out` (the guest output) |
| `πA`, `πB` | private | nothing — they are read from the input region and never exposed |
| how `out` becomes outer public inputs (raw `guest_output` bytes vs. their keccak in `guest_output_hash`) | — | the outer arithmetization; it changes the representation of the claim, not the claim |

An accepted outer proof therefore reads: *there exist two proofs, each accepted by `VK`, of the single statement `X` whose canonical encoding is `out`.*

### Accepted and rejected pairs

Accepted:

- Two independently produced proofs of the same `X`.
- Two proofs of the same `X` that differ in proof-side data such as `module_sizes`. Proof-side data is validated per proof by `verify` and is **not** compared across the pair.
- `πA == πB`: the claim is "`X` is proven", not "proven twice".

Rejected:

- Either proof fails `Verify`, in either slot.
- `X_A ≠ X_B`, even when both proofs are individually valid. This is the case only `verifyPair` can catch: without step 2, a proof of `X` could be paired with any valid proof of an unrelated `X'`, and `out` would bind only one of them.
- Either statement has the wrong length.

### Production instance

For the compiled RISC-V system `n = 17`: the inner guest's keccak output digest (8 `u32` words as 16 limbs) followed by the message-bus accumulator cell. The pair claim is "two accepted inner executions share this output digest and bus state".

### Non-goals

- **Chained aggregation**: `X_A ≠ X_B` linked by a predicate (e.g. A's post-state is B's pre-state), with `out = f(X_A, X_B)`. That is a different relation and must take a new domain tag.
- **Cross-`VK` pairs**: statement layouts differ between systems, so equality across them has no meaning. Both proofs are checked against the same comptime `spec`/`systems` by construction.
- **Outer commitment format**: how the zkVM exposes `out` is the outer system's choice (see the table above).
