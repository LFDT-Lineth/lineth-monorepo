# Poseidon hash count

Every Poseidon2 compression in the verifier comes from one of two sources: the
Fiat-Shamir transcript, or the row hashing a FRI query performs.

| source | when paid | q=1 | q=229 |
|--------|-----------|-----|-------|
| row hashing | per query | 6,458 | ~1,478,000 |
| transcript | once per proof | 13,423 | 13,423 |

Both are driven by
the **same 10,622 committed columns** — they just weight them differently:

| | what it hashes | weighted by | elements |
|---|---|---|---|
| row hashing | the columns' committed **values** | field type: 1 per base column, 6 per extension | 25,832 per row |
| transcript | those columns **evaluated at ζ** | shift count: one claim per (column, shift), always 6 limbs | 91,416 |

Two consequences of that split are easy to get backwards:

- A **base** column still produces an **extension** claim. A claim is `f(ζ)`, and
  ζ is drawn from F_p^6 for soundness, so even a base-field polynomial evaluates
  into the extension. Cheap to row-hash, full price in the transcript.
- The claim count exceeds the column count. Claims are per (column, shift): 6,027
  columns have one shift, 4,576 have two, 19 have three — 15,236 slots from 10,622
  columns.

So batch 0 (the execution trace) is 71% of the columns but only 29% of the
row-hashing elements, while still supplying 9,892 of the 14,462 witness claims.

## Row hashing: 6,458 per query

A FRI query authenticates **row preimages**, not bare digests, so its cost
scales with the number of committed columns rather than with tree depth.

`writeRowOpeningElements` (`src/crypto/merkle.zig`) unpacks a row:

```zig
hasher.writeElements(row.base);                 // 1 element per base column
for (row.ext) |e| {
    hasher.writeElements(&.{ e.B0.a0, e.B0.a1, e.B1.a0,
                             e.B1.a1, e.B2.a0, e.B2.a1 });  // 6 per ext column
}
```

The system commits **10,622 columns** — 7,580 base and 3,042 extension — and a
query opens a **conjugate row pair** (`hashRowPair`):

```
7,580 base × 1  +  3,042 ext × 6   = 25,832 elements per row
× 2 (conjugate pair)               = 51,664
÷ 8 (MDHasher block_size)          =  6,458 compressions
```

The extension columns are 29% of the columns but 71% of the elements:

| batch | columns | % of columns | elements | % of elements |
|-------|---------|--------------|----------|---------------|
| 0 (witness) | 7,546 base | 71.0% | 7,546 | 29.2% |
| 1 (logderiv/grandproduct) | 2,268 ext | 21.4% | 13,608 | 52.7% |
| 2 (quotient) | 774 ext | 7.3% | 4,644 | 18.0% |
| 3 (precomputed) | 34 base | 0.3% | 34 | 0.1% |

### Where 10,622 columns comes from

The column list is every committed column of every batch, in prover declaration
order:

```go
for i, b := range batches {              // pcscompiler.CommittedBatches(sys)
    for _, col := range b.Round.Columns {
        desc := PcsColumnDesc{
            BatchIdx: i,
            IsExt:    col.IsExtension,
            ...
```

So the count is a property of the compiled prover-ray WIOP system, not a
verifier-side choice. It is whatever `runCompilePipeline` (nonnative →
rangecheck → lookuptologderivsum → messagebus → grandproduct → logderivativesum
→ localvanishing → global → pcs) leaves committed.

#### Batches are rounds

`CommittedBatches` (prover-ray `wiop/compilers/pcs/pcs.go`) returns one batch per
protocol round that committed columns, then appends the precomputed round:

```go
for _, r := range sys.Rounds {
    if len(r.Columns) > 0 {
        refs = append(refs, BatchRef{Round: r})
    }
}
if len(sys.PrecomputedRound.Columns) > 0 {
    refs = append(refs, BatchRef{Round: &sys.PrecomputedRound.Round, IsPrecomp: true})
}
```

The generated `batch_roots` names which round backs each batch:

```zig
batch_roots = [_]pcs.BatchRoot{
    .{ .round = 0 },        // batch 0
    .{ .round = 2 },        // batch 1
    .{ .round = 3 },        // batch 2
    .{ .precomputed = ... } // batch 3
};
```

The system has 6 rounds with coin counts `{0, 111, 0, 118, 1, 0}`. Rounds 1 and 5
draw coins but commit no columns, so they are not batches.

| batch | round | columns | field | role |
|-------|-------|---------|-------|------|
| 0 | 0 | 7,546 | base | witness (execution trace) |
| 1 | 2 | 2,268 | ext | log-derivative and grand-product running sums |
| 2 | 3 | 774 | ext | quotient shares |
| 3 | precomputed | 34 | base | compile-time constant columns |

