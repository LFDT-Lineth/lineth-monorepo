# Plan: port `verifier-ray` (Zig) to a zkc guest accelerator (`verifier-ray(zkc)`)

Status: planning document, 2026-09-18. Nothing in this plan is implemented yet.

Companion background docs (read first, they are cited as "§" below):

- `lineth_overview.md` — zkc toolchain, Go APIs, how traces are obtained.
- `verifier-functionality.md` — what the Zig verifier does, file:line cited.

This document is written so that each phase can be handed to a low-cost agent
as a self-contained task: every phase names the Zig source to port, the zkc
file to create, the exact function signatures, the test fixture to reuse, and
an acceptance criterion. Read §3 (zkc rules) before writing any zkc.

---

## 0. Objective and scope

**Objective.** Replace the per-RISC-V-instruction interpretation of the Zig
verifier (`verifier-ray/src/**`, run as an R5 guest) by a **single custom
RISC-V instruction** whose semantics are implemented directly in zkc, so that
verifying one WIOP shard proof costs one interpreter row plus the accelerator's
own trace rows, instead of millions of interpreter rows.

**Deliverable.** A zkc library `arithmetization/src/main/lib/wiop/` plus one
generated file per compiled prover system, wired as accelerator
`WIOP_VERIFY` (custom-1, funct3 `0b011`), callable from a Zig guest via
`lineth_accelerators.zkvm_wiop_verify(input_ptr, system_id) -> bool`.

**Non-goals.** Changing the proof image format, changing prover-ray, verifying
several different systems in one accelerator (reserved via `system_id`),
proving the accelerator end-to-end (blocked on prover availability; the plan
stops at `zkc execute -c` / `zkc trace --stats`).

**Assumed baseline.** `origin/main` at `b75b5ca95` with PRs #3940, #3950 and
#3959 merged. Their effect on the verifier (verified against the PR heads,
fetched as local branches `pr-3940`, `pr-3950`, `pr-3959`):

| PR | Effect relevant here |
|---|---|
| #3940 | prover-ray message-bus/shared-randomness refactor; no verifier-ray source change. |
| #3950 | Removes `Round.PreSamplingHooks` and `Runtime.SetFSState` from prover-ray. The Fiat–Shamir schedule has **no state override any more**. |
| #3959 | verifier-ray follows: `protocol/root.zig` loses `gammaDigest`, `SharedRandomnessGammaRef`, `shared_randomness_coin_round`, `shared_randomness_gamma_refs`. `query/shared_randomness.zig` now hashes **only the coin round's own commitment** (or a zero octuplet when that round has no commitment) through `multiset_hashing.hash`, and compares 328 limbs. `fiat_shamir.Transcript.setState` becomes dead code. Codegen `spec_zig.go` no longer emits the two γ fields. |

Everything else in `verifier-ray/src/**` is byte-identical between `main` and
`pr-3959`, so the rest of this plan cites `main` line numbers.

PR #3927 (BLS12-381 pairing precompile) is the **reference implementation of
an accelerator**: `arithmetization/src/main/lib/bls12_381/impl.zkc` (RAM/ABI
wrapper), the five arithmetization touch points, and
`riscv-guests/lineth-accelerators/src/bls12_381.zig` (guest wrapper). §7 lists
those touch points concretely for `WIOP_VERIFY`.

---

## 1. Ground truth established during planning (corrections to earlier docs)

1. **Fiat–Shamir has no γ override.** `verifier-functionality.md` §1 step 4
   ("optional shared-randomness override", `gammaDigest`) describes code that
   PR #3959 deletes. The replay is exactly: absorb dynamic sizes → absorb
   commitment → absorb cells → squeeze the round's coins. Fixed in that doc.
