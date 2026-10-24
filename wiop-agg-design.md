# Design: `wiop-agg` — a standalone zkc 2-to-1 recursive aggregator

Status: design proposal, 2026-09-23; re-measured 2026-09-24 against
`ad/zkc-recursion-experiment` after merging `main` (`5ab76abb8`), which brings in
PRs #3940/#3950/#3959. Supersedes `verifier-ray-zkc-plan.md` for the *aggregation*
use case; that document remains valid for the R5-accelerator use case.

**The 1-to-1 step has its own plan.** `verifier-ray-zkc/PLAN.md` is the implementation
plan for the compressor: standalone zkc program, flat felt inputs, baked R5 key,
~5–7× compression, single shard at today's frozen FRI parameters. This document
remains the design for the 2-to-1 aggregator on top of it, and §9a here is the
compression analysis that plan summarises.

**Re-measurement note.** The merge changed nothing in the row budget: every PCS,
vanishing and transcript figure in §0 is byte-identical before and after. It changed
one thing in the aggregation semantics, and changed it decisively — the
shared-randomness contribution is now inert (§6.1). Sections 1 to 5 stand as written;
section 6 was rewritten.

This answers: can a **pure zkc program**, with no R5 VM underneath, take two WIOP
shard proofs as inputs, verify both, and emit one WIOP proof — inside a single
shard (2^22 rows)?

**Short answer: not at today's FRI parameters, and the gap is 6×, not 20%. The
blocker is measured, not estimated. Section 1 gives the numbers, section 2 the
four ways out, section 3 the recommended configuration. Everything after that is
the design proper, which is largely parameter-independent.**

---

## 0. The measurement that decides everything

Run today against `origin/main` + PRs #3940/#3950/#3959, by compiling the real
`arithmetization/src/main/riscv/main.zkc` through the full wiop compiler pipeline.
No proving required, so this needs no working prover — it takes about one second.
The probe is committed as `verifier-ray/codegen/shape_probe_test.go`
(`go test -run TestR5SystemShape ./codegen -v`).

```
rounds                   5          total coins              230
dynamic modules          97         public inputs            337
transcript cells         17,842     (per round: 8, 328, 2270, 0, 15236)

--- PCS ---
num_queries              229        log codeword / plaintext  23 / 22   (rate 1/2)
batches                  4          final poly size log2      0
committed columns        10,622     (7,580 base + 3,042 ext)
OPENED FELT WIDTH        25,832     <-- felts in one full row set
(column, shift) pairs    15,236
witness / quotient claims 14,462 / 774

--- VANISHING ---
modules                  118        expression nodes          482,341
buckets / constraints    333 / 19,123                (231,748 of them ops)
max nodes in one module  58,797     cancelled positions       8,358

--- SCALAR CHECKS ---
logderiv queries / refs  1 / 2,265  rowlimit checks / modules 139 / 6,668
grandprod queries / refs 1 / 3      shared-randomness refs    328
gp[0] hasExpected=true expected=1   ld[0] resultIsZero=true
shared-randomness commitment round 1, hasCommitment = FALSE      <-- see §6.1
```

Two of these numbers govern the whole design:

- **`num_queries = 229`** — a consequence of rate 1/2 under *provable* (Johnson-bound)
  FRI soundness, where each query is worth `log₂(1/√ρ) = 0.5` bits, so 128 bits needs
  ~229 queries (`prover-ray/wiop/compilers/pcs/pcs.go:52-59` cites ethereum/soundcalc).
- **`opened felt width = 25,832`** — every committed column is opened at every query,
  because the DEEP quotient `Φ_N` batches all of them. This is a property of the R5
  arithmetization's no-CPU design (118 modules, 10,622 columns) and is ~50× wider than
  a typical monolithic STARK.

They multiply. That product is the aggregator's cost.

---

## 1. Row budget: why it does not fit today

### 1.1 What "fits in a single shard" means precisely

`wiop.ColumnSizeMaxSupported = 2^22 = 4,194,304` is a **per-module** ceiling: each
zkc function becomes one AIR module whose height is its invocation count, and each
such height must be ≤ 2^22. Total *cells* (Σ height × width) can be far larger; they
cost prover time, not feasibility.

So the constraint is: **no zkc function in the aggregator may be called more than
4.19M times, and no input memory may hold more than 4.19M entries.** With
`--padding next-power-of-two`, anything over 2^21 pads to the full 2^22, so the real
working target is ~2–3M.

The 2^22 figure is itself forced by two-adicity. KoalaBear `p − 1 = 2^24 · 127`, so
the largest power-of-two subgroup is 2^24 and the FRI codeword can never exceed it
(`field/koalabear.zig:4`, `max_order_root = 24`). At rate 1/2 the codeword is 2^23,
leaving exactly one spare bit — which section 2 spends.

### 1.2 Cost per verified proof, as a formula

Let `Q` = queries, `C` = committed columns (10,622), `S` = (column, shift) pairs
(15,236), `W` = opened felt width (25,832), `E` = expression nodes (482,341).

