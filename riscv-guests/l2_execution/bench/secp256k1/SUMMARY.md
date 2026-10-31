# Summary

The question: is a zkc ecrecover accelerator cheaper than the two software backends of the
`l2_execution` guest, `std` (Zig `std.crypto`) and `constantine`? This page covers three things:
- what stands in the way of a like-for-like comparison;
- what can be estimated today, and what only the benchmark can tell;
- the hardware the benchmark needs.

The implementation and benchmark steps are in [PLAN.md](PLAN.md). The benchmarking method is in
[RUNNING_L2_EXEC.md](RUNNING_L2_EXEC.md).

All measurements below were taken on 2026-10-29 with zkc v1.2.35, on a 12-core, 16 GB laptop:
- **std**: a small rv64im test guest running the same Zig code as
  `src/secp256k1_std.zig` (ReleaseSmall), traced against the stock `riscv/main.zkc`.
- **zkc**: the u256 ecrecover from PR #3719 (`fb95caf25`), unoptimized, run directly in zkc with
  the one-line fix described in blocker 2.
- **constantine**: not built on this machine, so its figures come from the reference results in
  `RUNNING_L2_EXEC.md`.

## Blockers to a like-for-like comparison

**1. `zkc exec --gogen` cannot run the zkc ecrecover. This is a real blocker.**
- Gogen rejects any `#[native]` function, any register wider than 128 bits, and any division wider
  than 64 bits.
- The design worth benchmarking needs all three. Its core is `#[native] mulmod_256`, which
  prover-ray proves with a dedicated gadget.
- Consequences:
  - Execution time is compared with plain `zkc exec` for all three backends. That is fair (same
    interpreter), but slower than gogen and not the guide's gogen numbers. Gogen numbers exist only
    for std and Constantine.
  - The zkc ecrecover has to be wired in opt-in, through a separate entry point
    `riscv/main_secp256k1.zkc`. Otherwise gogen stops working for every guest.
  - A version that gogen can run is possible (64-bit limbs, no `#[native]`), but it gives up
    prover-ray's gadget. It would benchmark a design we would not ship.
  - A real fix needs zkc support (upstream request).

**2. Plain `zkc exec` and sharded `zkc trace` crashed on the PR code. Solved.**
- Both compile the program for zkc's bytecode interpreter, which panics on any comparison between
  registers of different widths (`unsupported instruction form (4 v 5 limbs)`).
- The cause is a single comparison in `addmod_256`. Making it equal-width fixes it; checked with
  4 ecrecovers under `exec` and under `trace -c --sharding`.
- Without the fix only unsharded tracing works, which is impossible for real blocks.

**3. The PR code does not pass `zkc compile`. Solved by deleting code.**
- `zkc compile` (the CI gate) rejects functions that `main` cannot reach. The PR has several,
  including its secp256k1 verify path.
- Zesu never calls verify. Only `zkvm_secp256k1_ecrecover` is used: for transaction senders,
  EIP-7702 authorities and precompile 0x01. The unreachable functions can go.

**4. Raw trace cells overstate the zkc cost. Measurement adjustment, not a blocker.**
- `zkc trace --stats` counts `mulmod_256` at zkc's own width of 2,684–3,600 columns. prover-ray
  instead commits about 111 columns for it (80 limb columns + 31 carry columns).
- prover-ray leaves `expmod_unconstrained_256` unconstrained (a hint).
- std and Constantine have no native modules, so they are unaffected. Report both raw cells and
  prover-ray-adjusted cells.

**5. Per-call cells miss a fixed cost. Must be measured (step B0 in PLAN.md).**
- Every module added to the arithmetization adds committed columns to every shard's proof, even
  shards that never call ecrecover.
- Estimate from the PR's module widths, using prover-ray's narrow mulmod: about **5K extra
  columns**, against a system I last measured at about 10.6K columns (Sept 2026).
- We have not checked whether prover-ray skips modules that have no rows in a shard. If it does
  not, proof size and recursion cost grow for every block, not just ecrecover-heavy ones.
- How to measure: compare `zkc compile --stats` for `main.zkc` and `main_secp256k1.zkc`.

**6. Practical constraints. Not blockers.**
- `riscv-guests/l2_execution/build.zig` always links the Constantine staticlib. Building the guest
  for **any** backend therefore needs `nim`, `clang` and `llvm-ar`.
- The keccak accelerator fails on main, so all three backends are built with
  `-Dkeccak-accel=false`. That is the same for all of them, so the comparison stays fair.

## Performance: rough estimates

### What can be said now

Per ecrecover:

| Metric | std (measured) | zkc (measured) | zkc vs std |
|---|---|---|---|
| RISC-V instructions | ~3.6M | ~1 custom instruction + wrapper | |
| Trace cells, padded, zkc's own count | ~2.42G | ~50M (35M unpadded) | ~48× fewer |
| Trace cells as prover-ray commits them | ~2.42G | ~5.4M (estimated, see blocker 4) | ~450× fewer |
| `zkc trace` time | ~24 s wall (~70 CPU-s) on 12 cores | ~1.4 s on 1 core | ~17× wall, ~50× CPU |
| Plain `zkc exec` time | ~2.3 s | ~0.08 s | ~29× faster |
| `exec --gogen` time | tens of ms | cannot run (blocker 1) | N/A |

- **Constantine.** Your guide shows Constantine at 1.2–2.2× std per block, and about 25 ms slower
  per transfer under gogen, all of it from ecrecover. Its ecrecover is more expensive than std's, so
  zkc's lead over Constantine is at least as large as its lead over std.
- **Why zkc wins per call.** std spends about 3.6M RISC-V instructions at roughly 680 trace cells
  each. zkc replaces that with about 11K rows of modular-multiplication modules. Optimizing the zkc
  code (Phase B in PLAN.md) can only widen the gap.
- **Per block, transfer-heavy (unmeasured estimate).** The guide's numbers put ecrecover at roughly
  half or more of a plain transfer's cost (std ≈ 24 ms per transfer, of which ecrecover is most of
  the 25 ms Constantine–std gap). Blocks like `ether_transfers` would plausibly get **50–90%
  cheaper**, before the fixed cost in blocker 5.
- **Per block, opcode-bound.** Blocks like `arithmetic`, `mod` and `keccak` hold one transaction,
  so there is essentially no change.
- **Confidence.** High that zkc is far cheaper per call. Medium on the size of the block-level win,
  which depends on the share of ecrecover in each block and on the fixed width cost.

### What the benchmark will tell

- Per-call cost for **all three** backends on one machine, including Constantine:
  - instructions;
  - raw and prover-ray-adjusted trace cells;
  - trace time and memory;
  - plain `zkc exec` time;
  - gogen time for std and Constantine.
- Real block-level effect on the guide's sample blocks, the seven 30M fixtures, the 250- and
  1000-transfer blocks, `ether_transfers`, and an ecrecover-precompile-heavy block. That covers
  shard counts, total and adjusted cells, trace time and peak memory.
- Correctness: the guest output must be identical across all three backends on every input.
- Whether ecrecover-dense shards push zkc modules towards prover-ray's limit of 2^22 rows per module,
  or push tracer memory too high.
- How much each Phase B optimization buys.
- The fixed width cost (blocker 5, step B0 in PLAN.md), and whether every shard pays it or only
  shards that call ecrecover.

What the benchmark will **not** tell:
- a gogen time for zkc (blocker 1);
- real prover-ray proving time. Trace cells are the proxy; proving is listed as a follow-up.

## Machine requirements

| Task | Machine | Notes |
|---|---|---|
| Implementation and unit tests | A laptop is enough | Needs `nim`, `clang` and `llvm-ar` to build the l2_execution guest at all (blocker 6). `make -C riscv-guests install-constantine-deps` exists; it has not been tried on macOS. |
| Micro benchmark (per call) | Modest | The std trace of 4 ecrecovers took 6.5 GB and 97 s on 12 cores; zkc is tiny. Still run it on the benchmark machine so all three backends share one machine. |
| Block benchmark | Linux, ≥16 cores, **≥91 GB RAM** (the guide's reference machine). 128 GB would be safer. | The guide measured 42–63 GB peak to trace the 786k block. zkc's modules are very wide (2.7K columns per mulmod row in zkc's own count), so ecrecover-dense blocks may need more. Never run two traces at once. |

**Time budget for the block benchmark: about 1–1.5 days of machine time.** These are estimates
extrapolated from the guide's numbers and the laptop measurements above.
- Tracing `ether_transfers`: about 4–5 h for std on 16 cores, and roughly double that for
  Constantine. zkc should be shorter, because it executes far fewer instructions.
- Plain `zkc exec` runs on one core: about an hour per std run of `ether_transfers`.
- More cores speed up tracing, because shards are traced in parallel. They do not speed up `exec`.
- To keep the budget down:
  - do one repetition for the 30M blocks, as the guide already does for `ether_transfers`;
  - trace only the four inputs PLAN.md lists;
  - use the 250-transfer block as the main trace input if time runs short.

**Software:**
- Zig 0.16.0 and Go, with zkc pinned to v1.2.35 by `arithmetization/go.mod`;
- `nim`, `clang`, `llvm-ar`;
- `jq`, `xxd`, `gh` or `curl`;
- Python with `uv`, to fill the custom EEST blocks.

The 30M fixture tarball is 156 MB, and each guest JSON is about 10 MB.
