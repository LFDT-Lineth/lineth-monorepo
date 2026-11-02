# Granular field accelerators for ecrecover

The question: instead of one **monolithic** ecrecover accelerator (the PR #3719 zkc code planned in
[PLAN.md](PLAN.md)), should the guest keep a software ecrecover and accelerate only the 256-bit field
operations it is built from? This page covers:
- why the monolithic accelerator makes every shard proof bigger, and by how much;
- the granular design, its options, and whether to drop `#[native]`;
- a cost model and a comparison with std, Constantine and the monolithic accelerator;
- how to benchmark it, following [RUNNING_L2_EXEC.md](RUNNING_L2_EXEC.md) and [PLAN.md](PLAN.md),
  and on what machine.

Status: analysis, 2026-10-30, zkc v1.2.35, repository at `00a796726`. Every number carries one of
three labels:
- **measured**: from a run; the method is given (widths: §7, B0);
- **derived**: arithmetic on measured numbers;
- **estimate**: from code reading and operation counts; the benchmark step that confirms it is named.

## Verdict

- **Monolithic is the cheapest per call, but roughly doubles every shard proof** (measured, §1).
  - prover-ray commits every module in every shard, whether used or not.
  - The PR #3719 modules add 5,538 committed columns (7,558 opened felts) to today's `main.zkc`, which
    commits 3,137 columns (6,772 felts). That is +112% width, ~13.6 → ~27.4 MB per shard proof, on
    every block.
- **Granular acceleration keeps most of the per-call win at a fraction of the width.**
  - Measured: Fp mul/sq with the native product (G1-N) adds 547 columns (852 felts, +13%); adding
    add/sub too (G2-N) adds 1,193 columns (1,918 felts, +28%).
  - An ecrecover drops from ~3.6M RISC-V steps (std, measured) to ~40–270K (estimate, B1).
- **On total proof bytes (shards × width), native granular beats monolithic unless ecrecover is more
  than ~94–97% of a block's std steps** (§4).
  - Monolithic beats std only once ecrecover is more than ~53% of a block's std steps; G1-N does so
    above ~12%.
  - On a 786k-like block (ecrecover ≈ 70% of std steps) the model gives: std 1.00, monolithic 0.64,
    G1-N 0.39, G2-N 0.40.
- **On prover work (trace cells), the accelerators are within ~15% of each other** on such blocks.
  Monolithic is the cheapest per call (§4).
- **Dropping `#[native]` helps execution only with a gogen-compatible rewrite, and that rewrite is
  expensive** (measured, §2.2).
  - Removing the attribute alone makes the product +8,250 felts instead of +169, and gogen still
    rejects it (u512 registers, wide division).
  - A 64-bit-limb, division-free product (Crandall fold) runs under gogen and is fully constrained by
    zkc, but adds +3,776 felts on its own and +5,409 (+80%) at G2: most of the monolithic penalty.
- **Recommendation.**
  1. **Fix Constantine first** (ivokub's list in PR #4293). It adds no columns, runs under gogen, and
     every granular variant needs it.
  2. **Accelerate Fp multiplication/squaring from Constantine's limb layer, with the native product
     (G1-N).** Compute inversion and square root with it. Add add/sub/neg (G2-N) only if B2 shows
     that the blocks that matter spend more than ~80% of their steps in ecrecover.
  3. **Keep `#[native]`** and close the NonNative range-check gap (~+100 felts). Use the limb form
     only if gogen for the accelerated guest is a hard requirement. Do not ship the plain removal.

## 1. Why the monolithic accelerator costs every shard

**Every module is committed in every shard** (code reading). This settles SUMMARY blocker 5.
- The committed column set is fixed when prover-ray compiles the system
  (`prover-ray/zkcdriver/definition.go:236-333`).
- Every column must be assigned in every proof (`prover-ray/wiop/wiop_prove_verify.go:134-139`).
- A module the guest never calls is padded to the system's minimum height, in practice 4 rows
  (`prover-ray/zkcdriver/zkcdriver.go:97-121`, comment at L105-108).
- Every FRI query opens a row at every committed height
  (`prover-ray/crypto/koalabear/fri/pcs.go:596-602`). So a 4-row module costs the same per query as
  a 2^22-row one.
- Two mitigations exist but are off in the prover pipeline: column elision and a Merkle cap
  (`prover-ray/wiop/compilers/pcs/pcs.go:177-186, 218`). Neither has been evaluated here.

**Proof size is linear in the opened width W** (derived).
- A shard proof is ≈ Q · 2 · W · 4 B + ~1.2 MB of Merkle paths. At Q = 229 that is **1.83 KB per
  felt of W**.
- W counts 1 felt per base column and 6 per extension column.

**Today's baseline** (measured with prover-ray's compile pipeline, §7 B0).
- `riscv/main.zkc` commits **3,137 columns** (2,410 base + 727 extension), **W = 6,772 felts**,
  ~13.6 MB per shard proof.