| Work | Rows, per proof | Scales with |
|---|---|---|
| Read opened row data | `Q · 2 · W` | Q · W |
| Poseidon2 compressions | `Q · (2W/8 + ~196)` | Q · W |
| `ext_mul` (DEEP terms + Horner) | `Q · 2 · (C + S)` | Q · C |
| `ext_sub` / `ext_add` | `Q · 2 · (C + S)` | Q · C |
| `ext_inv` (denominators, cached per size × shift) | `Q · 2 · ~60` | Q |
| FRI fold arithmetic | `Q · 22 · ~4` | Q |
| Transcript element writes | `~91,400` | fixed |
| `eval_expr` (vanishing DAG) | `E = 482,341` | fixed |
| Claim routing, layout, scalar checks | `~40,000` | fixed |

The `~196` per query is Merkle node hashing: 4 trees × 14 sibling levels below a
cap of depth `capDepth(229, 23) = 8`, plus 112 running-layer siblings summed over
the 21 fold rounds.

A 2-to-1 node pays all of this **twice**.

### 1.3 The verdict

| | Q = 229 (today) | Q = 64 | Q = 32 | Q = 27 | budget |
|---|---|---|---|---|---|
| `ext_mul` rows, 2 proofs | **23.7M** | 6.6M | 3.31M | 2.79M | 4.19M |
| opened-felt reads, 2 proofs | **23.7M** | 6.6M | 3.31M | 2.79M | 4.19M |
| Poseidon2 rows, 2 proofs | **3.05M** | 852k | 426k | 359k | 4.19M |
| `eval_expr` rows, 2 proofs | 965k | 965k | 965k | 965k | 4.19M |
| input memory entries, 2 proofs | **23.7M** | 6.6M | 3.31M | 2.79M | 4.19M each |
| proof size, each | **47 MB** | 13 MB | 6.6 MB | 5.6 MB | — |

At today's parameters the aggregator is **5.7× over budget** on three independent
modules at once. The 47 MB proof size is the same fact seen from the other side: it
is `Q · 2 · W · 4 bytes`, and it is exactly what the verifier must read.

**The rows in this table assume one row per extension multiply and one row per felt
read, which is the naive structure, not the best one.** Both are batchable by
unrolling the inner loop `k` ways, at a width cost that is small because those
functions are narrow. That does not rescue the 2-to-1 node, which would need
`k ≈ 6` and would push the batched module's width past the point where it pays.
It does rescue the **1-to-1 shrink**, which needs only `k ≈ 3` or `4` and therefore
fits a single shard at today's parameters. See §9a, which supersedes the
"2.8× over" figure this paragraph used to carry.

`Q · (C + S) ≤ ~350,000` is the whole feasibility condition for a 2-to-1 node.
At `C + S = 25,858` that is **Q ≤ 27**, or **Q ≤ 40** if you are willing to run at
79% of a padded 2^22 module with no headroom.

Note what is *not* the problem: the vanishing DAG (965k rows for both proofs,
independent of Q) and the transcript are comfortable. Poseidon2 is comfortable once
Q drops. **The single dominant term is `Q × committed width`, in three modules
simultaneously.**

---

## 2. The four levers

### 2.1 Lever A — spend the spare two-adicity bit (free, 2×)

Rate 1/2 puts the codeword at 2^23 against a 2^24 ceiling. Moving to **rate 1/4 at
the same 2^22 plaintext** costs the prover one extra FFT doubling and no shard-size
reduction at all. Provable soundness doubles from 0.5 to 1.0 bits per query, so
Q drops 229 → ~128.

Not sufficient alone, but it is the only lever with no downside.

### 2.2 Lever B — grinding (proof-of-work)

Adding `g` bits of grinding subtracts `g` from the query requirement outright. The
verifier cost is one extra Poseidon2 compression and a range check; the prover cost
is 2^g hashes (48 bits is seconds on a machine that is about to spend minutes
proving). This is standard in every recursion-oriented STARK.

It requires a prover-ray protocol change: a nonce absorbed into the transcript
after the final-polynomial absorption and before query derivation, plus the
corresponding check in every verifier.

### 2.3 Lever C — rate versus shard size

Each further rate halving costs one bit of plaintext, because
`plaintext × blowup ≤ 2^24`:

| rate | max plaintext | shards vs today | provable bits/query | conjectured bits/query |
|---|---|---|---|---|
| 1/2 (today) | 2^22 | 1× | 0.5 | 1 |
| 1/4 | 2^22 (free) | 1× | 1.0 | 2 |
| 1/8 | 2^21 | 2× | 1.5 | 3 |
| 1/16 | 2^20 | 4× | 2.0 | 4 |
| 1/32 | 2^19 | 8× | 2.5 | 5 |
| 1/64 | 2^18 | 16× | 3.0 | 6 |

Queries needed for 128 bits with 48-bit grinding, i.e. `Q = 80 / (bits per query)`:

