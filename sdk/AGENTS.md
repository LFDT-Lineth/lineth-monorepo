# AGENTS.md — sdk

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

TypeScript SDK libraries for interacting with the Linea protocol. Three packages: `sdk-core` (framework-agnostic types and utilities), `sdk-ethers` (ethers.js v6 wrapper with typechain-generated contract bindings), and `sdk-viem` (Viem wrapper).

## How to Run

```bash
# Build all SDKs
pnpm -F @consensys/linea-sdk-core run build
pnpm -F @consensys/linea-sdk run build
pnpm -F @consensys/linea-sdk-viem run build

# Test all SDKs
pnpm -F @consensys/linea-sdk-core run test
pnpm -F @consensys/linea-sdk run test
pnpm -F @consensys/linea-sdk-viem run test

# Lint
pnpm -F @consensys/linea-sdk-core run lint
pnpm -F @consensys/linea-sdk run lint
pnpm -F @consensys/linea-sdk-viem run lint
```

## SDK-Specific Conventions

### Package Differences

| Package | Build Tool | Output | Key Dependency |
|---------|-----------|--------|----------------|
| `sdk-core` | tsup | CJS + ESM + DTS | abitype |
| `sdk-ethers` | tsc (+ typechain pre-step) | CJS + DTS | ethers 6.13.7 |
| `sdk-viem` | tsup | CJS + ESM + DTS | viem (peer dep >= 2.22.0) |

### Dependency Chain

```
bridge-ui -> sdk-viem -> sdk-core
postman -> sdk-ethers
```

- `sdk-ethers` requires `pnpm -F @consensys/linea-sdk run build:pre` (typechain) before build
- `sdk-viem` declares `viem` as a peer dependency — consumers must provide it

### Testing

- Framework: Jest 29.7.0 with ts-jest preset
- `sdk-ethers` uses `--forceExit` and `jest-mock-extended`
- Coverage: HTML, LCOV, and text reporters
- Test files: `*.test.ts` pattern

### Directory Structure

```
sdk/
├── sdk-core/       Core types, utilities (framework-agnostic)
├── sdk-ethers/     Ethers.js v6 integration with typechain contract bindings
└── sdk-viem/       Viem integration
```

## Agent Rules (Overrides)

- Changes to `sdk-core` affect both `sdk-ethers` and `sdk-viem` — test downstream packages
- `sdk-ethers` typechain types are generated from ABIs — regenerate with `build:pre` after ABI changes
- Public API changes must follow versioning rules (new versioned exports, deprecate old)