- The September 2026 shape probe (`git show e80c833224:verifier-ray-zkc/PLAN.md` §3) reported
  10,622 columns and W = 25,832 (~48.6 MB). That was an older, larger `main.zkc`; do not mix the two.
- `zkc compile --stats` counts 2,310 columns for today's `main.zkc`. prover-ray adds lookup,
  multiplicity and quotient columns on top, so zkc's own counts are not committed columns.

**What the monolithic accelerator adds** (measured, harness minus baseline, §7 B0).

| | Committed columns | Opened felts | vs today's W | Shard proof |
|---|---|---|---|---|
| today's `main.zkc` | 3,137 | 6,772 | — | ~13.6 MB |
| + PR #3719 ecrecover modules | **+5,538** | **+7,558** | **+112%** | ~27.4 MB |

- For reference, zkc's own count of the same modules is 5,119 columns (natives at prover-ray's 111 and
  64), which matches SUMMARY blocker 5's "~5K".
- The biggest modules are `add_projective_secp256k1` (1,473 zkc columns),
  `double_projective_secp256k1` (881) and `ecrecover_generic` (797).

**Recursion grows with it** (estimate).
- A shard-proof verifier's work grows with committed columns plus their opening shifts. Monolithic
  adds ~177% committed columns to today's system; G1-N adds ~17%, G2-N ~38%.
- The compressor plan's one-shard fit (`git show e80c833224:verifier-ray-zkc/PLAN.md` §8) was sized
  on the September system. Re-check it with the B0 numbers before committing to either design.

**Module height cap** (derived).
- Shards are cut by `interpreter` calls only (zkc `pkg/zkc/vm/machine.go:101-131`).
- A module over 2^22 rows makes the prover panic (`prover-ray/wiop/wiop_runtime.go:205-208`).
- A monolithic ecrecover produces ~19K `addmod_256` rows (PLAN.md), so at most ~220 ecrecovers fit
  in one shard.
- At 1M `interpreter` calls per shard, this binds only when ecrecovers are fewer than ~4.5K steps
  apart. Measure the precompile-ecrecover block (B2) before assuming it does; a block that dense
  needs a smaller `--sharding` value.

## 2. The granular design

The guest keeps a software ecrecover. Only the field arithmetic becomes custom instructions, and zkc
proves them with the building blocks from PR #3719's `lib/nonnative`.

```
Constantine ecrecover (Nim -> C -> rv64im)          zkc arithmetization
  scalar recoding, tables, loop, point formulas  -> RISC-V CPU AIR (unchanged)
  Fp mul / sq / add / sub / neg  --- .insn r --->  field-op wrapper: 8x read_64, op, 4x write_64
  Fp inverse = a^(p-2), sqrt = a^((p+1)/4)            product (native or limb form), addmod, submod
    (computed on the accelerated mul)
```

Why Constantine and not Zig std (code reading):