Each batch is homogeneous — entirely base or entirely extension.

#### Batch 0 — witness, 7,546 base columns

Every committed column of the RISC-V arithmetization's execution trace:
interpreter registers, RAM accesses, opcode flags, per-instruction module
columns. Committed in round 0, before any Fiat-Shamir challenge exists (hence
`round_coin_counts[0] == 0`).

Base-field because a RISC-V trace holds ordinary integers.

#### Batch 1 — logderiv/grandproduct, 2,268 extension columns

Running sums committed in round 2, after round 1 draws its 111 coins.

These *must* be extension-field. A log-derivative argument accumulates
`Σ 1/(γ + RLC(T))` where γ is a Fiat-Shamir challenge in F_p^6; the running sum
inherits that field. Grand-product Z columns accumulating `∏(β + ...)` are the
same. Committing them in the base field would collapse the soundness argument,
since the challenge has to range over a large field.

They land in round 2 because they cannot be computed until round 1's coins exist.

#### Batch 2 — quotient shares, 774 extension columns

Created by `global.Compile` (prover-ray `wiop/compilers/global/global.go`):

```go
quotientRound := sys.NewRound()
evalRound := sys.NewRound()
```

The count matches `total_quotient_claims = 774` exactly. Each column is a share
of the PLONK quotient `Q(X) = P_agg(X) / Z_H(X)`, split because the aggregate
degree exceeds the domain size — that is the `bucket.ratio` the verifier
recombines in `verifyBucket` (`src/query/vanishing.zig`).

Extension-field because `P_agg` is built by folding constraints with the merge
coin α ∈ F_p^6. Round 3 because it needs round 3's 118 per-module merge coins;
round 4's single coin is the shared eval point `r`.

#### Batch 3 — precomputed, 34 base columns

Compile-time constant columns (lookup tables, selectors, domain constants) from
`sys.PrecomputedRound`. Its root is a literal in the generated system rather
than a proof-supplied value:

```zig
.{ .precomputed = .{ .{ .value = 301555191 }, ... } }
```

That is the security property: a precomputed batch's root is baked at codegen
time, so a prover cannot substitute different tables. The other three batches
are bound to transcript rounds instead.

## Transcript: 13,423 per proof

`replayWithTranscript` (`src/protocol/root.zig`) walks 5 round advances. Per
advance it absorbs:

1. one base element per dynamic module (`dynamic_module_count = 97`),
2. the round's commitment, when present (8 base elements),
3. every cell of the round message (1 element if base, 6 if extension),

then squeezes that round's coins. Each `randomExt` calls `sumDigest` and then
absorbs a zero element, so the buffer is never empty at the next squeeze and
every coin costs one compression.

Only rounds 2 and 4 carry cells at all — rounds 0, 1 and 3 commit columns (or
draw coins) without sending any scalar in the clear, so a committed batch's
values are revealed later as claims, not as cells. Every cell that does exist is
extension, so the table needs no base/ext split:

| round message | cells | cell elements | commitment |
|---------------|-------|---------------|------------|
| 0 | 0 | 0 | 8 |
| 1 | 0 | 0 | — |
| 2 | 2,269 | 13,614 | 8 |
| 3 | 0 | 0 | 8 |
| 4 | 15,236 | 91,416 | — |

Three rounds carry a commitment (0, 2 and 3 — the rounds `batch_roots` names),
contributing 24 elements.

```
cells                                = 105,030 elements
commitments (3 × 8)                  =      24
dynamic-module sizes (97 × 5 rounds) =     485
total absorbed (summing above)       = 105,539
÷ 8 (MDHasher block_size)            =  13,193 compressions
+ 230 coin squeezes                  =     230
                                       -------
transcript total                     =  13,423 compressions
```

Round 4 alone is 87% of the absorbed elements.

### Where the round-2 and round-4 cells come from

**Round 4 — 15,236 cells** are the PCS claim cells (`total_claim_slots`), one
per opened (column, shift) pair, built by `pcsShiftClaimCells`
(`codegen/pcs.go`) from `sys.LagrangeEvals`. Summing each column's shifts over
the same 10,622 columns the row hashing walks gives exactly that total:

| batch | columns | claim slots |
|-------|---------|-------------|
| 0 (witness/trace) | 7,546 | 9,892 |
| 1 (logderiv/grandproduct) | 2,268 | 4,536 |
| 2 (quotient) | 774 | 774 |
| 3 (precomputed) | 34 | 34 |
| total | 10,622 | 15,236 |

The generated system splits the same 15,236 a second way, by role rather than by
batch:

```
total_witness_claims  = 14,462   (batches 0, 1 and 3 — i.e. everything non-quotient)
total_quotient_claims =    774   (batch 2)
                        ------
                        15,236
```

