# Tracer Constraints

> Lisp-based constraint definitions for Linea's zkEVM proof system.

> **Diagrams:** [Constraint System](../diagrams/constraint-system.mmd) | [Module Interactions](../diagrams/module-interactions.mmd)

## Overview

The tracer-constraints directory contains:
- ZK circuit constraint definitions in Lisp (Corset DSL)
- zkasm files for specific operations
- Module-specific constraint logic
- Lookup tables between modules

These constraints define what the ZK prover must verify.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                         CONSTRAINT SYSTEM                              │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                        Constraint Files                          │  │
│  │                         (.lisp / .zkasm)                         │  │
│  │                                                                  │  │
│  │  Define columns, constraints, lookups for each module            │  │
│  │                                                                  │  │
│  └─────────────────────────────────┬────────────────────────────────┘  │
│                                    │                                   │
│                                    ▼                                   │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                          go-corset                               │  │
│  │                                                                  │  │
│  │ ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐    │  │
│  │ │    Compile      │  │    Generate     │  │     Check       │    │  │
│  │ │                 │  │                 │  │                 │    │  │
│  │ │  .lisp → .bin   │  │  .bin → .java   │  │  Validate trace │    │  │
│  │ │                 │  │                 │  │  against .bin   │    │  │
│  │ └─────────────────┘  └─────────────────┘  └─────────────────┘    │  │
│  │                                                                  │  │
│  └─────────────────────────────────┬────────────────────────────────┘  │
│                                    │                                   │
│        ┌───────────────────────────┼───────────────────────────┐       │
│        │                           │                           │       │
│        ▼                           ▼                           ▼       │
│  ┌─────────────────┐  ┌─────────────────────────┐  ┌─────────────────┐ │
│  │  zkevm.bin      │  │     Trace.java          │  │   Validation    │ │
│  │                 │  │     TraceOsaka.java     │  │                 │ │
│  │  Binary         │  │                         │  │  Trace files    │ │
│  │  constraints    │  │  Java interfaces for    │  │  checked        │ │
│  │  for prover     │  │  tracer module          │  │  against        │ │
│  │                 │  │  generation             │  │  constraints    │ │
│  └─────────────────┘  └─────────────────────────┘  └─────────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
tracer-constraints/
├── hub/                    # Central Hub module
│   └── osaka/
│       ├── columns/        # Column definitions
│       │   ├── account.lisp
│       │   ├── context.lisp
│       │   ├── scenario.lisp
│       │   ├── stack.lisp
│       │   ├── storage.lisp
│       │   └── transaction.lisp
│       ├── constants.lisp
│       ├── constraints/    # Constraint logic
│       │   ├── account-rows/
│       │   ├── consistency/
│       │   ├── instruction-handling/  # Per-opcode constraints
│       │   ├── stack-patterns/
│       │   ├── tx_init/
│       │   ├── tx_finl/
│       │   └── system/
│       └── lookups/        # Cross-module lookups
│
├── alu/                    # Arithmetic Logic Unit
│   ├── add.zkasm
│   ├── mul.zkasm
│   ├── mod.zkasm
│   └── ext/
│       ├── columns.lisp
│       └── constraints.lisp
│
├── mmu/                    # Memory Management Unit
│   ├── columns.lisp
│   ├── constants.lisp
│   ├── constraints.lisp
│   ├── instructions/
│   └── lookups/
│
├── mmio/                   # Memory-Mapped I/O
│   ├── columns.lisp
│   ├── constraints.lisp
│   ├── patterns.lisp
│   └── lookups/
│
├── txndata/                # Transaction Data
│   └── [58 .lisp files]
│
├── rlptxn/                 # RLP Transaction Encoding
│   ├── columns/
│   ├── constraints/
│   └── lookups/
│
├── blockdata/              # Block Metadata
│   └── [22 .lisp files]
│
├── ecdata/                 # Elliptic Curve Operations
│   ├── columns.lisp
│   ├── constraints.lisp
│   └── lookups/
│
├── blsdata/                # BLS12-381 Operations
│   └── [33 .lisp files]
│
├── shakiradata/            # SHA256/RIPEMD160
│   └── [6 .lisp files]
│
├── rom/                    # Read-Only Memory (Bytecode)
│   └── [3 .lisp files]
│
├── romlex/                 # ROM Lexer
│   └── [5 .lisp files]
│
├── logdata/                # Event Log Data
├── loginfo/                # Event Log Info
├── blockhash/              # Block Hash Operations
├── rlpaddr/                # RLP Address Encoding
├── rlptxrcpt/              # RLP Transaction Receipt
│
├── exp/                    # Exponentiation
│   └── exp.zkasm
├── gas/                    # Gas Calculations
│   └── gas.zkasm
├── wcp/                    # Word Comparisons
│   └── wcp.zkasm
├── shf/                    # Shift Operations
│   └── shf.zkasm
├── trm/                    # Trimming
│   └── trm.zkasm
├── stp/                    # Storage Operations
│   └── stp.zkasm
├── oob/                    # Out-of-Bounds
│   └── [3 .zkasm files]
├── euc/                    # Extended Euclidean
│   └── euc.zkasm
│
├── reftables/              # Reference Tables
│   ├── bls_reftable.lisp
│   └── inst_decoder.lisp
│
├── library/                # Shared Utilities
│   └── [2 .lisp files]
│
├── constants/              # Global Constants
│   └── constants.lisp
│
├── util/                   # Utility Functions
│   └── [19 .zkasm files]
│
└── Makefile
```

## Module Structure

### Column Definitions

Columns define the trace matrix structure:

```lisp
;; hub/osaka/columns/stack.lisp

