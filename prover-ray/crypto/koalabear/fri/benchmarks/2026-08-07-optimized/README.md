# 2026-08-07 optimized commit benchmarks

Comparison of the optimized prover-ray FRI commit (this branch, patch in
`prover-ray.patch` applied to base SHA in `environment.txt`) against the same
pinned native Plonky3 build as `../2026-08-07-parallel-tree` (binary SHA-256
verified identical; AVX-512 `-Ctarget-cpu=native`, fat LTO, Rayon enabled).

Files:

- `samples.csv` — every full-commit sample. Each row is one isolated process
  (cold one-shot), Go and Plonky3 runs interleaved per configuration.
  `implementation` distinguishes plain prover-ray from the
  `+consume+memlimit=…` memory-tuned variants (witness ownership transfer via
  `fri.WithConsumeWitness()` plus `GOMEMLIMIT`).
- `summary.csv` — per-configuration median/min/max and median peak RSS
  (`/usr/bin/time -v`, whole process, includes input generation).
- `merkle-only.csv` — isolated Merkle diagnostics at rate 4. prover-ray rows
  come from `BenchmarkPCSMerkleShapes` (warm process, benchtime=1x count=5);
  Plonky3 rows from `--phase=merkle` one-shot processes. Encode rows are
  prover-ray only (`BenchmarkPCSEncodeShapes`); their ~52 ms outliers are GC
  cycles landing inside a warm iteration.
- `environment.txt` — host, toolchains, SHAs, binary checksums. The measured
  prover-ray code is the content of the PR branch containing this directory
  (post-measurement cleanups were spot-checked for unchanged timings/roots).

Protocol notes:

- Commit timers cover encode + Merkleize + root, excluding input generation
  and setup, as in the previous baseline.
- Cold-process numbers matter: per-column codeword allocation used to spend
  >90% of a wide rate-4 commit in the kernel page-fault path, invisible in
  warm-process benchmarks. See PERFORMANCE.md ("the cold-allocation trap").
- Deterministic roots are recorded per sample in `samples.csv` (`marker`) and
  are unchanged from the pre-optimization implementation.

SHA-256 checksums:

- `samples.csv`: `71fe5e939b23791de06132a685352a33214b6db9996476df1f0460a3fe978db6`
- `summary.csv`: `e6470ed337aed4efa5449248ef296cc23d5590e3949b420adf113ecb874a385f`