| rate | Q provable | Q conjectured |
|---|---|---|
| 1/4 | 80 | 40 |
| 1/8 | 54 | **27** |
| 1/16 | 40 | 20 |
| 1/32 | 32 | 16 |
| 1/64 | **27** | 14 |

Smaller shards are not free but they are cheap: they mean more leaves and one extra
aggregation level per halving, and every level above the first is inexpensive
(section 5.4).

### 2.4 Lever D — accept a multi-shard first level

If shard FRI parameters cannot change, the first aggregation level is inherently
3–6 shards wide. That is not fatal: the *output* of level 1 is a proof of the
aggregator circuit, which is 4–8× narrower than an R5 shard proof (section 5.4), so
compression still happens — 47 MB in, ~1 MB out. Every level above is single-shard
2-to-1.

The cost is that level 1 needs its own internal aggregation (3–6 shards → 1), which
is two extra tree levels, and the level-1 shards must be wired together by the
message bus. It is strictly more machinery than levers A–C.

---

## 3. Recommended configuration

**Aggregation layer and shard layer get different FRI parameters.** They have very
different widths, so one setting cannot serve both.

| | shard layer (R5) | aggregation layer (`wiop-agg`) |
|---|---|---|
| rate | **1/8** | 1/8 |
| max plaintext | 2^21 (2M rows/shard) | 2^21 |
| grinding | **48 bits** | 48 bits |
| queries | **27** (conjectured) / 54 (provable) | 32 |
| committed width | 25,832 felts | ~3,000–6,000 felts (estimate, section 5.4) |
| proof size | ~5.6 MB | ~0.5–1 MB |

With shard `Q = 27`, a 2-to-1 node costs **2.79M rows** in its hottest module — 67%
of a padded 2^22 shard. That fits, with the caveat that it fits *once*: there is no
room to also widen R5.

If the team will not accept conjectured soundness, the same Q needs rate 1/64 and
2^18-row shards (16× more shards, 4 extra tree levels). That is a real cost but it
is a scheduling cost, not a feasibility cost. **This is the single decision that
should be made before any code is written**, because at 229 queries no amount of
zkc optimisation closes a 5.7× gap.

A parameter-independent second recommendation: **the first level should be 1-to-1
(a "shrink") if `Q · (C + S)` lands above ~175,000.** It halves the hardest level's
cost for one extra proof per shard, and the tree above is unaffected.

At today's frozen parameters that condition is met by a wide margin, so the first
level is 1-to-1. It fits one shard, and it compresses about 5× to 7×; §9a works
this out and is the section to read if the FRI parameters are not going to move
soon.

---

## 4. Why this is a better target than the accelerator design

Dropping the R5 VM removes three whole layers that `verifier-ray-zkc-plan.md` had to
plan around:

- **No image loading.** `riscv/main.zkc` copies every blob byte into `ram` with
  `write_8` before execution. For a 5.6 MB proof that is 5.6M rows of pure
  marshalling, over the 2^22 budget on its own. Reading felts from an `input` memory
  costs one lookup per felt and no copy.
- **No pointer ABI.** The Zig `proof_abi.zig` layout — absolute pointers, slice
  headers, union tags, `?RowPair` presence flags, little-endian byte assembly — exists
  only because the Zig verifier casts guest memory. A zkc program defines its own
  input layout (section 6) and can drop every tag the verification key already knows.
- **No interpreter dispatch, no `decoded` table, no predecoding.**

It also gains one thing that matters specifically for recursion: **untaken branches
cost zero rows in zkc.** A single uniform node can contain both "child is a shard
proof" and "child is an aggregate proof" paths and pay only for the one taken. In a
fixed-circuit SNARK both paths are always paid for; gnark's own BLS pairing
precompile is deliberately sub-optimal for exactly this reason. This is what makes a
single self-verifying program practical here.

---

## 5. Program architecture

### 5.1 One uniform node

One program, `wiop-agg`, verifying two children, each independently a shard proof or
an aggregate proof:

```
wiop-agg(left, right) -> proof
  ├ verify(left)   against  vk_leaf  if left.kind  == LEAF  else vk_node
  ├ verify(right)  against  vk_leaf  if right.kind == LEAF  else vk_node
  ├ check the two children compose (ranges, γ, vk propagation)
  ├ combine their deferred cross-shard state
  └ publish the combined statement
```

`fail` is the right failure mode here, not a status code. An aggregator that cannot
verify its children must not produce a proof at all, which is the opposite of the
precompile convention in PR #3927.

### 5.2 Breaking the verification-key circularity

A self-verifying program cannot bake its own verification key: the key commits to the
program, which would contain the key. The standard resolution (Plonky2 calls it
cyclic recursion) is:

- `vk_node` is supplied as **input data** and hashed in-circuit to `H(vk_node)`.
- `vk_self_digest` is a **public input**, propagated unchanged from children to parent.
- The node checks `H(vk_node) == vk_self_digest`, and that each recursive child's own
  `vk_self_digest` public input equals its own.
