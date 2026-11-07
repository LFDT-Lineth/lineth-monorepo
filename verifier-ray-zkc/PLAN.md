# `verifier-ray-zkc` — a 1-to-1 shard-proof compressor

Implementation plan, 2026-09-24. Nothing here is built yet.

**What this is.** A standalone zkc program that reads one R5 WIOP shard proof as
input, verifies it, and produces a WIOP proof of having done so. One proof in, one
proof out. The output proof is **5 to 7 times smaller** than the input, which is the
entire point: it turns a ~48 MB shard proof into a ~7–10 MB proof of the same
statement, cheaply enough to do once per shard.

**What this is not.** Not an accelerator: it is not a custom RISC-V instruction and
there is no R5 guest underneath it. Section 1 argues that choice. Not an aggregator
either: 2-to-1 aggregation is deliberately out of scope and is designed separately in
`../wiop-agg-design.md`. This document covers only the 1-to-1 step, which is the
level that has to exist first and is the harder of the two at today's parameters.

**Reading order.** Sections 1 to 3 are the case and the numbers. Section 4 is the
API and is the contract an implementer works against. Sections 5 to 8 are how to
build it. Sections 9 to 11 are testing, phasing and risk.

**Provenance.** The zkc cost model (§5), the data representation (§6.2) and the
module-by-module port specification (§7) are adapted from
`../verifier-ray-zkc-plan.md`, which planned the same port as an accelerator. Where
that document and this one disagree, this one is right for the compressor and that
one is right for the accelerator; §12 lists what changed and why. The measured R5
figures come from `../wiop-agg-design.md` §0 and the probe that produced them.

---

## 1. Why standalone rather than an accelerator

The earlier plan wrapped this verifier as custom-1 instruction `WIOP_VERIFY`, callable
from a Zig guest. That is the right shape when the caller is already an R5 program and
you want to make one expensive step cheap. It is the wrong shape here, for five
reasons, the second of which is on its own decisive.

1. **There is no guest.** The compressor is not a step inside a RISC-V program; it is
   the whole computation. As an accelerator it would need a Zig guest whose only job
   is to execute one instruction, and that guest still costs an ELF, a predecode
   table, an interpreter dispatch per instruction and an entry in the R5 CI gate — all
   to call a single opcode.

2. **The blob copy alone disqualifies it.** `arithmetization/src/main/riscv/main.zkc`
   copies every blob byte into `ram` with `write_8` before execution begins. A 48 MB
   proof is ~48 million such writes. The per-module row ceiling is 2^22 ≈ 4.19M, so
   the input would blow the budget by more than 11× **before any verification
   happens**. A standalone program reads its input memories directly and the copy
   does not exist.

3. **We would inherit a byte ABI we do not want.** The accelerator has to consume the
   `proof_abi.zig` image — absolute pointers, slice headers, union tags, `?RowPair`
   presence flags, little-endian byte assembly — because that is what the Zig verifier
   casts out of guest memory. A standalone program defines its own input format and
   drops every one of those, since the verification key already implies them (§4.2).

4. **The objective function is different.** An accelerator is judged on rows and cells,
   because those are prover time. A compressor is judged on the **width of its own
   AIR**, because width is what sets the output proof size (§2). That changes concrete
   engineering decisions in the opposite direction — see the width rules in §5.4,
   which contradict the straight-line-everything advice that is correct for the
   accelerator.

5. **Smaller blast radius.** No `ComputeOp` renumbering (a silent-corruption hazard
   documented in the accelerator plan), no predecoder change, no guest Zig, no new
   dependency on `riscv/main.zkc` compiling.

The one thing the accelerator route would have bought — reuse of the R5 RAM
marshalling helpers — is worth nothing here, because the input is field elements
already and never needs to become bytes.

---

## 2. Expected compression

### 2.1 The closed form

A WIOP proof's size is dominated by its opened row data:

```
proof_bytes  ≈  Q · 2 · W · 4   +   merkle
```

where `Q` is the FRI query count and `W` is the **opened felt width**: summed over
every committed column, 1 felt for a base column and 6 for an extension column.

The load-bearing property is that **`W` counts columns, not rows**. Each query opens
one row per committed size per tree, and those row widths sum to `W` however tall the
columns are. A column of height 2^22 and a column of height 8 each contribute the
same one felt per opened row.

So for a 1-to-1 compressor:

```
compression  =  (Q · 2 · W_R5 · 4 + merkle) / (Q · 2 · W_zkc · 4 + merkle)
             ≈  W_R5 / W_zkc
```

**`Q`, the rate and the grinding all cancel.** That is why this number can be
committed to now, while the FRI parameters are still frozen at rate 1/2 and Q = 229.
Those parameters change the absolute sizes on both sides and leave the ratio alone.

The Merkle part does not shrink. Each query reveals roughly 168 sibling digests
(4 input trees × 14 levels below a depth-8 cap, plus ~112 running-layer siblings), so
`229 · 168 · 32 B ≈ 1.2 MB` survives any narrowing of the program and caps compression
at about 32×.

### 2.2 Calibrating `W_zkc` against the compiler

Two widths were measured, not estimated, by compiling a probe with
`go tool zkc compile --stats` and reading post-register-splitting column counts at
`KOALABEAR_16`:

| measured | columns |
|---|---|
| straight-line `ext_mul` (F_p^6 Karatsuba, 12 in / 6 out) | **91** |
| existing loop-based Poseidon2, all 12 functions summed | **657** |
| `memory m[u32](a:u5) -> (v:𝔽)` module | **17** |
| `input m(i:u5) -> (v:u32)` module | **4** |

The loop-based Poseidon2 is the cautionary measurement. It is narrow, but costs
several hundred rows per permutation, and at 1.52M compressions per proof that is on
the order of 10^9 rows — three orders of magnitude over budget. It has to be unrolled,
and unrolling is exactly the trade that buys rows at the price of width. Scaling its
measured per-round pieces to a fully unrolled permutation gives about **1,600 columns**
at one row per compression.

Rolling up the whole verifier:

