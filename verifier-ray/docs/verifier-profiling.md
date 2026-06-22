# Verifier R5 Profiling

This workflow measures the RISC-V cost of the verifier by running the normal
`src/main.zig` guest through the shared zkc RISC-V interpreter. It does not use
a benchmark-only Zig guest or a benchmark-specific zkc runner.

## Fixture Model

The executable imports `testdata/generated/verify.zig` as typed comptime data.
The profiler runs every generated verifier case. For each case it builds the R5
binary with:

```bash
-Dembedded-spec=<N> -Dembedded-input=valid
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

The shared runner prints the normal interpreter trace. The Go profiling tool
streams that output and keeps only:

- the latest `clock cycle: <N>` line, used as the final interpreted cycle count;
- `VERIFIER-MARK <phase> <value>` writes, used as phase checkpoints;
- each marker value, currently Poseidon2 compressions so far.

The full zkc trace is not written to disk by the report generator. Only the
compact CSV report is stored. Invalid cases are intentionally excluded: their
cycle counts depend on the particular failure path and are not useful for
comparing verifier phase costs.

## Profiling Markers

`src/profiling.zig` exposes build-time-gated helpers. In normal builds these are
compiled away. The profiling target always builds with:

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

The Go parser associates each marker with the most recent `clock cycle` printed
by the shared runner. These numbers are useful for attribution, but the marker
syscalls themselves add overhead.

## Commands

Profile every generated valid case:

```bash
make profile-zkc
```

By default the report is written to:

```text
bench/verifier-profile.csv
```

Useful variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `PROFILE_OUT` | `bench/verifier-profile.csv` | CSV output path |
