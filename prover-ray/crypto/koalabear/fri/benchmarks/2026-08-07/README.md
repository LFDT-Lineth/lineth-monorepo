# KoalaBear FRI commitment benchmarks — 2026-08-07

> **Superseded comparison:** this initial Plonky3 build had Rayon enabled but
> omitted `-Ctarget-cpu=native`, so it did not compile Plonky3's AVX2/AVX-512
> paths. Do not use its cross-implementation ratios. The corrected native
> comparison is in `../2026-08-07-parallel-tree/README.md`.

## Result

The current PCS is fast on wide traces, but not on tall, narrow traces.
At a fixed 64 MiB logical trace size, prover-ray ranges from 9.3x slower
than Plonky3 for `2^20 x 16` to 2.8x faster for `2^12 x 4096`.
The crossover is almost entirely in Merkle construction; prover-ray's
rate-four Reed-Solomon encoder is faster for all three shapes.

At the directly measured 10 GiB maximum-height shape (`2^22 x 640`), the
unsampled commit times are close: 11.38 s for prover-ray and 13.81 s for
Plonky3. Peak RSS differs materially: 52.9 GiB versus 41.1 GiB.

These results say the PCS is competitive for sufficiently wide matrices, but
is not yet shape-robust enough to call optimized for arbitrary preflight
traces. The first optimization target should be the internal Merkle levels,
not the LDE.

## Equivalence contract

“Input bytes” means logical base-field cells times four bytes. It excludes Go
slice headers, Rust `Vec` capacity, FFT tables, tree nodes, and allocator
overhead. Every measured full commit performs:

1. one KoalaBear base-field matrix commitment;
2. a rate-four two-adic LDE;
3. a binary Poseidon2 Merkle commitment with an eight-field digest; and
4. retention of the prover data needed to open the commitment.

Input generation, setup, transcript work, opening, verification, and process
destruction are outside the timed region. Inputs are deterministic and fully
materialized before timing. Both implementations use 96 threads unless a
thread count is named explicitly.

This is the closest matched PCS comparison, not identical cryptography:

- prover-ray stores columns contiguously; Plonky3 stores rows contiguously;
- prover-ray's LDE is over the subgroup, while Plonky3 uses its production
  generator-shifted coset;
- prover-ray adds a three-field leaf domain header and uses its feed-forward
  Merkle-Damgard construction; Plonky3 uses `PaddingFreeSponge` and
  `TruncatedPermutation`;
- permutation constants and roots therefore differ; deterministic roots were
  checked within each implementation, not across implementations.

## Fixed-size shape sweep

Five one-iteration samples per cell; values are medians. Logical throughput is
input bytes divided by full-commit wall time.

| Shape | prover-ray | Plonky3 | prover-ray / Plonky3 | prover-ray GiB/s | Plonky3 GiB/s |
|---|---:|---:|---:|---:|---:|
| `2^20 x 16` (tall/narrow) | 2.027 s | 0.217 s | 9.32x slower | 0.031 | 0.287 |
| `2^16 x 256` | 0.147 s | 0.117 s | 1.25x slower | 0.426 | 0.533 |
| `2^12 x 4096` (short/wide) | 0.0366 s | 0.1019 s | 2.78x faster | 1.708 | 0.613 |

Phase medians explain the reversal:

| Shape | Go RS encode | P3 LDE | Go Merkle | P3 Merkle |
|---|---:|---:|---:|---:|
| `2^20 x 16` | 36.6 ms | 54.3 ms | 2.013 s | 164.4 ms |
| `2^16 x 256` | 11.3 ms | 34.5 ms | 135.2 ms | 82.1 ms |
| `2^12 x 4096` | 19.6 ms | 28.5 ms | 18.0 ms | 74.6 ms |

The isolated phase sums need not exactly equal full commit because each phase
is a separate process/benchmark fixture and garbage-collection timing differs.

## Concurrency

This sweep uses the `2^16 x 256`, 64 MiB shape. It uses the standalone harness
for both implementations, so the Go numbers are slightly slower than the Go
testing benchmark above because the harness records runtime metrics.

| Threads | prover-ray | Speedup | Plonky3 | Speedup |
|---:|---:|---:|---:|---:|
| 1 | 650.1 ms | 1.0x | 6.010 s | 1.0x |
| 24 | 163.3 ms | 3.98x | 271.2 ms | 22.16x |
| 96 | 160.3 ms | 4.06x | 116.8 ms | 51.48x |

prover-ray is 9.2x faster at one thread because its gnark-crypto kernels use
AVX-512, but it gains almost nothing beyond 24 threads for this shape.
Plonky3 makes much better use of the 96 cores.

## Size and memory scaling

Clean timings disable the Go heap sampler. Peak RSS comes from separate
`/usr/bin/time -v` subprocesses with heap sampling enabled for prover-ray.
“Input RSS” is a separate input-only process; for prover-ray it also includes
the eagerly precomputed PCS/FFT domains. “Incremental” is peak commit RSS minus
input RSS, not a claim about instantaneous allocator working set.