| | Constantine (fork `Consensys-Incorporated/constantine@1ad824d5f5`) | Zig std 0.16.0 |
|---|---|---|
| Fp representation | **Crandall, i.e. plain residues** (`config_fields_and_curves.nim`: `modulusKind: Crandall(0x1000003D1)`), so `a·b mod p` plugs in directly | Montgomery (fiat-crypto, `std/crypto/pcurves/common.zig:19`); needs a plain-form shim, or 2 products per multiplication |
| Hook point | the `when UseASM_X86_64 … else` branch of `mulCran`/`squareCran`/`*PartialReduce` (`limbs_crandall.nim:193-261`) | not pluggable; you would vendor `secp256k1.zig`, `field.zig`, `scalar.zig` and `common.zig` |
| Ownership | our fork, already a build dependency (`riscv-guests/guest_crypto_ctt/build.zig.zon`) | upstream std |
| Algorithm today | ~4 constant-time scalar multiplications, 7 Fp + 2 Fr inversions (ivokub, PR #4293) | 1 double-base multiplication without GLV: 256 doublings, ~122 additions (`pcurves/secp256k1.zig:504-541`) |
| Algorithm after fixes | vartime GLV double-base: ~128 doublings, 1 affine inversion | GLV only through the private `mulDoubleBasePublicEndo` (`:469`) |
| Inversion / sqrt | Bernstein–Yang divsteps (no products) / `pow_vartime` with window 5 | 741 divsteps / addition chain (253 S + 13 M) |
| Extra toolchain | Nim 2.2.10, clang, llvm-ar | none |

Both libraries invert with divsteps, which use no field products. The accelerated build must
therefore switch Fp inversion to Fermat (a^(p−2)) on the accelerated multiplication. The Fr inversion
(one per ecrecover) can stay in software.

### 2.1 Instruction set

**Encoding.**
- custom-1 (`0x2b`, "internal" in `arithmetization/src/main/lib/README.md`), with funct3 `0b011`
  (free on main). funct7 selects the operation, and each operation gets one ComputeOp.
- The R-type ComputeOp range is full, so this shifts the S/B/J/U bases. Keep Go and zkc in sync with
  the `TestComputeOpBasesMatchConstantsZkc` guard proposed in PLAN.md (F9).

**Operands.**
- rd = destination pointer, rs1 = a, rs2 = b (sq and neg ignore rs2).
- Each operand is a 32-byte, 8-byte-aligned, little-endian array of four u64, which is
  Constantine's own limb layout.
- rd is a pointer, so the op is exempt from the rd = x0 → NO_OP fold, like keccak. Nothing is
  written back to registers.
- The modulus is implied by the ComputeOp (secp256k1 p), so it costs no operand and no column.

**Semantics.**
- Outputs are canonical (< p).
- Inputs may be anywhere in [0, 2^256), because Constantine reduces lazily (`lazyReduce=true` in
  `affine`, `batchAffine`, `pow`).
- The destination may alias either source: read both, then write.
- Never `fail` on guest data; a `fail` aborts the proof.

**Per-call cost** (estimate).
- 1–4 RISC-V steps at ~680 cells each (SUMMARY: 2.42G cells / 3.6M steps).
- 12 RAM helper calls at ~100 cells each (`read_64`/`write_64` already exist on main).
- One product row and one wrapper row.
- Total **~2–4K cells** with the native product. The same multiplication in software is a few
  hundred instructions, ~100–270K cells.

**Later option: a "field register file".** This is a small zkc memory of u256 slots, addressed by the
rd/rs1/rs2 fields. It removes the RAM traffic, bringing the cost to ~0.9K cells per op. But
Constantine's memory-resident code cannot use it, so it means hand-written point formulas.

### 2.2 The `#[native]` question

The product has three possible zkc forms. Widths are measured with prover-ray's compile pipeline on a
harness holding only the product, minus a baseline harness (§7 B0). The zkc column is
`zkc compile --stats` on the same harness, for reference.

| Form | What it is | zkc columns | Committed columns added | Opened felts added | gogen | Constrained by |
|---|---|---|---|---|---|---|
| **N**, native | today's `#[native] mulmod_256` | — (prover-ray: 111) | **+194** | **+169** | no (`gogen: native function … unsupported`) | prover-ray NonNative gadget |
| **U**, plain removal | same body with `#[native]` deleted (u512 product, u512/u256 division) | 3,586 | +4,380 | **+8,250** | no (`register "a" wider than 128 bits (u256) unsupported`) | zkc |
| **L**, limb form | 4×u64 limbs, u128 partial products, reduction by folding 2^256 ≡ 0x1000003D1 (no division) | 1,710 | +2,021 | **+3,776** | **yes** | zkc |

**Notes on the measurements** (measured unless stated).
- The deltas include a few dozen columns of harness noise (each harness's `main` differs slightly).
- `zkc exec --gogen` on the L harness accepts a correct product and rejects a wrong one.
  `zkc trace -c` is clean on the correct one and reports a constraint failure on the wrong one.
- The L prototype is unoptimized. It reassigns registers, so it runs as 16 rows per call instead of
  one. An SSA rewrite should make it a single row; it would not shrink the width much, because zkc
  splits every product into 16-bit limbs either way.
- L only works for Crandall-like primes. The bn254 or P-256 fields would need a Barrett or
  Montgomery limb version, which is wider.

**N has a soundness gap to close first** (code reading; confirm with the prover-ray owners). The same
gap affects the monolithic design.
- zkc emits no constraints for native functions (zkc `pkg/zkc/constraints/translate_fn.go:47-51`).
- The NonNative pass adds only the polynomial identity and the carry columns
  (`prover-ray/wiop/compilers/nonnative/nonnative.go`). Nothing range-checks the quotient or carry
  limbs.
- `remainder < modulus` is checked only in `NonNative.Check`, which is "not registered as a verifier
  action" (`prover-ray/wiop/query_nonnative.go:66-68`).
- Range-checking the quotient and carries would cost ~+100 felts (estimate).

**What dropping `#[native]` buys, in the L form.**
- The accelerated guest runs under gogen. That is `zkc exec --gogen`, the fast mode used by the
  guide's benchmarks and the CI act4 job.
- The field op can sit in the default `riscv/main.zkc` without breaking gogen for other guests.
- prover-ray needs no special path and no register-name contract.
- The soundness gap above does not apply.
- `zkc trace` itself does not use gogen in v1.2.35, so witness tracing is unaffected either way.

**What it costs.** L adds ~3.6K felts more than N, i.e. ~+53% of today's W, on every shard. U adds
~8.1K more and gains nothing; do not use it.

### 2.3 Which operations to accelerate

The rule (derived from the model in §4): **an operation earns its columns if and only if the share
of RISC-V steps it removes, on the blocks we care about, is larger than the share of W it adds.**

| Tier | Accelerated | Opened felts added | Steps per ecrecover (estimate, B1) |
|---|---|---|---|
| Constantine fixed | nothing | 0 | ~1–3M (ivokub: at or below std) |
| G1-N | Fp mul, sq | **+852 (+13%)**, measured | ~160–270K (add/sub stay in software, ~40–60 steps each) |
| G2-N | + add, sub, neg | **+1,918 (+28%)**, measured | ~40–90K |
| G2-L | as G2, limb-form product and limb add/sub | **+5,409 (+80%)**, measured | ~40–90K |
| G3 | + inversion and sqrt instructions (hint plus one product check) | a few hundred each (estimate) | saves only ~1–3K more; fails the rule |
| Point ops | point add / double as instructions | ~+5–6K (+75–90%), estimate from the monolithic modules | ~10–20K; dominated by G1-N/G2-N |

- **G1 vs G2** (derived, §4). Adding add/sub costs another ~1.1K felts (+16%) and removes ~120–180K
  steps per ecrecover. It earns its columns only where ecrecover is more than ~78% of a block's std
  steps.

Where the G2 estimate comes from (fixed Constantine, operation counts read from its source):
- **~3.1K products per ecrecover:**
  - ~128 Jacobian doublings × 7;
  - ~85 mixed additions × 11;
  - ~300 for the square root;
  - ~270 per Fermat inversion, of which there are two;
  - the R table.
- **~2–3K additions and subtractions.**
- **Glue, ~20–50K steps:** GLV split and recoding of the scalars, table selection, loop control.
- **The Fr inversion in software, ~10–20K steps.**

### 2.4 What changes where

**Constantine fork.**
- ivokub's fixes in `recoverPubkeyImpl_vartime` (`ecdsa.nim`):
  - drop the `verifyImpl` re-verification;
  - call `getAffine` once;
  - drop the `r + i·n` loop for the Ethereum entry point;
  - use a vartime GLV double-base multiplication;
  - precompute the table for G.
- Behind a define (proposed: `-d:CTT_LINETH_FIELD_ACCEL`):
  - emit `.insn r` in the Crandall mul/sq routines (G1), and add/sub/neg routines for G2;
  - invert Fp by Fermat.

**riscv-guests.**
- Forward the define from `guest_crypto_ctt/build.zig` to the nimble task.
- Add a build option in `l2_execution/build.zig`.
- Keep the software path for host builds.

**arithmetization.**
- A field-op library: the product (N or L), plus, for G2, the logic of PR #3719's
  `addmod_256`/`submod_256`.
- A RAM wrapper.
- The dispatch touch points from PLAN.md A2: `constants.zkc`, `interpreter_c`, `decode.go`,
  `doc.go`, and the decode tests.
- For N only: an opt-in entry point, like PLAN.md's `main_secp256k1.zkc`.

**prover-ray.**
- For N's product: nothing new beyond the NonNative range checks.
- For L: nothing.

## 3. Comparison per ecrecover

| Variant | Opened felts added per shard | RISC-V steps per call | Shards per call (1M) | Prover cells per call | gogen | New trusted zkc code | Basis |
|---|---|---|---|---|---|---|---|
| std | 0 | 3.6M | 3.6 | 2.42G | yes | none | measured (SUMMARY) |
| Constantine, pinned | 0 | ~8M | ~8 | ~5.6G | yes | none | derived (k ≈ 2.3, below) |
| Constantine, fixed | 0 | ~1–3M | ~1–3 | ~0.7–2G | yes | none | estimate (B0b, B1) |
| G1-N | +852 (+13%) | ~160–270K | ~0.2 | ~150M | no | small field op | width measured, rest estimated |
| G2-N | +1,918 (+28%) | ~40–90K | ~0.07 | ~45–80M | no | small field ALU | width measured, rest estimated |
| G2-L | +5,409 (+80%) | ~40–90K | ~0.07 | ~50–85M | **yes** | field ALU + limb product | width measured, rest estimated |
| Monolithic (PLAN.md) | +7,558 (+112%) | ~100s | ~0 | ~5.4M | no | several hundred lines of curve code | width measured, cells from SUMMARY |

k ≈ 2.3 is Constantine's ecrecover cost relative to std's. It is derived from the guide's gogen
times: ~50 vs ~24 ms per transfer, of which ~20 ms is std's ecrecover.

## 4. Comparison per block

**Model.**
- Proof bytes and recursion work per block are proportional to shards × W.
- Shards are proportional to RISC-V steps (`--sharding interpreter:N`).
- Let f be the share of a block's std steps spent in ecrecover, r a variant's ecrecover steps
  divided by std's, and δ its added W divided by today's W (6,772). Then:

  relative cost vs std = (1 − f + f·r) · (1 + δ)

**Parameters** (δ measured, r estimated).

| Variant | r | δ |
|---|---|---|
| Monolithic | ≈ 0 | 1.12 |
| G1-N | ≈ 0.06 | 0.13 |
| G2-N | ≈ 0.018 | 0.28 |
| G2-L | ≈ 0.018 | 0.80 |
| Constantine fixed | ≈ 0.5 | 0 |

**Relative proof bytes, by block type.**

| Block | f | std | Constantine fixed | Monolithic | G1-N | G2-N | G2-L |
|---|---|---|---|---|---|---|---|
| opcode-bound, one tx | 0 | 1 | 1 | 2.12 | 1.13 | 1.28 | 1.80 |
| mixed | 0.5 | 1 | 0.75 | 1.06 | 0.60 | 0.65 | 0.92 |
| 786k-like | 0.7 | 1 | 0.65 | 0.64 | 0.39 | 0.40 | 0.56 |
| transfer-heavy | 0.85 | 1 | 0.57 | 0.32 | 0.23 | 0.21 | 0.30 |
| ecrecover loop | 0.99 | 1 | 0.51 | 0.021 | 0.078 | 0.036 | 0.050 |

**Break-even points.**

| Variant | Beats std when | Beats monolithic when |
|---|---|---|
| Monolithic | f > 0.53 | — |
| G1-N | f > 0.12 | f < 0.94 |
| G2-N | f > 0.22 | f < 0.97 |
| G2-L | f > 0.45 | f < 0.91 |

G1-N beats G2-N when f < 0.78.

**Sensitivity.**
- **Steps.** If granular steps per call are twice the estimate, the "beats monolithic" bounds move to
  0.88 (G1-N), 0.95 (G2-N) and 0.83 (G2-L).
- **Baseline.** δ is relative to today's W. If `main.zkc` grows back to September's size (W = 25,832),
  the δs shrink about 4×: monolithic 0.29, G1-N 0.03, G2-N 0.07, G2-L 0.21. Monolithic then beats std
  above f ≈ 0.23, and native granular still beats monolithic below f ≈ 0.8 (G1-N) to 0.92 (G2-N).
  The added felts and MB per shard do not depend on the baseline.

**What f looks like** (derived).
- **786k block:** it traces to 60 shards with std and 117 with Constantine (guide). So
  f = (117/60 − 1)/(k − 1) ≈ 0.63–0.95, for k between 2.5 and 2.0.
- **`ether_transfers`:** ~10K std shards (gogen time scaled from the 786k block). ~9K of those come
  from its 2,500 ecrecovers, so f ≈ 0.85–0.9.
- **Opcode-bound fixtures:** f ≈ 0.
- **Real Linea blocks:** unmeasured. B2 measures f per input.

**Prover cells** (relative to std; the same model with δ = 0).
- At f = 0.7: monolithic 0.30, G2-N and G2-L 0.32, G1-N 0.34.
- Monolithic is the cheapest prover, by a small margin.
- It is also the only variant that can hit the module height cap (§1).

## 5. Wins and losses

**std**
- Wins: no new columns, runs under gogen, nothing to maintain.
- Losses: 3.6M steps per ecrecover. Transfer-heavy blocks are almost entirely ecrecover.

**Constantine, pinned**
- Wins: no new columns, runs under gogen.
- Losses: ~2.3× std per ecrecover, because of redundant work: constant-time paths on public data,
  re-verification, and repeated inversions.

**Constantine, fixed**
- Wins: no new columns, runs under gogen, and at or below std (ivokub's estimate). It is the
  prerequisite for every granular variant.
- Losses: still millions of steps per ecrecover; fork maintenance.

**Granular, native product (G1-N / G2-N)**
- Wins:
  - smallest proofs for f below ~0.94–0.97; G1-N is the better of the two below f ≈ 0.78;
  - 13–90× fewer steps than std;
  - a tiny trusted zkc surface: the curve logic stays in a library and is proven by the CPU AIR;
  - no module height risk;
  - the same instruction family serves any Constantine code over secp256k1's Fp.
- Losses:
  - +13% (G1-N) or +28% (G2-N) width on every shard;
  - no gogen: a separate entry point and bytecode `exec` only, as with monolithic;
  - needs the NonNative range-check fix;
  - changes to the Constantine fork (the hook and Fermat inversion);
  - reuse for bn254 or P-256 needs a Montgomery-product variant, because Constantine keeps those
    fields in Montgomery form.

**Granular, limb product (G2-L)**
- Wins: the per-call cost of G2-N, plus gogen for the accelerated guest, a single entry point,
  fully zkc-constrained arithmetic, and no prover-ray dependency.
- Losses: +80% width on every shard, close to the monolithic penalty; beats std only above f ≈ 0.45;
  per-prime reduction code.

**Monolithic (PLAN.md)**
- Wins: cheapest prover per call (~5.4M cells); ~1 step per call; best when ecrecover is
  essentially the whole block.
- Losses:
  - +112% width on every shard of every block (+13.8 MB per shard proof);
  - worse than std on proof size when f < ~0.53;
  - ~177% more committed columns for the recursion verifier to process;
  - only ~220 ecrecovers fit in a shard before the 2^22 cap;
  - several hundred lines of curve code in the trusted arithmetization;
  - no gogen;
  - ecrecover-specific: each further precompile adds its own columns.

## 6. Risks and open questions

- **The step estimates need B0b and B1.** r (steps per call) drives the comparison with the
  monolithic design; δ is already measured on harnesses and needs confirming on the real entry
  points (B0).
- **The baseline moves.** Today's W is 6,772; the September system was 25,832. As `main.zkc` grows
  (keccak and other accelerators), every relative figure here shrinks while the absolute felts stay.
  Re-run B0 whenever `main.zkc` changes.
- **NonNative soundness** (§2.2). It applies to N and to the monolithic design.
- **Lazy reduction.** Constantine passes operands in [p, 2^256).
  - With N, the product of two such operands can push the quotient past 256 bits, which
    `mulmod_256` rejects with `fail`. The wrapper must canonicalize first, with one conditional
    subtraction (the measured G1-N/G2-N harnesses include it).
  - L's fold accepts any 256-bit input.
- **Host and comptime builds** must keep the software path. Map the accelerated backend to the plain
  one for host tools, as PLAN.md does for `native_options`.
- **Silent misdispatch.** An unhandled ComputeOp is a silent no-op in `interpreter_c` (PLAN.md F7).
  The guest wrapper should check a sentinel on its first call.
- **Bytecode `exec` panics on mixed-width comparisons** (PLAN.md F4). Write the wrapper with equal
  widths. L uses no u256 registers, so it avoids this by construction.
- **Upstream.** gogen support for `#[native]` and for registers wider than 128 bits would remove N's
  main disadvantage.

## 7. How to benchmark

Follow [RUNNING_L2_EXEC.md](RUNNING_L2_EXEC.md) for:
- inputs and wrapping;
- `IN_ORIGIN` `0x08800000`;
- the `exec` and `trace` commands;
- the methodology: run sequentially, discard the first run, report the median of ≥3, alternate
  variants.

Reuse PLAN.md's `bench.sh` and `summarize_trace.py`, with the variants below.

```bash
export REPO=$(git rev-parse --show-toplevel)
export WORK=/tmp/l2-bench
```

### 7.1 Variants

| Variant | Guest build | zkc entry point | gogen |
|---|---|---|---|
| `std` | `-Dsecp256k1-backend=std` | `riscv/main.zkc` | yes |
| `constantine` | `-Dsecp256k1-backend=constantine`, pinned fork | `riscv/main.zkc` | yes |
| `ctt-fixed` | constantine, fork branch with the fixes | `riscv/main.zkc` | yes |
| `ctt-g1n`, `ctt-g2n` | fork branch with the hook, accel level g1 or g2 | `riscv/main_fieldaccel.zkc` (opt-in) | no |
| `ctt-g2l` (optional) | as `ctt-g2n`, with the L-form product | `riscv/main.zkc` (or its own entry) | yes |
| `zkc` (optional) | monolithic, per PLAN.md | `riscv/main_secp256k1.zkc` | no |

- **Building the fork variants.** Build them one after another:
  1. Pin the fork commit in `riscv-guests/guest_crypto_ctt/build.zig.zon`:
     `zig fetch --save=constantine 'git+https://github.com/Consensys-Incorporated/constantine#<commit>'`.
  2. Pass the accel level through a new build option. Proposed: `-Dctt-field-accel=off|g1|g2`,
     forwarded to the nimble task as a `-d:` define.
- **Prefixes.** Build every variant into its own prefix, always with `-Dkeccak-accel=false`:
  `zig build --prefix $WORK/guest-<variant> …`.
- **Stale JSON.** Delete `$WORK/json/*.<variant>.json` after any rebuild (the guide's stale-JSON
  pitfall).

### 7.2 Steps

**B0, width.** Run first; it takes seconds on a laptop. It gives δ for every variant. This is how the
widths in this document were measured.
- `zkc compile --stats` gives zkc's own column counts. From `arithmetization/`, run
  `go tool zkc compile --stats <entry>.zkc` (leave out `-o`: with `-o` the table is not printed).
  These are **not** committed columns.
- Committed columns and W come from prover-ray's compile pipeline, without proving:
  1. `go tool zkc compile -o <entry>.bin <entry>.zkc` from `arithmetization/` (zkc v1.2.35).
  2. A short Go test inside `prover-ray/` that, for each binary, calls
     `zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(bin))`, then
     `compilers.CompileFull(sys, compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()))`,
     and counts the columns of `pcs.CommittedBatches(sys)`: 1 felt per base column, 6 per extension
     column.
  3. Run it in `prover-ray/`, not `verifier-ray/`: verifier-ray pins zkc v1.2.33, which cannot
     read v1.2.35 binaries. This is also why the September shape probe
     (`verifier-ray/codegen/shape_probe_test.go` on `e80c833224`) cannot be reused as is.
- **On the real entry points**, compare `main.zkc` against `main_fieldaccel.zkc` and
  `main_secp256k1.zkc`.
- **Before the wiring exists**, isolate a module set with a standalone harness:
  - the harness holds the new modules, a `main` that calls them, and the same `ram` memory and
    `read_64`/`write_64` as main;
  - subtract a baseline harness that has only that `main` scaffolding;
  - every range width the modules don't use must still be reachable, or `zkc compile` rejects the
    unused `$range_uN` tables.
- **Per-shard proof bytes** ≈ 1.83 KB × W + 1.2 MB.

**B0b, operation counts.** Run on the host; a laptop is enough.
- Build Constantine's ecrecover on the host with `-d:CTT_METER` (`platforms/metering/tracer.nim` in
  the fork), for the pinned and the fixed versions.
- Record Fp and Fr products, additions and inversions per call. This checks the counts in §2.3.

**B1, per call.** Run `ecrecover_bench.zig` from PLAN.md (T2) with K ∈ {0, 16} for every variant.
The per-call cost is (X(16) − X(0)) / 16, where X is each of:
- `interpreter` calls: the shard count at `--sharding interpreter:100000`, or the `interpreter` row
  count in `trace --stats -r`. This gives r.
- trace cells, raw and prover-ray-adjusted (only N needs the `mulmod` adjustment);
- rows of the new modules;
- `zkc exec` time: bytecode for every variant, plus `--gogen` for the variants marked yes.

**B2, per block.** Use PLAN.md's inputs: the guide's sample blocks, the seven 30M fixtures, the 250-
and 1000-transfer blocks, `ether_transfers`, and the EEST `precompile/ecrecover` benchmark.
- Run `exec` for every variant (bytecode), plus `--gogen` where possible.
- Trace `786k`, the 250-transfer block, `ether_transfers` and precompile-ecrecover with:
  `GOMEMLIMIT=64GiB taskset -c 0-15 go tool zkc trace --stats -r -vv --sharding interpreter:1000000`.
- For each input and variant, report:
  - shards;
  - Σ padded cells, and adjusted cells;
  - the **proof-size proxy**: shards × (1.83 KB × W + 1.2 MB), with W from B0;
  - the recursion proxy: shards × (committed columns + shifts);
  - the max module height per shard, against 2^22;
  - f = (shards_std − shards_ctt-g2n) / shards_std.
- Correctness: the `guest_output` bytes must be identical across all variants (PLAN.md T6).

**Report.** Add a granular section to `RESULTS.md` (from PLAN.md) with:
- the B0 widths for N, L and monolithic on the real entry points;
- the per-call table of §3, with measured r;
- the per-block table of §4, with measured f;
- the verdict.

## 8. Machine requirements

| Task | Machine | Notes |
|---|---|---|
| Implementation, unit tests, B0, B0b | laptop | Zig 0.16.0, Go, Nim 2.2.10, clang, llvm-ar (`make -C riscv-guests install-constantine-deps`). B0 takes seconds per binary. |
| B1 | modest; a laptop works | std and Constantine traces at K = 16 need ~6.5 GB (SUMMARY); granular ones need far less. Run on the B2 machine anyway, so all variants share one machine. |
| B2 | Linux, ≥16 cores, ≥91 GB RAM (the guide's reference machine); 128 GB is safer | Shards are 1M `interpreter` calls for every variant, so per-shard RSS is about std's. G2 adds ~3K product rows per ecrecover, and ~11–25 ecrecovers fit in a shard: far below 2^22. Never run two traces at once. |

Time budget (estimate):
- std and Constantine traces still dominate, at ~1–1.5 days of machine time, as in SUMMARY.
- `ctt-fixed` costs about as much as std.
- Each granular variant traces `ether_transfers` in ~1–2.5K shards, against ~10K for std, so it
  takes ~10–25% of std's trace time. Three of them add roughly half a day.
- Granular `exec` is fast. `ctt-g2l` also gets gogen timings that compare directly with the
  guide's.
- Real prover-ray proof bytes are optional.
  - `prover-ray/zkcdriver/r5_benchmark_test.go` reports a `proof-bytes` metric.
  - The production backend asserts a single shard (`prover-ray/backend/core.go:376-378`).
  - Proving hardware is not sized here.