| group | columns | basis |
|---|---|---|
| Poseidon2 permutation | 800–1,600 | 1,600 unrolled; ~800 as two reused half-permutations at 2 rows each (§8.3) |
| batched DEEP inner loop | 360–450 | `k`-way unrolled (§8.2) |
| extension-field ops | ~500 | anchored on the measured 91 for `ext_mul` |
| Merkle (node, row, row-pair, branch, cap, input branch) | ~350 | |
| PCS layout and verify | ~900 | long tail of ~25 functions |
| transcript, folds, vanishing eval, scalar checks, readers | ~700 | ~35 more functions |
| memories and statics | ~170 | measured 17 and 4 per module |
| **total `W_zkc`** | **3,000–5,000** | |

### 2.3 The estimate

| | felts wide | proof | vs shard |
|---|---|---|---|
| R5 shard proof | 25,832 | ~48.6 MB | 1× |
| compressor, straight-line first cut | ~5,000 | ~10.4 MB | **~4.7×** |
| compressor, width-aware | ~3,000 | ~6.8 MB | **~7.1×** |
| floor from `W ≥ cells / 2^22` | ~1,030 | ~3.1 MB | ~16× |
| Merkle-only floor | 0 | ~1.5 MB | ~32× |

**Plan for 5×, aim for 7×.** Forty-eight megabytes becomes seven to ten.

Three consequences worth internalising before building:

- **Compression is one-shot.** Running the compressor on its own output gives
  `W_zkc / W_zkc = 1×`. Recursion is a fixed point, not a ladder. What the later
  2-to-1 tree buys is a reduction in the *number* of proofs: `N` shards become one
  proof of ~7–10 MB, so batch-level compression is `≈5N×`. A final SNARK wrapper is
  still needed to reach L1 calldata size, and is out of scope for zkc.
- **The optimisation target is Σ module width subject to every module ≤ 2^22 rows.**
  Not cells, not rows. §5.4 turns that into rules.
- **Poseidon2 is the whole game**: the widest single module and ~60% of all cells.
  Everything else is a long tail of functions called too few times to be row-bound,
  which makes them pure width with no compression lever.

---

## 3. The measured facts this rests on

Produced by `verifier-ray/codegen/shape_probe_test.go` (`TestR5SystemShape`), which
compiles `arithmetization/src/main/riscv/main.zkc` through `runCompilePipeline` and
`BuildCompiledSystem`. Compilation only, no proving, about one second. Re-run it
before trusting any number here.

```
rounds 5 | coins 230 | dynamic modules 97 | public inputs 337
transcript cells 17,842        (per round: 8, 328, 2270, 0, 15236)

PCS       num_queries 229,  codeword 2^23 / plaintext 2^22  (rate 1/2),  4 batches
          committed columns 10,622  (7,580 base + 3,042 ext)
          OPENED FELT WIDTH 25,832
          (column, shift) pairs 15,236
          witness / quotient claims 14,462 / 774
VANISHING 118 modules,  482,341 expression nodes,  333 buckets,  19,123 constraints
SCALAR    logderiv 1 query / 2,265 refs;  grandproduct 1 / 3;  rowlimit 139 / 6,668
          shared-randomness 328 refs, commitment round 1, hasCommitment FALSE
```

Derived quantities used throughout:

| quantity | value | derivation |
|---|---|---|
| opened felts per query | 51,664 | `2 · 25,832` |
| opened felts per proof | 11.83M | `229 · 51,664` |
| proof size | ~48.6 MB | `11.83M · 4 B` + ~1.2 MB Merkle |
| Poseidon2 compressions | 1.52M | `229 · (51,664/8 + ~196)` |
| DEEP entry-terms | 5.92M | `229 · 2 · (10,622 + 15,236) / 2` |

**One caveat carried from `../wiop-agg-design.md` §6.1.** The shared-randomness check
is live but currently vacuous: the coin round has no commitment, so both sides hash a
zero octuplet and compare constants. The compressor must still implement it, because
it is part of the verification the shard proof claims, but implementing it faithfully
yields a faithful no-op and does **not** give cross-shard binding. Do not describe the
compressor as providing that.

---

## 4. API

Four surfaces: the zkc program's I/O contract, the input encoding, the Go driver, and
the generated verification-key tables.

### 4.1 The zkc program contract

The program is `src/main.zkc`. Its whole interface is its memory declarations.

```zkc
// ── statement (public) ───────────────────────────────────────────────────────
// The 337 public inputs of the shard proof, in prover-ray registration order.
// Written once, in order, by main() after verification succeeds.
pub output vrz_statement(i:u10) -> (v:𝔽)

// ── proof (private) ──────────────────────────────────────────────────────────
input vrz_hdr(i:u8)      -> (v:u64)       // section counts and base offsets (§4.2.1)
input vrz_rows_0(i:u22)  -> (v:𝔽)         // opened row values, shard 0 of 4
input vrz_rows_1(i:u22)  -> (v:𝔽)
input vrz_rows_2(i:u22)  -> (v:𝔽)
input vrz_rows_3(i:u22)  -> (v:𝔽)
input vrz_dig(i:u20)     -> (d0:𝔽, d1:𝔽, d2:𝔽, d3:𝔽, d4:𝔽, d5:𝔽, d6:𝔽, d7:𝔽)
input vrz_cells(i:u18)   -> (e0:𝔽, e1:𝔽, e2:𝔽, e3:𝔽, e4:𝔽, e5:𝔽)
input vrz_pubin(i:u10)   -> (v:𝔽)         // the 337 public inputs
input vrz_sizes(i:u8)    -> (v:u32)       // the 97 dynamic module sizes
```

Semantics:

- **Everything except `vrz_statement` is private.** Declaring proof data `pub input`
  would force the next level to absorb 12M felts into its transcript. Only the
  statement is public.
- **`main()` either completes or `fail`s.** There is no status code. A compressor that
  cannot verify its input must not produce a proof, so every condition that makes the
  Zig verifier return an error becomes `fail`, which makes the AIR unsatisfiable. This
  is the opposite of the precompile convention in PR #3927 and it removes the `ok:u1`
  threading (and with it the multi-return conditional hazard) that the accelerator
  plan had to carry through every signature.
- **The R5 verification key is baked**, not supplied. It arrives as generated `static`
  tables in `src/system/r5_system.zkc` (§4.4). The compressor is therefore specific to
  one arithmetization, which is correct for 1-to-1: there is no self-verification to
  make the key circular. Supplying the key as input, hashing it in-circuit and
  comparing against a propagated digest is the 2-to-1 problem and is deferred.
  Baking also saves hashing ~2.1M felts of key material per run.
