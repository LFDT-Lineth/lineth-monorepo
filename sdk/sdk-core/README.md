# @consensys/linea-sdk-core

Core utilities for the [Linea](https://linea.build) bridge SDK — Merkle tree, message types, chain and contract helpers.

This package is framework-agnostic and serves as the foundation for higher-level SDKs like [`@consensys/linea-sdk-viem`](../sdk-viem/).

## Installation

```bash
npm install @consensys/linea-sdk-core
# or
pnpm add @consensys/linea-sdk-core
```

## Usage

```ts
import {
  SparseMerkleTree,
  parseBlockExtraData,
  formatMessageStatus,
  getContractsAddressesByChainId,
  OnChainMessageStatus,
} from "@consensys/linea-sdk-core";
```

### Sparse Merkle Tree

```ts
import { SparseMerkleTree } from "@consensys/linea-sdk-core";

const tree = new SparseMerkleTree();
// Build and verify Merkle proofs for L2 → L1 message claiming
```

### Chain and Contract Utilities

```ts
import {
  getContractsAddressesByChainId,
  isMainnet,
  isLineaMainnet,
  isSepolia,
  isLineaSepolia,
} from "@consensys/linea-sdk-core";

const addresses = getContractsAddressesByChainId(1); // Ethereum Mainnet
```

### Message Utilities

```ts
import { formatMessageStatus, OnChainMessageStatus } from "@consensys/linea-sdk-core";

const status = formatMessageStatus(OnChainMessageStatus.CLAIMED);
```

## API

| Export | Description |
|--------|-------------|
| `SparseMerkleTree` | Sparse Merkle tree for proof generation and verification |
| `parseBlockExtraData` | Parse Linea-specific block extra data |
| `formatMessageStatus` | Format on-chain message status to human-readable form |
| `getContractsAddressesByChainId` | Resolve contract addresses for a given chain ID |
| `isMainnet` / `isLineaMainnet` / `isSepolia` / `isLineaSepolia` | Chain identification helpers |
| `OnChainMessageStatus` | Enum of on-chain message statuses |

### Types

| Type | Description |
|------|-------------|
| `L1PublicClient` / `L2PublicClient` | Public client type constraints |
| `L1WalletClient` / `L2WalletClient` | Wallet client type constraints |
| `Message` / `ExtendedMessage` / `MessageProof` | Message-related types |

## License

[Apache-2.0](../../LICENSE-APACHE)
