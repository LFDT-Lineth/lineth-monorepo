# Parallel Merkle-tree follow-up — 2026-08-07

See [`../../PERFORMANCE.md`](../../PERFORMANCE.md) for the long-term
architecture, optimization roadmap, benchmark protocol, and SOTA criteria.

## Result

`NewTree` now hashes each internal level in parallel while preserving the
existing Poseidon2 compression and commitment roots. On the fixed 64 MiB
single-matrix sweep, prover-ray is competitive with a native, SIMD-enabled
Plonky3 build: results range from 13% slower to 1.53x faster depending on rate
and shape.

The change fixes the measured shape sensitivity; it does not prove a universal
claim over arbitrary matrices. Production preflight shapes, the distribution
of multi-size buckets, and the base/extension-column mix are still unknown.

## Host and comparison contract

- verified EC2 instance: `c8a.24xlarge`;
- 96 vCPUs, exposed as one 96-core socket with one thread per core;
- AMD EPYC 9R45, one NUMA node, AVX-512;
- 185 GiB usable RAM and no swap;
- Go 1.26.0;
- Rust 1.98.0-nightly (2026-05-26);
- Plonky3 commit `a31a1443a114c58735850daa5b5fc5c43c138d9d`.
- Plonky3 flags: `-Ctarget-cpu=native`, fat LTO, one codegen unit, release
  optimization, and the `p3-maybe-rayon/parallel` feature.

Input size is logical base-field cells times four bytes. Input generation,
setup, transcript work, opening, verification, and destruction are outside the
timed full-commit region. Both implementations retain the committed data needed
for openings and use 96 threads unless stated otherwise.

This remains the closest matched comparison, not identical cryptography. Both
sides perform a KoalaBear LDE and a binary Poseidon2 Merkle commitment with an
eight-field digest, but layout, coset choice, leaf framing, permutation
constants, and roots differ. Deterministic roots are stable within each
implementation; roots are not expected to match across implementations.

## Fixed 64 MiB single-matrix sweep

Values are medians of five one-shot samples.

### Production rate two

| Shape | prover-ray | Plonky3 | Relative |
|---|---:|---:|---:|
| `2^20 x 16` | 77.9 ms | 69.0 ms | Plonky3 1.13x faster |
| `2^16 x 256` | 17.1 ms | 25.3 ms | prover-ray 1.48x faster |
| `2^12 x 4096` | 17.9 ms | 19.1 ms | prover-ray 1.07x faster |

### Matched rate four

| Shape | prover-ray | Plonky3 | Relative |
|---|---:|---:|---:|
| `2^20 x 16` | 105.1 ms | 115.2 ms | prover-ray 1.10x faster |
| `2^16 x 256` | 27.0 ms | 41.2 ms | prover-ray 1.53x faster |
| `2^12 x 4096` | 30.0 ms | 30.4 ms | effectively tied |

The rate-four Merkle-only medians are 84.7/16.8/11.0 ms for prover-ray and
72.3/14.1/7.27 ms for Plonky3. Plonky3 still wins the Merkle phase by roughly
17% to 51%; prover-ray's faster LDE compensates in the full commitment.

Relative to the serial-tree baseline, prover-ray's rate-four full commit is
about 19.3x faster tall, 5.4x faster balanced, and 1.2x faster wide. The tall
Merkle phase alone is about 23.8x faster.

## Concurrency

This is the rate-four `2^16 x 256` standalone full-commit harness. Values are
five-sample medians.

| Threads | prover-ray | Speedup | Plonky3 | Speedup |
|---:|---:|---:|---:|---:|
| 1 | 650.0 ms | 1.00x | 592.3 ms | 1.00x |
| 24 | 44.1 ms | 14.75x | 44.1 ms | 13.42x |
| 96 | 30.3 ms | 21.46x | 38.8 ms | 15.28x |

With native SIMD enabled, Plonky3's one-thread baseline is already fast and its
relative scaling is lower than in the generic build. Prover-ray finishes this
particular 96-thread case 1.28x sooner.

## Multi-size/base-extension smoke benchmark

