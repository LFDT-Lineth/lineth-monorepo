# FRI PCS performance handoff

This document records the performance investigations completed on 2026-08-07
and defines a reproducible path toward a state-of-the-art KoalaBear FRI
commitment implementation. It is a handoff for an engineer or agent continuing
the work, not a permanent claim that the current implementation is fastest.

Statements use these labels:

- **Measured:** observed on the recorded benchmark host.
- **Code fact:** derived from the current implementation.
- **Hypothesis:** a proposed explanation or optimization that still needs a
  controlled experiment.
- **Unknown:** information that must be collected before making a production
  capacity claim.

## Current conclusion

**Measured (2026-08-07, optimized branch):** prover-ray's full commit is now
faster than the fully native, SIMD-enabled, Rayon-parallel Plonky3 build in
every measured configuration, cold one-shot processes, medians of five:

- 64 MiB rate two: 1.7x to 2.1x faster across tall/balanced/wide shapes.
- 64 MiB rate four: 1.5x to 2.5x faster.
- 10 GiB rate two: 1397 ms versus 1758 ms (1.26x) in the latency-optimal
  configuration.

**Measured:** the isolated Merkle phase, which Plonky3 previously won by 17%
to 51%, is now 3.2x faster (tall) and 1.2x faster (balanced) in prover-ray;
Plonky3 keeps a 1.2x edge only on the widest leaf shape (2^12 x 4096).

**Measured:** peak memory remains prover-ray's only deficit. On the 10 GiB
case: 32.2 GiB (latency-optimal) versus Plonky3's 20.5 GiB. With
`fri.WithConsumeWitness()` plus `GOMEMLIMIT` the peak drops to 22.3-23.9 GiB at
a latency between 1.44 s and 1.84 s across observed runs (run-to-run variance
under GC pressure is real; report ranges, not best cases).

**Unknown (unchanged, still P0):** the real preflight distribution of table
heights, base widths, extension widths, rounds, commitments, and shards.
Aggregate preflight size alone is not a PCS workload description.

Do not claim universal "SOTA." The defensible claim today: on this host and
this synthetic single-size workload matrix, prover-ray leads full-commit
latency everywhere and is approaching Plonky3's memory envelope when the
memory-tuned mode is enabled. Roots and proof behavior are unchanged.

## What changed on 2026-08-07 (optimization session)

Six changes, all preserving bit-identical commitment roots (verified by
deterministic root markers across every benchmark sample and by
scalar-equivalence tests):

1. **Parallel internal tree levels** (previous session): tree levels are
   hashed level-by-level with `parallel.Execute` within a level.
2. **Cache-friendly bit-reversal** (`bitreverse.go`): gnark-crypto's
   `utils.BitReverse` falls back to a naive swap loop whenever
   `sizeof(T) < 8`, so 4-byte KoalaBear columns always took the cache-hostile
   path — measured at 43% of all commit CPU on the 4 GiB case. Ported the
   COBRA tiled algorithm specialized for 4-byte elements. The naive/COBRA
   crossover was tuned under 96-way concurrent load (the regime Encode
   actually runs in): naive wins through 2^20, COBRA wins 1.4x at 2^21 and
   2.1x at 2^22.
3. **Batched internal-node compression** (`poseidon2_batch*.go/.s` in
   `crypto/koalabear/poseidon2`): a copy of gnark-crypto's AVX-512
   `permutation16x16xN_columns_avx512` kernel modified to load the 16
   Merkle-Damgard chain states from memory instead of zeroing them. A single
   call therefore computes 16 direct `C(left,right)` compressions (nbSteps=1)
   or `C(C(left,right),aux)` chains (nbSteps=2), and the row-major result is
   scattered directly into `nodes[k0:k0+16]`. Replaces the scalar
   one-node-at-a-time `hashNode` loop that was 55% of the tall-shape commit.
   Bit-identity with scalar `Compress` is tested lane-by-lane, including the
   purego fallback. This kernel is a candidate for upstreaming to
   gnark-crypto.