- The node never learns its own digest. The **final verifier** (L1 contract, or a
  gnark wrapper) pins `vk_self_digest == H(vk_agg)` once, at the root.

`vk_leaf` (the R5 shard system) is fixed once R5 is frozen, so it can be baked as
`static` tables — but propagating it as a public input too costs almost nothing and
decouples the aggregator from R5 revisions. Recommended: **propagate both, bake
neither.**

Hashing the R5 vk is not free: 482k expression nodes plus 10,622 column descriptors
is roughly 600k felts → ~75k compressions. That is affordable, and it buys
uniformity. Cache it: hash once even though both children may reference it.

### 5.3 Ragged trees

Left and right may be at different levels only if the node can hold two different
vks as data. Simplest is to **pad the tree to a perfect binary tree** with identity
nodes (a node that verifies one child and republishes its statement). At 2^k leaves
the padding is at most one extra proof per level.

### 5.4 Convergence

Recursion converges only if the aggregator's own committed width is not larger than
what it verifies. Rough estimate for `wiop-agg`:

| module | width (columns) | note |
|---|---|---|
| `p2_perm` (straight-line Poseidon2) | ~1,350 | 27 rounds × 16 lanes of intermediates |
| `ext_inv` | ~300 | one base Fermat chain + tower norm |
| `ext_mul`, `ext_sqr` | ~60 each | Karatsuba, 6 E2 products |
| `eval_expr`, readers, layout, folds | ~40 each × ~40 functions | |
| **total committed width** | **~3,000–6,000 felts** | vs R5's 25,832 |

So an aggregate proof is roughly **4–8× narrower** than a shard proof, and level 2
onward costs `Q · 2 · ~5,000 · 2 ≈ Q · 20,000` rows — comfortable at any Q ≤ 64.
**The first level is the only hard one.** This is the load-bearing assumption of the
whole design and section 9 makes measuring it the first task.

It also yields a design rule that is specific to recursion and counter-intuitive
elsewhere: **total committed width is the sum over distinct modules, so the next
level pays per query for every function you write, but nothing for how often you
call it.** Prefer few, heavily reused functions. Splitting a wide function into
three narrower ones does not help — it adds interface registers and leaves the sum
unchanged.

---

## 6. What a node actually aggregates

Verifying two proofs is the easy half. The reason this is an *aggregator* and not
two verifiers side by side is the cross-shard state, which the earlier accelerator
plan did not cover at all.

### 6.1 Shared randomness is scaffolded but currently binds nothing

An earlier draft of this section called the cross-shard binding "fully specified and
cheap". The merge of `main` shows that is wrong, and the probe says so in one line:

```
shared-randomness refs 328 (commitment round 1, hasCommitment = false)
```

The mechanism is all still present. Every shard proof carries two public-input groups
(`messagebus/shared_randomness.go:15-16`), and together they are 336 of the system's
337 public inputs:

| group | limbs | round | probe's per-round cell count |
|---|---|---|---|
| `SharedRandomnessSeedPI` (γ) | 8 | 0 | 8 |
| `SharedRandomnessSeedContributionPI` | 328 | 1 | 328 |

The contribution is defined as `multisetHash(rt.Commitments[coinRound.ID])`
(`shared_randomness.go:152-163`) — the coin round's own commitment, and nothing else.
**In the R5 system as it compiles today, the coin round has no commitment.** The Go
map lookup misses and yields the zero octuplet, prover-ray emits
`logrus.Warnf("No commitment found for round ...")`, and the Zig checker deliberately
mirrors the miss with `@splat(field.Element.zero())`
(`query/shared_randomness.zig:63-68`). Both sides therefore compute
`multisetHash(0)`, a compile-time constant, and `shared_randomness.verify` passes for
every proof while binding none of the shard's data.

There is a second signal saying the same thing. `messagebus.Compile` panics if any bus
participant column lives off the coin round, on the grounds that "a column off the coin
round is not covered by this shard's contribution, so γ would not bind it"
(`messagebus.go:142-155`). That guard passes, and the coin round is uncommitted, which
together mean **the R5 system has no bus participant columns at all**. The message bus
is compiled in and empty.

Consequences for this design, in order of importance:

- **The row budget is unaffected.** Combining 328 limbs was ~328 additions out of
  millions of rows. Sections 1 to 5 do not move.
- **There is nothing for a node to aggregate yet.** Combining two constants gives a
  constant. The `contribution` slot in the statement (§6.3) stays in the layout, but
  the identity that would give it meaning does not exist.
- **The root check cannot be written.** `ToSeed(Σ contributions) == γ` is only a real
  constraint once contributions actually depend on shard data. Today it would compare
  `ToSeed(n · Hash(0))` against a γ that nothing derived.
