# Tracer

> EVM trace generation and arithmetization for ZK proof inputs.

## Overview

The tracer is a Besu plugin that re-executes blocks to produce execution traces — detailed records of every EVM operation performed. These traces are then arithmetized (converted into constraint-compatible columns) for consumption by the prover's ZK circuits.

The tracer serves two functions via JSON-RPC:
1. **Trace counting** — Returns per-module line counts for a block (used by the coordinator for conflation decisions).
2. **Conflated trace generation** — Merges traces across multiple blocks in a batch and writes them to the shared file system.

## Components

| Component | Path | Role |
|-----------|------|------|
| Tracer Plugin | `tracer/plugins/` | Besu plugin registration |
| Arithmetization | `tracer/arithmetization/` | EVM operation → constraint columns |
| Reference Tests | `tracer/reference-tests/` | Ethereum reference test execution |
| Testing Utils | `tracer/testing/` | Test harness and utilities |
| Corset | `corset/` | Rust-based trace expander (prover-side) |
| Tracer Constraints | `tracer-constraints/` | Kotlin modules defining constraint structure |

## Arithmetization Modules

Each EVM operation maps to one or more arithmetization modules:

| Module | Scope |
|--------|-------|
| Hub | Central dispatch and state tracking |
| Rom | Bytecode read-only memory |
| ALU | Arithmetic/logic operations |
| MOD, MUL, Shf | Modular arithmetic, multiplication, shifting |
| WCP | Word comparison |
| MXP | Memory expansion |
| STP | Stipend (gas forwarding) |
| TRM | Address trimming |
| OOB | Out-of-bounds checks |
| Blake2f | BLAKE2f precompile |
| Gas | Gas accounting |

Trace counts per module determine whether a batch can be proven within circuit capacity.

## RPC Endpoints

```
linea_getBlockTracesCountersV2(
    blockNumber:                 string,
    expectedTracesEngineVersion: string
) → {
    tracesEngineVersion: string,
    tracesCounters:      map[string, string]
}
```

```
linea_generateConflatedTracesToFileV2(
    startBlockNumber:            string,
    endBlockNumber:              string,
    expectedTracesEngineVersion: string
) → {
    tracesEngineVersion:      string,
    conflatedTracesFileName:  string  // $first-$last.conflated.v$version.lt
}
```

`linea_generateConflatedTracesToFileV2` is a blocking call. The output file (100-500 MB gzipped) is written to the shared file system at `/shared/traces/conflated/`.

## Proactive Trace Generation

Each tracer instance proactively generates block trace files as blocks arrive via P2P. Instances are configured to handle blocks where `blockNumber mod instanceCount == instanceIndex`, distributing trace work across a static number of instances.

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `tracer/arithmetization/src/test/java/` | JUnit | ModexpEIP7883, TraceRequestParams, LineCountsRequestParams |
| `tracer/reference-tests/` | JUnit | Ethereum reference test execution |
| Nightly/Weekly suites | JUnit | Extended coverage, regression |

## Related Documentation

- [Architecture: Traces API](../architecture-description.md#traces-api)
- [Tech: Tracer Component](../tech/components/tracer.md) — Module system, trace file format, Besu integration
- [Tech: Tracer Constraints](../tech/components/tracer-constraints.md) — Lisp constraint definitions, column types, lookup tables
- [Official docs: Trace Generator](https://docs.linea.build/protocol/architecture/sequencer/traces-generator)
