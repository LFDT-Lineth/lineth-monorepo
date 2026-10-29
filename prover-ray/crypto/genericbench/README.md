# genericbench: generic vs non-generic Poseidon2 / Fiat-Shamir

Benchmarks that measure the cost of the generic field abstraction
(`maths/generic`, `crypto/poseidon2`, `crypto/fiatshamir`) against
non-generic baselines, for KoalaBear and MamaBear. The package contains only
tests and benchmarks; nothing here is used by the prover.

## Running on a benchmark machine

These steps are written to be followed as-is, including by an agent.

### 1. Machine

- Prefer an x86-64 instance **with AVX-512** (e.g. AWS c7i / m7i). This is
  where gnark-crypto's KoalaBear baseline uses its assembly path, which is
  what production runs on. Check with `grep -c avx512f /proc/cpuinfo` (must
  be > 0). On other CPUs the run is still valid but the KoalaBear baseline
  is plain Go; say so in the report.
- Nothing else should run on the machine during the benchmark. Do not run
  builds or other tests in parallel.

### 2. Setup

From the repository root:

```bash
cd prover-ray
go version                      # must satisfy the version in go.mod
go install golang.org/x/perf/cmd/benchstat@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

### 3. Sanity check (about 30 s)

```bash
scripts/bench-generic-poseidon.sh small /tmp/genbench-small
```

It must end with a benchstat table. If `TestAgreement` fails, **stop and
report the failure**: it means the implementations compute different
outputs and the timings would be meaningless. Do not edit the code to make it
pass.

### 4. Full run (roughly 1-2 h)

Run it detached so it survives a dropped SSH session:

```bash
nohup scripts/bench-generic-poseidon.sh full /tmp/genbench-full \
  > /tmp/genbench-full.log 2>&1 &
tail -f /tmp/genbench-full.log      # Ctrl-C only stops tail
```

When finished, `/tmp/genbench-full/` contains:

- `raw.txt`: `go test -bench` output, with header comments recording the
  commit, Go version, CPU, AVX-512 presence and core count. This is the
  file to keep; benchstat can be re-run on it anywhere.
- `summary.txt`: `benchstat -col /impl raw.txt`, with one column per
  implementation and ratios vs `baseline`.

### 5. Report

Return both files, plus a short summary that includes:

- the machine (instance type, CPU model, AVX-512 yes/no, core count);
- for each benchmark family (Permutation, Compress, CompressParallel,
  MDHasher, FiatShamir) and each field, the `generic` / `baseline` ratio,
  and for KoalaBear also `current` / `baseline`;
- the allocs/op of `generic` vs `baseline`;
- any result `benchstat` does not mark as significant (shown as `~`).

## What is compared

Benchmark names are `key=value` paths (`field=…/n=…/impl=…`), and `impl` is
one of:

| impl       | meaning |
|------------|---------|
| `baseline` | Non-generic code. KoalaBear: gnark-crypto's permutation and `vortex.CompressPoseidon2`, and the pre-generic MD hasher / transcript frozen in `legacy_test.go` (from commit `3c01857c7`). MamaBear (never had a non-generic hasher): gnark-crypto's permutation under the generic hasher / transcript. |
| `current`  | KoalaBear MD hasher / Fiat-Shamir only: what the prover runs today, the generic hasher / transcript over gnark-crypto's compression. Expected to be on par with `baseline`. |
| `generic`  | Everything generic, permutation included. For KoalaBear the permutation's hot loops branch to concrete `koalabear.Element` code. |
| `generic-vec` | KoalaBear only: `generic` with `koalabear.Vector` batch operations in the round-key, full S-box and internal-matrix loops. |
| `generic-asm` | KoalaBear only: `generic` with the whole permutation delegated to gnark-crypto's (AVX-512 assembly). |
| `generic-asm-static-call` | KoalaBear only: `generic-asm`, compressing with `poseidon2.CompressStaticCall`, which calls gnark-crypto's permutation statically on concrete arrays and so allocates nothing. |

Benchmark families:

- `Permutation`: one width-16 (and, at full scale, width-24) permutation.
- `Compress` / `CompressParallel`: one 8+8 → 8 compression, single-threaded
  and with `GOMAXPROCS` goroutines.
- `MDHasher`: hashing `n` field elements (also reported in B/s).
- `FiatShamir`: a protocol-shaped workload. Each round absorbs `update`
  elements, samples a digest, samples query indices and derives a named
  challenge from a seed.

Sizes come from `GENERIC_BENCH_SCALE` (`small` by default, `medium` or
`full`), defined in `scales` in `bench_test.go`. The script sets the
repetitions and benchtime (small: 3 × 200 ms, medium: 6 × 200 ms, full:
10 × 2 s). `medium` takes a few minutes and is enough for benchstat
confidence intervals.

## Running pieces by hand

```bash
# only the agreement test
go test ./crypto/genericbench/ -run TestAgreement -v

# a single family at full scale
GENERIC_BENCH_SCALE=full go test ./crypto/genericbench/ -run '^$' \
  -bench 'BenchmarkMDHasher/field=koalabear' -benchmem -count 10

# compare two runs (e.g. before/after an optimization of the generic code)
benchstat old/raw.txt new/raw.txt
```

## Rules for editing this package

- `legacy_test.go` is the frozen baseline. Do not modify it.
- Any new implementation must be added to the field suites in
  `bench_test.go`, so that `TestAgreement` checks it against the others.