- **γ has no consumer.** With the Fiat-Shamir pre-sampling hook removed by #3950, γ is
  a public input that no constraint reads. `preflight/preflight.go:48-53` still derives
  a seed as `ToSeed(Σ Hash(tree.Root()))`, but over preflight column-set trees, which is
  a different path from the PCS commitment the contribution hashes. Those two need to be
  reconciled before either can be checked in-circuit.

So cross-shard binding moves from "solved, implement it" to **a blocking upstream
dependency**. It does not threaten the row budget or the program architecture, and the
aggregator can be built and measured against single-shard proofs without it. It does
mean the aggregator is not yet sound *as a cross-shard aggregator*, only as a proof
compressor, and shipping it as the former before this lands would be a soundness bug,
not a missing feature.

### 6.2 Message-bus handles — same story, same cause

```
gp[0] hasExpected=true expected=1 zrefs=3      <- in-shard permutation check
ld[0] resultIsZero=true      zrefs=2265        <- in-shard lookup check
```

Both bus checks are discharged in-shard, which is the same fact as §6.1 seen from the
query side: an empty bus has nothing to defer. The deferral path exists
(`HasExpected == false` means "skipped in favour of a downstream cross-shard layer",
`codegen/grandproduct.go:45-49`) but is not exercised by any R5 compilation.

A node's handle-combination rule — product for grand-product handles, sum for
log-derivative handles, identity at the root — can be designed, but the concrete
combiner cannot be pinned down until the multi-shard messagebus configuration lands.
Keep a defined slot in the statement and settle it with the prover-ray team.

### 6.3 The node statement

Public inputs of `wiop-agg`, all propagated or combined:

| Field | Rule at a node | Root check |
|---|---|---|
| `vk_self_digest[8]` | equal in both children; republished | `== H(vk_agg)` |
| `vk_leaf_digest[8]` | equal in both children; republished | `== H(vk_r5)` |
| `gamma[8]` | `left == right`; republished | `== ToSeed(contribution)`, once §6.1 lands |
| `contribution[328]` | `left + right` | feeds the γ check above; **inert today**, §6.1 |
| `bus_acc[...]` | combiner per §6.2 | `== identity`; **unspecified today** |
| `shard_range` (start, end) | `left.end + 1 == right.start`; publish `(left.start, right.end)` | covers every shard |
| `statement_digest[8]` | `H(left ‖ right)` | matches the rollup statement |

`statement_digest` is what carries the actual rollup claim (block range, state roots,
the `RollupOutput` fields the stub guest in `riscv-guests/rollup/` enumerates) up the
tree without the aggregator understanding them. Only the root verifier opens it.

---

## 7. Input format

Because this is not an R5 guest, nothing forces the `proof_abi.zig` pointer image.
Define a flat, dense, felt-addressed format instead and drop everything the
verification key already implies.

```zkc
// one set of memories, both children concatenated; header gives per-child bases
input agg_hdr(i:u8)      -> (v:u64)     // counts, log-sizes, per-child base offsets
input agg_rows(i:u23)    -> (v:𝔽)       // opened row data, self then conjugate   <- the bulk
input agg_dig(i:u20)     -> (d0..d7:𝔽)  // Merkle siblings, caps, roots, commitments
input agg_cells(i:u18)   -> (e0..e5:𝔽)  // transcript cells (always 6 limbs)
input agg_sizes(i:u8)    -> (v:u32)     // per-proof dynamic module sizes
input agg_vk(i:u20)      -> (a:u32, b:u32, c:u32, d:u32)   // the node verification key
```

Four things the Zig image carries that this format drops, because the vk fixes them:

- **`Scalar` base/ext tags.** The cell kind is a static property of the system
  (`proofserialization/README.md` §11.5 notes prover-ray does not even validate it).
  Store every cell as 6 limbs, or as 1 limb where the vk says base.
- **`?RowPair` presence flags.** Presence is a function of cap depth and level, which
  the verifier derives anyway (`pcs.zig:567-581`).
- **Slice headers and absolute pointers.** Offsets come from the header and from
  counts the vk already implies.
- **Struct padding.**

Two constraints on the layout:

- **A zkc memory cannot be selected at runtime**, so the two children share one set of
  memories with a per-child base offset, not two sets. Module height is therefore the
  sum over both proofs and must stay under 2^22 — one more reason the 2.79M figure in
  §1.3 is the real ceiling.
- **If a single array exceeds 2^22, split it across `agg_rows_0`, `agg_rows_1`, …**
  and dispatch on a high bit in an `#[inline]` reader. Each memory is its own module,
  so K memories buy K × 2^22 entries. The *reads* still all land in the reading
  function's module, so this raises the storage ceiling, not the work ceiling.

Whether an input memory's module height is the data provided or only the addresses
actually read is the one behaviour I did not confirm in the sources; §9 makes it the
first probe. It barely matters in practice here, since the verifier reads essentially
every opened felt.

---

## 8. zkc implementation notes specific to this program