2. **Shared randomness sub-verifier changed shape** (PR #3959): it hashes one
   commitment (the message-bus coin round's), not every preceding committed
   round. Fixed in that doc.
3. **The PCS is "multi-size FRI"**, never Vortex, in verifier-ray. Residual
   "FRI/Vortex" wording in `verifier-functionality.md` §0/§2 is fixed.
4. **Proof image elements are canonical `u32`, not Montgomery.**
   `prover-ray/wiop/proofserialization/README.md` §5 says elements are stored
   in Montgomery form; `types.go:159-196` converts every limb with `Bits()[0]`
   (from-Montgomery) and `verifier-ray/src/field/koalabear.zig` does canonical
   arithmetic (`init(raw) = raw % p`). The README sentence is stale; the zkc
   reader casts `u32 as 𝔽` with no conversion.
5. **The `zkc/` directory at the repo root is an untracked clone of the zkc
   compiler at commit `26ee2a0` (2026-07-31), older than the pinned
   `v1.2.32`.** Its parser lacks `done`, the `f!(...)` never-returning call,
   `#[global]`, the `memory m[uN]` timestamp bound, and fixed arrays. Do **not**
   use it as the language reference. Use
   `/Users/arijitdutta/go/pkg/mod/github.com/!l!f!d!t-!lineth/zkc@v1.2.32/`
   (`docs/ZKC_LANGUAGE.md`, `testdata/zkc/unit/*.zkc`).
6. **Poseidon2 parameters match.** `arithmetization/src/main/lib/poseidon2/`
   (width 16, 6 full + 21 partial rounds, S-box 3, gnark-crypto constants) is
   the same permutation `verifier-ray/src/crypto/poseidon2.zig` uses, and the
   Zig verifier already delegates to it via the `RTYPE_POSEIDON2` accelerator
   when accelerators are enabled. The MD compression is
   `state' = right + perm(state ‖ right)[8..16]` (`poseidon2.zig:39-56`), i.e.
   feed-forward of the **right** input, and partial blocks are **left**
   zero-padded (`sumDigest`, `:124-133`).
7. **The proof image is loaded into guest RAM byte by byte.** `IN_BYTES=@image.bin`
   becomes a blob (`elf_to_json`), and `riscv/main.zkc:57-67` copies every blob
   byte with `write_8` before execution starts. For a multi-MB proof this
   fixed cost exists for the Zig verifier today and is unchanged by this work;
   see §9.6.

---

## 2. Target architecture

### 2.1 Runtime flow

```
Zig guest (rollup guest, later)                zkc interpreter (riscv/main.zkc)
────────────────────────────────               ─────────────────────────────────
input region at 0x08800000 holds               main: copy blobs into `ram`
the proof image (VerifyInput at +0)            interpreter!: fetch/dispatch …
                                                 case RTYPE_WIOP_VERIFY_WB:
status = zkvm_wiop_verify(_in_start, 0) ──►      rd = wiop_verify(v1 as Address, v2)
   .insn r 0x2b, 0b011, 0, rd, rs1, rs2            ├─ read proof fields from `ram`
                                                    ├─ replay transcript (Poseidon2)
if (!status) exit(1)                                ├─ PCS + FRI (Merkle, DEEP, folds)
write_output(public inputs…)                        ├─ vanishing / logderiv / gp / rowlimit / sr
                                                    └─ status 1 (accept) / 0 (reject)
```

The accelerator **never `fail`s on a bad proof**: it returns status 0 and the
guest decides (same rule as #3927; a rejected proof is a provable outcome). It
does `fail` on conditions that mean the *compiled system* and the *program*
disagree (codegen bugs), mirroring the Zig `@compileError`/`unreachable`s.

### 2.2 ABI (R-type, custom-1 = opcode `0x2b`)

| Field | Value |
|---|---|
| funct3 / funct7 | `0b011` / `0b0000000` (next free custom-1 slot after `write_output`; add a row to `arithmetization/src/main/lib/README.md`) |
| rs1 | absolute guest address of `verifier.VerifyInput` (the image base; must equal the base the image was relocated for, `proofserialization.GuestBase = 0x08800000`) |
| rs2 | `system_id`; only `0` is implemented, any other value → status 0 |
| rd | `1` accept, `0` reject. **Writeback op**: folds to `NO_OP` when `rd = x0` (like `RTYPE_BLS12_PAIRING_CHECK_WB`, unlike keccak/poseidon2 which read rd). |

### 2.3 Files

```
arithmetization/src/main/lib/wiop/
  README.md                 encoding, ABI, memory map, how to regenerate system.zkc
  field.zkc                 base-field helpers: inverse, pow2k, roots of unity, bitrev
  ext.zkc                   E2 / Ext (F_{p^6}) arithmetic on 6 felts
  poseidon2_compress.zkc    straight-line compress(l[8], r[8]) -> d[8]  (§5.3)
  transcript.zkc            MD hasher + Fiat–Shamir over memory `wiop_fs`
  image.zkc                 typed accessors over the proof image in `ram` (§4.3)
  public_input.zkc          bindRoundMessages equivalent (cell lookup with PI merge)
  replay.zkc                transcript replay → coins
  merkle.zkc                hashNode, branch/cap authentication, row hashing
  pcs_layout.zkc            reconstruct(), buildEntryClaims(), routeInputRoots()
  pcs_verify.zkc            deriveChallenges(), input caps, per-query DEEP + FRI folds
  vanishing.zkc             expression evaluator, buckets, selector, cancellation
  scalar_checks.zkc         logderivativesum, grandproduct, rowlimit, shared_randomness
  verify.zkc                wiop_verify(input_ptr, system_id) -> status   (top level)
  impl.zkc                  include glue for riscv/interpreter.zkc
  system_0.zkc              GENERATED: static tables + consts for compiled system 0
verifier-ray/codegen/zkc/   GENERATED-FILE EMITTER (Go): CompiledSystem -> system_0.zkc
verifier-ray/codegen/generate-riscv-system/  gains a `-zkc` output next to riscv_system.zig
arithmetization/src/test/zkc/wiop/           unit harnesses (§8)
riscv-guests/lineth-accelerators/src/wiop.zig  guest wrapper
```

Every file must be reachable from `riscv/main.zkc` (see §3.6, dead code is a
compile error), so `impl.zkc` includes everything and `verify.zkc` calls
everything.

### 2.4 Generated versus hand-written

The Zig verifier takes the compiled system as `comptime` data
(`riscv_system.zig`, produced by `verifier-ray/codegen`). The zkc port keeps
that split: **hand-written engine + generated data**. The generated file holds
only `const`s and `static` tables (§4.5); the engine reads them. This keeps the
engine testable against the small synthetic systems in
`verifier-ray/testdata/generated/*.zig` and the real R5 system alike.

---

## 3. zkc cost model and coding rules (read before writing code)

Verified against zkc v1.2.32 (`docs/ZKC_LANGUAGE.md`, `docs/ZKC_ARCHITECTURE_*.md`,
`testdata/zkc/unit/`) and against measurements recorded in project memory
from the BLS12-381 work.

### 3.1 What costs what

| Construct | Trace cost |
|---|---|
| Function call | one lookup; the callee gets **one row per call** if it is *atomic* (no loops, single vector bundle). Every local/param/return is a **column** of the callee module. Identical calls (same args) can be shared by the lookup. |
| Loop (`for`/`while`) | forces a *multi-line* function: `PC` and `RET` columns, one row **per iteration per bundle**, constancy constraints on every unwritten register. Expensive when the body is small. |
| Recursion | each call is one atomic row; no PC/RET; tail recursion `f!(…)` for never-returning functions. **Prefer recursion over loops for unbounded iteration** (this is the language doc's own advice). |
| `if`/`switch` arms | all arms are materialised as columns; **untaken arms cost zero rows** (measured: k=0 scalar mul 753 cells vs 11.8M). |
| Memory access (`memory m[uN]`) | one row in the memory's module per read or write, plus a timestamp column of width N. |
| `input`/`static` read | a lookup into a fixed table; cheaper than RW memory (no timestamp). |
| `ram` read (`read_32`, aligned) | measured ≈ 100 cells per word (2,553 cells for 12 aligned words incl. call rows); unaligned ≈ 4×. Keep image reads 4-byte aligned; they are (all image types are 4- or 8-aligned). |
| `u32 as 𝔽` | free reduction. `𝔽 as u32` inserts a canonicality check (`value < p`). |
| `𝔽` arithmetic | `+`, `-`, `*` allowed; `==`/`!=` allowed; **no `/`, `%`, shifts, bitwise** on `𝔽`. Inverse must be computed (Fermat chain) — there is no hint mechanism. |

### 3.2 Straight-line beats everything

A function with no loops compiles to **one row** regardless of how many
operations it contains; the operations become columns. So a fully unrolled
Poseidon2 permutation (≈1.3k columns) is one row per compression, whereas the
existing loop-based `lib/poseidon2/poseidon2.zkc` spends hundreds of rows (each
state access is a memory row) per permutation. Cells = rows × columns, so the
straight-line form wins by roughly an order of magnitude. Same for Ext
multiplication (Karatsuba, 6 E2 muls, 1 row) and Ext inverse (1 row).

Rule: **any fixed-size computation is written straight-line** (Poseidon2
compress, Ext ops, base inverse, `pow` by a compile-time power of two when the
exponent is known). **Any data-dependent iteration is written as recursion**
over an index, with state carried in a small RW memory rather than in long
argument lists.

### 3.3 Argument/return limits and register splitting

- A call may pass at most 255 *split* registers (measured cap; a `𝔽` is one
  register at `KOALABEAR_16`, a `u64` is 4). 16 felts (Poseidon2) or 12 felts
  (Ext × Ext) are fine. Returns are not limited.
- Fixed arrays exist (`var a:[𝔽;6]`, params `x:[u8;3]` confirmed in
  `fixed_array_04.zkc`) but indices must be **constant expressions**. Use them
  for readability only where the index is a literal; otherwise pass felts
  individually. Confirm array-typed **returns** and `[𝔽;N]` compile before
  relying on them (Phase 0 probe).
- Multi-column RW memory is allowed: `memory wiop_coins[u16](i:u10) -> (c0:𝔽, c1:𝔽, c2:𝔽, c3:𝔽, c4:𝔽, c5:𝔽)`
  (`testdata/zkc/unit/ram_10.zkc`). Multi-column `static` too, with tuple rows
  `(a, b),` (`partial_call_05.zkc`); partial reads `lo, _ = tbl[i]` are allowed.

### 3.4 Known compiler gotchas (from project memory, all still relevant)

- `zkc execute -c` / `Check` can report `FAILURES` while exiting 0. Harness
  scripts must grep for failures, not trust exit codes.
- Branching on **two** values returned by one multi-return call inside a loop
  panicked the tracing transform ("conflicting read on register"). Return one
  value, or branch on one.
- `fp12 squaring + conditional dense multiply inside a loop` broke the AIR
  silently. Avoid conditional heavy arithmetic inside loops; use recursion or
  make the arithmetic unconditional.
- `zkc execute --fast` panics on some wide-limb types; unit harnesses must
  run in tracing mode (`execute -c`), as the bls12 harness does.
- Every fixture that reaches guest RAM must supply all five R5 inputs
  (`entry_point_and_blobs_count`, `blobs_offset_and_size`, `blobs_data`,
  `instruction_base`, `decoded`), else zkc nil-derefs.
- Top-level `const` names share one flat namespace across the whole include
  graph: prefix everything with `WIOP_`.
- Boolean-valued `pub input` columns must be `u8`, not `u1`; every hex literal
  in fixtures must have even length.

### 3.5 Memory timestamps

`memory name[uN](addr:…) -> (…)` caps accesses at `2^N − 1` per execution.
Size each scratch memory for the real proof (§9.1 gives the counts): the
transcript memory sees ~16 writes per compression; use `u32` everywhere unless
a probe shows column pressure.

### 3.6 Dead code is a compile error

`zkc compile riscv/main.zkc` (CI gate `riscv-check-compilation`) rejects any
function unreachable from `main`. Land library code only together with a call
path from `verify.zkc`, or keep it in files included only by test harnesses
until its consumer lands (the bls12 split pattern).

### 3.7 Never-returning functions

`fn f(...) -> !` with `f!(args)` at the call site is a tail call that does not
return; `done` terminates the whole program successfully. Only `interpreter`
uses this; the verifier must **return** a status, so it is an ordinary
function.

## 4. Data representation

### 4.1 Field elements

- Base field element: `𝔽` (KoalaBear, `p = 2^31 − 2^24 + 1 = 2130706433`). From
  the image: `read_32(addr) as 𝔽`.
- `E2 = 𝔽[u]/(u² − 3)`: two felts `(a0, a1)`. Non-residue is **3**
  (`koalabear_ext.zig:115`: `x0*y0 + 3*x1*y1`).
- `Ext = E2[v]/(v³ − (u+1))`: six felts in order `B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1`
  — this is also the image byte order (`ext.Ext` @0/@8/@16) and the coin limb
  order (`randomExt`: digest limbs 0..5). `nr(x) = (x0 + 3·x1, x0 + x1)` for the
  cubic reduction (`koalabear_ext.zig:152-159`).
- Digest / commitment: 8 felts, image order.
- Constants that must be reproduced exactly: `inv_two = 1065353217`, `root_of_unity = 1791270792`
  (order 2^24), `multiplicative_gen = 3`, `leaf_domain_tag = 0x4c66_7269_5f6c_6631`
  (absorbed as `Element.init(tag)` i.e. `tag mod p` — compute the reduced
  constant at codegen time and pin it in a unit test against the Zig value).

Zig helpers → zkc functions (all straight-line, one row each):

| Zig (`field/koalabear.zig`, `field/koalabear_ext.zig`) | zkc (`lib/wiop/field.zkc`, `ext.zkc`) |
|---|---|
| `Element.inverse` (48 sqr + 6 mul chain, `:113-132`) | `fn wiop_f_inv(x:𝔽) -> (r:𝔽)`; copy the chain b1→b2→b4→b6→b12→b24, `sqn(b6,25)·b24`. Inverse of 0 must return 0 (used by `1/0 = 0` semantics in Lagrange path? No: the Zig `unreachable`s. Return 0 and let the caller reject; document.) |
| `rootOfUnityBy(2^k)` (`:146-159`) | `static WIOP_ROOTS(k:u5) -> (w:𝔽)` with 25 rows, `w_k = root_of_unity^(2^(24−k))`; codegen computes them. |
| `Element.pow(e)` for domain points | never a bit loop: `fn wiop_f_pow2k(x:𝔽, k:u5) -> (r:𝔽)` = recursive squaring `k` times (one row per squaring); general `pow` only via `WIOP_ROOTS` + bit-reverse. |
| `bitReverse(v, width)` | `fn wiop_bitrev(v:u32, width:u5) -> (r:u32)`: recursion over 1 bit per row, or a 32-step straight-line reverse followed by `>> (32 − width)`. |
| `E2.add/sub/neg`, `Ext.add/sub/neg/lift/isZero/isBase/eql` | 6-felt in / 6-felt out; `eql` returns `u1`. |
| `Ext.mul` (Karatsuba `:82-166`) | `fn wiop_ext_mul(x0..x5, y0..y5) -> (z0..z5)`; write the six E2 products explicitly; no `% p` bookkeeping needed (𝔽 arithmetic is exact). |
| `Ext.square` (`:168-235`) | `wiop_ext_sqr(x0..x5) -> (z0..z5)`. |
| `Ext.mulByBase`, `divByBase` | `wiop_ext_mul_base(x0..x5, s) -> (…)`; div = mul by `wiop_f_inv(s)`. |
| `Ext.inverse` (`:237-333`) | `wiop_ext_inv(x0..x5) -> (…)`: adjugate A,B,C, norm `d ∈ E2`, `E2` norm `d0² − 3·d1²`, one `wiop_f_inv`. |
| `Ext.pow(2^k)` (`powModuleSize`, dynamic n) | `wiop_ext_pow2k(x, k)` recursive squaring; static `n` → codegen emits `k` as a constant per module, same function. |

### 4.2 Scratch memories (declared in `lib/wiop/memories.zkc`, threaded through `main`/`interpreter*`)

| Memory | Geometry | Holds |
|---|---|---|
| `wiop_fs[u32](i:u5) -> (v:𝔽)` | 32 cells | transcript: state `[0..8)`, buffer `[8..16)`, `buffer_len` at 16 (stored as 𝔽 of a small int; cast back with `as u4` after a `< 9` check) |
| `wiop_coins[u16](i:u10) -> (c0..c5:𝔽)` | one Ext per row | `all_coins` (index = flat coin index), then PCS challenges appended: `fold_alphas[j]` at `WIOP_TOTAL_COINS + j`, `deep_alpha` after them |
| `wiop_pos[u32](i:u8) -> (pos:u32)` | 229+ cells | query positions |
| `wiop_ext[u32](i:u16) -> (e0..e5:𝔽)` | general Ext scratch | entry claims (`backing`), `derived_witness`, `derived_quotient`, `final_buf`, `rounds_buf`/`aux_buf` (§5.8), per-module annihilator/eval coin |
| `wiop_dig[u32](i:u16) -> (d0..d7:𝔽)` | digests | resolved batch roots, distinct input roots, input/running frontiers, cap aux nodes |
| `wiop_u[u32](i:u16) -> (v:u64)` | integers | reconstructed layout arrays (`entry_size_log2`, `entry_batch`, `entry_is_ext`, `entry_row_idx`, `entry_col_decl_idx`, `col_to_entry`), bucket counters, aux presence flags, cached pointers (image sub-slices) |

A **region map** (constants `WIOP_R_*`) documents which index range each
logical array occupies, as `lib/bls12_381/fp12.zkc` does for its Fp12 slots.
zkc memories are sparse; untouched cells cost nothing.

### 4.3 The proof image in guest RAM (`lib/wiop/image.zkc`)

All offsets are from `verifier-ray/src/proof_abi.zig` and
`prover-ray/wiop/proofserialization/README.md` §6 (little-endian, absolute
pointers, `usize` = 8 bytes). `P` below is the value of rs1 (image base).

| Object | Size | Field offsets |
|---|---|---|
| `VerifyInput` @ `P` | 144 | `proof` @0, `public_inputs` slice @128 (`ptr` @128, `len` @136) |
| `Proof` | 128 | `rounds` slice @0, `module_sizes` slice @16, `pcs_opening.proof` @32 |
| `OpeningProof` | 96 | `input_queries` slice @+0, `input_caps` slice @+16, `fri_proof` @+32 |
| `fri.Proof` | 64 | `round_roots` @+0, `round_caps` @+16, `final_poly` @+32, `running_queries` @+48 |
| `RoundMessage` | 56 | `cells` slice @0, `commitment` payload @16 (32 B), presence tag `u8` @48 |
| `Scalar` | 28 | payload @0 (24 B), tag `u8` @24: `0` base (only limb 0 meaningful), `1` ext |
| `Ext` / `Digest` / `usize` | 24 / 32 / 8 | dense |
| `InputTreeOpening` | 32 | `siblings` slice @0 (Digest elems), `leaves` slice @16 (`?RowPair` elems) |
| `?RowPair` | 72 | `RowOpening[0]` @0, `RowOpening[1]` @32, tag `u8` @64 |
| `RowOpening` | 32 | `base` slice @0 (𝔽 elems), `ext` slice @16 (Ext elems) |
| `Branch` | 48 | `siblings` slice @0, `leaf` Digest @16 |
| `MerkleCap` | 32 | `nodes` slice @0 (Digest), `aux` slice @16 (`?Digest` elems, 36 B: payload @0, tag @32) |
| `InputCap` | 32 | `nodes` slice @0, `tables` slice @16 (`InputCapTable`, 24 B: `rows` slice @0, `size_log2` `u8` @16) |
| `[]const []const Branch` element | 16 | a slice header per query |

Accessors (all `<ram>`; each is a thin `#[inline]` over `read_64`/`read_32`):

```zkc
fn wiop_slice(p:Address) -> (ptr:Address, len:u64)         // read_64(p), read_64(p+8)
fn wiop_felt(p:Address) -> (v:𝔽)                           // read_32(p) as 𝔽
fn wiop_ext_at(p:Address) -> (e0..e5:𝔽)                     // 6 × read_32
fn wiop_digest_at(p:Address) -> (d0..d7:𝔽)                  // 8 × read_32
fn wiop_scalar_at(p:Address) -> (e0..e5:𝔽)                  // tag==0: (limb0,0,0,0,0,0); tag==1: 6 limbs; other tag → caller rejects (return a flag)
fn wiop_round_ptr(P:Address, r:u64) -> (q:Address)          // rounds.ptr + 56*r  (bounds-check r < rounds.len)
fn wiop_cell_ptr(P:Address, r:u64, i:u64) -> (q:Address)    // cells.ptr + 28*i   (bounds-check)
```

**Bounds.** The host validated the image shape (`Decode/Validate`), but the
zkc side still checks every `len` it relies on against the compiled system
(`rounds.len == WIOP_ROUND_COUNT`, `cells.len` per round vs proof cell counts,
query counts, branch lengths), exactly where the Zig code returns an error.
A failed check sets status 0 through a single `wiop_reject()` path (return
value `u1 = 0` propagated), never `fail`.

**Public-input merge** (`protocol/public_input.zig:69-110`): the proof's
`rounds[r].cells` omit public-input cells; the statement supplies them. Rather
than materialising merged rounds, implement `wiop_cell(P, round, index) -> Ext`
that consults the generated table `WIOP_PI_REFS` (sorted by `(round, index)`):
if `(round, index)` is a public-input slot, read `public_inputs[statement_index]`,
else read `cells[index − (#PI slots before index in this round)]`. Precompute
per round the PI slot list and its count so the subtraction is a table lookup
(`WIOP_PI_BEFORE(round, index)` is not tabulable for all indices; instead store
for each PI ref its `(round, index, statement_index)` and, for each round, the
count of PI slots; the cell lookup does a small recursive scan over that
round's PI refs, which are few). Reject when `public_inputs.len != WIOP_PI_COUNT`
or when the proof's `cells.len != round_cell_count − pi_count(round)`.

### 4.4 Transcript memory layout and the two absorb paths

`Transcript` (`crypto/fiat_shamir.zig`) + `MDHasher` (`crypto/poseidon2.zig:78-148`):

- `wiop_fs_reset()` zeroes cells 0..16.
- `wiop_fs_write(v:𝔽)`: `buf[len] = v; len += 1; if len == 8 { state = compress(state, buf); len = 0 }`.
  Written as one function with a `switch len` (constant indices into the
  memory) so it stays a single row; the compress call is a conditional arm.
- `wiop_fs_write_ext(e0..e5)` = six writes (matches `updateExt` limb order).
- `wiop_fs_sum() -> (d0..d7)`: if `len != 0`, compress with the buffer **left
  zero-padded** (`block[8−len..8] = buf[0..len]`) — implement with a `switch len`
  selecting one of 7 straight-line pad shapes; then return `state`.
- `wiop_fs_random_digest()` = `sum()` then `write(0)`.
- `wiop_fs_random_ext()` = digest limbs 0..5.
- `wiop_fs_random_ints(n:u32, mask:u32)`: recursive; each digest yields 8
  positions `limb as u32 & mask` (`upper_bound − 1`, power of two), written to
  `wiop_pos`. Note the Go/Zig both take `Bits()[0] % upperBound` on the
  **canonical** value; `𝔽 as u32` gives that value.

### 4.5 Generated system file (`system_0.zkc`) — table schema

Emitted by the Go emitter (§6) from `codegen.CompiledSystem`
(`verifier-ray/codegen/system.go`). Field names below mirror the Zig `System`
structs so a reader can diff them against `riscv_system.zig`.

```zkc
// protocol.Spec
const WIOP_ROUND_COUNT:u8            // spec.round_coin_counts.len - 1
const WIOP_TOTAL_COINS:u16
const WIOP_DYN_MODULE_COUNT:u8
const WIOP_COL_SIZE_MAX_LOG2:u5 = 22 // column_size_max_supported = 1<<22
static WIOP_ROUND_COINS(r:u8) -> (count:u16, offset:u16)   // index 1..ROUND_COUNT (index 0 = 0,0)
// public_input.Spec
const WIOP_PI_COUNT:u16
static WIOP_ROUND_CELLS(r:u8) -> (total:u32, pi_count:u32, pi_base:u16)
static WIOP_PI_REFS(i:u16) -> (round:u8, index:u32, statement_index:u16)  // sorted (round,index)
// pcs.System
const WIOP_LOG_CODEWORD:u5, WIOP_LOG_PLAINTEXT:u5, WIOP_LOG_FINAL_POLY:u5, WIOP_NUM_QUERIES:u16
const WIOP_NUM_BATCHES:u8, WIOP_NUM_COLUMNS:u16, WIOP_ZETA_COIN:u16
static WIOP_COLUMNS(c:u16) -> (batch:u8, is_ext:u1, is_dyn:u1, size_or_dyn_idx:u8, shift_base:u16, shift_count:u8)
static WIOP_SHIFTS(s:u16) -> (shift_mod_2_24:u32, claim_round:u8, claim_index:u32)
        // raw shift normalised later mod runtime N; store raw as two's complement u32, N ≤ 2^22 so mod works
static WIOP_WITNESS_MAP(k:u16) -> (col:u16, shift_slot:u8)
static WIOP_QUOTIENT_MAP(k:u16) -> (col:u16, shift_slot:u8)
static WIOP_BATCH_ROOTS(b:u8) -> (kind:u1, round:u8, dig_idx:u8)    // kind 0 = round commitment, 1 = precomputed
static WIOP_PRECOMPUTED_ROOTS(i:u8) -> (d0..d7:𝔽)
// vanishing.System
const WIOP_MODULE_COUNT:u16, WIOP_TOTAL_WITNESS_CLAIMS:u16, WIOP_TOTAL_QUOTIENT_CLAIMS:u16
static WIOP_MODULES(m:u16) -> (is_dyn:u1, size_log2_or_dyn_idx:u8, expr_base:u32, bucket_base:u16, bucket_count:u8, witness_claim_offset:u16, merge_coin:u16, eval_coin:u16)
static WIOP_BUCKETS(b:u16) -> (ratio:u8, van_base:u32, van_count:u16, quotient_claim_offset:u16)
static WIOP_VANISHINGS(v:u32) -> (expr:u32, cancel_base:u32, cancel_count:u8)
static WIOP_CANCELLED(c:u32) -> (position:u32)               // i32 stored two's complement
static WIOP_EXPR(n:u32) -> (kind:u3, op:u3, a:u32, b:u32)
        // kind: 0 column_claim(a) 1 cell(a=round,b=index) 2 coin(a) 3 constant(a) 4 op(op,a,b) 5 lagrange_selector(a=position i32)
        // node indices are GLOBAL (module expr_base already added by the emitter)
// logderivativesum / grandproduct / rowlimit / shared_randomness
static WIOP_LD_QUERIES(q:u16) -> (z_base:u16, z_count:u16, res_round:u8, res_index:u32, result_is_zero:u1)
static WIOP_GP_QUERIES(q:u16) -> (z_base:u16, z_count:u16, res_round:u8, res_index:u32, has_expected:u1, expected:u64)
static WIOP_REFS(i:u16) -> (round:u8, index:u32)              // shared by LD/GP z_final_refs
static WIOP_RL_CHECKS(k:u16) -> (inc_base:u16, inc_count:u16, incl_base:u16, incl_count:u16, limit:u64)
static WIOP_RL_MODULES(i:u16) -> (is_dyn:u1, value:u32)
const WIOP_SR_ENABLED:u1, WIOP_SR_ROUND:u8, WIOP_SR_HAS_COMMITMENT:u1
static WIOP_SR_REFS(i:u16) -> (round:u8, index:u32)           // 328 entries when enabled
```

Empty tables are a problem for a `static` (a zero-row static may not parse):
the emitter always emits at least one dummy row and a `_COUNT` const of 0.
Widths (`u16`/`u32`) are chosen from the real R5 system's counts once
`riscv_system.zig` has been regenerated (§9.2); the emitter asserts each value
fits.

## 5. Module-by-module port specification

Each subsection: Zig source → zkc file, signatures, algorithm notes, recursion
shape, test fixture, rough cost. "Row" means one trace row of the function's
module. `Ext` in a signature means six `𝔽` params/returns in §4.1 order.

### 5.1 `field.zkc` and `ext.zkc`

Covered by the table in §4.1. Additional functions:

```zkc
fn wiop_domain_point(log_size:u5, position:u32) -> (x:𝔽)
   // fri.zig:277-279: g = WIOP_ROOTS[log_size]; x = g^bitrev(position, log_size)
   // implement pow by square-and-multiply over the 24 bits of the reversed
   // position as a fixed 24-step straight-line chain (bits are u1 selects), one row.
fn wiop_shifted_point(size_log2:u5, shift_raw:u32, z0..z5) -> (Ext)
   // pcs.zig:1067-1074: N = 1<<size_log2; k = shift mod N (two's complement raw shift);
   // rotation = WIOP_ROOTS[size_log2]^k (same fixed chain); zeta.mulByBase(rotation)
fn wiop_point_in_domain(e0..e5, log_size:u5) -> (in:u1)
   // pcs.zig:1082-1087: isBase && e0^(2^log_size) == 1  (wiop_f_pow2k)
```

Tests: `verifier-ray/testdata/generated/vectors.zig` `field_cases`, `ext_cases`
(add/sub/mul/square/neg/mul_by_base/inverse), regenerated as `.accepts` JSON by
the emitter's `-vectors` mode (§8.1).

Cost target (Phase 0 probe, fill in): `wiop_ext_mul` ≤ 60 columns × 1 row;
`wiop_ext_inv` ≤ 250 columns × 1 row.

### 5.2 `poseidon2_compress.zkc`

Zig: `crypto/poseidon2.zig:39-62` (`compressInPlace`), `:164-188`
(`permutationNative`), `:227-328` (linear layers), constants
`crypto/poseidon2_constants.zig`. zkc constants already exist as
`lib/poseidon2/constants.zkc` (`round_keys`, `internal_diag`); include and
reuse them, do not duplicate.

```zkc
fn wiop_p2_perm(s0..s15:𝔽) -> (t0..t15:𝔽)          // straight-line, 1 row
fn wiop_p2_compress(l0..l7, r0..r7) -> (d0..d7)      // perm(l‖r); d_i = r_i + t_{8+i}
```

Write `wiop_p2_perm` **fully unrolled**: initial external layer; 3 full rounds
(add all 16 keys, cube all 16, external layer); 21 partial rounds (add key to
lane 0, cube lane 0, internal layer `Σ + diag_i·s_i`); 3 full rounds. Read
keys as `round_keys[const]` (static lookups with literal indices), or have the
emitter/hand-writer inline the literal constants. `M4` and the cross-chunk sums
follow `matMulExternalInPlace` exactly (`:227-262`).

Test: `vectors.zig` `poseidon_cases` (compress and MD cases) and the existing
`arithmetization/src/test/zkc/poseidon2/permutation.accepts`. Acceptance: all
vectors pass under `zkc execute -c`, and `zkc trace --stats` shows **1 row per
compress** in the `wiop_p2_perm` module.

Why not call the existing `permutation<poseidon2_state>()`: it is loop-based
over a memory and costs hundreds of rows per call; the verifier issues tens of
thousands of compressions per proof (§9.1).

### 5.3 `transcript.zkc`

§4.4 gives the design. Functions (all `<wiop_fs>`):

```zkc
fn wiop_fs_reset()
fn wiop_fs_write(v:𝔽)
fn wiop_fs_write_ext(e0..e5)
fn wiop_fs_write_digest(d0..d7)                 // updateElements(&commitment)
fn wiop_fs_sum() -> (d0..d7)                    // sumDigest with left zero-pad
fn wiop_fs_random_digest() -> (d0..d7)          // sum, then write(0)
fn wiop_fs_random_ext() -> (e0..e5)
fn wiop_fs_random_ints<wiop_pos>(n:u32, mask:u32, i:u32)   // recursive: one digest → up to 8 positions
```

Fast path from Zig `writeElements` (compress full blocks directly when the
buffer is empty) is an optimisation, not semantics; the byte-for-byte result is
identical. Implement `write_digest` as 8 writes first; optimise later if the
probe shows the per-write row cost matters.

Test: `vectors.zig` `fiat_shamir_cases` (`base_updates`, `ext_updates`,
`random_field`, `random_ext`) and `runtime_trace_cases` (rounds → expected
coins).

### 5.4 `image.zkc` and `public_input.zkc`

§4.3. Functions are `#[inline]` wrappers where trivial. The one non-trivial
piece is `wiop_cell(P, round, index) -> (ok:u1, e0..e5)`; the Zig error
`CellRefOutOfRange` becomes `ok = 0`.

Test: an `.accepts` fixture built from `verifier-ray/testdata/proof_image.bin`
(the small synthetic ABI image, 1176 B, encoded at base `0x400000000`) is
**not** usable directly because its base differs from `0x08800000`; instead the
emitter re-encodes the same `VerifyInput` at `GuestBase` (Go:
`proofserialization.Encode(input, GuestBase)`) into a harness fixture and
asserts a handful of known values (both Scalar tags, present/absent
commitment, a null and a present `?RowPair`, `Branch` field order).

### 5.5 `replay.zkc`

Zig: `protocol/root.zig:90-140` at `pr-3959`.

```zkc
fn wiop_replay<ram, wiop_fs, wiop_coins>(P:Address) -> (ok:u1)
   // rounds.len == WIOP_ROUND_COUNT else ok=0
   // module_sizes.len >= WIOP_DYN_MODULE_COUNT else ok=0; each size <= 1<<22 else ok=0
   // wiop_replay_round!(P, 1)  (recursive over round_index 1..=ROUND_COUNT)
fn wiop_replay_round<…>(P:Address, r:u8) -> (ok:u1)
   // for d in 0..DYN_MODULE_COUNT: write(module_sizes[d] as 𝔽)      (recursive helper)
   // msg = rounds[r-1]; if commitment tag == 1: write_digest
   // for each cell i in 0..ROUND_CELLS(r-1).total: write_ext or write(base) per wiop_cell tag  (recursive helper)
   // count, offset = WIOP_ROUND_COINS[r]; for k in 0..count: wiop_coins[offset+k] = random_ext()
   // recurse r+1 while r < ROUND_COUNT
```

Absorption of a **base** scalar is one element, of an **ext** scalar six
elements (`absorbScalar`); the tag drives it, exactly as in Zig — the Scalar
kind is *not* checked against the system (documented prover-side gap, README
§11.5); do the same, do not "fix" it here.

Test: `verify.zig` cases (69 synthetic systems, 4 queries) — the emitter dumps
each case's `spec` + image + expected coins; and `runtime_trace_cases`.

### 5.6 `merkle.zkc`

Zig: `crypto/merkle.zig`.

```zkc
fn wiop_hash_node(l0..l7, r0..r7, has_aux:u1, a0..a7) -> (d0..d7)
   // merkle.zig:101-105: compress(l,r); if has_aux: compress(node, aux)   — 1 row, 2 compress arms
fn wiop_cap_depth(num_queries:u32, height:u32) -> (depth:u32)
   // merkle.zig:24-27: 0 if nq<=1 or h<=1 else min(bitlen(nq-1), h-1); num_queries is a const → codegen precomputes capDepth for every height 0..23 into static WIOP_CAP_DEPTH(h)
fn wiop_branch_auth<ram, wiop_dig>(branch_p:Address, idx:u32, frontier_base:u16, frontier_len:u32) -> (ok:u1)
   // Branch.authenticateToCap :47-64. Recursive over siblings from deepest (i = len-1) to 0:
   //   ancestor = leaf; step: sibling = siblings[i]; (left,right) by idx&1; ancestor = hash_node; idx >>= 1
   //   end: ancestor == wiop_dig[frontier_base + idx]
   //   ok=0 when siblings.len == 0 or idx >> siblings.len >= frontier_len
fn wiop_cap_auth<ram, wiop_dig>(cap_p:Address, depth:u32, root_idx:u16) -> (ok:u1)
   // MerkleCap.validate + authenticate :69-95. recoverNode is naturally recursive:
   //   recover(node_depth, index): if node_depth == depth: nodes[index] else hash_node(recover(d+1, 2i), recover(d+1, 2i+1), aux[heap(d,i)])
   //   nodes.len == 1<<depth, aux.len == (1<<depth)-1, depth < 64
fn wiop_hash_row<ram, wiop_fs>(row_p:Address) -> (d0..d7)
   // hashRowOpening :148-153: reset; write(tag, base.len, ext.len) [absorbLeafHeader]; write all base; write all ext (6 limbs each); sum
fn wiop_hash_row_pair<ram, wiop_fs>(pair_p:Address, self_is_even:u1) -> (d0..d7)
   // hashRowPair :159-170: header from row[0] widths, then rows in even-before-odd order
fn wiop_input_branch_auth<ram, wiop_dig, wiop_fs>(opening_p:Address, idx:u32, frontier_base:u16, frontier_depth:u32) -> (ok:u1)
   // InputTreeOpening.authenticateToCap :186-204:
   //   height = leaves.len; reject height==0 or depth>=height or siblings.len != height-1-depth
   //   bottom = leaves[height-1] (tag must be 1); leaves[0..depth] must all be null
   //   step = foldOneLevel(hash_row(bottom[0]), hash_row(bottom[1]), null, idx)
   //   for i = height-1 down to depth+1: step = foldOneLevel(step.anc, siblings[i-depth], leaves[i], step.pos)
   //   frontier check
```

`hashRowOpening`/`hashRowPair` use their **own** fresh hasher, not the
transcript. Since `wiop_fs` is the only sponge memory, either (a) give row
hashing a second memory `wiop_fs2`, or (b) save/restore is impossible mid-replay
— but row hashing only happens in the PCS phase, *after* all transcript
squeezes are done (`deriveChallenges` is the last transcript use). So (b) is
safe: reuse `wiop_fs` after `deriveChallenges`; assert in code comments and in
the phase ordering of `verify.zkc`.

Row hashing dominates PCS cost (§9.1). Reading a row of `w` values costs `w`
RAM reads plus `w/8` compressions; nothing to shave there beyond aligned reads.

Tests: `vectors.zig` has no Merkle vectors; use the `pcs.zig` cases (4 PCS
cases include caps, aux tables, running branches) and the `fri.zig` fold cases
via §5.8's harness. Add a Go-emitted micro-fixture for `wiop_branch_auth`
built with prover-ray's `tree.go` (`newCompleteBinaryTree`) if debugging needs
isolation.

### 5.7 `pcs_layout.zkc` — reconstruct, entry claims, root routing

Zig: `query/pcs.zig:271-365` (`reconstruct`), `:414-438` (`buildEntryClaims`),
`:444-472` (`routeInputRoots`), `verifier.zig:284-299` (`resolveRoots`),
`:245-264` (`routeClaims`).

Layout arrays live in `wiop_u` (§4.2): `SZ[c]` (per-column size_log2),
`POS[c]` (per-column position within its `(batch, size, is_ext)` bucket),
`E_SIZE[e]`, `E_BATCH[e]`, `E_EXT[e]`, `E_ROW[e]`, `E_COL[e]`, `COL2E[c]`,
plus bucket counters `CNT[batch][size][ext]` (`NUM_BATCHES × 23 × 2` cells) and
bucket offsets `OFF[batch][size][ext]`.

```zkc
fn wiop_reconstruct<ram, wiop_u>(P:Address) -> (ok:u1, top_size:u5)
```

Do **not** port the Zig triple loop (`size desc × batch × ext × columns`,
`pcs.zig:336-360`), which is `O(23 × batches × columns)`. Same result in three
linear passes (counting sort), each a recursion over the column table:

1. **Pass 1** (`c = 0..NUM_COLUMNS`): `sz = static size` or
   `log2(module_sizes[dyn_idx])` (reject non-power-of-two, missing index, or
   `sz > 22`); `SZ[c] = sz`; `POS[c] = CNT[b][sz][ext]`; `CNT[b][sz][ext] += 1`;
   `top_size = max`.
2. **Pass 2** over buckets in canonical order (size **descending** 22..0, batch
   ascending, base before ext): running `entry = 0`; `OFF[b][sz][ext] = entry;
   entry += CNT[...]`. At the end `entry == NUM_COLUMNS` else reject
   (`LayoutOverflow`).
3. **Pass 3** (`c = 0..NUM_COLUMNS`): `e = OFF[b][SZ[c]][ext] + POS[c]`;
   `E_SIZE[e] = SZ[c]; E_BATCH[e] = b; E_EXT[e] = ext; E_ROW[e] = POS[c]; E_COL[e] = c; COL2E[c] = e`.

This reproduces `canonicalLayout` byte-for-byte because within a bucket the
Zig enumeration order is declaration order, which is exactly `POS[c]`.
Restricted params: `log_codeword = WIOP_LOG_CODEWORD − (WIOP_LOG_PLAINTEXT − top_size)`,
`log_plaintext = top_size`, `num_rounds = top_size − WIOP_LOG_FINAL_POLY`
(`fri.zig:74-85`); reject if `top_size < WIOP_LOG_FINAL_POLY`.

```zkc
fn wiop_build_entry_claims<ram, wiop_u, wiop_ext>(P:Address) -> (ok:u1)
   // for e in 0..NUM_COLUMNS: c = E_COL[e]; for k in 0..shift_count(c):
   //   (round, index) = WIOP_SHIFTS[shift_base(c)+k]; wiop_ext[CLAIMS + slot] = wiop_cell(P, round, index)
   //   CLAIM_BASE[e] = first slot of entry e (store in wiop_u)
fn wiop_resolve_roots<ram, wiop_dig>(P:Address) -> (ok:u1)
   // verifier.zig:284-299: for b: kind 0 → rounds[round].commitment (tag must be 1), kind 1 → WIOP_PRECOMPUTED_ROOTS
fn wiop_route_input_roots<wiop_dig, wiop_u>() -> (ok:u1, distinct:u8)
   // pcs.zig:444-472: walk entries; first time a batch appears, dedupe its root by VALUE against
   // the distinct list (8-felt equality), INDEX_BY_BATCH[b] = branch index
fn wiop_route_claims<wiop_u, wiop_ext>(kind:u1) -> (ok:u1)
   // verifier.zig:245-264 for WITNESS_MAP (kind 0) and QUOTIENT_MAP (kind 1):
   //   out[k] = claims[CLAIM_BASE[COL2E[col]] + shift_slot]; shift_slot < shift_count(col) else reject
```

Test: `pcs.zig` cases exercise `reconstruct` on 1–2 batches, static sizes 0..3;
`verify.zig` cases include dynamic modules (`DynamicFibonacci`, `LeftPadDynamic`,
`Dynamic*`). Acceptance: `E_*` arrays equal the Zig `Reconstructed` for every
case (the emitter can dump the Zig-side expected arrays by re-implementing
`reconstruct` in Go — prover-ray's `GetLayout`/`canonicalLayout` are the
source of truth; use them directly).

### 5.8 `pcs_verify.zkc` — challenges, caps, per-query DEEP + FRI folds

Zig: `query/pcs.zig:499-533` (`deriveChallenges`), `:540-703` (cap info,
cap authentication, query source), `:709-901` (`verify`), `:909-1074`
(seed pair, widths, `bindInputTreeOpenings`, `reconstructQueryValueAt`,
`entryDeepTerm`, `shiftedPoint`), `query/fri.zig` (all of it).

#### 5.8.1 Challenges

```zkc
fn wiop_derive_challenges<ram, wiop_fs, wiop_coins, wiop_pos>(P:Address, num_rounds:u5, log_codeword:u5) -> (ok:u1)
   // want_round_roots = num_rounds>0 ? num_rounds-1 : 0; round_roots.len == want else ok=0
   // for i in 0..want: fold_alpha[i] = random_ext(); write_digest(round_roots[i])        (recursive)
   // final = random_ext(); deep_alpha = final; if num_rounds>0: fold_alpha[num_rounds-1] = final
   // for each final_poly coefficient: write_ext                                          (recursive)
   // random_ints(NUM_QUERIES, (1<<log_codeword) - 1)
```

Store `fold_alpha[j]` at `wiop_coins[WIOP_TOTAL_COINS + j]`, `deep_alpha` at
`wiop_coins[WIOP_TOTAL_COINS + 23]`.

#### 5.8.2 Shape checks (`fri.zig:140-164`, `pcs.zig:716-744`)

One function `wiop_pcs_shape(P, …) -> ok` doing, in this order: entry claim
count (implicit: built from tables), `zeta != 0` when any column has more than
one shift (**codegen const** `WIOP_HAS_MULTI_SHIFT`), `zeta` not in any bundle
domain (`wiop_point_in_domain` per distinct size present — loop over sizes 0..top_size
and skip sizes with zero columns, or per entry as Zig does), `input_queries.len == NUM_QUERIES`,
`input_caps.len == distinct`, `round_caps.len == want_round_roots`,
`running_queries.len == NUM_QUERIES`, `final_poly.len == 1 << LOG_FINAL_POLY`,
every query position `< codeword_size` (guaranteed by the mask; keep the check),
each cap `validate(capDepth)` and each `running_queries[q].len == want_round_roots`.

#### 5.8.3 Input caps (`pcs.zig:547-643`, `:752-769`)

Per distinct tree `t` (recursive over `t`):

- `buildInputCapInfo`: `rate_log = log_codeword − log_plaintext`; `bottom` =
  max `E_SIZE` over entries of that tree; `height = rate_log + bottom`
  (reject `0` or `> log_codeword`); `depth = WIOP_CAP_DEPTH[height]`;
  `query_rows[height−1] = 1`; for every entry of the tree with
  `aux_depth = inputAuxDepth(rate_log, size, bottom)` (`:540-545`): if
  `aux_depth < depth` record `size` in `REVEALED[t]` (deduped, then sorted
  ascending — sizes are ≤ 23 so a 23-cell presence bitmap replaces the sort),
  else `query_rows[aux_depth] = 1`; `cap_table_by_depth[aux_depth] = table index`
  in ascending-size order.
- `authenticateInputCap` (`:611-643`): `depth == 0` → cap must be empty, the
  frontier is the root itself (copy into `wiop_dig`); else `nodes.len == 1<<depth`,
  `tables.len == revealed_count`, each table `size_log2` matches, `rows.len == 1 << (rate_log+size)`
  and even, each row's widths equal `inputSizeWidths(batch, size)` (count of
  base/ext entries with that batch and size — precompute `WIDTHS[b][size]` in
  Pass 1 of §5.7), `aux[level_start + row/2] = hash_row_pair(rows[row], rows[row+1], even)`
  for even rows, then `wiop_cap_auth` over `nodes` with those aux digests
  (the Zig builds a `MerkleCap{nodes, aux}` in memory; here the aux digests
  are written to `wiop_dig` and `wiop_cap_auth` takes an aux base index).
  Frontier for tree `t` = the cap's `nodes` (a RAM pointer + `1<<depth`); keep
  frontiers as **pointers into the image** (`FRONTIER_PTR[t]`, `FRONTIER_LEN[t]`)
  and let `wiop_branch_auth` read frontier digests via RAM, avoiding a copy.
  (Adjust `wiop_branch_auth`'s frontier parameter accordingly: pointer, not
  `wiop_dig` index; the depth-0 case points at the resolved root copy in
  `wiop_dig`, so support both via a `kind` flag or copy every frontier into
  `wiop_dig` — copying costs `Σ 2^depth` digests, at most `2^8 × trees`; copy.)

#### 5.8.4 Running-layer caps (`pcs.zig:771-786`)

For `j = 1..num_rounds`: `depth = WIOP_CAP_DEPTH[log_codeword − j]`; if `0`,
frontier = `round_roots[j−1]`; else `wiop_cap_auth(round_caps[j−1], depth, root = round_roots[j−1])`
and frontier = its `nodes`. Store `RUN_FRONTIER[j]` (base index into `wiop_dig`
after copying, plus length).

#### 5.8.5 Per query (`pcs.zig:796-898`) — the hot loop

Recursive over `q = 0..NUM_QUERIES`; each query is one call to
`wiop_pcs_query(P, q, …) -> ok`, itself decomposed into:

1. `source.authenticate()` (`:658-678`): for each tree `t`:
   `leaves.len == height[t]`, `siblings.len == height − 1 − depth`,
   bottom leaf present, leaves below `depth` null, presence pattern equals
   `query_rows[t][level]`, `num_leaves = 1 << height` divides `codeword_size`,
   `leaf_index = pos / (codeword_size / num_leaves)` (= `pos >> (log_codeword − height)`),
   `wiop_input_branch_auth(opening[t], leaf_index, frontier[t])`.
2. `resolveRunningLayers` (`fri.zig:174-201`): for `j = 1..num_rounds`:
   `height = log_codeword − j`, `depth = CAP_DEPTH[height]`,
   `siblings.len == height − depth`, `wiop_branch_auth(branch, pos >> j, RUN_FRONTIER[j])`,
   `ROUNDS[q][j] = (octupletToExt(leaf), octupletToExt(siblings[last]))` —
   `octupletToExt` rejects if limbs 6, 7 are non-zero (`fri.zig:302-309`).
   Store in `wiop_ext` at `RB + 2·j`/`RB + 2·j + 1` (only the current query's
   buffers are needed: process one query completely before the next, so
   `rounds_buf`/`aux_buf` are **per-query scratch, not per-query arrays**).
3. `final = evaluateExtAtExt(final_poly, domainPointExt(log_codeword − num_rounds, pos >> num_rounds))`
   (`polynomial/canonical.zig:34-42`, Horner from the top coefficient; recursive over coefficients).
4. **Bundle walk** (`:827-883`): entries in canonical order are grouped by
   equal `E_SIZE`; for each run `[e0, e1)` with size `s`:
   `round = top_size − s`, `domain_log = log_codeword − round`,
   `level_size = 1 << domain_log`;
   - `bindInputTreeOpenings` (`:939-965`): for each distinct batch in the run,
     `pair = pairAtLevel(batch, level_size)` (§5.8.6) and both rows' widths must
     equal `bundleBatchWidths` (= `WIDTHS[b][s]`).
   - `alpha_deep = round < num_rounds ? fold_alpha[round]² : (num_rounds > 0 ? fold_alpha[num_rounds−1] : deep_alpha)`.
   - `level_pos = pos >> round`; `seed = (round == 0 || round == num_rounds) ? 0 : ROUNDS[round]`.
   - `self = reconstructQueryValueAt(…, x = domainPointExt(domain_log, level_pos), sibling=0, seed.self)`,
     `sib = …(x' = domainPointExt(domain_log, level_pos ^ 1), sibling=1, seed.sibling)`;
     `AUX[round] = (self, sib)`, `AUX_SET[round] = 1`.
5. `num_rounds == 0` special case (`:885-891`): `AUX[0]` must be set; check
   `self == final` and `sib == evaluateExtAtExt(final_poly, domainPointExt(log_codeword, pos ^ 1))`.
6. `checkFolds` for this query (`fri.zig:213-262`), see §5.8.7.

#### 5.8.6 `pairAtLevel` and the DEEP term (`pcs.zig:680-701`, `:971-1058`)

```zkc
fn wiop_pair_at_level<ram, wiop_u>(P, q_opening_p:Address, batch:u8, level_log:u5, pos:u32) -> (ok:u1, row_self:Address, row_sib:Address)
   // branch = INDEX_BY_BATCH[batch]; info = tree branch
   // if level_log > 0 and cap_table_by_depth[level_log-1] is set:
   //     table = caps[branch].tables[idx]; num_leaves = 1<<height; base = leaf_index / (num_leaves / level_size)
   //     rows[base], rows[base ^ 1]          (RowOpening pointers, 32 B stride)
   // else: InputTreeOpening.pairAtLevel: levelIndex(level_size) (:209-222): bottom level → leaves.len-1,
   //     else ctz(level_size)-1; leaf tag must be 1; return RowOpening[0]/[1] pointers
```

Returning **pointers** (not values) keeps the call cheap; the caller reads
`row.base[row_idx]` or `row.ext[row_idx]` for the entries it needs.

```zkc
fn wiop_deep_horner<…>(P, q…, e:u16, e0:u16, s:u5, level_log:u5, pos:u32, sibling:u1, acc:Ext, alpha:Ext, zeta:Ext, x:Ext) -> (Ext)
   // reconstructQueryValueAt: walks i = e1-1 down to e0:  acc = acc*alpha + term(i)
   // recursion on i; term(i):
   //   pair = pair_at_level(E_BATCH[i], …); row = sibling ? sib : self
   //   v = E_EXT[i] ? row.ext[E_ROW[i]] : lift(row.base[E_ROW[i]])
   //   term = Σ_{k over shifts of E_COL[i], deduped by shifted point} (v - claim_k) * inv(x - point_k)
```

**Inverse caching (important for cost).** `point_k = zeta·ω_N^{shift}` and
`x` depend only on `(size s, shift, query, conjugate)`, not on the column, so
`inv(x − point_k)` is shared by every column of the bundle with the same raw
shift. Distinct raw shifts per system are few (`WIOP_SHIFT_VALUES`, emitted
deduplicated). Per `(q, bundle, conjugate)`: compute `inv_k` for each distinct
shift value once (recursion over `WIOP_SHIFT_VALUES`), store in
`wiop_ext[INV + k]`, and have `term(i)` look them up by the shift's value index
(`WIOP_SHIFTS` gets a `value_idx` column). Also precompute per bundle the
**alias map** for `entryDeepTerm`'s dedupe (`:1041-1051`): two raw shifts alias
iff `shift_a ≡ shift_b (mod N)`; codegen cannot know `N` for dynamic modules, so
compute the alias flag at runtime once per `(bundle, value_idx pair)` — or
simply, per entry, compare `shift_k mod N` against earlier `shift_j mod N` for
`j < k` (shift counts per column are tiny, ≤ 3 in practice). Aliased repeats
require `claim_k == claim_j` (`InconsistentAliasedClaim`) and are skipped.
`x − point == 0` → reject (`ClaimPointOnQueryPoint`).

#### 5.8.7 Folds (`fri.zig:213-262`)

```zkc
fn wiop_check_folds<wiop_ext, wiop_coins>(q_pos:u32, num_rounds:u5, log_codeword:u5, final:Ext) -> (ok:u1)
   // x_inv = wiop_f_inv(domain_point(log_codeword, pos))
   // recursive over j = 0..num_rounds:
   //   pair = AUX_SET[j] ? AUX[j] : ROUNDS[j]
   //   sum = pair.self + pair.sib; diff = (pair.self - pair.sib)·x_inv (mul_base) · fold_alpha[j]
   //   sum = (sum + diff)·inv_two
   //   j < num_rounds-1: sum == ROUNDS[j+1].self else FoldMismatch ; j == num_rounds-1: sum == final else FinalPolyMismatch
   //   x_inv = x_inv²
   // boundary: if num_rounds>0 and AUX_SET[num_rounds]: AUX.self == AUX.sib else BoundaryAuxNotConstant
```

`ROUNDS[0]` is never read (`aux[0]` is always set: round 0 introduces the top
level). Zero `AUX_SET[0..num_rounds]` and `ROUNDS[0..num_rounds]` at the start
of every query (Zig `:797-798`) — a recursion of `num_rounds+1` memory writes.

#### 5.8.8 Tests for §5.7–5.8

- `verifier-ray/testdata/generated/pcs.zig` (4 cases: `normal_flow`,
  `d1_top_level`, `boundary_round`, `boundary_round_corrupted_claim` expecting
  `BoundaryAuxNotConstant`). The emitter converts each case's `system` into a
  tiny `system_pcs_N.zkc` and its proof + roots + claims + coins into a RAM
  image (§8.2) — since these cases bypass the transcript, the harness entry
  point is `wiop_pcs_verify_with_challenges(...)`, taking `zeta`, `fold_alphas`,
  `query_positions` from the fixture (mirrors `pcs.verify`'s `VerifyInput`).
- `fri.zig` fold cases (`verifier-ray/test/fri_fold_cases.zig`) for
  `wiop_check_folds` in isolation.
- `verify.zig` (69 cases, 4 queries, real FS) through the full `wiop_verify`.
- The opt-in large fixture (`make -C verifier-ray generate-large-pcs-benchmark`,
  2^19 rows) for cost measurement only.

### 5.9 `vanishing.zkc`

Zig: `query/vanishing.zig` (all). Data: `WIOP_MODULES`, `WIOP_BUCKETS`,
`WIOP_VANISHINGS`, `WIOP_CANCELLED`, `WIOP_EXPR`; claims in
`wiop_ext[WITNESS + k]` / `wiop_ext[QUOTIENT + k]` (from `wiop_route_claims`).

```zkc
fn wiop_vanishing<…>(P:Address) -> (ok:u1)                     // recursive over modules m
fn wiop_van_module(P, m:u16) -> (ok:u1)
   // merge = coins[merge_coin], r = coins[eval_coin]
   // n_log2 = static size or log2(module_sizes[dyn_idx]) (power of two, else reject)
   // annihilator = ext_pow2k(r, n_log2) - 1 ; store r, annihilator, n_log2, witness_claim_offset in wiop_ext/wiop_u "module ctx" cells
   // recursive over buckets
fn wiop_van_bucket(P, m, b:u16) -> (ok:u1)
   // r_pow_n = annihilator + 1
   // quotient = Σ_{i<ratio} r_pow_n^i · quotient_claims[offset + i]        (recursive Horner over i, or fixed since ratio is tiny)
   // aggregate = Σ_v merge^v · eval(expr_v) · cancellation(cancel_v)         (recursive over vanishings)
   // ok = aggregate == annihilator · quotient
fn wiop_eval_expr(P, m, node:u32) -> (Ext)
   // switch kind:
   //   0 column_claim  → wiop_ext[WITNESS + witness_claim_offset(m) + a]
   //   1 cell          → wiop_cell(P, a, b)  (reject flag must propagate: return ok too, see below)
   //   2 coin          → wiop_coins[a]
   //   3 constant      → lift(WIOP_CONSTS[a])      // a separate static of 𝔽 constants
   //   4 op            → x = eval(a); binary ops also y = eval(b); switch op {add, mul, sub, div, double, square, negate, inverse}
   //   5 lagrange_sel  → wiop_lagrange(m, a)
fn wiop_lagrange(m, position_i32:u32) -> (ok:u1, Ext)
   // vanishing.zig:303-335: n = 1<<n_log2; validPosition (|pos| <= n for negative, pos < n for positive) else reject
   // k = normalize(pos, n); omega_pos = WIOP_ROOTS[n_log2]^k (fixed 24-step chain)
   // num = annihilator · omega_pos (mul_base); den = (r - lift(omega_pos)); den == 0 → reject (LagrangeSelectorInDomain)
   // den = den · (n as 𝔽); result = num · inv(den)
fn wiop_cancellation(m, cancel_base:u32, count:u8) -> (ok:u1, Ext)
   // vanishing.zig:337-369: Π_k (r - lift(omega^{norm(pos_k)})), same validPosition check; count==0 → 1
```

`wiop_eval_expr` returns `(ok:u1, e0..e5)`; the `cell` leaf can fail (out of
range) and Zig propagates `CellRefOutOfRange`. **Gotcha §3.4**: branching on
two returned values from one call inside a *loop* panicked the compiler;
this evaluator is recursive (no loop) and only ever branches on `ok`, so it
should be fine — but if the panic appears, fold `ok` into the value (return a
7th felt) and check it once at the bucket level.

`div` and `inverse` nodes use `wiop_ext_inv`; Zig's `1/0 = 0` convention is
irrelevant here because prover-ray never emits these at zero on honest input
and a malicious zero simply yields a wrong aggregate.

Sharing: identical `(m, node)` calls are deduplicated by the lookup argument,
so a DAG node referenced by several constraints costs one row.

Sizes: the real R5 modules have "many thousands of vanishing constraints"
(`vanishing.zig:165-169`) → `WIOP_EXPR` may hold ~10^5 rows; `u32` indices.

Test: `verifier-ray/testdata/generated/vanishing.zig` (590 KB: systems with
matching honest and invalid proof views) — the emitter turns each scenario into
`system_van_N.zkc` + an image of `rounds` + coins + claims, and the harness
entry is `wiop_vanishing_with_claims(...)`. Every invalid view must yield
`ok = 0`; every honest view `ok = 1`.

### 5.10 `scalar_checks.zkc`

- **logderivativesum** (`query/logderivativesum.zig:38-60`): per query,
  `Σ z_final_refs (as Ext) == result` else reject; if `result_is_zero` and
  `result != 0` reject. Recursion over queries and over refs.
- **grandproduct** (`query/grandproduct.zig:47-71`): `Π z_final == result`;
  if `has_expected`, `result == lift(expected as 𝔽)`.
- **rowlimit** (`query/rowlimit.zig:36-65`): `Σ rows(included) < limit` and
  `Σ rows(includings) < limit` (strict: `>= limit` rejects); dynamic sizes from
  `module_sizes`, missing index rejects. `u64` sums.
- **shared_randomness** (`query/shared_randomness.zig` at `pr-3959`): if
  `WIOP_SR_ENABLED == 0` skip. `commitment = WIOP_SR_HAS_COMMITMENT ? rounds[WIOP_SR_ROUND].commitment (tag must be 1) : zero digest`;
  `contribution = multiset_hash(commitment)` (`crypto/multiset_hashing.zig:21-33`):
  fresh hasher, write the 8 limbs, then 41 times `chunk_i = sum(); write 8 zeros (except after the last)`;
  compare `contribution[i]` with the 328 ref cells, which must be **base**
  scalars (`ext` tag rejects, `ContributionNotBaseField`).

Tests: the `verify.zig` cases with logderiv/grandproduct/rowlimit (cases 49–89
in `bench/verifier-profile.csv` naming) and the real R5 system for shared
randomness (synthetic cases don't enable it; the riscv system does via
`messagebus.CompileOptions{SharedRandomness: true}`).

### 5.11 `verify.zkc` — top level

Zig: `verifier.zig:124-238`. Order is load-bearing (transcript continuity,
then row hashing reuses `wiop_fs`):

```zkc
fn wiop_verify<ram, wiop_fs, wiop_coins, wiop_pos, wiop_ext, wiop_dig, wiop_u>(P:Address, system_id:u64) -> (status:u1) {
    status = 0
    if system_id != 0 { return }
    // 0. shape: rounds.len, public_inputs.len == WIOP_PI_COUNT, per-round cells.len   (public_input.zkc)
    // 1. wiop_fs_reset(); replay → wiop_coins                                          (replay.zkc)
    // 2. resolve_roots → wiop_dig                                                       (pcs_layout.zkc)
    // 3. reconstruct → layout, top_size, restricted params
    // 4. build_entry_claims
    // 5. derive_challenges (continues the SAME transcript; last transcript use)
    // 6. pcs shape checks; input caps; running caps; per-query recursion; folds        (pcs_verify.zkc)
    // 7. route_claims(witness), route_claims(quotient)
    // 8. vanishing                                                                       (vanishing.zkc)
    // 9. logderivativesum, grandproduct, rowlimit, shared_randomness                    (scalar_checks.zkc)
    status = 1
}
```

Each step returns `ok:u1`; on `0` the function returns with `status = 0`.
Because zkc has no early-exit across calls, write it as a chain
`if wiop_replay(P) != 1 { return }` … one `if` per step (all in one function,
no loop, so a single row plus the callees).

## 6. Codegen: `CompiledSystem` → `system_0.zkc` (Go)

The Zig emitter already extracts everything from the compiled `wiop.System`
(`verifier-ray/codegen/{coin_routing,public_input,vanishing,logderivativesum,grandproduct,rowlimit,shared_randomness,pcs}.go`
→ `codegen.CompiledSystem`, `system.go:14-27`). **Reuse those Go structs
unchanged**; add a second renderer next to the `*_zig.go` files.

- New package `verifier-ray/codegen/zkcgen/` (or files `*_zkc.go` in
  `codegen`): `WriteSystemZkc(w io.Writer, sys codegen.CompiledSystem, opts)`
  emitting the §4.5 tables with `text/template`, plus:
  - `WIOP_ROOTS` (25 roots of unity, computed with prover-ray's
    `field.RootOfUnity`), `WIOP_CAP_DEPTH(h)` for `h = 0..23` at
    `NumQueries`, `WIOP_LEAF_TAG_REDUCED`, `WIOP_INV_TWO`, `WIOP_CONSTS`
    (deduplicated `constant` leaves), `WIOP_SHIFT_VALUES` (deduplicated raw
    shifts) and the `value_idx` column of `WIOP_SHIFTS`, `WIOP_HAS_MULTI_SHIFT`.
  - Global expression indexing: the Zig `Module.expressions` are per module
    with local indices; the emitter offsets each module's operand indices by
    `expr_base` so `WIOP_EXPR` is one flat table.
  - Width selection: compute every table's max value and pick `u8/u16/u32`;
    keep the **same** widths across all generated systems used by tests so the
    engine's signatures do not change (fix them at the R5 system's needs).
  - Empty-table rule (§4.5): one dummy row + `_COUNT = 0`.
- Hook into `verifier-ray/codegen/generate-riscv-system/main.go`: after
  writing `riscv_system.zig`, also write
  `arithmetization/src/main/lib/wiop/system_0.zkc`. Add it to
  `make generate-testdata` and to `verify-testdata` (drift check), mirroring
  `riscv_system.zig`. Note both `riscv_system.zig` and `riscv_proof_image.bin`
  are **gitignored** today (`verifier-ray/.gitignore`); `system_0.zkc` must be
  **committed** because `riscv/main.zkc` includes it and CI compiles `main.zkc`.
  Decide with the team whether to commit the proof image too (needed for the
  e2e harness in CI) or keep the e2e harness opt-in.
- Fixture emitters for tests (§8): `-vectors` (field/ext/poseidon/FS vectors
  as `.accepts` JSON), `-pcs-cases`, `-vanishing-cases`, `-verify-cases`
  (`.zkc` system stub + image `.hex` per case). Reuse
  `verifier-ray/testdata/generate/{main.go,pcs_emit.go}` builders and
  `proofserialization.Encode(input, GuestBase)`; do not write a second
  projection (README §3's warning).
- Go tests: `zkcgen` renders each `wioptest` scenario, then compiles the
  result together with the engine via the in-process API
  (`compiler.Compile` → `ast.Compile` → `constraints.NewBinaryFile`, see
  `lineth_overview.md` §5) and asserts zero compile errors. This catches width
  overflows and table-shape mistakes without running zkc CLI.

---

## 7. Accelerator wiring (mirror of PR #3927, adapted)

Arithmetization side (five touch points from project memory, verified against
`pr-3927`'s diff):

1. `arithmetization/src/main/common/constants.zkc`
   - `const RTYPE_WIOP_VERIFY_WB:ComputeOp = 56` (on `main`; `57` if #3927 has
     landed first) and **shift** `STYPE_*`, `BRANCH_*`, `JTYPE_*`, `UTYPE_*` by
     one, updating the header comment ranges.
   - `FUNCT3_WIOP_VERIFY:Funct3 = 0b011`, `FUNCT7_WIOP_VERIFY:Funct7 = 0b0000000`,
     `R_WIOP_VERIFY_INST:u17 = OPCODE_CUSTOM1::FUNCT3::FUNCT7`,
     `R_WIOP_VERIFY_FOLD:u18` / `R_WIOP_VERIFY:u18`.
2. `arithmetization/src/main/riscv/interpreter.zkc`
   - `include "../lib/wiop/impl.zkc"`.
   - Add the six new memories to `interpreter`, `interpreter_a/b/c` and
     `main<…>` effect lists (`riscv/main.zkc:23`).
   - `case RTYPE_WIOP_VERIFY_WB:` in **`interpreter_c`** (cold path):
     `result = wiop_verify(v1 as Address, v2) as DoubleWord`.
3. `arithmetization/src/main/predecoding/check/check_r_type.zkc`: two cases in
   `check_r_type_with_fold` — `R_WIOP_VERIFY_FOLD` must be `NO_OP`,
   `R_WIOP_VERIFY` must be `RTYPE_WIOP_VERIFY_WB` (writeback op; **not** in the
   top-level pointer-rd switch).
4. `arithmetization/gopkg/predecoding/decode.go`: `rtypeOpWiopVerify = 31`
   (or 32), the `opcodeCUSTOM1` arm of `decodeRTypeSemantic` gains
   `funct3 == 0b011 && funct7 == 0 → rtypeOpWiopVerify`; it must **not** be
   listed in `shouldUseNoOp`'s pointer-rd exemption; bump
   `computeSTypeBase/BTypeBase/JTypeBase/UTypeBase`. Run
   `TestComputeOpBasesMatchConstantsZkc` (it regex-parses `constants.zkc`).
5. `arithmetization/gopkg/predecoding/doc.go` and
   `arithmetization/src/main/lib/README.md`: document the new row
   (`wiop_verify | 🟢 | custom-1 | 0b011 | 0b0000000`).

Guest side (`riscv-guests/`):

6. `lineth-accelerators/src/wiop.zig`:
   ```zig
   pub fn zkvm_wiop_verify(input: *const anyopaque, system_id: usize) callconv(.c) bool {
       var status: usize = undefined;
       asm volatile (
           \\.insn r 0x2b, 0b011, 0b0000000, %[st], %[in], %[sys]
           : [st] "=r" (status), : [in] "r" (@intFromPtr(input)), [sys] "r" (system_id) : .{ .memory = true });
       return status == 1;
   }
   ```
   re-export in `lineth-accelerators/src/root.zig`; declare in
   `include/lineth_accelerators.h`.
7. A minimal guest `riscv-guests/wiop-verify-smoke/` (or a new entry point in
   `verifier-ray/src/main.zig` behind `-Dwiop-accel=true`): load
   `_in_start`, call `zkvm_wiop_verify`, exit 0/1. The verifier-ray Makefile
   variant `zkc-verify-accel` runs it against `testdata/riscv_proof_image.bin`.
   The rollup guest (`riscv-guests/rollup/`, currently a stub with "no proof
   verification") is the eventual consumer; wiring it is out of scope here.

Verification that the instruction landed: scan the ELF for 4-byte-aligned
words with `opcode == 0x2b && funct3 == 3 && funct7 == 0` (memory note:
unaligned matches are false positives).

---

## 8. Test plan

### 8.1 Unit harnesses (`arithmetization/src/test/zkc/wiop/`)

Template: `arithmetization/src/test/zkc/bls12_381/` (a `.zkc` driver with
`pub input` arrays, `.accepts`/`.rejects` with one JSON object per line and
`;;` comments, a `Makefile` splitting lines into temp JSON and running
`go tool zkc execute -c --field KOALABEAR_16`, **grepping for `FAILURES`**).
Harness files and their fixture sources:

| Harness | Exercises | Fixture source (Go emitter mode) |
|---|---|---|
| `field.{zkc,accepts,rejects}` | `wiop_f_inv`, `wiop_f_pow2k`, roots table, bitrev, domain point | `vectors.zig` `field_cases` + hand values (`-vectors`) |
| `ext.*` | all Ext ops | `vectors.zig` `ext_cases` |
| `poseidon2_compress.*` | perm + compress | `vectors.zig` `poseidon_cases` (compress, MD) |
| `transcript.*` | write/sum/random_ext/random_ints | `vectors.zig` `fiat_shamir_cases`, `runtime_trace_cases` |
| `image.*` | accessors, cell lookup with PI merge | re-encoded `proof_image.bin` at `GuestBase` |
| `merkle.*` | branch auth, cap auth, row hashing | derived from `pcs.zig` cases |
| `pcs.*` | `wiop_pcs_verify_with_challenges` | `pcs.zig` 4 cases (incl. one expected reject) |
| `folds.*` | `wiop_check_folds` | `fri.zig` / `fri_fold_cases.zig` |
| `vanishing.*` | `wiop_vanishing_with_claims` | `vanishing.zig` honest + invalid views |
| `verify.*` | full `wiop_verify` on synthetic systems | `verify.zig` 69 cases (4 queries) |

Harnesses that touch `ram` must include `riscv/memory.zkc` and supply the five
R5 inputs (§3.4); the image bytes go into `blobs_data` at `blob_offset = 0x08800000`
exactly like production, and `main` of the harness copies them with `write_8`
(or, cheaper for tests, a word-wise `write_32` loop — semantics identical).

Each generated-system harness includes a **per-case** `system_*.zkc`; because
`static`/`const` names are global, the emitter prefixes them per case or the
Makefile compiles one case at a time (preferred: one `.zkc` per case,
`HARNESSES=…` chunking as in the bls12 Makefile).

### 8.2 End-to-end

1. `make -C verifier-ray generate-testdata` → `riscv_system.zig`,
   `riscv_proof_image.bin`, and (new) `system_0.zkc`.
2. `make -C arithmetization riscv-check-compilation` (compile gate, dead-code
   gate) and `riscv-check-lint` (`zkc format --check`).
3. Smoke guest (§7.7) through `elf_to_json` with `IN_BYTES=@riscv_proof_image.bin`
   and `zkc exec --fast -vvv` → exit 0, `clock cycle` count recorded.
4. Negative: corrupt one byte of a cell / a root / a query branch (Go helper
   flips a byte in the image and re-encodes); guest must exit 1 with
   status 0 and **no** `fail`.
5. `zkc trace --stats` (or the Go `Trace` + `Check` path with sharding, as in
   `prover-ray/zkcdriver/r5_benchmark_test.go`) on the accepting run: zero
   constraint failures; record cells (§9).

### 8.3 CI

- Add `arithmetization/src/test/zkc/wiop` to whatever target runs the bls12
  harnesses (they are Makefile-driven today; keep the same shape).
- `verify-testdata` must cover `system_0.zkc`.
- The compile gate already covers the wiring.

## 9. Measuring the improvement

### 9.1 What to measure, and why R5 cycles alone mislead

Two metrics, always reported together:

1. **R5 cycles** — the `clock cycle: N` printf emitted by
   `riscv/interpreter.zkc:215` once per interpreted instruction (needs
   `zkc exec -vvv`). This is what `verifier-ray/bench/verifier_profile/main.go`
   and `bench/bench_pcs/run.go` already parse (`cycleRE`). It measures
   interpreter iterations only: **work done inside an accelerator (Poseidon2
   today, `WIOP_VERIFY` tomorrow) is invisible to it.** The Zig verifier with
   the Poseidon2 accelerator enabled already under-reports its hashing cost this
   way (case `MultiColumnBench`: 2.44 M "transcript" cycles for 7,559
   compressions is the marshalling and R5 code around the accelerator, not the
   permutation).
2. **Trace cells** (rows × columns, per module and total) — the quantity the
   prover actually pays for. Sources: `zkc trace --stats` (prints overall and
   per-module rows/columns/cells; `pkg/cmd/zkc/trace.go:186-320`), or the Go
   path `binFile.Trace(inputs, cfg)` + per-shard `module.Height() × module.Width()`
   exactly as `prover-ray/zkcdriver/r5_benchmark_test.go:97-104` does. Use the
   Go path for the real proof (the run is millions of cycles; use
   `vm.NewShardingStrategy("interpreter", 500000)` and `WithParallelism(true)`
   as that benchmark does).

Secondary: `zkc compile --stats` column counts of the new modules (column
pressure is a proving-cost multiplier for every row), Poseidon2 compression
count (the verifier's `profiling.poseidon2Compress` counter / the new
`wiop_p2_perm` module height), and end-to-end **prover wall time** once the
prover pipeline is available (`BenchmarkRisc5Arithmetization`-style
trace → `Prove`).

### 9.2 Baseline (Zig verifier as R5 guest)

Real system, honest proof:

```bash
cd verifier-ray
make generate-testdata               # produces riscv_system.zig + riscv_proof_image.bin (needs prover-ray to prove AllInOne guest)
make zkc-build                       # R5 ELF + JSON with the proof image at 0x08800000
go -C ../arithmetization tool zkc exec --fast -vvv zig-out/bin/verifier-ray.json \
    ../arithmetization/src/main/riscv/main.zkc | grep -o 'clock cycle: [0-9]*' | tail -1
```

Record with accelerators enabled (default) **and** `DISABLE_ACCELERATORS=true`
(pure-Zig Poseidon2), so the hashing share is visible. Then cells: a Go test
(new, next to `r5_benchmark_test.go`) that traces the same ELF+image with
sharding and prints total rows/cells and the top-20 modules by cells.

Synthetic systems: `make profile-zkc` regenerates
`bench/verifier-profile.csv` (91 cases, per-phase cycles via `VERIFIER-MARK`).
Keep it as the fine-grained baseline; its `poseidon2_compressions` column is
the compression count to compare the zkc `wiop_p2_perm` height against.

Note the profile target's `-Dembedded-input` mode embeds the proof in rodata,
so its cycle counts exclude image loading; the real-image path pays the blob
copy (§9.6).

### 9.3 New path (zkc accelerator)

Same JSON input (same ELF layout convention, proof at `0x08800000`), smoke
guest of §7.7:

```bash
zkc exec --fast -vvv smoke.json main.zkc | grep -o 'clock cycle: [0-9]*' | tail -1   # expect a few hundred cycles
# cells: Go trace with sharding, sum per module; report wiop_* modules separately
```

Report a table:

| | Zig verifier (accel on) | Zig verifier (accel off) | zkc `WIOP_VERIFY` |
|---|---|---|---|
| R5 cycles | | | |
| total trace cells | | | |
| cells in interpreter modules | | | |
| cells in Poseidon2 / `wiop_p2_perm` | | | |
| cells in `ram` module | | | |
| cells in other `wiop_*` modules | n/a | n/a | |
| Poseidon2 compressions | | | |
| prover wall time (when available) | | | |

The expected shape of the result: interpreter cells drop to the smoke guest's
few hundred rows; `ram` cells stay roughly equal (both paths read the whole
image once) or drop (aligned word reads vs. R5 `lw` through the interpreter);
Poseidon2 cells drop by the ratio of the straight-line permutation to the
loop-based one (§5.2); new `wiop_*` cells appear and should be dominated by
`wiop_p2_perm`, `ram`, and the DEEP Horner rows.

### 9.4 Cost probes to run first (Phase 0) — fill this table before Phase 1

Each probe is a 20-line `.zkc` harness run under `zkc trace --stats`:

| Probe | Number to record |
|---|---|
| `wiop_p2_perm` straight-line | columns of its module; cells per call |
| existing `permutation<poseidon2_state>()` | cells per call (for the ratio) |
| `wiop_ext_mul`, `wiop_ext_sqr`, `wiop_ext_inv`, `wiop_f_inv` | columns; cells per call |
| `read_32` aligned from `ram` vs `blobs_data[i]` ×4 vs a hypothetical `pub input wiop_image(i:u32)->(word:u32)` | cells per 32-bit word read (decides §9.6) |
| RW memory write + read (`wiop_ext` 6-column row) | cells per access |
| a 1,000-step recursion vs a 1,000-iteration `for` loop with a 6-felt body | cells (confirms the recursion rule quantitatively) |
| fixed-array `[𝔽;6]` param/return compile check | compiles? (decides Ext calling convention) |

### 9.5 Rough size model for the real proof (to sanity-check measurements)

From `prover-ray/wiop/proofserialization/README.md` §11.2/§11.3a: 229 queries,
blow-up 2, envelope 2^22, 4 input trees and 17 levels for the measured
circuits, ~140 values per opened row, 15 running layers, cap depth 8. Per
query: ≈ 4 trees × (17 − 8) levels of input-tree hashing + 15 × (≤ 9) running
siblings ≈ 170 `hash_node` calls (each 1–2 compressions) plus row hashing
≈ 4 trees × 2 rows × ⌈(3 + 140)/8⌉ ≈ 150 compressions, so **≈ 300–400
compressions per query, ≈ 70–90 k per proof**, plus caps once. The R5
interpreter system has more columns than `modexp_u256`, so expect more; the
counter in §9.2 gives the truth.

### 9.6 The image-loading floor

`riscv/main.zkc:57-67` copies every blob byte into `ram` with `write_8`: for
a 40 MiB image that is ~4×10^7 `write_8` calls before either verifier runs,
and both verifiers then read the same bytes back. This floor is **outside**
this plan's scope but will dominate the "after" numbers once the interpreter
cost is gone. Two follow-ups to measure with the §9.4 probe and decide later:
(a) word-wise blob loading (`write_32` on aligned runs) in `main.zkc`;
(b) letting `WIOP_VERIFY` read the image straight from the `blobs_data` input
memory (lookup, no timestamp) instead of `ram`, which also removes the copy
for the proof blob. Option (b) changes only `image.zkc`'s accessors
(`ram` → `blobs_data` with `internal_offset + (addr − blob_offset)` indexing)
and is why all image access is funnelled through §4.3's accessors.

---

## 10. Phased implementation checklist

Every phase ends with: harness green under `zkc execute -c` with `FAILURES`
grep, `zkc format --check` clean, and (from Phase 4 on) full-VM
`zkc compile src/main/riscv/main.zkc` exit 0 with the new code reachable.
Phases 1–3 can proceed in parallel; 4→11 are sequential.

| Phase | Deliverable | Zig source ported | Acceptance |
|---|---|---|---|
| 0 | §9.4 probe table; decision on Ext calling convention and image access path | — | table filled, numbers in `lib/wiop/README.md` |
| 1 | `field.zkc`, `ext.zkc` + harnesses | `field/koalabear.zig`, `koalabear_e2.zig`, `koalabear_ext.zig` | all `field_cases`/`ext_cases` pass; inv of 0 documented |
| 2 | `poseidon2_compress.zkc`, `transcript.zkc` + harnesses | `crypto/poseidon2.zig`, `crypto/fiat_shamir.zig` | `poseidon_cases`, `fiat_shamir_cases`, `runtime_trace_cases` pass; 1 row/compress |
| 3 | `image.zkc`, `public_input.zkc` + harness; Go emitter skeleton (`zkcgen`) producing `WIOP_ROUND_*`, `WIOP_PI_*` | `proof_abi.zig`, `protocol/public_input.zig` | re-encoded ABI image values read back correctly |
| 4 | `replay.zkc`; emitter emits spec; wire memories + `impl.zkc` stub into `main.zkc`/`interpreter.zkc` (so the compile gate covers the code) | `protocol/root.zig` (pr-3959) | coins equal `verify.zig` expected coins for all 69 cases |
| 5 | `merkle.zkc` | `crypto/merkle.zig` | pcs-derived micro-fixtures pass |
| 6 | `pcs_layout.zkc`; emitter emits PCS tables | `pcs.zig:271-472`, `verifier.zig:245-299` | layout arrays match prover-ray `GetLayout`/`canonicalLayout` on all `pcs.zig`/`verify.zig` cases |
| 7 | `pcs_verify.zkc` (challenges, caps, per-query, folds) | `pcs.zig:499-1087`, `fri.zig` | 4 `pcs.zig` cases (3 accept, 1 reject with the right reason), fold cases |
| 8 | `vanishing.zkc`; emitter emits vanishing tables | `query/vanishing.zig` | all `vanishing.zig` honest views accept, invalid views reject |
| 9 | `scalar_checks.zkc`; emitter emits LD/GP/RL/SR tables | `query/{logderivativesum,grandproduct,rowlimit,shared_randomness}.zig` | `verify.zig` cases with those queries pass |
| 10 | `verify.zkc`; full accelerator wiring (§7 items 1–6); `system_0.zkc` from the real R5 system; smoke guest | `verifier.zig`, `main.zig` | 69 `verify.zig` cases pass end to end; real proof image → status 1; corrupted → 0; compile gate green; `TestComputeOpBasesMatchConstantsZkc` green |
| 11 | Measurement (§9.2–9.3 table), `lib/wiop/README.md`, CI hooks | — | table in README; `verify-testdata` covers `system_0.zkc` |

Suggested agent hand-off per phase: give the agent this document, the Zig
file(s) named in the row, `lib/bls12_381/impl.zkc` as style reference, the
bls12 test Makefile as harness reference, and the fixture emitter mode to use.

---

## 11. Risks and open questions

1. **Fixed-array support scope** (params confirmed, returns and `[𝔽;N]`
   unconfirmed). Fallback: six explicit felts everywhere; signatures in this
   plan already assume that.
2. **Column explosion in straight-line functions.** `wiop_p2_perm` ≈ 1.3k
   columns is expected fine (bls12 `mulmod_384` was 15k columns). If the
   compiler struggles, split the permutation into `full_rounds_a`,
   `partial_rounds`, `full_rounds_b` (3 rows/compress).
3. **Recursion depth.** No call-depth limit was found in the VM, but the
   per-query recursion nests ~5 levels and the query recursion is 229 deep;
   the vanishing DAG depth is "a few dozen" (`vanishing.zig:229-231`). If the
   VM's frame stack is bounded, convert the outermost recursions (queries,
   modules) to `for` loops — they iterate over big bodies, so the loop
   overhead is negligible there.
4. **Emitted-table sizes** for the real R5 system are unknown until
   `riscv_system.zig` is regenerated (§9.2 step 1 needs a working prover).
   If the prover is unavailable, the synthetic 69 cases still validate
   everything except scale and shared randomness.
5. **Two-return branching panic** (§3.4) may reappear in `wiop_eval_expr`
   (`ok` + value). Mitigation described in §5.9.
6. **Dead-code gate vs incremental landing** (§3.6): phases 1–3 must either
   be included from `verify.zkc` immediately or kept out of `main.zkc`'s
   include graph until Phase 4. The plan chooses: land phases 1–3 as
   test-only includes, wire in Phase 4.
7. **Soundness parity, not improvement.** The port reproduces the Zig
   verifier's checks exactly, including the known prover-side gap (cell kind
   not validated, README §11.5). Any new check is a protocol change and needs
   prover-ray agreement.
8. **Image relocation base.** The accelerator reads absolute pointers; if the
   rollup guest ever places the image elsewhere than `0x08800000`, the image
   must be encoded for that base. `rs1` is the base; nothing else needs to
   change.

---

## Appendix A — where things are

| Topic | Path |
|---|---|
| Zig verifier | `verifier-ray/src/{verifier.zig,protocol/*,query/*,crypto/*,field/*,polynomial/*,proof_abi.zig}` |
| Zig codegen (source of the tables) | `verifier-ray/codegen/*.go`, `generate-riscv-system/main.go`, `riscv_bootstrap.go` |
| Fixtures | `verifier-ray/testdata/generated/{vectors,pcs,fri,vanishing,verify}.zig`, `testdata/generate/` |
| Proof image format | `prover-ray/wiop/proofserialization/{README.md,layout.go,encode.go,types.go}` |
| FRI parameters | `prover-ray/wiop/compilers/pcs/pcs.go:47-100` (229 queries, rate 1/2, envelope 2^22) |
| Accelerator reference | `pr-3927`: `arithmetization/src/main/lib/bls12_381/impl.zkc`, `riscv-guests/lineth-accelerators/src/bls12_381.zig` |
| Interpreter dispatch | `arithmetization/src/main/riscv/{main.zkc,interpreter.zkc,memory.zkc,ram/read.zkc}` |
| Existing Poseidon2 zkc | `arithmetization/src/main/lib/poseidon2/{constants,poseidon2,ram}.zkc` |
| zkc language reference (pinned) | `$GOMODCACHE/github.com/!l!f!d!t-!lineth/zkc@v1.2.32/docs/ZKC_LANGUAGE.md` |
| Profiling tools | `verifier-ray/bench/verifier_profile/main.go`, `bench/bench_pcs/run.go`, `bench/verifier-profile.csv` |
| Trace/cell accounting example | `prover-ray/zkcdriver/r5_benchmark_test.go:60-105` |

## Appendix B — zkc idioms to copy

```zkc
// Recursion instead of a loop (one row per step, no PC/RET columns)
fn wiop_absorb_cells<ram, wiop_fs>(P:Address, r:u8, i:u32, n:u32) {
    if i < n {
        var ok:u1
        var e0:𝔽 var e1:𝔽 var e2:𝔽 var e3:𝔽 var e4:𝔽 var e5:𝔽
        var is_ext:u1
        ok, is_ext, e0, e1, e2, e3, e4, e5 = wiop_cell_tagged(P, r, i)
        if is_ext == 1 { wiop_fs_write_ext(e0, e1, e2, e3, e4, e5) } else { wiop_fs_write(e0) }
        wiop_absorb_cells(P, r, i + 1, n)
    }
}

// Straight-line squaring chain (k known at the call site → unrolled by hand; k runtime → recursion)
fn wiop_ext_pow2k(e0:𝔽, e1:𝔽, e2:𝔽, e3:𝔽, e4:𝔽, e5:𝔽, k:u5) -> (r0:𝔽, r1:𝔽, r2:𝔽, r3:𝔽, r4:𝔽, r5:𝔽) {
    if k == 0 { r0 = e0 r1 = e1 r2 = e2 r3 = e3 r4 = e4 r5 = e5 return }
    var s0:𝔽 var s1:𝔽 var s2:𝔽 var s3:𝔽 var s4:𝔽 var s5:𝔽
    s0, s1, s2, s3, s4, s5 = wiop_ext_sqr(e0, e1, e2, e3, e4, e5)
    r0, r1, r2, r3, r4, r5 = wiop_ext_pow2k(s0, s1, s2, s3, s4, s5, k - 1)
}

// Multi-column memory and static table access
memory wiop_coins[u16](i:u10) -> (c0:𝔽, c1:𝔽, c2:𝔽, c3:𝔽, c4:𝔽, c5:𝔽)
static WIOP_ROUND_COINS(r:u8) -> (count:u16, offset:u16) { (0, 0), (1, 0), (1, 1), (0, 2) }
var count:u16 var offset:u16
count, offset = WIOP_ROUND_COINS[r]
```

