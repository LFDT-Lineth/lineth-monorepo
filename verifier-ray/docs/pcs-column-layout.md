# PCS Column Layout and Per-Query Hashing Cost

This note explains where the PCS system's committed columns come from, why they
are split into four batches, and how that layout determines the Poseidon2
hashing cost of a single FRI query.

The short version: a query's hashing cost scales with **column count weighted by
field type**, not with Merkle tree depth. Extension-field columns cost 6x a base
column, and the two extension batches are 29% of the columns but 71% of the
hashing.

All figures below are for the real RISC-V system
(`testdata/generated/riscv_system.zig`, all-in-one guest).

## Where `max_entries` Comes From

`pcs.System.max_entries` is `len(columns)` (`codegen/pcs.go`, `BuildPcsSystem`).
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

For the current system that is **10,622 columns**: 7,580 base and 3,042
extension.

## Batches Are Rounds

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

## The Four Batches

| batch | round | columns | field | role |
|-------|-------|---------|-------|------|
| 0 | 0 | 7,546 | base | witness (execution trace) |
| 1 | 2 | 2,268 | ext | log-derivative and grand-product running sums |
| 2 | 3 | 774 | ext | quotient shares |
| 3 | precomputed | 34 | base | compile-time constant columns |

Each batch is homogeneous — entirely base or entirely extension.

### Batch 0 — witness, 7,546 base columns

Every committed column of the RISC-V arithmetization's execution trace:
interpreter registers, RAM accesses, opcode flags, per-instruction module
columns. Committed in round 0, before any Fiat-Shamir challenge exists (hence
`round_coin_counts[0] == 0`).

Base-field because a RISC-V trace holds ordinary integers.

### Batch 1 — logderiv/grandproduct, 2,268 extension columns

Running sums committed in round 2, after round 1 draws its 111 coins.

These *must* be extension-field. A log-derivative argument accumulates
`Σ 1/(γ + RLC(T))` where γ is a Fiat-Shamir challenge in F_p^6; the running sum
inherits that field. Grand-product Z columns accumulating `∏(β + ...)` are the
same. Committing them in the base field would collapse the soundness argument,
since the challenge has to range over a large field.

They land in round 2 because they cannot be computed until round 1's coins exist.

### Batch 2 — quotient shares, 774 extension columns

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

### Batch 3 — precomputed, 34 base columns

Compile-time constant columns (lookup tables, selectors, domain constants) from
`sys.PrecomputedRound`. Its root is a literal in the generated system rather
than a proof-supplied value:

```zig
.{ .precomputed = .{ .{ .value = 301555191 }, ... } }
```

That is the security property: a precomputed batch's root is baked at codegen
time, so a prover cannot substitute different tables. The other three batches
are bound to transcript rounds instead.

## Per-Query Hashing Cost

A FRI query authenticates **row preimages**, not bare digests. The cost
therefore scales with column count, not tree height.

`writeRowOpeningElements` (`src/crypto/merkle.zig`) unpacks each row:

```zig
hasher.writeElements(row.base);                 // 1 element per base column
for (row.ext) |e| {
    hasher.writeElements(&.{ e.B0.a0, e.B0.a1, e.B1.a0,
                             e.B1.a1, e.B2.a0, e.B2.a1 });  // 6 per ext column
}
```

`Ext` is F_p^6 = 3 × `E2`, each `E2` being 2 base elements, so 6 KoalaBear limbs
per extension element.

`MDHasher` (`src/crypto/poseidon2.zig`) is a sponge that fires one Poseidon2
permutation per full `block_size = 8` element block.

A query opens a **conjugate row pair** (`hashRowPair`), so:

```
7,580 base × 1  +  3,042 ext × 6   = 25,832 elements per row
× 2 (conjugate pair)               = 51,664
÷ 8 (MDHasher block_size)          =  6,458 compressions
```

Measured against the instrumented `profiling.poseidon2_compress` counter:
**~6,000 per query**. The ~7% overshoot in the derivation is accounted for by
the multi-size tree (not every column appears at every level), the bottom level
hashing single rows rather than pairs, and partial 8-element tails flushing once
per row rather than per column.

### Element cost by batch

| batch | columns | % of columns | elements | % of elements |
|-------|---------|--------------|----------|---------------|
| 0 (witness) | 7,546 | 71.0% | 7,546 | 29.2% |
| 1 (logderiv/grandproduct) | 2,268 | 21.4% | 13,608 | 52.7% |
| 2 (quotient) | 774 | 7.3% | 4,644 | 18.0% |
| 3 (precomputed) | 34 | 0.3% | 34 | 0.1% |

**Batches 1 and 2 are 28.7% of the columns but 70.7% of the hashing.** The
7,546-column witness — the batch that looks largest — is the cheap part.

## Two Modelling Traps

**Do not estimate query hashing from tree depth.** A `queries × rounds × depth`
formula describes the *running-layer* path, where each step hashes two 8-element
digests. After Merkle capping that path is only ~112 compressions per query —
roughly 1/50th of the real cost. The input-tree path dominates, and it scales
with columns.

**Merkle capping and per-round shrinkage both reduce path length.**
`merkle.capDepth(num_queries, height)` truncates the top `ceil(log2(num_queries))`
levels, and `fri.zig` computes `height = log_codeword_size - j`, so round `j`
authenticates a tree of height `23 - j`, not a constant 23.

## If Per-Query Hashing Needs to Shrink

The lever is reducing **extension-field commitments**, not tree depth:

- fewer log-derivative / grand-product arguments, or
- more lookups merged per Z column.

Both are prover-ray compile-pipeline decisions, upstream of verifier-ray.

For context on where hashing sits in overall verifier cost: with the Poseidon2
accelerator enabled (`DISABLE_ACCELERATORS=false`, the default), each compression
is a single zkVM instruction, so hashing is a small fraction of the guest's total
interpreted cycles. See `docs/verifier-profiling.md` for measuring phase costs
through zkc.