The existing multi-size Go benchmark also passes after the scheduling change.
At rate four and 96 threads:

| Workload | Logical input | Full commit |
|---|---:|---:|
| sizes `2^8..2^10`, 64 base + 64 extension columns/size | 3.06 MiB | 3.74 ms |
| sizes `2^8..2^12`, 400 base + 400 extension columns/size | 84.8 MiB | 32.1 ms |

There is no apple-to-apple Plonky3 multi-size number in this run.

## Rate-two size and memory scaling

Peak RSS is from isolated `/usr/bin/time -v` processes. Commit time excludes
input generation; process wall time does not. These are one-shot large-memory
measurements.

| Input | Shape | Go commit | Go peak RSS | P3 commit | P3 peak RSS |
|---:|---|---:|---:|---:|---:|
| 1 GiB | `2^20 x 256` | 0.217 s | 3.42 GiB | 0.202 s | 2.12 GiB |
| 4 GiB | `2^22 x 256` | 1.177 s | 13.57 GiB | 0.685 s | 8.53 GiB |
| 10 GiB | `2^22 x 640` | 2.670 s | 31.59 GiB | 1.746 s | 20.53 GiB |

For the 10 GiB case, input-only RSS is 10.82 GiB for prover-ray and 10.00 GiB
for Plonky3. The incremental peak is therefore about 20.77 GiB versus 10.52
GiB. Native Plonky3 is 1.53x faster and prover-ray uses 54% more peak memory.

Using only the same-height 4-to-10 GiB slope gives this capacity projection:

| Input | Go projected commit | Go projected RSS | P3 projected commit | P3 projected RSS |
|---:|---:|---:|---:|---:|
| 50 GiB | 12.6 s | 152 GiB | 8.8 s | 101 GiB |
| 100 GiB | 25.1 s | 302 GiB | 17.7 s | 201 GiB |

These are linear extrapolations for a single maximum-height matrix, not
measurements and not a model of aggregate preflight data. A 50 GiB prover-ray
commit would consume roughly 82% of this host before contingency; 100 GiB is
impossible in RAM. The current API retains the input alongside the encoded
table and tree, so streaming/ownership changes are the next bottleneck.

## Implementation and validation

Levels remain sequential because each depends on its children. Nodes within a
level use `parallel.Execute` above a 512-node cutoff and call the unchanged
scalar `hashNode`; small upper levels remain serial. The existing x16 API was
not reused because it is an IV-zero Merkle-Damgard chain and would change tree
roots.

Validation completed:

- every node and auxiliary leaf matches a scalar reference for zero and
  deterministic-random inputs, multiple heights, and the parallel path;
- branch recovery and complete-binary-tree equivalence pass;
- full `crypto/koalabear/fri` tests pass;
- targeted race and `purego` tests pass;
- `gofmt` and `golangci-lint run ./crypto/koalabear/fri/...` pass.

Raw timing samples are in `samples.csv`; large-process measurements are in
`memory.csv`.

## Reproduction

```bash
FRI_COMMIT_BENCH_CELLS_LOG2=24 GOMAXPROCS=96 \
  go test ./crypto/koalabear/fri -run '^$' \
  -bench '^BenchmarkPCS(Commit|Merkle)Shapes$' \
  -benchmem -benchtime=1x -count=5

go build -o /tmp/prover-ray-fri-bench ./crypto/koalabear/fri/bench
/tmp/prover-ray-fri-bench --min-log2=20 --max-log2=20 \
  --base-polys=16 --ext-polys=0 --rate=2 --queries=32 \
  --phase=commit --json --gomaxprocs=96

CARGO_TARGET_DIR=/tmp/plonky3-native-target \
RUSTFLAGS='-Ctarget-cpu=native' \
CARGO_PROFILE_RELEASE_LTO=fat \
CARGO_PROFILE_RELEASE_CODEGEN_UNITS=1 \
  cargo build --release -p p3-fri --example commit_shapes

RAYON_NUM_THREADS=96 \
  /tmp/plonky3-native-target/release/examples/commit_shapes \
  --rows-log2=20 --columns=16 --log-blowup=1 --phase=commit
```