| Input | Shape | Go time | P3 time | Go input RSS | Go peak RSS | P3 input RSS | P3 peak RSS |
|---:|---|---:|---:|---:|---:|---:|---:|
| 256 MiB | `2^18 x 256` | 0.582 s | 0.418 s | 0.355 GiB | 1.437 GiB | 0.252 GiB | 1.058 GiB |
| 1 GiB | `2^20 x 256` | 2.377 s | 1.460 s | 1.425 GiB | 5.790 GiB | 1.002 GiB | 4.243 GiB |
| 4 GiB | `2^22 x 256` | 9.714 s | 5.690 s | 5.311 GiB | 22.835 GiB | 4.002 GiB | 17.051 GiB |
| 10 GiB | `2^22 x 640` | 11.381 s | 13.812 s | 11.315 GiB | 52.863 GiB | 10.003 GiB | 41.052 GiB |

At the same maximum height, increasing width from 256 to 640 amortizes the
fixed cost of prover-ray's internal tree levels. That is why its 10 GiB case is
only 17% slower than its 4 GiB case. Plonky3 scales almost linearly with cells.

The rejected `2^23 x 320` 10 GiB shape is important: a rate-four codeword would
need `2^25` points, beyond KoalaBear's two-adicity. The maximum plaintext
height for this configuration is `2^22`; larger traces must add columns,
matrices, commitments, or shards.

## 50–100 GiB outlook

The direct measurements do not justify allocating 50 or 100 GiB inputs on this
185 GiB machine. At fixed maximum height, the 4-to-10 GiB RSS slope projects:

| Input | prover-ray projected peak RSS | Plonky3 projected peak RSS |
|---:|---:|---:|
| 50 GiB | about 253 GiB | about 201 GiB |
| 100 GiB | about 503 GiB | about 401 GiB |

Both exceed available RAM before contingency. These are linear capacity
projections, not measurements.

Latency projection is shape-dependent. Scaling by rows at width 256 gives
about 0.41 GiB/s for prover-ray and 0.70 GiB/s for Plonky3. Scaling width at
the fixed `2^22` height amortizes prover-ray's serial tree cost and gives a
much more optimistic local model. For 50–100 GiB, those models bound
prover-ray at roughly 22–243 s and Plonky3 at roughly 68–142 s, before memory
pressure. The range is deliberately wide: no single-number extrapolation is
credible without the real trace-shape distribution.

Operationally, 10 GiB-or-smaller commitments per process are a safer planning
unit on this host. Larger preflight data should be split across commitments or
machines, and peak fleet memory should be measured with the production mix of
table heights and widths.

## Optimization priorities

1. Parallelize and batch the internal Merkle levels. The tall-profile has
   2.00 s cumulative in `NewTree`; scalar `permutation16_avx512` accounts for
   49.6% of samples, while the x16 leaf kernel is only 17.2%. The leaves are
   already parallel and SIMD-batched, but `NewTree` hashes internal nodes in a
   serial descending loop. Level-parallel x16 compression is the clearest
   high-value target.
2. Preserve the current RS encoder path. It beats the matched Plonky3 LDE in
   every 64 MiB shape (1.5x to 3.1x).
3. Reduce retained memory. At 10 GiB, prover-ray keeps the original 10 GiB
   witness alongside the 40 GiB encoded table and tree/setup memory. An
   ownership-taking or staged API that releases plaintext columns after LDE
   could save close to one input-size, subject to opening/caller lifetimes.
4. Benchmark the actual preflight table distribution. Wider leaf payloads
   amortize tree depth dramatically; optimizing against aggregate GiB alone
   will choose the wrong design.
5. Recheck thread partitioning and GC behavior after tree parallelization.
   Today, 24 and 96 threads are effectively tied for the representative Go
   shape.

## Reproduction

Go shape sweep:

```bash
FRI_COMMIT_BENCH_CELLS_LOG2=24 GOMAXPROCS=96 \
  go test ./crypto/koalabear/fri -run '^$' \
  -bench '^BenchmarkPCS(Commit|Encode|Merkle)Shapes$' \
  -benchmem -benchtime=1x -count=5
```

Build the standalone Go harness:

```bash
go build -o /tmp/prover-ray-fri-bench ./crypto/koalabear/fri/bench
```

Clone Plonky3 at the recorded commit, add the two dev-dependencies described in
`plonky3-cargo.patch`, copy `plonky3-commit-shapes.rs` to
`fri/examples/commit_shapes.rs`, then build:

```bash
cargo build --release -p p3-fri --example commit_shapes
RAYON_NUM_THREADS=96 target/release/examples/commit_shapes \
  --rows-log2=16 --columns=256 --log-blowup=2 --phase=commit
```

See `samples.csv` for individual timing samples and `memory.csv` for subprocess
RSS/CPU measurements.
