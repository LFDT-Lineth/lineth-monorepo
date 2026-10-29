# Plan: zkc ecrecover accelerator, benchmarked against std and Constantine

Status: proposal, for review. Paths are relative to the repository root. Implement on branch
`precompile/zkc-ecrecover`. Do not modify PR #3719.

## Context

**The question.** Is a secp256k1 ecrecover implemented as a zkc accelerator (one custom RISC-V
instruction proved by zkc modules) cheaper to prove and execute than the two software backends the
`l2_execution` guest already has?
- `std`: Zig `std.crypto`
- `constantine`: Nim, built as a staticlib

PR #4293 (`f129577bb7`) added the switch `-Dsecp256k1-backend={std,constantine}`. This plan adds a
third value, `zkc`, and benchmarks all three with the method of the l2-execution benchmarking guide
in this directory, [`RUNNING_L2_EXEC.md`](RUNNING_L2_EXEC.md) ("the guide"). It adapts that method
where it cannot run zkc code.

**Starting point.** PR #3719 (head `fb95caf25`, branch `origin/feat/ecrecover-zkc-precomp`)
contains a working u256 ecrecover in zkc, but it is wired into an interpreter layout that no longer
exists on main. Reuse its **algorithm and test vectors** file by file, with
`git show fb95caf25:<path>`. Do not cherry-pick its commits:
- The PR used old paths (`l2-execution`, `riscv/instruction_processing/r_type.zkc`,
  `riscv/utils/constants.zkc`).
- Main now uses a **predecoded** interpreter: `common/constants.zkc`, `riscv/interpreter.zkc`, and a
  Go predecoder.

Credit "ported from PR #3719 (fb95caf25)" in each commit message.

## Facts established during planning (zkc v1.2.35, 2026-10-29)

| # | Fact | Evidence |
|---|---|---|
| F1 | Zesu calls only `zkvm_secp256k1_ecrecover`, from three places: tx sender recovery (`tx_signing.zig:304`), EIP-7702 authorities, and precompile 0x01. `zkvm_secp256k1_verify` is **never called**. | Zesu pin `2835883c…` |
| F2 | recid is always 0 or 1. Transactions are already low-s checked before the call. Precompile 0x01 does **not** range-check r or s and passes high-s through. Zesu keccak-hashes the returned 64-byte `x‖y` itself. | same |
| F3 | `zkc exec --gogen` cannot run the PR code. It rejects any `#[native]` function (`gogen: native function "mulmod_64" unsupported`), any register wider than 128 bits, divisions wider than 64 bits, and memory registers wider than 64 bits. **No zkc ecrecover that uses the native mulmod can run under gogen.** | ran |
| F4 | The default `zkc exec` (bytecode / "fast mode") and **sharded** `zkc trace` (its checkpoint pass uses the same bytecode interpreter) panic with `unsupported instruction form (4 v 5 limbs)` when **compiling** any program that compares registers with different limb counts. In the PR code the offender is `addmod_256`'s `sum >= (modL as u257)`. Fix: `var m:u257 = modL as u257` and compare against `m`. With that one change the full PR ecrecover runs under `zkc exec` and under `zkc trace -c --sharding …`. | ran |
| F5 | `zkc compile` (the CI gate `riscv-check-compilation`) hard-errors on modules unreachable from `main`. In the PR library those are `secp256k1_verify_generic`, `assert_on_curve_affine_secp256k1`, `is_on_curve_affine_secp256k1` and `is_equal_affine_secp256k1`. This is the review comment "doesn't compile with latest zkc". `exec` and `trace` do not run this check. | ran |
| F6 | prover-ray does **not** prove `#[native]` modules with zkc's AIR. `mulmod_<bits>` becomes a NonNative polynomial-identity query: 5L limb columns + (2L−1) carry columns, with L = ⌈bits/16⌉, so **111 columns** for `mulmod_256`. zkc's own AIR uses 2,684–3,600 columns for it. `expmod_unconstrained_<bits>` is left **unconstrained** (4L columns), which makes it a real hint. Register names are an interface and must be kept exactly: `lhsL rhsL modL remainder quotient` / `base exp mod res`. | `prover-ray/zkcdriver/native_modules.go`, `prover-ray/wiop/compilers/nonnative/nonnative.go` |
| F7 | `interpreter_c`'s `default` arm turns `COMPUTE_INVALID` and any unhandled op into a **silent no-op**. A mis-wired opcode does not crash. | `arithmetization/src/main/riscv/interpreter.zkc` ~L829 |
| F8 | The keccak accelerator case on main is `fail "keccak accelerator unsupported for now"`. Always build with `-Dkeccak-accel=false`. | `arithmetization/src/main/riscv/interpreter.zkc:535` |
| F9 | The R-type ComputeOp range 25–55 is full, so adding one op shifts S/B/J/U. No test cross-checks the Go bases against `constants.zkc`, so a mismatch fails silently. | code reading |
| F10 | A `fail`-only stub `fn f<ram>(a:u64,b:u64){ fail "…%x", a }` passes both `zkc compile` and `--gogen`. | ran |