4. **Rate-two encode restructure** (`reedsolomon.go`): in N-bit-reversed
   order, the even codeword positions of a rate-two subgroup LDE are exactly
   the input evaluations and land contiguously in the first half; the odd
   positions are the ω_N-coset evaluations and fill the second half. `Encode`
   therefore writes a bit-reversed copy of the input as the first half, and
   computes only a half-size coset FFT for the second: inverse FFT in DIT
   (consumes the bit-reversed copy directly, emits natural-order
   coefficients), then `FFT(DIF, OnCoset())` on a domain constructed with
   `WithShift(Domain.Generator)`. This removes the full-size FFT and the
   standalone coefficient bit-reversal of the generic path. Note this trick is
   unavailable to Plonky3, whose LDE uses a generator-shifted coset. The
   generic path remains for other rates and is verified equivalent by
   `TestEncodeRateTwoFastPath`.
5. **Zero-copy tree bottom** (`commitment.go`, `tree.go`, `fri.go`):
   `Merkleize` hashes the bottom leaves directly into the tree's node array
   instead of allocating a leaf array and copying it in; `buildTreeExt` fills
   octuplets in place the same way. Saves an allocation and a memmove the size
   of the encoded bottom table.
6. **Slab-allocated codewords** (`commitment.go`, `EncodeInto`): see the
   cold-allocation trap below.

Memory (opt-in): `fri.WithConsumeWitness()` transfers witness ownership to
`Commit`, releasing each plaintext column as its codeword is computed. The GC
only collects them under heap pressure, so pair it with `GOMEMLIMIT` sized
below (witness + codewords + tree); with a limit above that sum it has no
effect. For a rate-two code the plaintext values remain recoverable from the
first half of each codeword.

### The cold-allocation trap (read before benchmarking Go against Rust)

**Measured:** encoding thousands of columns used to allocate one codeword
buffer per column inside a 96-thread `parallel.Execute` loop. Codewords above
32 KiB are Go large objects: every allocation takes the page-heap path, and
under high thread count in a *cold* process the concurrent first-touch page
faults serialize in the kernel. The wide rate-four commit spent 24.9 s system
time versus 0.7 s user time — 372 ms wall instead of 17 ms — and the CPU
profile misattributed the kernel time to the FFT kernel the faults landed in.

This is invisible in warm processes: `go test -count=N` reuses heap spans, so
the recorded 2026-08-07-parallel-tree shape numbers (measured warm) never saw
it. The fix is one contiguous slab per size bucket, sliced per column
(`RSEncoder.EncodeInto` / `EncodeExtInto`). After the fix the cold wide
rate-four commit is 17.5 ms and even the cold rate-two case improved from
20 ms to 11 ms.

**Protocol rule:** always measure cold one-shot and warm steady-state
separately; a Go-versus-Rust comparison that only measures one of them is not
trustworthy.

## Read this first: the benchmark mistake (kept from the previous handoff)

The first Plonky3 comparison was built without native CPU flags, so its
AVX-512 paths were compiled out and the resulting prover-ray wins were
invalid. Always build Plonky3 with:

```bash
CARGO_TARGET_DIR=/tmp/plonky3-native-target \
RUSTFLAGS='-Ctarget-cpu=native' \
CARGO_PROFILE_RELEASE_LTO=fat \
CARGO_PROFILE_RELEASE_CODEGEN_UNITS=1 \
  cargo build --release -p p3-fri --example commit_shapes
```

and confirm Rayon thread count and hot-function AVX sampling before accepting
results. The generic-build data is preserved under `benchmarks/2026-08-07`
(name `plonky3-generic`); never mix it with `plonky3-native` rows.

## PCS architecture and workload shape

### Production path

**Code fact:** the WIOP compiler commits one round through this path:

```text
round columns
  -> commitToRound
  -> pad and copy columns into fri.MultiSizeTable
  -> fri.Commit
       -> MultiSizeTable.encode   (slab per size; optional witness consume)
       -> MultiSizeTable.Merkleize (bottom leaves hash into tree nodes)
  -> retain EncodedTable and Tree for openings
```

Relevant entry points:

- `wiop/compilers/pcs/pcs.go`: `commitToRound`, production rate, padding.
- `multi_size_table.go`: table invariants and base/extension layout.
- `commitment.go`: `Commit`, `Encode`/`encode`, `Merkleize`,
  `WithConsumeWitness`.
