# Verifier R5 Profiling

This workflow measures the RISC-V cost of the verifier by running the normal
`src/main.zig` guest through the shared zkc RISC-V interpreter. It does not use a
benchmark-only Zig guest or a custom zkc interpreter.

## Fixture Model

The executable imports `testdata/generated/verify.zig` as typed comptime data.
Each profiling run selects one generated verifier case with:

```bash
-Dembedded-spec=<N> -Dembedded-input=<valid|invalid>
```

The selected spec and systems remain comptime inputs to `verifier.verify`, while
the selected proof is embedded as static read-only data and passed as a runtime
`ProofData` value. This keeps profiling close to the verifier path used by the
R5 smoke target while avoiding proof serialization/parsing cost, which does not
exist yet in verifier-ray.

## What Is Measured

The profiling runner builds the R5 binary, converts it to zkc JSON, then runs:

```bash
zkc exec zig-out/bin/verifier-ray.json ../arithmetization/src/main/riscv/main.zkc
```

The command is intentionally run without `-q`. The shared interpreter prints a
`clock cycle: N` line for every executed instruction and prints each decoded
instruction mnemonic. The Go profiling tool streams that output line-by-line and
extracts:

- final interpreted cycle count;
- verifier phase marker cycles;
- Poseidon2 compression count from the verifier profiling counter;
- top decoded RISC-V instruction mnemonics.

The full zkc trace is not stored. Only the compact markdown report is written.

## Profiling Markers

`src/profiling.zig` exposes build-time-gated helpers. In normal builds these are
compiled away. For profiled R5 runs the Make target builds with:

```bash
-Dverifier-profiling=true -Dr5-marks=true
```

The verifier emits marker lines through the RISC-V `write` syscall:

```text
VERIFIER-MARK	<phase>	<value>
```

The current marker phases are:

| Phase | Meaning | Value |
| ---: | --- | --- |
| 1 | verifier start | 0 |
| 2 | transcript replay done | Poseidon2 compressions so far |
| 3 | vanishing verifier start | Poseidon2 compressions so far |
| 4 | vanishing verifier done | Poseidon2 compressions so far |
| 5 | verifier done | Poseidon2 compressions so far |

Phase cycle deltas are computed from the interpreter cycle number seen when the
marker write is printed. These numbers are useful for attribution, but the marker
syscalls themselves add overhead. Use `PROFILE_MODE=raw` when you want the
smallest total-cycle number without marker overhead.

## Commands

Profile one case:

```bash
make profile-zkc PROFILE_CASES=6
```

Profile a range:

```bash
make profile-zkc PROFILE_CASES=0-10
```

Profile multiple selectors:

```bash
make profile-zkc PROFILE_CASES=0,6,62-68
```

Profile all generated verifier cases:

```bash
make profile-zkc-all
```

By default the report is written to:

```text
bench/verifier-profile.md
```

Useful variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `PROFILE_CASES` | `0` | `all`, `N`, `A-B`, or comma-separated selectors |
| `PROFILE_INPUT` | `valid` | embedded input fixture kind |
| `PROFILE_MODE` | `profiled` | `raw`, `profiled`, or `both` |
| `PROFILE_OUT` | `bench/verifier-profile.md` | markdown output path |
| `PROFILE_TOP_INSTRUCTIONS` | `10` | number of instruction histogram entries |