Everything in `verifier-ray-zkc-plan.md` §3–§5 (straight-line fixed-size work,
recursion instead of loops, memory layouts, the Poseidon2 and extension-field ports,
the counting-sort layout reconstruction, the per-query DEEP walk) carries over
unchanged. Four things change or are new:

**Do not batch-invert.** The CPU instinct is Montgomery batch inversion: `n`
inversions become 1 inversion plus `3(n−1)` multiplications. In zkc that is strictly
worse, because a straight-line `ext_inv` is **one row** just like `ext_mul`, so the
batch trades `n` rows for `1 + 3(n−1)` rows. Call `ext_inv` directly. (Batching still
saves *cells*, about 40%; if prover time rather than the row ceiling turns out to
bind, revisit.)

**Do cache the DEEP denominators.** `1/(x − ζ·ω_N^shift)` depends only on
(query, conjugate, size, shift value), never on the column. Computing it per
(entry, shift) would be `Q · 2 · 15,236` inversions per proof; hoisting it to
per (size, shift) makes it `Q · 2 · ~60`. This is a 250× reduction on that term and
is not optional.

**Leaf nodes in the expression DAG are worth inlining.** Of the 482,341 expression
nodes, 231,748 are operators and the remaining ~250,000 are leaves (column claim,
cell, coin, constant, selector). Evaluating a leaf through the same recursive
`eval_expr` call as an operator costs a row each. Reading a leaf operand directly
inside the operator's own row roughly halves the module height, 965k → ~465k for two
proofs.

**`fail`, not a status code.** The whole program is the statement "both children
verified". Anything that would make the Zig verifier return an error becomes `fail`,
which makes the AIR unsatisfiable. That removes the `ok:u1` threading through every
signature that the accelerator plan needed, and with it the multi-return conditional
hazard recorded in project memory.

---

## 9. What to do first

Four measurements, in order. The first three need no new zkc code and no prover.

1. **Confirm the input-memory height rule.** A ten-line zkc program with a large
   `input` memory of which one entry is read, under `zkc trace --stats`. Decides
   whether §7's split-across-memories escape hatch is needed and at what threshold.
2. **Cost-probe the hot functions** — the Phase 0 table in
   `verifier-ray-zkc-plan.md` §9.4, plus specifically `ext_inv` versus a
   3-multiplication batch step, to confirm the no-batch-inversion rule.
3. **Estimate the aggregator's own committed width** (§5.4). Write the engine's
   function signatures as stubs, compile with `zkc compile --stats`, and sum the
   module widths. This is the convergence check and it gates everything above
   level 1. It can be done before any function body is written.
4. **Decide the FRI parameters** (§3). This is a team decision, not a measurement:
   conjectured versus provable soundness, and how much shard-size reduction is
   acceptable. No implementation should start before it is made, because the answer
   changes whether level 1 is one shard or four.
5. **Raise the cross-shard binding upstream** (§6.1). The contribution hashes an
   absent commitment, the bus is empty, γ has no consumer, and preflight seeds from a
   different tree. None of this blocks building or measuring the aggregator, so it can
   run in parallel, but it does block calling the result a cross-shard aggregator.
   The question for prover-ray is concrete: when a multi-shard R5 configuration lands,
   which round carries the bus participant columns, and is the contribution meant to
   hash that round's PCS commitment or the preflight tree root?

Then the build order is the phase list in `verifier-ray-zkc-plan.md` §10 with three
substitutions: `image.zkc` becomes the flat-input reader of §7; the accelerator
wiring phase (§7 of that document) is deleted; and a new final phase adds the
aggregation semantics of §6 and the recursion harness.

The Go side needs one new component beyond the codegen renderer already planned: a
**flattener** that turns a `wiop.Proof` into the felt arrays of §7. Build it next to
`proofserialization.Project`, reusing that projection rather than writing a second
one — the README's warning that "two projections that must agree is a bug factory"
applies with equal force here.

---

## 9a. Compression at TODAY's parameters, and how the proof gets in

Added 2026-09-24 in answer to: assuming rate 1/2, Q = 229 and no grinding for now,
what does a 1-to-1 zkc "shrink" of one R5 shard proof actually buy, and how is a
47 MB proof fed to a zkc program?

### 9a.1 Compression has a closed form, and Q cancels

A proof's size is dominated by its opened row data, which is
`Q · 2 · W · 4 bytes` where `W` is the **opened felt width**: the sum, over every
committed column, of 1 felt for a base column and 6 for an extension column. `W`
is a pure column count. **It does not depend on trace height**, because each query
opens one row per committed size per tree, and those rows' widths sum to `W`
however tall the columns are.

So for a shrink step at fixed FRI parameters:

```
compression  =  (Q · 2 · W_R5 · 4 + merkle)  /  (Q · 2 · W_zkc · 4 + merkle)
             ≈  W_R5 / W_zkc            for W_zkc still large
```

`Q`, the factor 2 and the 4 bytes all cancel. **The entire compression question
reduces to: how narrow is the zkc verifier?** That is a statement about the
program, not about the proof system, which is why it is worth answering now even
though the FRI parameters are frozen.

