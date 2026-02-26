# Tracer

> Java-based Besu plugin that generates EVM execution traces for ZK proof generation.

> **Diagram:** [Tracer Architecture](../diagrams/tracer-architecture.mmd) (Mermaid source)

## Overview

The Tracer is a Besu plugin that:
- Hooks into EVM execution via `OperationTracer` interface
- Captures execution data into structured trace matrices
- Produces binary `.lt` files for the prover
- Validates traces against constraint system

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                              TRACER                                    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                         ZkTracer                                 │  │
│  │                   (Main Tracer Implementation)                   │  │
│  │                                                                  │  │
│  │  ┌──────────────┐                                                │  │
│  │  │  Besu EVM    │                                                │  │
│  │  │  Execution   │                                                │  │
│  │  └──────┬───────┘                                                │  │
│  │         │                                                        │  │
│  │         ▼ tracePreExecution / tracePostExecution                 │  │
│  │  ┌──────────────────────────────────────────────────────────┐    │  │
│  │  │                         Hub                              │    │  │
│  │  │               (Central Coordination Module)              │    │  │
│  │  │                                                          │    │  │
│  │  │   Dispatches to specialized modules based on opcode      │    │  │
│  │  └────────────────────────────┬─────────────────────────────┘    │  │
│  │                               │                                  │  │
│  │  ┌────────────────────────────▼─────────────────────────────┐    │  │
│  │  │                       Modules                            │    │  │
│  │  │                                                          │    │  │
│  │  │ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐          │    │  │
│  │  │ │ ADD │ │ MUL │ │ MOD │ │ MMU │ │ ROM │ │ RLP │          │    │  │
│  │  │ └─────┘ └─────┘ └─────┘ └─────┘ └─────┘ └─────┘          │    │  │
│  │  │                                                          │    │  │
│  │  │ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐          │    │  │
│  │  │ │ EXP │ │ SHF │ │ GAS │ │ TRM │ │ EC  │ │ BLS │          │    │  │
│  │  │ └─────┘ └─────┘ └─────┘ └─────┘ └─────┘ └─────┘          │    │  │
│  │  │                                                          │    │  │
│  │  │ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │    │  │
│  │  │ │ BLOCKDATA│ │ TXNDATA  │ │ LOGINFO  │ │ SHAKIRA  │      │    │  │
│  │  │ └──────────┘ └──────────┘ └──────────┘ └──────────┘      │    │  │
│  │  │                                                          │    │  │
│  │  └──────────────────────────────────────────────────────────┘    │  │
│  │                               │                                  │  │
│  │                               ▼ commit()                         │  │
│  │  ┌──────────────────────────────────────────────────────────┐    │  │
│  │  │                    Trace Object                          │    │  │
│  │  │                                                          │    │  │
│  │  │   Column data from all modules, aligned by register ID   │    │  │
│  │  └────────────────────────────┬─────────────────────────────┘    │  │
│  │                               │                                  │  │
│  │                               ▼ writeToFile()                    │  │
│  │  ┌──────────────────────────────────────────────────────────┐    │  │
│  │  │                    LtFile (Binary)                       │    │  │
│  │  │                                                          │    │  │
│  │  │   {start}-{end}.conflated.{tracerV}.{besuV}.lt.gz        │    │  │
│  │  └──────────────────────────────────────────────────────────┘    │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
tracer/
├── arithmetization/           # Core trace generation
│   └── src/main/java/net/consensys/linea/
│       ├── zktracer/
│       │   ├── ZkTracer.java           # Main tracer
│       │   ├── ConflationAwareOperationTracer.java
│       │   ├── module/
│       │   │   ├── hub/                # Central Hub module
│       │   │   │   ├── Hub.java        # Coordinator
│       │   │   │   ├── fragment/       # Data fragments
│       │   │   │   └── section/        # Processing sections
│       │   │   ├── add/                # ADD module
│       │   │   ├── mul/                # MUL module
│       │   │   ├── mod/                # MOD module
│       │   │   ├── mmu/                # Memory Management
│       │   │   ├── mmio/               # Memory-mapped I/O
│       │   │   ├── rom/                # Read-only Memory
│       │   │   ├── rlptxn/             # RLP encoding
│       │   │   ├── ecdata/             # Elliptic curve
│       │   │   ├── blsdata/            # BLS operations
│       │   │   └── ...                 # Other modules
│       │   ├── container/
│       │   │   └── module/
│       │   │       └── Module.java     # Module interface
│       │   └── lt/                     # Trace file format
│       │       ├── LtFile.java         # Binary format interface
│       │       ├── LtFileV1.java       # Version 1 format
│       │       └── LtFileV2.java       # Version 2 format
│       ├── tracewriter/
│       │   └── TraceWriter.java        # File output
│       └── corset/
│           └── CorsetValidator.java    # Constraint validation
│
├── plugins/                   # Besu plugin integration
│   └── src/main/java/net/consensys/linea/plugins/
│       ├── rpc/
│       │   ├── tracegeneration/
│       │   │   └── TracesEndpointServicePlugin.java
│       │   ├── capture/
│       │   │   └── CaptureEndpointServicePlugin.java
│       │   ├── linecounts/
│       │   │   └── LineCountsEndpointServicePlugin.java
│       │   └── batchlinecount/
│       │       └── ConflatedLineCountsEndpointServicePlugin.java
│       └── readiness/
│           └── TracerReadinessPlugin.java
│
├── reference-tests/           # Reference test suite
└── testing/                   # Test utilities
```

## Module System

### Module Interface

```java
public interface Module {
    // Called at block boundaries
    void traceStartBlock(ProcessableBlockHeader blockHeader);
    void traceEndBlock(ProcessableBlockHeader blockHeader);
    
