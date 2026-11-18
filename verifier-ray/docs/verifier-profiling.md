# Verifier R5 Profiling

This workflow measures the RISC-V cost of the verifier by running the normal
`src/main.zig` guest through the shared zkc RISC-V interpreter. It does not use
a benchmark-only Zig guest or a benchmark-specific zkc runner.

## Fixture Model

The executable imports `testdata/generated/verify.zig` as typed comptime data.
Each profiled case is built into the R5 binary with:

```bash
-Dembedded-spec=<N> -Dembedded-input=valid
```

The selected spec and systems remain comptime inputs to `verifier.verify`, while
the selected proof is embedded as static read-only data and passed as a runtime
`Proof` value. This keeps profiling close to the verifier path used by the
R5 smoke target while avoiding proof serialization/parsing cost, which does not
exist yet in verifier-ray.

## What Is Measured

The Makefile's `profile-zkc-case` target owns the R5 build flags, ELF-to-JSON
conversion, zkc input origin, and shared zkc runner invocation. The shared
runner prints the normal interpreter trace; the useful lines are:

- the latest `clock cycle: <N>` line, the final interpreted cycle count;
- `VERIFIER-MARK <phase> <value>` writes, the phase checkpoints;
- the final marker value, currently total Poseidon2 compressions for the case.

Invalid cases are not worth profiling: their cycle counts depend on the
particular failure path and are not useful for comparing verifier phase costs.

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

Profile one generated valid case (`N` indexes the cases in
`testdata/generated/verify.zig`):

```bash
make profile-zkc-case EMBEDDED_INPUT=valid EMBEDDED_SPEC=<N>
```