- **The statement is published verbatim, not as a digest.** 337 felts is 1.3 KB and
  costs nothing, a digest would need its preimage carried separately anyway, and
  publishing the raw values keeps the cross-shard option open for when §3's caveat is
  resolved. A consumer that wants a digest can hash them itself.

Because zkc has one flat namespace for top-level functions and constants across the
whole include graph, **every identifier in this package is prefixed `vrz_` / `VRZ_`**.
This is not cosmetic: the accelerator work hit a silent duplicate-declaration collision
from unprefixed curve constants that per-file unit tests could not catch.

### 4.2 Input encoding

The encoding is defined by this document and by the flattener that produces it; it is
deliberately **not** the `proof_abi.zig` image. Four things that image carries are
dropped because the verification key already determines them:

| dropped | why it is recoverable |
|---|---|
| `Scalar` base/ext tags | the cell's field kind is a static property of the system |
| `?RowPair` presence flags | presence is a function of cap depth and level |
| slice headers and absolute pointers | offsets come from `vrz_hdr` and from key-implied counts |
| struct padding | no structs |

**4.2.1 `vrz_hdr`** — a small fixed table of `u64`s: the base offset of each logical
section within `vrz_rows_*` / `vrz_dig` / `vrz_cells`, plus the per-query stride and
the runtime counts the key cannot fix (number of FRI rounds after restriction, cap
depths, distinct input-tree count). Every offset is in **element units**, not bytes.
The exact field list is fixed by the flattener and asserted by a Go test against the
engine's reader constants.

**4.2.2 `vrz_rows_*`** — the bulk, ~11.83M felts. Laid out query-major, then
conjugate, then canonical entry order, so the verifier's DEEP walk is a linear scan:

```
for q in 0..229:  for conj in {self, sibling}:  for entry in canonical order:
    base column  -> 1 felt
    ext column   -> 6 felts
```

At 11.83M felts and a 2^22 per-memory ceiling, this needs at least three memories;
**use four** so no memory exceeds ~2.96M and there is headroom for a wider R5. An
`#[inline]` reader hides the split:

```zkc
#[inline]
fn vrz_row(i:u24) -> (v:𝔽)      // switches on i >> 22; only the taken arm costs rows
```

The switch adds registers to whichever function inlines it. If that width cost
measures badly, the fallback is to partition by **whole queries** rather than by flat
index, so the memory changes only three times over the run, at the cost of a
per-memory copy of the query driver. Decide with the Phase 0 probe, not in advance.

**4.2.3 `vrz_dig`** — every 8-felt digest in one table: round commitments, input-tree
siblings, FRI round roots, cap nodes, running-layer branch leaves and siblings. One
row per digest keeps the reader trivial and the memory 8 felts wide.

**4.2.4 `vrz_cells`** — transcript cells, always six limbs. A base cell puts its value
in limb 0 and zeroes the rest; the key says which cells are base, and the replay
absorbs one limb or six accordingly. Uniform width beats a tagged encoding.

**4.2.5 Element encoding on the Go side.** Each memory is one `[]byte` in the
`map[string][]byte` that `BinaryFile.Trace` takes, with each element big-endian in the
memory's declared data width. Field elements are **canonical**, not Montgomery: the
`prover-ray/wiop/proofserialization` README §5 says Montgomery, but `types.go:159-196`
converts every limb with `Bits()[0]`, which is the from-Montgomery conversion, and the
Zig verifier does canonical arithmetic. Follow the code, not that sentence.

### 4.3 Go driver API

Package `verifier-ray-zkc/gopkg/compressor`.

```go
// Flatten encodes one projected shard proof as the compressor's zkc input map.
// vi comes from proofserialization.Project, which is reused rather than
// re-deriving the round-major ordering: two projections that must agree is a
// bug factory (proofserialization/README.md §3).
func Flatten(sys *wiop.System, vi proofserialization.VerifyInput) (map[string][]byte, error)

// Inputs is the one-call path from a freshly produced shard proof.
func Inputs(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) (map[string][]byte, error)

// Program compiles src/main.zkc (embedded) into a BinaryFile, once per process.
func Program() (*constraints.BinaryFile[koalabear.Element], error)

// Execute runs the compressor without tracing. The fast pre-flight check that
// the proof verifies at all; returns the published statement.
func Execute(p *constraints.BinaryFile[koalabear.Element], in map[string][]byte) (Statement, error)

// Trace produces the compressor's execution trace, sharded if requested.
func Trace(p *constraints.BinaryFile[koalabear.Element], in map[string][]byte,
    cfg vm.TraceConfig) (trace.Trace[koalabear.Element], Statement, error)

// Statement is the compressor's public output: the shard proof's public inputs
// in prover-ray registration order.
type Statement struct{ PublicInputs []koalabear.Element }
```

Proving the compressor's own trace is the ordinary `zkcdriver` path
(`NewZkCDriver`, `AssignTraceShard`, `sys.Prove`) and needs nothing new from this
package.

Two conventions the driver must enforce, both learned the hard way:

- **`Execute` is not an oracle.** `zkc execute -c` and `Check` report constraint
  failures while still returning without a Go error. Callers must test
  `len(failures) == 0` explicitly, never the absence of an error or a zero exit code.
- **Supply every declared input.** Omitting one does not produce a missing-input
  error; zkc nil-dereferences inside the constraint checker.

### 4.4 Generated verification-key tables

Package `verifier-ray-zkc/gopkg/zkcgen`, a second renderer beside
`verifier-ray/codegen`'s existing `*_zig.go` files, consuming the **same**
`codegen.CompiledSystem` so the two cannot drift.

```go
func WriteR5SystemZkc(w io.Writer, cs codegen.CompiledSystem, opts Options) error
```

It emits `src/system/r5_system.zkc`: `const`s for the scalar shape (round count, coin
totals, query count, log sizes, column/claim counts) and `static` tables for the rest
— columns, shifts and their claim cells, witness and quotient claim maps, batch roots,
vanishing modules, buckets, constraints, the flat expression DAG, cancelled positions,
row-limit checks, log-derivative and grand-product refs, shared-randomness refs. Plus
derived constants the engine would otherwise recompute: the 25 roots of unity, the
per-height cap depths at `Q = 229`, the reduced leaf domain tag, and `1/2`.

Three rules for the emitter:

- **Global expression indices.** The Zig `Module.expressions` are per module with local
  operand indices; the emitter offsets each module by its `expr_base` so `VRZ_EXPR` is
  one flat table.
- **A zero-row `static` may not parse.** Always emit at least one dummy row plus a
  separate `_COUNT` constant of 0.
- **Assert every value fits its declared width**, and keep widths fixed across all
  generated systems used in tests so engine signatures do not change.

The generated file is **committed**, because `zkc compile` must see it. Regeneration
goes in the package `Makefile` and is drift-checked in CI the way
`verifier-ray`'s `verify-testdata` target checks its generated Zig.

---

## 5. zkc cost model, and the rules that follow

Verified against zkc v1.2.32 (the version pinned in every consuming `go.mod`). Use
`$GOMODCACHE/github.com/!l!f!d!t-!lineth/zkc@v1.2.32/` as the language reference —
**not** the untracked `zkc/` clone at the repo root, which is at commit `26ee2a0` and
predates `done`, `f!(...)`, `#[global]`, memory timestamp bounds and fixed arrays.

### 5.1 What costs what

| construct | cost |
|---|---|
| function call | one lookup; an *atomic* callee (no loops, single vector bundle) is **one row per call**, and every parameter, return and local is a **column** of its module. Identical calls dedupe through the lookup. |
| loop | forces a multi-line function: `PC` and `RET` columns, one row per iteration per bundle, constancy constraints on every unwritten register |
| recursion | one atomic row per call, no `PC`/`RET`. Preferred over loops for unbounded iteration |
| `if` / `switch` arm | all arms materialise as columns; **untaken arms cost zero rows** |
| RW memory access | one row in the memory's module, plus a timestamp column of the declared width |
| `input` / `static` read | a lookup into a fixed table; cheaper than RW (no timestamp) |
| `𝔽` arithmetic | `+`, `-`, `*` and `==`/`!=` only. No `/`, `%`, shifts or bitwise. Inverse must be computed; there is no hint mechanism |
| `u32 as 𝔽` | free. `𝔽 as uN` inserts a canonicality check |

Integer widths split into 16-bit registers at `KOALABEAR_16`: a `u64` is 4 columns, a
`u32` is 2, a `𝔽` is 1. Prefer `𝔽` for anything that is a field element and never
round-trip it through an integer type.

### 5.2 Straight-line versus loops

A function with no loops compiles to **one row** however many operations it contains;
the operations become columns. So the two dials are:

- **fixed-size computation → straight-line.** One row, wide.
- **data-dependent iteration → recursion** over an index, with state in a small RW
  memory rather than long argument lists.

### 5.3 Limits and hazards

- A call passes at most **255 split registers**. Twelve `𝔽` (an `Ext` pair) or sixteen
  (a Poseidon2 state) are fine; a batched call of `k` entries must stay under it.
- **Fixed arrays** exist (`var a:[𝔽;6]`, and array parameters are confirmed) but
  indices must be constant expressions. Array *returns* were not confirmed — probe
  before relying on them; every signature in §7 assumes individual felts.
- **Multi-column memories** are supported for both RW and `static`, including partial
  reads (`lo, _ = tbl[i]`).
- `zkc compile` **hard-errors on any function unreachable from `main`**, with no
  escape hatch. Land library code only with a live call path, or keep it in files only
  the test harnesses include.
- Branching on **two** values returned by one multi-return call inside a loop has
  panicked the tracing transform. Return one value, or branch on one. The `fail`
  convention (§4.1) avoids most of this by removing `ok` returns entirely.
- `zkc execute --fast` has panicked on wide-limb types; unit harnesses run in tracing
  mode.

### 5.4 The width rules — specific to a compressor

These are the rules that differ from the accelerator plan, and they follow directly
from §2: output proof size is `Q · 2 · W · 4`, so **Σ module width is the product
metric**, subject to every module staying under 2^22 rows.

1. **Minimise Σ width subject to rows ≤ 2^22.** The floor for any function is
   `its cells / 2^22`. A function that is not row-bound has no compression lever and
   is pure width, so keep it small; a function that is row-bound should be widened
   only until it fits, and no further.
2. **Splitting a function into several does not reduce total width.** Width is summed
   over distinct modules and splitting adds interface registers. Three narrow modules
   in place of one wide one is a wash at best.
3. **Reuse is free in width, so prefer few heavily-called functions.** One function
   called two million times costs its width once. This is the lever: fold variants
   into one parameterised function wherever the arithmetic allows.
4. **Rolling a loop back up reduces width and multiplies rows.** This is the only real
   width lever, and it is bounded by rule 1. It is how Poseidon2 gets from 1,600
   columns to ~800 (§8.3).
5. **Batch a row-bound hot loop by unrolling it `k` ways**, choosing the smallest `k`
   that fits 2^22. Width grows by `k ×` the per-item registers, which is cheap when
   the per-item function is narrow, and it is what makes the 1-to-1 fit at all (§8.2).
6. **Do not batch-invert.** Montgomery batch inversion turns `n` inversions into
   `1 + 3(n−1)` operations, which is a win on a CPU and a **loss** here, because a
   straight-line `ext_inv` is one row exactly like `ext_mul`. Call `ext_inv` directly.
   (It does save ~40% of *cells*; revisit only if prover time, not the row ceiling,
   turns out to bind.)
7. **`#[inline]` functions called only a handful of times**, so they do not create a
   module of their own for work that happens twice.

---

## 6. Program structure

### 6.1 Layout

Mirrors `verifier-ray/src/` so the two can be read side by side during the port.

```
verifier-ray-zkc/
  PLAN.md                    this document
  Makefile                   compile, format-check, test, regenerate
  src/
    main.zkc                 entry point (§6.3)
    io/
      inputs.zkc             input declarations + batched readers (§4.2)
      statement.zkc          pub output + the publish step
    field/
      koalabear.zkc          base inverse, pow2k, roots, bitrev, domain points
      ext.zkc                F_p^6 = E2[v]/(v^3 - (u+1)), E2 = F_p[u]/(u^2 - 3)
    crypto/
      poseidon2.zkc          unrolled permutation, split for width (§8.3)
      transcript.zkc         Merkle-Damgard sponge + Fiat-Shamir
      merkle.zkc             hash_node, row / row-pair hashing, branch + cap auth
    protocol/
      replay.zkc             transcript replay -> coins
      public_input.zkc       cell lookup with public-input merge
    query/
      pcs_layout.zkc         reconstruct, entry claims, root routing, claim routing
      pcs_verify.zkc         challenges, caps, per-query DEEP
      fri.zkc                fold recurrence
      vanishing.zkc          expression evaluator, buckets, selector, cancellation
      scalar.zkc             logderiv, grandproduct, rowlimit, shared randomness
    system/
      r5_system.zkc          GENERATED (§4.4), committed
  gopkg/
    compressor/              flattener + driver (§4.3)
    zkcgen/                  CompiledSystem -> r5_system.zkc (§4.4)
  test/
    zkc/                     per-module harnesses (§9)
```