Note that `total_witness_claims` does **not** mean batch 0. "Witness" here is the
PCS's own split between the thing being proven and the quotient that proves it,
so it spans three batches; batch 0's own contribution is 9,892 of that 14,462.

They land in round 4 because every claim is an opening at the shared eval point
`r`, which is round 4's single coin.

**Round 2 — 2,269 cells** come from the two accumulation queries, one
log-derivative and one grand-product. Their cell indices tile the round:

| index | contents | count |
|-------|----------|-------|
| 0 | logderiv `result_ref` | 1 |
| 1 | grandproduct `result_ref` | 1 |
| 2–4 | grandproduct `z_final_refs` | 3 |
| 5–2,269 | logderiv `z_final_refs` | 2,265 |

Each query contributes a result cell (the claimed final sum/product) plus the
last row of each running-sum column, which is what the check reads.

Note that `round_cell_counts[2]` is 2,270 — one more than the proof's 2,269 —
because the spec value is the highest referenced index plus one, i.e. a
capacity. The arithmetic above uses the measured 2,269.

## Ideas on reducing the cost

Per-query hashing is dominated by the **extension** columns: batches 1 and 2 are
29% of the columns but 71% of the hashed elements, purely from the 6x limb
unpacking. So the levers all reduce extension columns; nothing here touches tree
depth, and shrinking the base-field witness buys at most its 29% share.

Two upstream quantities set those counts:

- **Z columns (2,265)** scale with the number of lookup/permutation *fractions*,
  packed `packingArity` per column.
- **Quotient shares (774)** scale with constraint *degree*: they are exactly
  `sum(bucket.ratio)` over all buckets.

**None of the levers below have been implemented or measured.** The figures are
arithmetic projections: they take the current column counts, apply the stated
change, and re-run the element/compression formula from the top of this
document. They are not benchmark results, and the `packingArity` rows in
particular ignore a known feedback effect (see below), so they read high.

| lever | per-query compressions (estimated) | change (estimated) |
|-------|------------------------------------|--------------------|
| current — **measured** | 6,458 | — |
| `packingArity` 3 → 4 | ~5,600 | ~−13% |
| `packingArity` 3 → 6 | ~4,800 | ~−26% |
| halve the ratio-4 buckets | ~6,200 | ~−5% |
| arity 6 + halved ratio-4 | ~4,500 | ~−31% |
| floor: zero extension columns | 1,895 | −71% |

Only the first and last rows are exact: the current cost is measured, and the
floor is what the 7,580 base columns alone would cost.

### 1. Raise `packingArity`

`packingArity = 3` is a hardcoded constant in prover-ray
(`wiop/compilers/logderivativesum/logderivativesum.go`), and its comment notes
the value "matches the linea/logderivativesum compiler" — it is inherited, not
tuned for this system. It caps how many fractions pack into one Z column, so
Z columns ≈ fractions / arity, and 2,265 × 3 ≈ 6,795 fractions.

The catch is that the Z recurrence multiplies the packed denominators, so the
constraint degree grows roughly with the arity. That can push buckets into a
higher `ratio` and add quotient shares back. The table above assumes no such
feedback, so treat it as an upper bound: the real effect needs a recompile and a
fresh ratio histogram. Which arity actually wins is unknown until that is run —
the give-back could be small or could cancel the saving outright.

Testing it is cheap: change the constant, regenerate, and compare the new column
counts and ratio histogram against the ones in this document.

### 2. Lower constraint degree

Quotient shares are exactly `sum(ratio × count)`. This histogram is read from
the current generated system, so unlike the projections above it is **measured**:

| ratio | buckets | shares | % of 774 |
|-------|---------|--------|----------|
| 1 | 118 | 118 | 15% |
| 2 | 112 | 224 | 29% |
| **4** | **98** | **392** | **51%** |
| 8 | 5 | 40 | 5% |

**98 buckets at ratio 4 produce half the quotient columns**, so they are the
targeted fix. Degree reduction usually means adding intermediate witness
columns, which are base-field (1 element against the share's 6), so the trade is
favourable even before the share count drops.

### 3. Merge lookup arguments

Fewer distinct `LogDerivativeSum` queries, or range-check tables shared across
modules, cut the fraction count at the source. Unlike packing, this costs no
constraint degree.

### Scope

All three are prover-ray compile-pipeline decisions, upstream of verifier-ray.

They also mostly help prover time and proof size rather than the zkc cycle
budget: with the Poseidon2 accelerator enabled (the default) each compression is
a single zkVM instruction, so even 1.48M of them at `num_queries = 229` stays
well under 1% of the guest's interpreted cycles. `system_decode` dominates that
budget — see `docs/verifier-profiling.md`.