    // Called at transaction boundaries
    void traceStartTx(WorldView worldView, Transaction tx);
    void traceEndTx(WorldView worldView, Transaction tx, boolean isReverted);
    
    // Called at call frame boundaries
    void traceContextEnter(MessageFrame frame);
    void traceContextExit(MessageFrame frame);
    
    // Write trace data to output
    void commit(Trace trace);
    
    // Report line count (for conflation limits)
    int lineCount();
    
    // Define column structure
    ColumnHeader[] columnHeaders();
}
```

### Module Categories

| Category | Modules | Purpose |
|----------|---------|---------|
| **Arithmetic** | ADD, MUL, MOD, EXP, SHF | ALU operations |
| **Memory** | MMU, MMIO | Memory management |
| **Storage** | Hub (storage sections) | State operations |
| **Transaction** | TXN_DATA, RLP_TXN | Tx processing |
| **Block** | BLOCK_DATA, BLOCK_HASH | Block metadata |
| **Logs** | LOG_DATA, LOG_INFO | Event logging |
| **Precompiles** | EC_DATA, BLS_DATA, SHAKIRA | Precompile ops |
| **Bytecode** | ROM, ROM_LEX | Code execution |

### Hub Module

The Hub is the central coordination point:

```java
public class Hub implements Module {
    // 40+ sub-modules
    private final Add addModule;
    private final Mul mulModule;
    private final Mod modModule;
    private final Mmu mmuModule;
    // ...
    
    @Override
    public void tracePreExecution(MessageFrame frame) {
        OpCode opCode = frame.getCurrentOperation();
        
        // Dispatch to appropriate module
        switch (opCode) {
            case ADD, SUB -> addModule.trace(frame);
            case MUL, MULMOD -> mulModule.trace(frame);
            case DIV, MOD, SDIV, SMOD -> modModule.trace(frame);
            // ...
        }
    }
}
```

## Trace File Format

### LtFile Structure (v2)

```
┌────────────────────────────────────────────────┐
│                   Header                       │
│  - Version: 2                                  │
│  - Metadata (block range, chain config)        │
│  - Module count                                │
├────────────────────────────────────────────────┤
│              Module Headers                    │
│  For each module:                              │
│  - Module ID                                   │
│  - Column count                                │
│  - Column definitions (name, type, size)       │
├────────────────────────────────────────────────┤
│               Column Data                      │
│  For each module:                              │
│  - Row count                                   │
│  - Encoded column values                       │
│  - Variable-length integer encoding            │
├────────────────────────────────────────────────┤
│                   Heap                         │
│  Shared byte data referenced by columns        │
└────────────────────────────────────────────────┘
```

### File Naming

```
{startBlock}-{endBlock}.conflated.{tracerVersion}.{besuVersion}.lt.gz

