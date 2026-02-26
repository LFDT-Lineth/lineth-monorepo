# SDK

> TypeScript SDKs for programmatic interaction with Linea messaging and bridging.

## Overview

The Linea SDK is split into three packages providing different integration paths:

| Package | npm | Dependency |
|---------|-----|------------|
| `@consensys/linea-sdk-core` | `sdk/sdk-core/` | None (pure types + utilities) |
| `@consensys/linea-sdk` | `sdk/sdk-ethers/` | ethers |
| `@consensys/linea-sdk-viem` | `sdk/sdk-viem/` | viem, `@consensys/linea-sdk-core` |

## SDK Core (`sdk/sdk-core/`)

Provides framework-agnostic types, utilities, and the sparse Merkle tree implementation.

### Exports

| Export | Description |
|--------|-------------|
| `SparseMerkleTree` | SMT implementation for Merkle proof construction |
| `parseBlockExtraData` | Parse Linea gas pricing from block `extraData` |
| `formatMessageStatus` | Human-readable message status |
| `getContractsAddressesByChainId` | Contract address lookup by chain ID |
| `isLineaMainnet`, `isLineaSepolia`, `isMainnet`, `isSepolia` | Chain identification helpers |

### Client Interfaces

**L1PublicClient**:
- `getL2ToL1MessageStatus(messageHash)` — Query L2→L1 message finalization status
- `getMessageProof(messageHash)` — Retrieve Merkle proof for L1 claiming

**L2PublicClient**:
- `getL1ToL2MessageStatus(messageHash)` — Query L1→L2 message anchoring status
- `getBlockExtraData(blockNumber)` — Parse gas pricing from block header

**PublicClient** (shared):
- `getMessageByMessageHash(hash)` — Lookup message by hash
- `getMessagesByTransactionHash(txHash)` — All messages in a transaction
- `getTransactionReceiptByMessageHash(hash)` — Claiming receipt

### Actions

| Action | Description |
|--------|-------------|
| `deposit` | Token bridge deposit (L1→L2) |
| `withdraw` | Token bridge withdrawal (L2→L1) |
| `claimOnL1` | Claim message on L1 with Merkle proof |
| `claimOnL2` | Claim message on L2 |
| `getMessageProof` | Construct Merkle proof for claiming |

## SDK Ethers (`sdk/sdk-ethers/`)

Wraps `linea-sdk-core` with ethers bindings and TypeChain-generated contract types. Provides `LineaSDK` class for high-level bridging operations.

## SDK Viem (`sdk/sdk-viem/`)

Provides viem-native client decorators:

```typescript
import { lineaPublicActionsL1 } from "@consensys/linea-sdk-viem";

const client = createPublicClient({ ... }).extend(lineaPublicActionsL1());
const status = await client.getL2ToL1MessageStatus({ messageHash });
```

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `sdk/sdk-core/src/merkle-tree/smt.test.ts` | Jest | Sparse Merkle tree operations |
| `sdk/sdk-ethers/src/LineaSDK.test.ts` | Jest | SDK initialization, ethers bindings |
| `sdk/sdk-viem/src/**/*.test.ts` | Jest | Decorators, actions, error handling |

## Related Documentation

- [Tech: SDK Component](../tech/components/sdk.md) — Installation, usage examples, contract address lookup