**Preliminary numbers.** Measured on a 12-core, 16 GB laptop. They are a hypothesis to confirm, not
results.
- The std figures come from a throwaway rv64im guest that runs Zig-std ecrecover K times, traced
  against the stock `main.zkc`.
- The zkc figures come from tracing the PR's `ecrecover_generic`, with the F4 fix applied for the
  `exec` timing.

| Per ecrecover | std (Zig, RISC-V) | zkc (PR #3719 code, unoptimized) |
|---|---|---|
| RISC-V instructions | ~3.6M | ~1 custom instruction + wrapper |
| Trace cells, padded, zkc AIR | **2.42G** (15 shards / 4 calls) | **~50M** (35M unpadded) |
| Cells with prover-ray native accounting (F6) | ≈ 2.42G | **~5.4M** |
| `zkc exec` (bytecode) | ~2.3 s | ~0.08 s |
| `zkc trace` | ~24 s wall / ~70 CPU-s, sharded | ~1.4 s, one core, unsharded |
| `exec --gogen` | tens of ms | N/A (F3) |

Of the ~11.2K `mulmod_256` rows per zkc call, about 30M of the 35M unpadded zkc-AIR cells are
mulmod. Under prover-ray accounting, `addmod_256` (19K rows × 139 columns) becomes the largest item.
Constantine was not measured; the guide reports it at about 2× std per transaction.

## Design (decided)

1. **Representation.** Port PR #3719's u256 code with `#[native] mulmod_256` and
   `#[native] expmod_unconstrained_256`. This is the design prover-ray can prove cheaply (F6). It
   cannot run under gogen (F3); every comparison is made equal-width (F4) so bytecode `exec` and
   sharded `trace` work.
2. **Opt-in wiring.** `riscv/main.zkc` must stay gogen-compatible for every other guest.
   - Move the current body of `arithmetization/src/main/riscv/main.zkc` (includes plus `fn main`)
     **unchanged** into `riscv/boot.zkc` with `git mv`.
   - `riscv/main.zkc` becomes: `include "boot.zkc"` and `include "../lib/secp256k1/stub.zkc"`.
   - New `riscv/main_secp256k1.zkc`: `include "boot.zkc"` and `include "../lib/secp256k1/impl.zkc"`.
   - `riscv/interpreter.zkc` calls `secp256k1_ecrecover(...)` but includes **neither** of the two;
     the entry file provides it.
   - `arithmetization/gopkg/embedded` follows includes from `riscv/main.zkc`
     (`//go:embed main` covers the tree), so prover-ray keeps compiling the stub variant.
3. **Scope.** ecrecover only (F1). For `zkvm_secp256k1_verify`, the `zkc` backend re-exports std's
   verify.
4. **ABI.**
   - Encoding: custom-0 (`0x0b`), funct3 `0b000`, funct7 `0b0000001`, as reserved for ECRECOVER in
     `arithmetization/src/main/lib/README.md`. No register writeback, rd = `x0`, exempt from the
     rd=x0 → NO_OP fold (same style as keccak / write_output).
   - `rs1` → input buffer, 8-byte aligned, 104 B: `z[32] ‖ r[32] ‖ s[32] ‖ recid[1] ‖ pad[7]`.
     z, r and s are **little-endian u256** (byte 0 = least significant), so zkc reads them with 4×
     the existing `read_64` and needs no new big-endian RAM helpers. This addresses the PR #3719
     review comment asking for an endianness bit instead of duplicated `_be` read/write modules.
   - `rs2` → output buffer, 8-byte aligned, 72 B: `x[32] ‖ y[32] ‖ status[1] ‖ pad[7]`. x and y are
     LE. status is 1 = ok, 0 = invalid. On failure x and y are written as zeros.
   - status = 0 iff `r==0 ∨ r≥n ∨ s==0 ∨ s≥n ∨ recid>1 ∨ r³+7 is a non-residue ∨ Q=O`. High-s is
     **accepted** (F2). z is used unreduced (`mulmod` reduces it).
   - The Zig wrapper does the big-endian ↔ little-endian conversion and maps recid 0/27 → 0 and
     1/28 → 1, matching `secp256k1_std.ecrecover`. It pre-fills `status = 0xA5` and panics if status
     is not 0 or 1 afterwards, to catch F7. It copies x‖y to the caller only on success.

## Phase A: implementation

### A1. zkc library (`arithmetization/src/main/lib/`)

**`lib/nonnative/u256.zkc`**
- Start from **main's** `prover-ray/zkcdriver/testdata/lib/nonnative/u256.zkc`. It is known to pass
  prover-ray's native tests (`TestNative`, `TestSecp256k1*`).
- Copy it; leave prover-ray untouched.
- Apply the F4 fix in `addmod_256`, and audit **every** comparison for equal widths.
- Do not `switch` on a wide register: the bytecode interpreter panics on split switches
  (zkc `transform/split/switch.go`).
- Keep only reachable functions. `reduce_256` is used by `sqrt`; `expmod_unconstrained_256` and
  `invmod_256` are used.

**`lib/secp256k1/secp256k1.zkc`**
- From the PR version (it has `Gx`/`Gy`, `p_plus_1_over_4` and `sqrt_secp256k1`).
- Delete the unreachable helpers (F5) and inline `generator_secp256k1` into its callers.

**`lib/secp256k1/ecrecover.zkc`**
- From the PR's `ecrecover_generic`, renamed to
  `fn ecrecover_secp256k1(z:u256, r:u256, s:u256, recid:u8) -> (qx:u256, qy:u256, ok:u1)`.
- `if recid > 1 { return }`, then derive the parity bit with a ternary / `recid as u1`, instead of
  the v=27/28 mapping (PR #3719 review comment). Note that `==.` is not a zkc operator; plain `==`
  works.
- Drop `secp256k1_verify_generic` and `n_half`.

**`lib/secp256k1/impl.zkc`**
- `fn secp256k1_ecrecover<ram>(in_addr:Address, out_addr:Address)`.
- A local `read_u256_le` assembles 4× `read_64` with `::`.
- Write the outputs with 4× `write_64` per coordinate, using `::` splits. Narrowing `as` is checked,
  so never use it to extract bytes or limbs.
- Then `write_8(out_addr+64, ok as u8)`.
- Do not branch on more than one of the multi-return values (known zkc "conflicting read on
  register" panic).

**`lib/secp256k1/stub.zkc`**
- Same signature; body is
  `fail "secp256k1 ecrecover accelerator not compiled in (use riscv/main_secp256k1.zkc) in=%x out=%x", in_addr, out_addr`.

Run `go tool zkc format` on all new files; the lint target covers `src/main/lib/**` and
`src/main/riscv/**`.

### A2. Dispatch wiring

**`arithmetization/src/main/common/constants.zkc`**
- Add `OPCODE_CUSTOM0:Opcode = 0b0001011` (next to `OPCODE_CUSTOM1`, ~L134).
- Add `FUNCT3_SECP256K1_ECRECOVER = 0b000` and `FUNCT7_SECP256K1_ECRECOVER = 0b0000001`
  (~L308–314).
- Add `RTYPE_SECP256K1_ECRECOVER:ComputeOp = 56`, and renumber S 57–60, B 61–66, J 67–68, U 69–70.
- Update the range comment at L22–23.
- Add `R_SECP256K1_ECRECOVER_INST:u17`, following the `R_KECCAK_INST` pattern (~L346).

**`arithmetization/src/main/riscv/interpreter.zkc`**
- Add `case RTYPE_SECP256K1_ECRECOVER` in **`interpreter_c`**, the cold tier, before `default`:
  `secp256k1_ecrecover(v1 as Address, v2 as Address)`, then `new_pc = (pc + 4) as Address`, then
  `return`.
- Update the header comment ("Custom-0 — secp256k1 ecrecover (no register writeback)").
- Check with `zkc compile --stats` that the widths of `interpreter_a`/`_b` are unchanged.

**`arithmetization/src/main/predecoding/check/check_r_type.zkc`**
- Add a top-level `case R_SECP256K1_ECRECOVER_INST`, non-folding, like `R_KECCAK_INST`.
- This file is not compiled today; keep it consistent anyway.

**`arithmetization/gopkg/predecoding/decode.go`**
- Add `opcodeCUSTOM0 = 0b0001011`.
- `instructionTypeFromOpcode`: map custom-0 to `rType`.
- Add `rtypeOpSecp256k1Ecrecover = 31`.
- `decodeRTypeSemantic`: add `case opcodeCUSTOM0` (funct3 000 + funct7 0000001 → the op, anything
  else → `rtypeInvalid`).
- Add the `shouldUseNoOp` exemption.
- Bases: S 57, B 61, J 67, U 69.
- Update the tables in `gopkg/predecoding/doc.go`.

**`decode_test.go`**
- Add cases mirroring the keccak ones: `TestShouldUseNoOp`, `TestDecodeRTypeSemantic`,
  `TestClassifyInstructionExamples`, and an invalid custom-0 funct7.
- Add **`TestComputeOpBasesMatchConstantsZkc`**: regex-parse `src/main/common/constants.zkc` for the
  first S/B/J/U op values and `RTYPE_*` values and compare them with the Go constants (guards F9).

**Docs and Makefile**
- `arithmetization/src/main/lib/README.md`: mark the ECRECOVER row 🟢 with funct3/funct7, the ABI, and
  "opt-in via `riscv/main_secp256k1.zkc`".
- `arithmetization/Makefile`: add `riscv-check-compilation-secp256k1`
  (`$(ZKC) compile -o … src/main/riscv/main_secp256k1.zkc`).

### A3. Guest side

**`riscv-guests/lineth_accelerators/src/secp256k1.zig`** (new)
- Define the `extern struct` input and output layouts (`align(8)`).
- Raw call:
  `asm volatile (".insn r 0x0b, 0b000, 0b0000001, x0, %[in], %[out]" :: [in] "r" (..), [out] "r" (..) : .{ .memory = true })`.
- Add `pub fn zkvm_secp256k1_ecrecover(msg, sig, recid, output) callconv(.c) zkvm_status`
  implementing the standard C ABI from `include/zkvm_accelerators.h:207`, following the `keccak.zig`
  style (panic on null).
- Re-export in `src/root.zig`.

**`riscv-guests/l2_execution/src/secp256k1_zkc.zig`** (new)
- `pub fn ecrecover(...) bool` returns `lineth_accel.zkvm_secp256k1_ecrecover(...) == .ZKVM_EOK`.
- `pub const secp256k1Verify = @import("secp256k1_std").secp256k1Verify;`

**`riscv-guests/l2_execution/build.zig`**
- `Secp256k1Backend = enum { std, constantine, zkc }`, and update the option help text.
- Create `secp256k1_zkc_mod` with imports `lineth_zkvm_accel` and `secp256k1_std`, and add it to
  `provide_mod`.
- In `native_options` (~L214), map `.zkc` → `.std` so host tests and tools never assemble the custom
  instruction.

**`riscv-guests/l2_execution/src/zkvm_provide.zig`**
- Add the `.zkc => secp256k1_zkc` switch arm.
- Update the manifest comment.

**`riscv-guests/l2_execution/Makefile`**
- When `SECP256K1_BACKEND=zkc`, pass
  `ZKC_MAIN=$(abspath ../../arithmetization/src/main/riscv/main_secp256k1.zkc)` to the
  `elf-exec`/`elf-trace` sub-make (`exec`, `trace`, `debug` targets).
- Error out if `ZKC_EXEC_FLAGS` contains `--gogen`/`-g` for that backend.

**Zig test harness**
- `arithmetization/src/test/zig/build.zig`: forward a `secp256k1-backend` option to the
  `l2_execution` dependency, the same way `keccak-accel` is forwarded.
- `arithmetization/src/test/Makefile` `compile` line: add
  `$(if $(SECP256K1_BACKEND),-Dsecp256k1-backend=$(SECP256K1_BACKEND))`.

**READMEs**
- `riscv-guests/l2_execution/README.md` and `riscv-guests/README.md`: document `zkc`, the opt-in
  `ZKC_MAIN`, and "no gogen".

### A4. Correctness tests

**T1, zkc unit tests**
- Files: `arithmetization/src/test/zkc/secp256k1/{ecrecover.zkc, ecrecover.accepts, Makefile}`.
- Vectors: from `git show fb95caf25:prover-ray/zkcdriver/testdata/ecrecover.accepts` (14 cases), with
  v converted to recid (27→0, 28→1, others → 2, 0xff), plus a high-s case.
- The Makefile follows the pattern of `src/test/zkc/poseidon2/Makefile`, but runs **both**
  `go tool zkc execute` (guards F4) and `go tool zkc trace -c` per line.
- It fails on a non-zero exit **or** on `level=error` in stderr, because `trace -c` exits 0 on AIR
  violations.

**T2, guest end-to-end tests**
- File: `arithmetization/src/test/zig/src/ecrecover/ecrecover_bench.zig`. It calls the exported
  `zkvm_secp256k1_ecrecover`; follow `src/keccak/keccak_with_provider_single_input.zig` for how to
  import `zkvm_provide`.
- It first checks a fixed vector set with exit 1 on any mismatch:
  - the 4 valid PR vectors;
  - rejects: r=0, r=n, s=0, s=n, recid=2, QNR r=5, the Q=O case;
  - the 9 rows of `riscv-guests/l2_execution/test/testdata/ecrecover.csv`, copied as constants.
- It then runs K extra ecrecovers over the valid vectors, where K is the first input byte read via
  `read_input` (IN_BYTES). K is used by B1.
- It must exit 0 for `std`, `constantine` and `zkc`. The `zkc` run must use `main_secp256k1.zkc`.
- Running `zkc` against plain `main.zkc` must hit the stub's `fail`. That is the negative check.

**T3, compile gates**
- `make -C arithmetization riscv-check-compilation riscv-check-lint riscv-check-compilation-secp256k1`.

**T4, Go tests**
- `cd arithmetization && go test ./gopkg/predecoding/... ./gopkg/embedded/...`.

**T5, host tests unchanged**
- `make -C riscv-guests/l2_execution test` (host tools, default backend).

**T6, block differential**
- For every benchmark input, the `guest_output` bytes (not just the `0x0003` schema) must be identical
  across the three backends.

## Phase A: benchmark

**Where it runs.** A Linux box like the guide's reference machine (16 cores, ≥91 GB, `nim` + `clang` +
`llvm-ar` for Constantine). Follow the guide for inputs, wrapping, the IN_ORIGIN `0x08800000`, and
its methodology:
- run sequentially;
- discard the first run;
- report the median of 3 or more;
- alternate backends within each repetition.

**Committed scripts.** Put them in this directory (`riscv-guests/l2_execution/bench/secp256k1/`).
- `bench.sh` generalizes the guide's `bench_exec.sh`:
  - variants `std constantine zkc`;
  - one ELF per variant, built with
    `zig build --prefix $WORK/guest-$v -Dsecp256k1-backend=$v -Dkeccak-accel=false`;
  - `ZKC_MAIN` per variant;
  - JSON named per variant, deleted after a rebuild (the guide's stale-JSON pitfall).
- `summarize_trace.py` (run with `python3 -I`) parses `zkc trace --stats -r -vv` logs and outputs:
  - shards (= checkpoints);
  - steps (`Machine execution (N steps, M checkpoints)`);
  - Σ padded cells over shards;
  - **prover-ray-adjusted cells**: for module `mulmod_<b>`, replace the width with 5L+2L−1; for
    `expmod_unconstrained_<b>`, replace it with 4L; L=⌈b/16⌉; heights unchanged;
  - max module height per shard, flagged against 2^22 (prover-ray's per-module cap).

**B0, fixed width cost (every shard).** The accelerator adds modules, and therefore committed
columns, to the whole system, not only to shards that call ecrecover. Per-call cells do not show
this cost. Estimate from the PR's module widths: about 5K extra columns with prover-ray accounting.
- From `arithmetization/`, run `go tool zkc compile --stats` on `src/main/riscv/main.zkc` and on
  `src/main/riscv/main_secp256k1.zkc`. Record:
  - total columns for each, and the delta;
  - the new modules with their widths;
  - the delta again with prover-ray accounting, using the same substitution as
    `summarize_trace.py` (`mulmod_<b>` → 5L+2L−1, `expmod_unconstrained_<b>` → 4L).
- Find out from prover-ray code (`prover-ray/zkcdriver`, `prover-ray/wiop`) whether a module with
  no rows in a shard is still committed in that shard's proof. State the answer with file
  references. This decides whether the delta is paid by every block or only by blocks that use
  ecrecover.
- Check it empirically too: trace the std guest on the committed sample
  `riscv-guests/l2_execution/test/testdata/stateless_input.ssz` against both entry points and
  compare total padded cells.
- B0 is cheap and needs no downloaded inputs. Do it first.

**B1, micro (per ecrecover).**
- Use `ecrecover_bench.zig` with K ∈ {0, 16} for each backend.
- Per-call cost = (X(16) − X(0)) / 16.
- X is each of:
  - `zkc exec` wall time (bytecode), for all three;
  - `exec --gogen` wall time, for std and constantine only;
  - steps;
  - trace cells (padded and prover-ray-adjusted);
  - trace wall / CPU / RSS at `--sharding interpreter:1000000`.

**B2, macro (per block).**
- `exec` (bytecode) for all three, plus `--gogen` for std and constantine, on:
  - the guide's sample blocks (PR #4139; not on main yet, fetch them with
    `git show origin/pr-4139:…` as the guide describes);
  - the seven 30M fixtures;
  - the custom EEST transfer blocks (`test_fixed_count_transfers`, 250 and 1000 txs);
  - the EEST `precompile/ecrecover` benchmark (`fill -k ecrecover` at 30M, last block).
- `zkc trace --stats -r -vv --sharding interpreter:1000000` for all three on: `786k`, 250-transfer,
  `ether_transfers_0_30M`, and precompile-ecrecover.
- In ecrecover-dense shards zkc modules are tall (~11K `mulmod_256` rows per call) and wide
  (2.7–3.6K columns in zkc's AIR, so tracer RAM grows). If the max module height approaches 2^22 or
  RSS approaches RAM, re-run that input at `interpreter:500000` and say so.
- Run T6 on every input.

**Report.** Write `RESULTS.md` in this directory with:
- machine and commit;
- the fixed width cost (B0), and whether every shard pays it;
- the micro table;
- the macro table, with ratios zkc/std and constantine/std for exec, steps, shards, cells and
  adjusted cells;
- the Phase B progression;
- a one-paragraph verdict.

The verdict counts "zkc is more performant" when per-ecrecover **adjusted cells** and **trace time**
are lower than std's. State the block-level reduction for transfer-heavy blocks. Weigh it against
the fixed width cost from B0, especially for opcode-bound blocks that gain nothing per call. Say
explicitly that gogen is not applicable to zkc.

## Phase B: one optimization round (zkc only)

For each step:
- re-run T1, T2 and T3, plus B1 for zkc;
- record zkc-AIR cells, adjusted cells, `mulmod_256`/`addmod_256` rows, and trace time;
- re-run B0 and record the system column delta: removing modules lowers the fixed cost, while
  adding modules (for example the joint scalar multiplication) can raise it;
- keep the step only if adjusted cells drop and correctness holds;
- commit each step separately.

Steps, in order:
1. **Self-contained hint.** Rewrite the body of `expmod_unconstrained_256` with inline u512 `%`
   (`::`-split to u256) instead of calling `mulmod_256`. The hint's internal products currently land
   in the constrained `mulmod_256` module; prover-ray leaves expmod itself unconstrained (F6). Keep the
   module name and `base/exp/mod/res` register names. `invmod_256` keeps its single checking mulmod.
2. **One final inversion.** Keep `u1·G` and `u2·R` projective, add them projectively, and call
   `projective_to_affine` once. Today there are three conversions plus two `affine_to_projective`
   calls.
3. **Shamir / Straus joint double-and-add.** One recursion over the bit pairs of (u1, u2), with
   precomputed G+R. This saves about 255 doublings.
4. **`/%` in `mulmod_256`.** The PR's variant measured 2,684 vs 3,600 zkc-AIR columns. It only affects
   zkc cells and trace time. Verify that prover-ray `TestNative` and `TestSecp256k1*` still pass with a
   copy of the lib.
5. **Constants via native mulmod.** Replace the ×3, ×8 and ×21 addmod chains in add/double with
   `mulmod_256(x, c, p)`: 111 prover-ray columns vs 139 per addmod. Decide on adjusted cells, and report
   the zkc-AIR and trace-time cost.

Out of scope, list as follow-ups in RESULTS.md: a fixed-base G table via a zkc `static` memory; GLV;
lazy reduction with a wider native (`mulmod_264`); prover-ray proving time.

## Verification checklist (end to end)

1. T1–T5 green. `make -C arithmetization riscv-check-compilation` still compiles the stub `main.zkc`,
   and `go tool zkc exec --gogen` on a std guest JSON with `src/main/riscv/main.zkc` still works.
2. ELF check: build with `-Dsecp256k1-backend=zkc` and scan for 4-byte-**aligned** words with
   `(w&0x7f)==0x0b && ((w>>12)&7)==0 && ((w>>25)&0x7f)==1`. They must be present, and absent in std
   builds.
3. `make -C riscv-guests/l2_execution exec SECP256K1_BACKEND=zkc KECCAK_ACCEL=false INPUT=<abs path>/test/testdata/stateless_input.ssz`
   prints the same `guest_output` as std.
4. B1 numbers are in the ballpark of the preliminary table. If not, explain the difference.
5. RESULTS.md answers the question with numbers for all three backends, including the B0 fixed
   width cost.

## Gotchas (read before coding)

- `--gogen` with `main_secp256k1.zkc` fails with exit 4 by design (F3). Never pass `-g` for zkc runs.
- Bytecode-interpreter panics happen at **program load**. Any mixed-width comparison or wide `switch`
  that is reachable breaks `exec` and sharded `trace`. T1 runs `execute` to catch this.
- `zkc compile` is the only command that enforces reachability; `exec` and `trace` do not (F5).
- `trace -c` / `execute -c` exit 0 on AIR violations. Grep stderr for `level=error`.
- A ComputeOp mismatch between Go and zkc is silent and reinterprets every store, branch and jump
  (F9). An unhandled op is a silent no-op (F7).
- Narrowing `as` is checked. Use `::` splits for limbs and bytes.
- Changing native module names or register names breaks prover-ray (F6).
- Tracing large blocks needs tens of GB. Never run two traces at once on the 91 GB box.
- Keccak accel is broken on main (F8). Always use `-Dkeccak-accel=false`.

## Upstream zkc requests (record in RESULTS.md)

1. The bytecode `SkipIf` encoder should zero-extend mismatched limb vectors
   (`pkg/zkc/vm/internal/interpreter/encoding/skip_if.go:63` in v1.2.35).
2. gogen support for `#[native]` (`mulmod_*`, `expmod_unconstrained_*`) and for registers wider than
   128 bits. Without it, a zkc ecrecover can never run under gogen, and putting it in `main.zkc` would
   break gogen for every guest.