Example: 1-100.conflated.v0.2.0.v24.0.0.lt.gz
```

## Besu Integration

### Plugin Registration

```java
@AutoService(BesuPlugin.class)
public class TracesEndpointServicePlugin 
    extends AbstractLineaOptionsPlugin {
    
    @Override
    public void register(BesuContext context) {
        // Register RPC endpoint
        rpcEndpointService.registerRPCEndpoint(
            "linea",
            "getTracesCountersByBlockNumberV2",
            this::getTracesCounters
        );
    }
}
```

### Tracer Hooks

```java
public class ZkTracer implements ConflationAwareOperationTracer {
    
    @Override
    public void tracePreExecution(MessageFrame frame) {
        hub.tracePreExecution(frame);
    }
    
    @Override
    public void tracePostExecution(
        MessageFrame frame, 
        Operation.OperationResult result
    ) {
        hub.tracePostExecution(frame, result);
    }
    
    @Override
    public void traceStartBlock(
        BlockHeader header,
        BlockBody body
    ) {
        for (Module module : modules) {
            module.traceStartBlock(header);
        }
    }
    
    @Override
    public void traceEndConflation() {
        // Commit all modules and write file
        Trace trace = new Trace();
        for (Module module : modules) {
            module.commit(trace);
        }
        LtFile.write(trace, outputPath);
    }
}
```

## tracer-constraints Integration

### Build-Time Code Generation

```
tracer-constraints/*.lisp  →  go-corset generate  →  Trace.java
                                                      TraceOsaka.java
```

### Constraint Validation

```java
public class CorsetValidator {
    public ValidationResult validate(Path traceFile) {
        ProcessBuilder pb = new ProcessBuilder(
            "go-corset", "check",
            "--bin", zkevmBinPath,
            traceFile.toString()
        );
        // Run validation
        // Return pass/fail with details
    }
}
```

## Configuration

### Traces Limits (traces-limits-v2.toml)

```toml
# Module line limits for conflation
[limits]
hub = 4194304          # 2^22
add = 262144           # 2^18
mul = 262144
mod = 262144
mmu = 1048576          # 2^20
mmio = 2097152         # 2^21
rom = 2097152
rlptxn = 524288
txndata = 131072
blockdata = 16384
ecdata = 65536
blsdata = 65536
```

## Building

```bash
# Build tracer JAR
./gradlew :tracer:arithmetization:build

# Generate code from constraints
./gradlew :tracer:arithmetization:buildAllTracers

# Run tests
./gradlew :tracer:test

# Build with constraints binary
./gradlew :tracer:arithmetization:buildZkevmBins
```

## RPC Endpoints

### linea_getTracesCountersByBlockNumberV2

Returns trace line counts for a block.

```json
{
  "jsonrpc": "2.0",
  "method": "linea_getTracesCountersByBlockNumberV2",
  "params": ["0x1"],
  "id": 1
}
```

Response:

```json
{
  "result": {
    "blockNumber": 1,
    "tracesCounters": {
      "hub": 1234,
      "add": 567,
      "mul": 234,
      ...
    }
  }
}
```

## Pipeline Position

```
EVM Execution  →  Tracer  →  .lt Files  →  Prover  →  Proofs
     │               │            │            │
     │               │            │            │
   Besu          Modules      Binary      Go/gnark
                              Format
```

## Dependencies

- **Besu**: Plugin host, EVM execution
- **tracer-constraints**: Constraint definitions (Lisp)
- **go-corset**: Code generation and validation
- **Coordinator**: Triggers trace generation via RPC
- **Prover**: Consumes trace files

## Performance Considerations

- **Memory**: Trace data accumulates during conflation
- **Disk I/O**: Large trace files (GBs for big batches)
- **CPU**: Matrix operations during commit
- **Line Limits**: Conflation triggered by module limits

## Related Documentation

- [Feature: Tracer](../../features/tracer.md) — Business-level overview, trace modules, arithmetization, and test coverage