- `reedsolomon.go`: `EncodeInto`/`EncodeExtInto`, rate-two coset fast path.
- `bitreverse.go`: COBRA bit-reversal and `bitReverseCopy`.
- `commitment_simd.go`: 16-lane leaf hashing and column-major staging.
- `tree.go`: `allocTree`, `buildLevels`, batched `hashTreeLevel`.
- `crypto/koalabear/poseidon2/poseidon2_batch*.{go,s}`: state-seeded 16-lane
  compression kernel.

### Rate and table limits

**Code fact:** production sets `FRILogInverseRate = 1` (blowup two), which is
exactly the rate the coset fast path accelerates. `wiop.ColumnSizeMaxSupported`
caps plaintext columns at `2^22` rows; larger traces become wider tables, more
size buckets, more rounds/commitments, or more shards.

### What one Merkle leaf hashes

Unchanged from the previous handoff: a bottom leaf digests one encoded row, a
non-bottom multi-size leaf digests two adjacent encoded rows one level
shallower, and the stream is domain-separated by
`[leafDomainTag, BaseWidth, ExtWidth]` with left-padding of the final
eight-element block.

## Corrected results (2026-08-07, optimized)

### Reproduction manifest

Host, toolchains, and checksums are in
`benchmarks/2026-08-07-optimized/environment.txt`: same `c8a.24xlarge` (AMD
EPYC 9R45, 96 vCPU, 185 GiB, one NUMA node), Go 1.26.0, prover-ray at this
branch, and a Plonky3 binary bit-identical (SHA-256) to the pinned
`a31a1443...` native build of the previous baseline.
All full-commit samples are isolated cold processes with interleaved Go/P3
execution order; medians of five (three for 4 GiB, two to three for 10 GiB).

### Fixed 64 MiB, production rate two, 96 workers

| Source shape | prover-ray | Native P3 | Result |
|---|---:|---:|---|
| `2^20 x 16` | 31.8 ms | 65.7 ms | prover-ray 2.07x faster |
| `2^16 x 256` | 12.0 ms | 25.0 ms | prover-ray 2.08x faster |
| `2^12 x 4096` | 10.9 ms | 18.5 ms | prover-ray 1.70x faster |

Previous baseline for context: 77.9 / 17.1 / 17.9 ms, with Plonky3 winning
the tall shape.

### Fixed 64 MiB, diagnostic rate four, 96 workers

| Source shape | prover-ray | Native P3 | Result |
|---|---:|---:|---|
| `2^20 x 16` | 44.8 ms | 113.1 ms | prover-ray 2.5x faster |
| `2^16 x 256` | 20.7 ms | 39.4 ms | prover-ray 1.9x faster |
| `2^12 x 4096` | 19.2 ms | 28.7 ms | prover-ray 1.5x faster |

Rate-four Merkle-only medians (see `merkle-only.csv` for protocol):

| Source shape | prover-ray | Native P3 | Result |
|---|---:|---:|---|
| `2^20 x 16` | 21.9 ms | 70.4 ms | prover-ray 3.2x faster |
| `2^16 x 256` | 11.5 ms | 14.2 ms | prover-ray 1.2x faster |
| `2^12 x 4096` | 9.2 ms | 7.8 ms | P3 1.2x faster |

### Thread scaling

Rate-four `2^16 x 256` full commit, five-sample medians:

| Workers | prover-ray | Speedup | Native P3 | Speedup |
|---:|---:|---:|---:|---:|
| 1 | 544.2 ms | 1.00x | 592.3 ms | 1.00x |
| 24 | 30.8 ms | 17.7x | 44.0 ms | 13.5x |
| 96 | 20.7 ms | 26.3x | 39.4 ms | 15.0x |

### Large rate-two processes

One-shot isolated processes; peak RSS covers the whole process including
input generation.

