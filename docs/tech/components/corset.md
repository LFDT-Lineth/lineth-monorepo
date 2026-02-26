# Corset

> Rust-based constraint compiler that translates the Linea zkEVM constraint DSL into forms usable by the ZK prover.

> **See also:** [Tracer Constraints](./tracer-constraints.md) — the Lisp/Corset constraint definitions that corset compiles

## Overview

Corset is the constraint compilation toolchain for Linea's zkEVM. It:

- Parses arithmetic constraint definitions written in the Corset DSL (a Lisp-like language)
- Compiles them into binary formats for the prover
- Generates Java trace interfaces consumed by the tracer
- Exposes a C-compatible shared library (`cdylib`/`staticlib`) for FFI integration with Go (`cgo-corset`)

**Repository**: `corset/` — Rust, `cargo` build system

## Role in the Stack

```
tracer-constraints/          corset/                    prover/
 (.lisp / .zkasm files)  ──▶  (Rust compiler)  ──▶  (Go/gnark circuits)
                              │
                              ▼
                         tracer/ (Java)
                          (generated Trace.java interfaces)
```

## Build

```bash
# Standard debug build
cargo build

# Release build (used in CI/production)
cargo build --release

# Run tests
cargo test

# Build CLI only
cargo build --features cli

# Build shared library for Go FFI
cargo build --features exporters
```

## Key Features

| Feature flag | Purpose |
|-------------|---------|
| `cli` | Command-line interface (default on) |
| `exporters` | Handlebars template exporters for code generation |
| `inspector` | TUI inspector for interactive constraint exploration |

## Key Dependencies

| Dependency | Purpose |
|------------|---------|
| `ark-bls12-377`, `ark-ff` | Arkworks cryptographic field arithmetic |
| `cbindgen` | C header generation for FFI |
| `pest` | PEG grammar parser for the Corset DSL |
| `clap 4` | CLI argument parsing |
| `rayon` | Parallel constraint processing |

## Outputs

- **`zkevm.bin`** — compiled constraint binary consumed by the prover
- **`Trace.java`** — generated Java interfaces consumed by the tracer
- **`libcorset.so` / `libcorset.a`** — shared/static library for Go FFI

## Version Requirements

- Rust 1.70.0+
- Rust edition 2021
