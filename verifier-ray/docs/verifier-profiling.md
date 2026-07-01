# Verifier R5 Profiling

This workflow measures the RISC-V cost of the verifier by running the normal
`src/main.zig` guest through the shared zkc RISC-V interpreter. It does not use
a benchmark-only Zig guest or a benchmark-specific zkc runner.

## Fixture Model

The executable imports `testdata/generated/verify.zig` as typed comptime data.
The profiler runs every generated verifier case. For each case it builds the R5
binary with:

```bash
-Dembedded-spec=<N>
```

The selected spec and systems remain comptime inputs to `verifier.verify`, while
the selected valid proof is loaded from `testdata/inputs/verify_case_<N>.bin`
and decoded into a runtime `Proof` value. This keeps profiling close to the
verifier path used by the R5 smoke target while avoiding embedding large proof
values into `verify.zig`.

## R5 Memory Layout

R5 profiling uses one storage strategy for every case. The serialized proof is
loaded into the linker `IN` region, and the decoded `ProofBacking` lives in a
dedicated linker `BACKING` region. This keeps decoded proof storage off the R5
stack for both light and heavy fixtures.

The relevant regions are defined in `linker_script.ld`:

```text
STACK   0x08000000 .. 0x08800000
BACKING 0x08800000 .. 0x18800000
IN      0x18800000 ..
```

`Makefile`'s `IN_ORIGIN` must match the linker `IN` origin because the
ELF-to-JSON step writes serialized input bytes there.

## What Is Measured

For each case, the profiling runner calls the Makefile's `profile-zkc-case`
target. That target owns the R5 build flags, ELF-to-JSON conversion, zkc input
origin, and shared zkc runner invocation.

The shared runner prints the normal interpreter trace. The Go profiling tool
streams that output and keeps only:

- the latest `clock cycle: <N>` line, used as the final interpreted cycle count;
- `VERIFIER-MARK <phase> <value>` writes, used as phase checkpoints;
- the final marker value, currently total Poseidon2 compressions for the case.

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
| 2 | transcript replay done | 0 |
| 3 | vanishing verifier start | 0 |
| 4 | vanishing verifier done | 0 |
| 5 | log-derivative-sum verifier done | Poseidon2 compressions so far |

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

Use the profiler directly to choose a different output path:

```bash
go run bench/verifier_profile/main.go -out=<path>
```
