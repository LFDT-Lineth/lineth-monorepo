# Vanishing Benchmarking

This document explains how verifier-ray measures the RISC-V cost of the vanishing verifier. The generated single-case result lives in `bench/vanishing-benchmark.md`, and the generated comparison report lives in `bench/vanishing-benchmark-comparison.md`.

## Technique

The benchmark builds a tiny R5 guest from `bench/vanishing_main.zig`. That guest imports one generated vanishing fixture, reads selected proof-value arrays from mutable fixture data, replays the transcript with `protocol.replayWithStats`, calls `vanishing.verify`, and exits with code `0` on success or `1` on verifier rejection. `protocol.replayWithStats` uses the same replay implementation as `protocol.replay`, but also returns the number of Poseidon2 Merkle-Damgard compression calls made while generating transcript coins.

The build path is:

```text
Zig benchmark guest
  -> RISC-V ELF
  -> zkc JSON input
  -> zkc RISC-V interpreter
  -> cycle_count output
```

`make bench-vanishing-doc` drives that path. Zig builds the guest for the freestanding RISC-V target, `elf_to_json_gen` turns the ELF loadable sections into zkc input blobs, and `bench/riscv_main_bench.zkc` interprets the guest. The zkc runner increments `cycle_count` once per interpreted RISC-V instruction and exposes the final count as a public output.

## Separate Fixture

The ordinary vanishing test fixture in `testdata/generated/vanishing.zig` is shaped for unit tests. It stores proof values as raw `u32` limbs and the tests convert them into `protocol.RoundMessage`, `field.Element`, and `ext.Ext` values using allocator-backed helpers.

The shared verifier fixture in `testdata/generated/verify.zig` is shaped for measurement. It emits typed `verifier.ProofData`, `protocol.Spec`, and `verifier.Systems` data directly, so the guest does not spend measured RISC-V cycles parsing proof bytes, allocating arrays, or converting raw limbs into field elements. The generated protocol spec and verifier system metadata remain compile-time data; selected proof-value arrays are emitted as mutable fixture data, which keeps ReleaseSmall from treating proof values as compile-time constants.

## Measurement Scope

The reported value is an instruction-count proxy for verifier cost in the R5 environment. It includes the benchmark guest entry path, reads from embedded fixture data, transcript coin replay, `vanishing.verify`, and the guest exit path.

The zkc runner recognizes benchmark marker syscalls emitted by the guest and records three checkpoints: transcript start, transcript end, and vanishing end. The transcript-end marker also carries the Poseidon2 compression count in register `a1`, which the runner exposes as `poseidon2_compression_count`. The generated markdown derives:

- `Transcript replay cycles`: instructions between transcript-start and transcript-end markers.
- `Poseidon2 compressions in transcript`: the number of calls to `poseidon2.compress` made by the transcript hasher while replaying coins.
- `Transcript cycles / Poseidon2 compression`: `Transcript replay cycles` divided by that compression count. This is an aggregate transcript-replay average, not an isolated microbenchmark of only `poseidon2.compress`; it also includes the small runtime counter bookkeeping used to publish the count.
- `Vanishing verify cycles`: instructions between transcript-end and vanishing-end markers.
- `Benchmark overhead / markers`: the remaining instructions, including entry setup, marker ecalls, and exit.

It does not include generating the fixtures, converting raw test fixtures into typed inputs, serializing proof bytes, parsing proof bytes, or copying ELF blobs from the zkc JSON into RAM before the RISC-V loop starts.

One benchmark run measures one selected case:

```bash
make bench-vanishing-doc VANISHING_BENCH_CASE=1
```

The generated catalog currently has 83 honest benchmark cases. `VANISHING_BENCH_CASE` selects which one is compiled into the guest.

## Comparison Runs

Use `bench-vanishing-compare-doc` to benchmark more than one honest case and write a comparison table:

```bash
make bench-vanishing-compare-doc VANISHING_BENCH_CASES=0-10
```

`VANISHING_BENCH_CASES` accepts `all`, an inclusive range, a comma-separated list, or a mix:

```bash
make bench-vanishing-compare-doc VANISHING_BENCH_CASES=all
make bench-vanishing-compare-doc VANISHING_BENCH_CASES=0,2,5-8
```

`make bench-vanishing-all-doc` is a shortcut for the full generated catalog. The comparison target rebuilds the tiny R5 guest once per selected case, stores logs under `zig-out/vanishing-bench/case-<index>.log`, and renders the table from those logs plus `testdata/generated/verify.zig` metadata.