The Merkle part does not shrink with `W`. Per query the verifier reveals roughly
`4 trees × 14 + 112 running` sibling digests, about 168, so
`229 · 168 · 32 B ≈ 1.2 MB` is a floor that survives any narrowing. Compression
therefore saturates around `48.6 / 1.5 ≈ 32×` no matter how good the program is.

### 9a.2 Estimating `W_zkc`, calibrated against the compiler

Two widths were measured rather than guessed, by compiling a probe with
`go tool zkc compile --stats` (post-register-splitting column counts, `KOALABEAR_16`):

| measured | columns |
|---|---|
| straight-line `ext_mul` (F_p^6 Karatsuba, 12 in / 6 out) | **91** |
| the existing loop-based Poseidon2, all 12 functions summed | **657** |
| a `memory m[u32](a:u5) -> (v:𝔽)` RW memory module | **17** |
| an `input m(i:u5) -> (v:u32)` ROM module | **4** |

The loop-based Poseidon2 is narrow but costs several hundred rows per permutation,
which at 1.52M compressions is ~1 billion rows — three orders of magnitude over
budget. It has to be unrolled, and unrolling is what buys rows at the price of width.
Scaling the measured per-round pieces to a fully straight-line permutation gives
about **1,600 columns**, one row per compression.

Rolling that up across the whole verifier:

| group | columns | basis |
|---|---|---|
| Poseidon2 permutation | 800–1,600 | 1,600 fully unrolled; ~800 split into two reused half-permutations at 2 rows each |
| batched DEEP inner loop | ~360–450 | `k`-way unrolled, see §9a.3 |
| extension-field ops (`mul` 91, `sqr`, `inv`, `add/sub`, `mul_base`, `pow2k`) | ~500 | anchored on the measured 91 |
| Merkle (node, row, row-pair, branch, cap, input branch) | ~350 | |
| PCS layout + verify | ~900 | the long tail of ~25 functions |
| transcript, folds, vanishing eval, scalar checks, readers | ~700 | ~35 more functions |
| RW and input memories, statics | ~170 | measured 17 and 4 per module |
| **total `W_zkc`** | **~3,000–5,000** | |

### 9a.3 The answer

| | felts wide | proof | vs shard |
|---|---|---|---|
| R5 shard proof | 25,832 | ~48.6 MB | 1× |
| zkc shrink, straight-line first cut | ~5,000 | ~10.4 MB | **~4.7×** |
| zkc shrink, width-aware | ~3,000 | ~6.8 MB | **~7.1×** |
| theoretical floor, `W ≥ cells / 2^22` | ~1,030 | ~3.1 MB | ~16× |
| Merkle-only floor | 0 | ~1.5 MB | ~32× |

**Expect 5× on a first implementation and about 7× with width-aware engineering.**
47 MB becomes roughly 7 to 10 MB.

Three things about that number matter more than the number itself:

- **It fits in one shard, which the earlier draft of this document denied.** §1.3
  said even a 1-to-1 shrink was 2.8× over budget, because it assumed one row per
  extension multiply and one row per felt read. Both are batchable. Reading 8 felts
  per row inside the hashing step turns 11.8M reads into 1.48M rows; unrolling the
  DEEP inner loop `k = 4` ways turns 11.8M multiplies into 2.95M rows. The width
  cost is small because those functions are narrow to begin with. **A single-shard
  1-to-1 shrink at today's parameters is feasible.** Nothing in §2 or §3 changes,
  because 2-to-1 doubles both figures and puts them back over.
- **Recursion is a fixed point, not a ladder.** Shrinking a shrink proof gives
  `W_zkc / W_zkc = 1×`. The 5–7× is one-shot. What the 2-to-1 tree buys is a
  reduction in the *number* of proofs, not their size: `N` shards of 48.6 MB become
  one proof of ~7–10 MB, so the batch-level compression is `5N×` and a final SNARK
  wrapper is still needed to reach L1 calldata sizes.
- **The optimisation target is now crisp**: minimise Σ module width subject to every
  module ≤ 2^22 rows. The floor is `total cells / 2^22`. Poseidon2 is both the
  widest module and ~60% of all cells, so it is where width engineering pays;
  everything else is a long tail of functions that are called too few times to be
  row-bound and are therefore pure width.

### 9a.4 Feeding a 47 MB proof in, efficiently

The happy fact that makes this work: **input size is nearly free in the output
proof.** An input memory declared `input pf(i:u22) -> (v:𝔽)` is one data column plus
a small address column, about 3 to 4 columns measured. Whether it holds 10 felts or
4 million, it contributes those 3 to 4 felts to `W` and therefore a few bytes per
query to the shrink proof. Height is not what proofs pay for; width is.

Concretely:

- **Private, not public.** Declare the proof memories `input`, never `pub input`.
  Public inputs are the statement, and only the statement: vk digests, the
  accumulated claims, the range. A `pub input` proof would have to be absorbed by
  the next level.