### 6.2 Data representation

**Base field** `𝔽`, KoalaBear `p = 2^31 − 2^24 + 1`. From input: `v` directly, no
conversion.

**`E2 = 𝔽[u]/(u² − 3)`**, two felts. The non-residue is 3
(`koalabear_ext.zig:115`: `x0*y0 + 3*x1*y1`).

**`Ext = E2[v]/(v³ − (u+1))`**, six felts in the order
`B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1`. This is also the input encoding order and
the coin limb order, so no permutation is ever needed. The cubic reduction is
`nr(x) = (x0 + 3·x1, x0 + x1)`.

**Digest**, 8 felts.

**Constants to reproduce exactly**: `inv_two = 1065353217`, `root_of_unity = 1791270792`
(order 2^24), `leaf_domain_tag = 0x4c66_7269_5f6c_6631` absorbed as `tag mod p` — have
the emitter compute the reduced value and pin it in a unit test against the Zig one.

**Scratch memories**, declared in `src/io/inputs.zkc` and threaded through as memory
effects. A region map of `VRZ_R_*` constants documents which index range each logical
array occupies; zkc memories are sparse, so untouched cells cost nothing.

| memory | holds |
|---|---|
| `vrz_fs[u32](i:u5) -> (v:𝔽)` | sponge state, buffer, buffer length |
| `vrz_coins[u16](i:u10) -> (c0..c5:𝔽)` | `all_coins`, then fold alphas and deep alpha |
| `vrz_pos[u32](i:u8) -> (p:u32)` | query positions |
| `vrz_ext[u32](i:u16) -> (e0..e5:𝔽)` | entry claims, routed witness/quotient claims, per-query fold buffers, per-module context |
| `vrz_dgst[u32](i:u16) -> (d0..d7:𝔽)` | resolved batch roots, distinct input roots, frontiers, cap aux |
| `vrz_u[u32](i:u16) -> (v:u64)` | reconstructed layout arrays, bucket counters, presence flags |

### 6.3 `main.zkc`

```zkc
fn main<vrz_fs, vrz_coins, vrz_pos, vrz_ext, vrz_dgst, vrz_u, vrz_statement>() {
    vrz_check_shape()          // round count, cell counts, public-input count, sizes
    vrz_replay()               // transcript replay -> coins                     §7.4
    vrz_resolve_roots()        // batch roots from transcript-bound commitments  §7.6
    vrz_reconstruct()          // canonical layout + restricted FRI params       §7.6
    vrz_build_entry_claims()   //                                                §7.6
    vrz_derive_challenges()    // continues the SAME transcript; last use of it  §7.7
    vrz_pcs_verify()           // caps, per-query DEEP, folds                    §7.7
    vrz_route_claims()         // witness and quotient                           §7.6
    vrz_vanishing()            //                                                §7.8
    vrz_scalar_checks()        //                                                §7.9
    vrz_publish_statement()    // 337 felts to vrz_statement
}
```

Each step `fail`s on any inconsistency. Order is load-bearing twice: the transcript
must be replayed and then continued into `vrz_derive_challenges` with nothing in
between, and row hashing may only reuse `vrz_fs` **after** `vrz_derive_challenges` has
made its last transcript squeeze. Both facts belong in comments at the call site.

---

## 7. Port specification, module by module

