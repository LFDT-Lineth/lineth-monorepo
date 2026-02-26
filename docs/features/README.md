# Linea Feature Documentation

> Last updated: 2026-02-19

This directory contains per-feature documentation for all major Linea system components. Each document covers architecture, key contracts/services, interfaces, test coverage, and configuration.

For high-level system architecture see [`docs/architecture-description.md`](../architecture-description.md).
For technical deep dives by component see [`docs/tech/`](../tech/README.md).
For contract-level workflow diagrams see [`contracts/docs/workflows/`](../../contracts/docs/workflows/).

## Component Matrix

| Component | Language | Package Path | Runtime |
|-----------|----------|-------------|---------|
| Smart Contracts | Solidity | `contracts/` | Hardhat + Foundry |
| Coordinator | Kotlin | `coordinator/` | JVM / Vert.x |
| Prover | Go + Rust (Corset) | `prover/` | gnark circuits |
| Sequencer | Java/Kotlin | `besu-plugins/linea-sequencer/` | Besu plugin |
| Tracer | Java/Kotlin | `tracer/` | Besu plugin |
| State Recovery | Kotlin | `besu-plugins/state-recovery/` | Besu plugin |
| Transaction Exclusion API | Kotlin | `transaction-exclusion-api/` | Vert.x |
| Postman | TypeScript | `postman/` | Node.js |
| Yield Automation Service | TypeScript | `native-yield-operations/automation-service/` | Node.js |
| Lido Governance Monitor | TypeScript | `native-yield-operations/lido-governance-monitor/` | Node.js + Prisma — see [Yield Management](yield-management.md) |
| Operations CLI | TypeScript | `operations/` | oclif |
| Integrity Verifier | TypeScript | `contracts/integrity-verifier/` | Node.js + Browser |
| SDK | TypeScript | `sdk/` | Node.js |
| Bridge UI | TypeScript | `bridge-ui/` | Next.js |
| Corset | Rust | `corset/` | CLI + shared library |

## Feature Index

### Core Protocol

| Feature | Document | Key Contracts / Services |
|---------|----------|--------------------------|
| [Rollup](rollup.md) | Data submission, shnarf chaining, ZK finalization | `LineaRollup.sol`, `Validium.sol`, `PlonkVerifier*.sol` |
| [Messaging](messaging.md) | L1/L2 message send, anchor, claim | `L1MessageService.sol`, `L2MessageService.sol`, Postman |
| [Token Bridge](token-bridge.md) | ERC20 bridging, BridgedToken | `TokenBridge.sol`, `BridgedToken.sol` |

### Security, Governance, and Access Control

| Feature | Document |
|---------|----------|
| [Pause, Security, and Governance](pause-and-security.md) | Type-based pausing, rate limiting, access control, Timelock, upgrades, security council |

### Yield

| Feature | Document |
|---------|----------|
| [Yield Management](yield-management.md) | YieldManager, Lido integration, validator proofs, automation service, governance monitor |

### Infrastructure

| Feature | Document |
|---------|----------|
| [Coordinator](coordinator.md) | Conflation, blob orchestration, aggregation, gas pricing |
| [Prover](prover.md) | ZK circuits, execution/compression/aggregation proofs |
| [Sequencer](sequencer.md) | Transaction selection, trace limits, gas estimation, bundles |
| [Tracer](tracer.md) | EVM tracing, arithmetization, constraint system |
| [Postman](postman.md) | Automated message claiming, sponsorship model, retry logic |
| [State Recovery](state-recovery.md) | State recovery from L1 blobs |

### Specialized Features

| Feature | Document |
|---------|----------|
| [Forced Transactions](forced-transactions.md) | Force-include transactions via L1 events |
| [Transaction Exclusion](transaction-exclusion.md) | Rejected transaction tracking API |
| [Token Generation Event](token-generation-event.md) | Linea token (L1/L2), airdrops |

### Operational and Tooling

| Feature | Document |
|---------|----------|
| [Operational](operational.md) | Revenue vault, token burner, DEX adapters, uptime feed, operations CLI |
| [Integrity Verifier](integrity-verifier.md) | Contract bytecode, ABI, and on-chain state verification |
| [SDK](sdk.md) | Core, ethers, viem SDKs |
| [Bridge UI](bridge-ui.md) | Next.js bridge interface |