- **Felts, not bytes.** The proof is already field elements. Deliver
  `-> (v:𝔽)` and `-> (d0..d7:𝔽)` memories, not a byte image. This drops the whole
  `proof_abi.zig` layer: no pointers, no slice headers, no `Scalar` tags, no
  `?RowPair` presence flags, no little-endian byte assembly. Everything those encode
  is implied by the verification key (§7).
- **Split across memories to clear 2^22.** An R5 proof is ~11.8M felts, so at least
  three input memories; use four to six split by role (rows, digests, cells, sizes)
  with headroom. An `#[inline]` reader dispatching on the high bits hides the split
  from callers. Each split costs ~4 columns, so this is essentially free.
- **Never copy.** This is the single biggest difference from the R5-guest path,
  where `riscv/main.zkc` copies every blob byte into `ram` with `write_8` before
  execution starts. At 47 MB that is ~47M memory writes, over the row budget by
  itself, before any verification happens. A zkc program reads its input memories
  directly and the copy does not exist.
- **Batch the reads, because reads cost rows.** Each read is one lookup in the
  reading function's module. 11.8M unbatched reads exceed 2^22 six-fold. Reading
  eight felts per hashing row and four entries per DEEP row is what makes §9a.3's
  single-shard claim true, and it is the main structural requirement the input
  format imposes on the engine.
- **Delivery is in-process Go.** Build `map[string][]byte`, one entry per input
  memory, each the big-endian felt encoding, and hand it to
  `binFile.Trace(inputs, cfg)`. No JSON file and no CLI, per `lineth_overview.md`
  §5. The flattener belongs next to `proofserialization.Project` and should reuse
  that projection rather than re-deriving the round-major ordering.

One assumption still to confirm, unchanged from §9: whether an input memory's module
height is the data provided or only the addresses actually read. It barely matters
here, since the verifier reads nearly every opened felt, but it decides how much
headroom the split in point three needs.

---

## 10. Risks and open questions

1. **The FRI-parameter decision is the whole project.** At 229 queries no amount of
   zkc optimisation closes a 5.7× gap on three modules simultaneously. Everything
   else in this document is comparatively routine.
2. **Conjectured versus provable soundness.** The recommended configuration relies on
   the conjectured (capacity-bound) FRI analysis to reach Q = 27 at rate 1/8. Under
   the provable Johnson-bound analysis the same Q needs rate 1/64 and 2^18-row
   shards. The current 229 is a provable-soundness number, so adopting the
   conjectured one is a real change in security posture and needs an explicit
   decision, not a default.
3. **Grinding does not exist yet.** It needs a nonce in the transcript and a check in
   prover-ray, verifier-ray and this program. Small, but it is a protocol change
   across three components.
4. **The aggregator's own width is an estimate.** §5.4's 3,000–6,000 felts is
   reasoned, not measured. If it lands near R5's 25,832, recursion does not converge
   and the design needs narrower primitives — probe 3 in §9 settles it early and
   cheaply.
5. **Cross-shard binding does not exist yet, in either of its two halves** (§6.1, §6.2).
   The shared-randomness contribution hashes an absent commitment and is therefore a
   constant; the bus is empty so nothing is deferred; γ has no consumer now that #3950
   removed the Fiat-Shamir hook; and preflight derives its seed from a different tree
   than the contribution hashes. Until these are reconciled upstream, an aggregator
   built to this design is a sound **proof compressor** and an unsound **cross-shard
   aggregator**. That distinction should be stated wherever the component is described,
   because the two are easy to conflate and only one of them is true.
6. **Two-adicity leaves no headroom.** Rate × plaintext ≤ 2^24 is a hard field
   property. Every rate improvement is paid for in shard size, and the ladder in
   §2.3 is the complete set of options. There is no configuration with both 2^22
   shards and few queries.
7. **The `?RowPair`/tag removals in §7 change the wire format**, so the aggregator's
   input is not the image `proofserialization` produces today. That is deliberate,
   but it means the flattener is new code on a path with no existing test coverage;
   the cross-language golden-test discipline of `proofserialization` should be
   copied, not reinvented.
8. **`riscv_system.zig` and `riscv_proof_image.bin` are gitignored and absent
   locally**, so no end-to-end number in this document is measured against a real
   proof — only against the compiled system, which is what §0's probe reads. The
   first real proof will move the constants, not the conclusions.

---

## Appendix — reproducing the measurement

```bash
cd verifier-ray && go test -run TestR5SystemShape ./codegen -v
```

`verifier-ray/codegen/shape_probe_test.go` compiles
`arithmetization/src/main/riscv/main.zkc` through `runCompilePipeline` and
`BuildCompiledSystem`, then prints the table in §0. It proves nothing and takes about
a second, so it is cheap to re-run whenever the arithmetization changes. Re-run it
before trusting any number in §1: `opened felt width` and `num_queries` are the two
that move the budget.