Each subsection names the Zig source, the zkc file, the signatures, the recursion
shape and the fixture. `Ext` in a signature means six `𝔽` parameters in §6.2 order.
Line numbers are against `verifier-ray/src/` as of 2026-09-24 (post-merge, so
post-#3959 and post-#3977).

### 7.1 `field/koalabear.zkc`, `field/ext.zkc`

| Zig | zkc |
|---|---|
| `Element.inverse` (`koalabear.zig:113-132`) | `vrz_f_inv(x:𝔽) -> (r:𝔽)` — copy the 48-squaring / 6-multiply chain b1→b2→b4→b6→b12→b24 then `sqn(b6,25)·b24`. Straight-line, one row. Document that `inv(0)` returns 0 and that callers must reject rather than rely on it |
| `rootOfUnityBy` (`:146-159`) | `static VRZ_ROOTS(k:u5) -> (w:𝔽)`, 25 rows emitted by codegen |
| `Element.pow` | never a bit loop. `vrz_f_pow2k(x, k)` recurses on squarings; general powers go through `VRZ_ROOTS` |
| `bitReverse` (`fri.zig:290-298`) | `vrz_bitrev(v:u32, width:u5) -> (r:u32)` |
| `Ext.mul` (`koalabear_ext.zig:82-166`) | `vrz_ext_mul`, **measured at 91 columns**, one row |
| `Ext.square` (`:168-235`) | `vrz_ext_sqr` |
| `Ext.inverse` (`:237-333`) | `vrz_ext_inv` — adjugate, `E2` norm `d0²−3d1²`, one `vrz_f_inv` |
| `Ext.add/sub/neg/lift/mulByBase/eql/isZero/isBase` | one row each |
| `domainPointExt`, `shiftedPoint`, `pointInDomain` (`pcs.zig:1060-1087`) | `vrz_domain_point`, `vrz_shifted_point`, `vrz_point_in_domain` |

Fixtures: `verifier-ray/testdata/generated/vectors.zig` `field_cases` and `ext_cases`.

### 7.2 `crypto/poseidon2.zkc`

Zig: `poseidon2.zig:39-62` (`compressInPlace`), `:164-188`, `:227-328`. Constants
already exist in `arithmetization/src/main/lib/poseidon2/constants.zkc` — **include and
reuse the constants, but not the implementation**, which is loop-based and measured at
657 columns across 12 modules for several hundred rows per permutation (§2.2).

```zkc
fn vrz_p2_perm(s0..s15:𝔽) -> (t0..t15:𝔽)        // see §8.3 for the width split
fn vrz_p2_compress(l0..l7, r0..r7) -> (d0..d7)   // d_i = r_i + perm(l‖r)[8+i]
```

Note the feed-forward is of the **right** half, and partial blocks are **left**
zero-padded (`poseidon2.zig:124-133`). Both are easy to get backwards and neither is
caught by a round-trip test.

### 7.3 `crypto/transcript.zkc`

Zig: `crypto/fiat_shamir.zig`, `poseidon2.zig:78-148`.

```zkc
fn vrz_fs_reset()
fn vrz_fs_write(v:𝔽)                       // buffer, compress on the 8th
fn vrz_fs_write_ext(e0..e5)
fn vrz_fs_write_digest(d0..d7)
fn vrz_fs_sum() -> (d0..d7)                // left zero-pads a partial block
fn vrz_fs_random_digest() -> (d0..d7)      // sum, then write(0) for domain separation
fn vrz_fs_random_ext() -> (e0..e5)         // digest limbs 0..5
fn vrz_fs_random_ints(n:u32, mask:u32)     // recursive; 8 positions per digest
```

`Transcript.setState` **no longer exists** in the merged verifier-ray and must not be
reintroduced. `MDHasher.get/setState` survive one layer down and are equally not to be
ported. There is no state override anywhere in the replay.

### 7.4 `protocol/replay.zkc`, `protocol/public_input.zkc`

Zig: `protocol/root.zig:90-137`, `protocol/public_input.zig:69-110`.

Per round, in order: absorb each dynamic module size as a base element, absorb the
round commitment if present, absorb every cell (one limb if the key says base, six if
ext), then squeeze that round's coins into `vrz_coins`. Recursive over rounds and,
within a round, over sizes and cells.

`vrz_cell(round, index) -> Ext` merges the public inputs back in: if `(round, index)`
is a registered public-input slot it reads `vrz_pubin[statement_index]`, else it reads
the proof's cell at the index less the number of public-input slots earlier in that
round. The key emits the slot table sorted by `(round, index)`; the count per round is
small, so a recursive scan is cheaper than a second table.

Do **not** validate a cell's declared base/ext kind against its value. Prover-ray does
not (`proofserialization/README.md` §11.5), and adding the check here would reject
proofs the prover considers valid.

### 7.5 `crypto/merkle.zkc`

Zig: `crypto/merkle.zig`.

```zkc
fn vrz_hash_node(l0..l7, r0..r7, has_aux:u1, a0..a7) -> (d0..d7)
fn vrz_hash_row(row_base:u24, base_w:u16, ext_w:u16) -> (d0..d7)
fn vrz_hash_row_pair(row_base:u24, base_w:u16, ext_w:u16, self_even:u1) -> (d0..d7)
fn vrz_branch_auth(...) -> ()                    // running-layer branch, recursive
fn vrz_cap_auth(...) -> ()                       // recoverNode is naturally recursive
fn vrz_input_branch_auth(...) -> ()              // sparse row-opening branch
```

`capDepth` is a pure function of `Q` and height, and `Q` is fixed at codegen time, so
emit `static VRZ_CAP_DEPTH(h:u5) -> (d:u5)` rather than computing it.

Row hashing uses a **fresh** sponge, not the transcript. Since all row hashing happens
after the last transcript squeeze (§6.3), it can reuse `vrz_fs`; say so in a comment at
both ends.

### 7.6 `query/pcs_layout.zkc`

Zig: `pcs.zig:271-365` (`reconstruct`), `:414-438`, `:444-472`, `verifier.zig:245-299`.

Do **not** port the Zig triple loop (size descending × batch × ext × columns), which is
`O(23 · batches · columns)`. Three linear passes give the identical result as a
counting sort:

1. per column: resolve `size_log2` (static, or `log2(vrz_sizes[idx])` with a
   power-of-two check), record it, and take `POS[c]` from a running per-`(batch, size,
   is_ext)` counter;
2. walk buckets in canonical order (size descending, batch ascending, base before ext)
   accumulating `OFF[b][sz][ext]`;
3. per column: `entry = OFF[...] + POS[c]`, filling the per-entry arrays and
   `COL2E[c]`.

This reproduces `canonicalLayout` exactly because within a bucket the Zig enumeration
order is declaration order, which is what `POS[c]` counts.

Then `vrz_build_entry_claims` (read each column's claim cells into `vrz_ext`),
`vrz_resolve_roots` (per batch: the named round's commitment, or the emitted
precomputed root), `vrz_route_input_roots` (dedupe distinct roots by value in
first-appearance order), and `vrz_route_claims` (witness and quotient maps).

### 7.7 `query/pcs_verify.zkc`, `query/fri.zkc`

Zig: `pcs.zig:499-1087`, all of `fri.zig`. The largest piece of the port and the one
that carries the row budget; §8 sizes it.

Order within one query: authenticate the input-tree branches, resolve the running
layers, evaluate the final polynomial at the query's final-domain point, then walk the
bundles in canonical order computing for each the DEEP pair and writing it to the
aux slot, then run the fold recurrence.

Two things are not optional:

- **Cache the DEEP denominators.** `1/(x − ζ·ω_N^shift)` depends only on
  `(query, conjugate, size, shift value)` and never on the column. Computing it per
  `(entry, shift)` is `229 · 2 · 15,236 ≈ 7M` inversions per proof; hoisting it to per
  `(size, shift)` makes it `229 · 2 · ~60`. A 250× reduction, and without it nothing
  fits.
- **Dedupe aliasing shifts.** Distinct raw shifts can normalise to the same domain
  point at a small runtime size. Prover-ray's `RecoverBatchClaims` dedupes before
  batching, so an aliasing group must contribute exactly one term; a repeat must also
  be equality-checked against the kept claim, or a prover could route a different value
  into vanishing (`pcs.zig:1003-1031` explains the attack).

### 7.8 `query/vanishing.zkc`

Zig: `query/vanishing.zig`, **as changed by PR #3977**: `bucket` is now a runtime
parameter and both bucket loops are runtime, because comptime buckets produced 85
instantiations totalling ~6.0 MiB of a ~7.9 MiB `.text`. The zkc port is naturally
runtime throughout, so it matches the post-#3977 shape and should not follow
`verifier-ray/docs/system-codegen.md`'s "inline metadata loops" advice.

Recursion over modules, then buckets, then the bucket's constraints. Per module:
`annihilator = ζ^n − 1` via `vrz_ext_pow2k`; per bucket: recombine the quotient shares
in base `ζ^n`, accumulate `Σ merge^i · P_i(ζ) · C_i(ζ)`, and check
`aggregate == annihilator · quotient`.

`vrz_eval_expr(m, node) -> Ext` switches on the node kind. **Inline the leaf kinds
into the operator's own row**: of 482,341 nodes, 231,748 are operators and the rest are
leaves, so evaluating leaves through a separate recursive call roughly doubles the
module height for no benefit.

### 7.9 `query/scalar.zkc`

- **logderivativesum** (`logderivativesum.zig:38-60`): per query, `Σ z_final == result`,
  and `result == 0` when the key says so. R5 has one query with 2,265 refs.
- **grandproduct** (`grandproduct.zig:47-71`): `Π z_final == result`, and
  `result == expected` when the key carries one. R5 has one query, `expected = 1`.
- **rowlimit** (`rowlimit.zig:36-65`): both side sums strictly below the limit. R5 has
  139 checks over 6,668 module references.
- **shared randomness** (`shared_randomness.zig:56-86`): hash the coin round's
  commitment, or a zero octuplet when the key says that round has none, through the
  41-chunk multiset hash and compare 328 base-field limbs. On today's R5 system this
  compares two constants (§3) — implement it, but do not claim it binds anything.

---

## 8. Making it fit one shard

The budget is per module: every zkc function's invocation count must stay under
2^22 = 4,194,304, and `--padding next-power-of-two` means anything over 2^21 pads to
the full 2^22. The naive structure does not fit; two batching decisions make it fit.

### 8.1 The naive counts, per proof at Q = 229

| work | naive rows | verdict |
|---|---|---|
| opened-felt reads | 11.83M | **2.8× over** |
| `ext_mul` for DEEP terms and Horner | 11.83M | **2.8× over** |
| Poseidon2 compressions | 1.52M | fits |
| transcript element writes | ~105k | fits |
| `eval_expr` | 482k | fits |
| layout, claims, scalar checks | ~60k | fits |

### 8.2 Batch the two hot loops

Both offenders are narrow functions called many times, which is exactly the case
where rule 5 of §5.4 applies.

- **Reads → 8 per row.** The row data is consumed by the hashing pass 8 felts at a
  time anyway, since that is the sponge block size. A `vrz_hash_block` that reads 8
  felts and performs one compression turns 11.83M reads into **1.48M rows** at a width
  of about 50.
- **DEEP terms → `k` = 4 per row.** Unroll the inner walk four entries at a time:
  **2.96M rows** at a width of roughly 450. `k = 3` also fits at 3.94M rows with no
  headroom; take 4.

Both stay far inside the 255-register argument cap: four entries of value plus claim
is 48 registers.

This is what the earlier `../wiop-agg-design.md` §1.3 missed when it called a 1-to-1
shrink "2.8× over": it assumed one row per multiply and one row per read. A 2-to-1
node is **not** rescued the same way, because it would need `k ≈ 6` and the batched
module's width starts to cost more than it saves — which is why this plan stops at
1-to-1.

### 8.3 Poseidon2: the width decision

1.52M compressions is comfortably inside the row budget at one row each, so the
question is purely how much width to spend.

| shape | width | rows | note |
|---|---|---|---|
| fully unrolled, 1 row/compression | ~1,600 | 1.52M | simplest |
| `full_x3` reused twice + `partial_x21` | ~1,250 | 3.04M / 1.52M | |
| `full_x3` reused twice + `partial_x11` reused twice | **~900** | 3.04M / 3.04M | one wasted partial round |
| round-at-a-time loop | ~60 | 41M | **over budget** |

Take the third: about 900 columns instead of 1,600, which is worth roughly 1.5 MB off
the output proof. The wasted twenty-second partial round is a round with a zero round
key, which the constants table can supply. Start with the fully unrolled version to
get correctness, then narrow it — the fixtures do not change.

### 8.4 Resulting budget

| module | rows | width |
|---|---|---|
| `vrz_p2_perm` halves | 3.04M | ~900 |
| `vrz_deep4` | 2.96M | ~450 |
| `vrz_hash_block` | 1.48M | ~50 |
| `vrz_eval_expr` | 482k | ~120 |
| everything else | < 500k each | ~1,500 total |

Every module under 2^22, `Σ width ≈ 3,000`, which is the width-aware row of §2.3 and
the ~7× estimate.

---

## 9. Testing

### 9.1 Unit harnesses

Template: `arithmetization/src/test/zkc/bls12_381/` — a `.zkc` driver fed by
`pub input` arrays, `.accepts` / `.rejects` files with one JSON object per line and
`;;` comments, and a `Makefile` that splits each line into a temp JSON and runs
`go tool zkc execute -c --field KOALABEAR_16`, **grepping stderr for `FAILURES`**
rather than trusting the exit code.

| harness | covers | fixture source |
|---|---|---|
| `field`, `ext` | §7.1 | `vectors.zig` `field_cases`, `ext_cases` |
| `poseidon2` | §7.2 | `vectors.zig` `poseidon_cases`, plus the existing `test/zkc/poseidon2/permutation.accepts` |
| `transcript` | §7.3 | `vectors.zig` `fiat_shamir_cases`, `runtime_trace_cases` |
| `inputs` | §4.2 readers | a flattened fixture emitted by the Go flattener |
| `merkle` | §7.5 | derived from `generated/pcs.zig` |
| `pcs` | §7.6, §7.7 | `generated/pcs.zig`, 4 cases, one expecting `BoundaryAuxNotConstant` |
| `folds` | `fri.zkc` | `test/fri_fold_cases.zig` |
| `vanishing` | §7.8 | `generated/vanishing.zig`, honest and invalid views |
| `verify` | whole program | `generated/verify.zig`, 69 synthetic systems at 4 queries |

Because top-level names are global, each generated-system harness gets its own
`.zkc` and the Makefile runs one case at a time, chunked with `HARNESSES="..."` as the
bls12 Makefile does.

### 9.2 End-to-end

1. Regenerate the R5 key: `make -C verifier-ray generate-testdata`, then
   `make -C verifier-ray-zkc generate` for `r5_system.zkc`.
2. `make -C verifier-ray-zkc compile` — `zkc compile` must exit 0 with no unreachable
   module, and `zkc format --check` clean.
3. Flatten the committed `verifier-ray/testdata/riscv_proof_image.bin`'s underlying
   `VerifyInput` and run `Execute`; the statement must equal the shard proof's 337
   public inputs.
4. Negative: flip one byte of a cell, a root and a query branch in turn; each must
   `fail`.
5. `Trace` + `Check` on the accepting run: zero constraint failures.
6. `zkc trace --stats`: record per-module rows, columns and cells, and assert Σ width
   against a committed budget file so a width regression fails CI. **This is the
   product metric; treat a width regression like a performance regression.**

### 9.3 Measuring the actual compression

Once the compressor proves end to end, the number in §2.3 stops being an estimate:

```
compression = size(shard proof image) / size(compressor proof image)
```

using `proofserialization.Measure` on both sides, which takes any `wiop.Proof` and
needs no new plumbing. Record it next to the width budget.

---

## 10. Phases

Phase 0 needs no zkc code. Phases 1 to 3 can run in parallel; 4 onward are ordered.
Every phase ends green under `zkc execute -c` with the `FAILURES` grep, and
`zkc format --check` clean.

| phase | deliverable | acceptance |
|---|---|---|
| 0 | **Probes.** Confirm whether an input memory's module height is the data provided or only the addresses read (decides the §4.2.2 split factor). Measure `ext_inv` vs a 3-multiply batch step to confirm rule 6. Confirm fixed-array returns and `[𝔽;N]`. Measure a 1,000-step recursion against a 1,000-iteration loop. | numbers recorded in this file |
| 1 | `field/`, `ext.zkc` + harnesses | `field_cases`, `ext_cases` pass; `ext_mul` ≤ 100 columns |
| 2 | `crypto/poseidon2.zkc` (unrolled first), `transcript.zkc` + harnesses | vectors pass; 1 row per compression |
| 3 | `io/inputs.zkc`, the Go flattener, `zkcgen` skeleton emitting the spec and public-input tables | a flattened fixture reads back correctly |
| 4 | `protocol/replay.zkc`; wire `main.zkc` so the compile gate covers everything written so far | coins match `verify.zig` expectations on all 69 cases |
| 5 | `crypto/merkle.zkc` | pcs-derived micro-fixtures pass |
| 6 | `query/pcs_layout.zkc`; key emits the PCS tables | layout arrays match prover-ray's `GetLayout` / `canonicalLayout` |
| 7 | `query/pcs_verify.zkc`, `fri.zkc`, including the §8.2 batching | 4 pcs cases and the fold cases pass; `vrz_deep4` under 2^22 rows |
| 8 | `query/vanishing.zkc`; key emits the vanishing tables | honest views accept, invalid views reject |
| 9 | `query/scalar.zkc`; key emits the remaining tables | `verify.zig` cases with those queries pass |
| 10 | `main.zkc` complete, statement published, real R5 key and real proof | 69 synthetic cases end to end; real proof accepted; corrupted rejected |
| 11 | **Width pass.** §8.3's Poseidon2 split and any other rule-4 opportunity; measure compression (§9.3) | Σ width and the measured compression recorded; CI budget file added |

Phase 11 is deliberately last. Get it correct at ~5×, then narrow it to ~7×; the
fixtures do not change and the width work is mechanical once the arithmetic is pinned.

---

## 11. Risks

1. **`W_zkc` is an estimate.** Only `ext_mul` (91) and the loop-based Poseidon2 (657)
   are measured. If the long tail in §2.2 lands well above 5,000, compression falls
   below 5× and the case for the step weakens. Phase 1 and 2 give the first real
   reading; Phase 3 can total the widths of everything written so far and extrapolate
   long before Phase 10.
2. **The 2^22 input-memory ceiling is assumed, not confirmed.** If an input memory's
   height counts only addresses read, the split in §4.2.2 can shrink; if it counts
   provided data and R5's width grows, four memories may not be enough. Phase 0
   settles it.
3. **The `#[inline]` read dispatch may cost more width than expected** when inlined
   eight times into `vrz_hash_block`. Fallback in §4.2.2.
4. **Poseidon2 correctness has two silent traps**: feed-forward of the right half, and
   left zero-padding of partial blocks. Both pass a self-consistency test and fail
   against prover-ray. Use the `vectors.zig` cases, not a round trip.
5. **482,341 expression nodes is a large generated table.** If `zkc compile` struggles
   with a static table of that size, the fallback is to split it across several
   statics indexed by module range, at no width cost.
6. **Cross-shard binding is absent upstream** (§3). The compressor is sound as a proof
   compressor and says nothing about cross-shard consistency. State which one is meant
   wherever this component is described.
7. **The compressor is tied to one R5 key.** Any change to the arithmetization
   regenerates `r5_system.zkc` and changes the compressor's own key. That is correct
   for 1-to-1 and is exactly what the 2-to-1 design has to undo with a key-as-input
   and a propagated digest.

---

## 12. What changed from the accelerator plan

`../verifier-ray-zkc-plan.md` remains the reference for the accelerator. Imported
here largely unchanged: the zkc cost model (§5.1–5.3), the field and extension
representation (§6.2), the module-by-module port (§7), the harness conventions (§9.1),
and the phase structure (§10).

Changed, and why:

| | accelerator plan | here |
|---|---|---|
| shape | custom-1 instruction `WIOP_VERIFY` behind an R5 guest | standalone `main.zkc` |
| input | `proof_abi.zig` byte image copied into `ram` | flat felt `input` memories, no copy (§1.2, §4.2) |
| failure | status code in `rd` | `fail` (§4.1) |
| verification key | generated tables, same idea | same, and explicitly baked rather than input, because 1-to-1 needs no self-verification (§4.1) |
| objective | rows and cells | **Σ module width** (§2, §5.4) |
| Poseidon2 | fully unrolled, one row | split for width, ~900 columns (§8.3) |
| hot loops | one row per operation | batched `k`-way (§8.2) |
| batch inversion | not discussed | explicitly rejected (§5.4 rule 6) |
| wiring | five arithmetization touch points, predecoder, guest Zig | none |
| out of scope | — | 2-to-1 aggregation, cross-shard binding, the final SNARK wrapper |