| Input | Source shape | prover-ray (latency-optimal) | prover-ray (memory-tuned) | Native P3 |
|---:|---|---:|---:|---:|
| 1 GiB | `2^20 x 256` | 158 ms / 3.40 GiB | 155 ms / 3.40 GiB | 201 ms / 2.12 GiB |
| 4 GiB | `2^22 x 256` | 638 ms / 13.8 GiB | 612 ms / 13.8 GiB | 680 ms / 8.53 GiB |
| 10 GiB | `2^22 x 640` | 1397 ms / 32.2 GiB | 1.44-1.84 s / 22.3-23.9 GiB | 1758 ms / 20.5 GiB |

Memory-tuned means `--consume-input` plus `GOMEMLIMIT`. At 1 and 4 GiB the
24 GiB limit exceeds the live set, so consumption never triggers and RSS is
unchanged — the limit must be sized below (witness + codewords + tree) to have
an effect. At 10 GiB, limits of 21-24 GiB all landed in a 22.3-23.9 GiB peak;
latency varied between 1.44 s and 1.84 s across runs. Treat the memory-tuned
latency as "at or below Plonky3's", not as the 1397 ms headline.

**Measured:** prover-ray's remaining RSS excess over Plonky3 at 10 GiB
(~2-3 GiB) is the in-flight columns, Go runtime/allocator overhead, domain
tables, and GC lag; the previous 11 GiB excess was retained witness columns.

## Current bottlenecks and next experiments

Priorities must follow new profiles. The 4 GiB rate-two profile after this
session's changes: leaf-hash batch permutation ~27%, FFT kernels ~25%, COBRA
bit-reversal ~21%, memmove ~13%, memclr ~7%, batched tree compression ~1%.

### P0 (unchanged): obtain the production shape histogram

Per round/shard, record `(size_log2, BaseWidth, ExtWidth, padding direction,
logical bytes)` plus commitment coexistence and buffer lifetimes. Without this
histogram the synthetic sweep can still optimize the wrong regime. Add a
metadata-only dump near `commitToRound`; do not log witness values.

### P1: leaf hashing (now the top kernel)

**Hypothesis:** the column-major staging copy (`fillGroup`, ~7% CPU) can fuse
into the batch permutation kernel (in-register transpose), and the leaf kernel
itself may benefit from processing two blocks per load. The widest shape is
the only remaining Merkle deficit against Plonky3, and it is leaf-bound.

### P2: extension-field and multi-size coverage

**Unknown:** this matrix is base-field, single-size only. The rate-two ext
fast path (`EncodeExtInto`) and multi-size batches (aux-leaf levels, the
nbSteps=2 kernel path) are tested for correctness but not yet benchmarked
against Plonky3-equivalent workloads. Extend `commit_shapes` comparisons to a
realistic base/ext mix once P0 defines "realistic".

### P3: memory, second round

Remaining ideas, in expected-value order:

- reconstruct plaintext from rate-two codeword halves at opening time so the
  caller can always hand over witness ownership (removes the biggest retained
  buffer in production, not just in benchmarks);
- staged commitment API with explicit lifetimes (encode and merkleize size
  buckets as they are ready, release earlier);
- arena or pooled slabs for repeated commitments in one process;
- GC tuning guidance for the prover fleet (GOMEMLIMIT sizing rule above).

### P4: FFT and bit-reversal

- **Hypothesis:** fusing the two rate-two halves' page-touch order could cut
  memmove/memclr further (the copy into `_p[n:]` and the bit-reversed copy
  into `_p[:n]` traverse the same source twice).
- **Hypothesis:** a COBRA variant with software prefetch or non-temporal
  stores may lift the ~30 GB/s per-socket bit-reversal ceiling observed under
  full load.
- Upstream the 4-byte COBRA specialization and the state-seeded batch
  compression kernel to gnark-crypto; delete the local copies when released.

## Benchmark protocol

The required workload matrix, execution discipline, and result acceptance
rules from the previous handoff remain in force, with these additions:

1. Measure cold one-shot and warm steady-state separately (the
   cold-allocation trap above); never compare a warm Go number against a cold
   Rust number or vice versa.
2. Record `/usr/bin/time -v` user/system split for every large or anomalous
   run; >10% system time is a red flag that the number measures the kernel,
   not the algorithm.
3. When a memory-tuned mode is benchmarked, report the observed latency range
   across runs, not the best sample, and state the GOMEMLIMIT used.