(defcolumns
  ;; Stack item columns
  (STACK_ITEM_HEIGHT_1 :i32)
  (STACK_ITEM_HEIGHT_2 :i32)
  (STACK_ITEM_HEIGHT_3 :i32)
  (STACK_ITEM_HEIGHT_4 :i32)
  
  ;; Stack values (256-bit as 16-byte limbs)
  (STACK_ITEM_VALUE_HI_1 :i128)
  (STACK_ITEM_VALUE_LO_1 :i128)
  (STACK_ITEM_VALUE_HI_2 :i128)
  (STACK_ITEM_VALUE_LO_2 :i128)
  ;; ...
  
  ;; Stack operation flags
  (STACK_ITEM_STAMP :i32)
  (STACK_ITEM_POP :bool)
  (STACK_ITEM_PUSH :bool)
)
```

### Constraint Definitions

Constraints enforce correctness:

```lisp
;; hub/osaka/constraints/instruction-handling/add.lisp

(defconstraint add-instruction ()
  ;; ADD opcode: result = a + b (mod 2^256)
  (if (instruction-is-add)
    (begin
      ;; Pop two values from stack
      (= STACK_ITEM_POP_1 true)
      (= STACK_ITEM_POP_2 true)
      
      ;; Push result
      (= STACK_ITEM_PUSH_1 true)
      
      ;; Result is sum (with overflow handling)
      (= STACK_ITEM_VALUE_RESULT
         (+ STACK_ITEM_VALUE_1 STACK_ITEM_VALUE_2))
      
      ;; Lookup to ADD module for detailed arithmetic
      (lookup hub_into_add
        STACK_ITEM_VALUE_HI_1
        STACK_ITEM_VALUE_LO_1
        STACK_ITEM_VALUE_HI_2
        STACK_ITEM_VALUE_LO_2
        STACK_ITEM_VALUE_HI_RESULT
        STACK_ITEM_VALUE_LO_RESULT)
    )
  )
)
```

### Lookup Tables

Cross-module data validation:

```lisp
;; hub/osaka/lookups/hub_into_add.lisp

(deflookup hub_into_add
  ;; Source columns (from hub)
  (hub.STACK_ITEM_VALUE_HI_1
   hub.STACK_ITEM_VALUE_LO_1
   hub.STACK_ITEM_VALUE_HI_2
   hub.STACK_ITEM_VALUE_LO_2
   hub.STACK_ITEM_VALUE_HI_RESULT
   hub.STACK_ITEM_VALUE_LO_RESULT)
  
  ;; Target columns (in add module)
  (add.ARG_1_HI
   add.ARG_1_LO
   add.ARG_2_HI
   add.ARG_2_LO
   add.RES_HI
   add.RES_LO)
)
```

## zkasm Files

Assembly-like constraint definitions:

```zkasm
;; alu/add.zkasm

VAR GLOBAL arg1_hi
VAR GLOBAL arg1_lo
VAR GLOBAL arg2_hi
VAR GLOBAL arg2_lo
VAR GLOBAL res_hi
VAR GLOBAL res_lo
VAR GLOBAL carry

start:
    ; Load arguments
    ${getArg1Hi()} => arg1_hi
    ${getArg1Lo()} => arg1_lo
    ${getArg2Hi()} => arg2_hi
    ${getArg2Lo()} => arg2_lo
    
    ; Compute low part with carry
    arg1_lo + arg2_lo => res_lo
    res_lo >> 128 => carry
    res_lo & 0xFFFF...FFFF => res_lo
    
    ; Compute high part with carry
    arg1_hi + arg2_hi + carry => res_hi
    res_hi & 0xFFFF...FFFF => res_hi
    
    ; Output
    res_hi => ${setResHi()}
    res_lo => ${setResLo()}
```

## Key Modules

### Hub (Central Coordinator)

The Hub orchestrates all other modules:

- Instruction decoding
- Stack operations
- Context management
- Transaction boundaries
- Dispatch to specialized modules

### ALU (Arithmetic Logic Unit)

Handles arithmetic operations:

- ADD, SUB (addition, subtraction)
- MUL, MULMOD (multiplication)
- DIV, MOD, SDIV, SMOD (division, modulo)
- EXP (exponentiation)
- SHL, SHR, SAR (shifts)

### MMU/MMIO (Memory)

Memory management:

- MLOAD, MSTORE (memory load/store)
- MSTORE8 (byte storage)
- Memory expansion
- Copy operations

### Transaction Data

Transaction processing:

- RLP encoding/decoding
- Gas calculations
- Nonce management
- Value transfers

### Precompiles

Special contract handling:

- ECRECOVER, ECADD, ECMUL, ECPAIRING
- SHA256, RIPEMD160
- IDENTITY
- MODEXP
- BLS12-381 operations

## Build Process

```bash
cd tracer-constraints

# Compile constraints to binary
make build
# Output: zkevm_osaka.bin

# In tracer build:
./gradlew :tracer:arithmetization:buildZkevmBins
./gradlew :tracer:arithmetization:buildAllTracers
# Generates: Trace.java, TraceOsaka.java
```

## Validation

```bash
# Validate a trace file
go-corset check \
  --bin zkevm_osaka.bin \
  path/to/trace.lt

# Generate coverage report
go-corset coverage \
  --bin zkevm_osaka.bin \
  --trace path/to/trace.lt \
  --output coverage.html
```

## Module Interactions

```
┌────────────────────────────────────────────────────────────────────────┐
│                       MODULE INTERACTIONS                              │
│                                                                        │
│                            ┌─────────────┐                             │
│                            │     HUB     │                             │
│                            │  (central)  │                             │
│                            └──────┬──────┘                             │
│                                   │                                    │
│   ┌───────────────────────────────┼───────────────────────────────┐    │
│   │           │           │       │       │           │           │    │
│   ▼           ▼           ▼       ▼       ▼           ▼           ▼    │
│ ┌─────┐   ┌─────┐    ┌─────┐  ┌─────┐  ┌─────┐   ┌─────┐   ┌─────┐     │
│ │ ALU │   │ MMU │    │ ROM │  │ TXN │  │ BLK │   │ LOG │   │ EC  │     │
│ │add  │   │     │    │     │  │DATA │  │DATA │   │DATA │   │DATA │     │
│ │mul  │   │     │    │     │  │     │  │     │   │     │   │     │     │
│ │mod  │◄─►│     │◄──►│     │◄─┤     │◄─┤     │◄──┤     │◄──┤     │     │
│ │exp  │   │     │    │     │  │     │  │     │   │     │   │     │     │
│ │shf  │   │     │    │     │  │     │  │     │   │     │   │     │     │
│ └──┬──┘   └──┬──┘    └─────┘  └─────┘  └─────┘   └─────┘   └─────┘     │
│    │         │                                                         │
│    │         ▼                                                         │
│    │      ┌─────┐                                                      │
│    │      │MMIO │                                                      │
│    │      └──┬──┘                                                      │
│    │         │                                                         │
│    │         ▼                                                         │
│    │   ┌─────────────────────────────────────────────────┐             │
│    └──►│               LOOKUPS                           │             │
│        │  hub_into_add, hub_into_mmu, mmio_into_rom, etc │             │
│        └─────────────────────────────────────────────────┘             │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Relationship to Other Components

| Component | Relationship |
|-----------|--------------|
| **Tracer** | Generates traces matching column definitions |
| **Prover** | Uses compiled constraints for proof generation |
| **go-corset** | Compiles constraints, generates code, validates |
| **Specification** | Formal spec that constraints implement |