Standalone commands (the harness gained `-phase`, `-json`, `-consume-input`,
`-cpuprofile`):

```bash
go build -o /tmp/prover-ray-fri-bench ./crypto/koalabear/fri/bench
/tmp/prover-ray-fri-bench \
  --min-log2=22 --max-log2=22 --base-polys=640 --ext-polys=0 \
  --rate=2 --queries=32 --phase=commit --json --gomaxprocs=96 \
  [--consume-input]   # pair with GOMEMLIMIT for the memory-tuned mode

RAYON_NUM_THREADS=96 \
  /tmp/plonky3-native-target/release/examples/commit_shapes \
  --rows-log2=22 --columns=640 --log-blowup=1 --phase=commit
```

Go shape benches (warm-process diagnostics only):

```bash
FRI_COMMIT_BENCH_CELLS_LOG2=24 GOMAXPROCS=96 \
  go test ./crypto/koalabear/fri -run '^$' \
  -bench '^BenchmarkPCS(Commit|Encode|Merkle)Shapes$' \
  -benchmem -benchtime=1x -count=5
```

## Correctness gates

Every optimization must remain bit-identical to prover-ray's scalar
semantics. Required checks (all passing as of this handoff):

```bash
go test ./crypto/koalabear/fri ./crypto/koalabear/poseidon2 -count=1
go test -race ./crypto/koalabear/fri ./crypto/koalabear/poseidon2 -count=1
go test -tags purego ./crypto/koalabear/fri ./crypto/koalabear/poseidon2 -count=1
gofmt -l crypto/koalabear/fri crypto/koalabear/poseidon2
golangci-lint run ./crypto/koalabear/fri/... ./crypto/koalabear/poseidon2/...
```

Dedicated equivalence tests added this session:

- `TestCompressChain16MatchesScalar`: every lane of the batch kernel against
  scalar `Compress`, one/two/three-block chains, zero and random values,
  including the generic fallback.
- `TestEncodeRateTwoFastPath`: rate-two coset fast path against the generic
  zero-pad path, base and extension, sizes spanning the naive and COBRA
  bit-reversal regimes; rate four must not construct a coset domain.
- `TestBitReverseMatchesNaive`, `TestBitReverseCopyMatchesInPlace`.
- The pre-existing tree scalar-equivalence tests cover the batched
  `hashTreeLevel` (aux and no-aux levels, parallel thresholds, small trees).

Deterministic root markers are recorded in every benchmark sample and are
unchanged from the pre-optimization implementation. Do not compare Go and
Plonky3 roots; the cryptographic configurations remain intentionally
different (see the semantic-differences table in the previous handoff's
"What the benchmark actually compares" — still accurate).

## Definition of done

For a credible state-of-the-art claim, all of the following are required:

1. The production shape histogram is known and represented in benchmarks
   (**still missing — P0**).
2. Plonky3 is pinned, native-SIMD, parallel, equivalently optimized. (Done.)
3. The comparison boundary and semantic differences are documented. (Done.)
4. Prover-ray is on the latency/RSS Pareto frontier for every declared
   production envelope. (Latency: leading everywhere measured. Memory: within
   ~10-15% of Plonky3 in memory-tuned mode at 10 GiB; worse without it.)
5. Large-case memory is safe for the intended host with explicit headroom.
   (10 GiB fits a 185 GiB host trivially; 50-100 GiB cases remain unmeasured
   and must not be attempted without staged allocation and an RSS guard.)
6. Roots and proof behavior remain compatible with prover-ray. (Done,
   verified.)
7. Raw results, manifests, and commands are retained
   (`benchmarks/2026-08-07-optimized/`); the measured code is this branch.
8. The claim covers exactly the measured workloads and host — nothing
   broader.

Accurate status: prover-ray leads native Plonky3 on full-commit latency for
every measured synthetic shape at rates two and four and every measured size
up to 10 GiB, leads the isolated Merkle phase except the widest leaf shape,
and closes most of the memory gap only in the opt-in memory-tuned mode. The
production preflight shape histogram remains the most important missing
input, and extension-field/multi-size comparative benchmarks are the largest
untested surface.
