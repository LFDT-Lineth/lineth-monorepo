# SDK

> TypeScript SDK for bridging and cross-chain messaging between Ethereum and Linea.

> **Diagrams:** [SDK Architecture](../diagrams/sdk-architecture.mmd) | [L1→L2 Deposit Flow](../diagrams/l1-to-l2-deposit-flow.mmd) | [L2→L1 Withdrawal Flow](../diagrams/l2-to-l1-withdrawal-flow.mmd)

## Overview

The Linea SDK enables developers to:
- Bridge ETH and ERC20 tokens between L1 and L2
- Send cross-chain messages
- Track message and bridge transaction status
- Claim messages on destination chains

## Package Structure

```
sdk/
├── sdk-core/           # @consensys/linea-sdk-core
│   │                   # Shared types, utilities, constants
│   └── src/
│       ├── types/
│       ├── utils/
│       └── constants/
│
├── sdk-viem/           # @consensys/linea-sdk-viem
│   │                   # Viem-based implementation
│   └── src/
│       ├── actions/
│       └── decorators/
│
└── sdk-ethers/         # @consensys/linea-sdk
                        # Ethers.js v6 implementation
```

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                            SDK ARCHITECTURE                            │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       User Application                           │  │
│  │                                                                  │  │
│  │  import { LineaSDK } from "@consensys/linea-sdk"                 │  │
│  │  // OR                                                           │  │
│  │  import { publicActionsL1 } from "@consensys/linea-sdk-viem"     │  │
│  │                                                                  │  │
│  └──────────────────────────────┬───────────────────────────────────┘  │
│                                 │                                      │
│                 ┌───────────────┴───────────────┐                      │
│                 │                               │                      │
│                 ▼                               ▼                      │
│  ┌────────────────────────────┐  ┌────────────────────────────┐        │
│  │       sdk-ethers           │  │        sdk-viem            │        │
│  │                            │  │                            │        │
│  │  ┌──────────────────────┐  │  │  ┌──────────────────────┐  │        │
│  │  │     LineaSDK         │  │  │  │   Wallet Actions     │  │        │
│  │  │                      │  │  │  │                      │  │        │
│  │  │ - getL1Contract()    │  │  │  │ - deposit()          │  │        │
│  │  │ - getL2Contract()    │  │  │  │ - withdraw()         │  │        │
│  │  └──────────────────────┘  │  │  │ - claimOnL1()        │  │        │
│  │                            │  │  │ - claimOnL2()        │  │        │
│  │  ┌──────────────────────┐  │  │  └──────────────────────┘  │        │
│  │  │ LineaRollupClient    │  │  │                            │        │
│  │  │ L2MessageService     │  │  │  ┌──────────────────────┐  │        │
│  │  │    Client            │  │  │  │   Public Actions     │  │        │
│  │  └──────────────────────┘  │  │  │                      │  │        │
│  │                            │  │  │ - getMessageStatus() │  │        │
│  └────────────────────────────┘  │  │ - getMessageProof()  │  │        │
│                                  │  │ - getBlockExtraData()│  │        │
│                 │                │  └──────────────────────┘  │        │
│                 │                │                            │        │
│                 │                └────────────────────────────┘        │
│                 │                               │                      │
│                 └───────────────┬───────────────┘                      │
│                                 │                                      │
│                                 ▼                                      │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                          sdk-core                                │  │
│  │                                                                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐   │  │
│  │  │   Types     │  │ SparseMerkle│  │   Contract Addresses    │   │  │
│  │  │             │  │    Tree     │  │                         │   │  │
│  │  │ - Message   │  │             │  │ - Mainnet               │   │  │
│  │  │ - Proof     │  │ - Generate  │  │ - Sepolia               │   │  │
│  │  │ - Status    │  │   proofs    │  │                         │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────────────────┘   │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Installation & Usage

Each SDK package has its own detailed documentation:

| Package | Install | Documentation |
|---------|---------|---------------|
| **sdk-viem** (recommended) | `npm install @consensys/linea-sdk-viem viem` | [README](../../sdk/sdk-viem/README.md) |
| **sdk-ethers** | `npm install @consensys/linea-sdk` | [README](../../sdk/sdk-ethers/README.md) |
| **sdk-core** | `npm install @consensys/linea-sdk-core` | Types & utilities only |

> **Note:** `viem@>=2.22.0` is a required peer dependency for `@consensys/linea-sdk-viem`.

### Quick Comparison

| Feature | sdk-viem | sdk-ethers |
|---------|----------|------------|
| **Library** | Viem >= 2.22.0 | Ethers.js v6 |
| **Pattern** | Decorators extending Viem clients | Class-based SDK instance |
| **Wallet Actions** | `deposit()`, `withdraw()`, `claimOnL1()`, `claimOnL2()` | `claim()` |
| **Public Actions** | `getL1ToL2MessageStatus()`, `getL2ToL1MessageStatus()`, `getMessageProof()`, `getMessageByMessageHash()` | `getMessageStatus()`, `getMessageByMessageHash()`, `getMessagesByTransactionHash()` |

## Core Types

```typescript
// Message structure
interface Message {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: Hex;
  messageHash: Hex;
}

// Extended message with metadata
interface ExtendedMessage extends Message {
  blockNumber: bigint;
  transactionHash: Hex;
  status: OnChainMessageStatus;
}

// Message status
enum OnChainMessageStatus {
  UNKNOWN = 0,
  CLAIMABLE = 1,
  CLAIMED = 2,
}

// Message proof for L2→L1 claims
interface MessageProof {
  proof: Hex[];
  root: Hex;
  leafIndex: number;
}
```

## Contract Addresses

### Mainnet

```typescript
const mainnetAddresses = {
  l1: {
    lineaRollup: '0xd19d4B5d358258f05D7B411E21A1460D11B0876F',
    tokenBridge: '0x051F1D88f0aF5763fB888eC78378F1109b52Cd01',
  },
  l2: {
    messageService: '0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec',
    tokenBridge: '0x353012dc4a9A6cF55c941bADC267f82004A8ceB9',
  },
};
```

### Sepolia (Testnet)

```typescript
const sepoliaAddresses = {
  l1: {
    lineaRollup: '0xb218f8a4bc926cf1ca7b3423c154a0d627bdb7e5',
    tokenBridge: '0x5506A3805fB6C857D16e3ce28e8D13fCB12F6433',
  },
  l2: {
    messageService: '0x971e727e956690b9957be6d51ec16e73acac83a7',
    tokenBridge: '0x3E2Ea8BfA28b1e8EE8cf00B7b1A6B38DAb6c3ECe',
  },
};
```

## Bridging Flow Diagrams

### L1 → L2 Deposit

```
┌────────────────────────────────────────────────────────────────────────┐
│                       L1 → L2 DEPOSIT FLOW                             │
│                                                                        │
│  User                L1                   Coordinator        L2        │
│   │                   │                        │             │         │
│   │  deposit()        │                        │             │         │
│   │──────────────────▶│                        │             │         │
│   │                   │                        │             │         │
│   │                   │  MessageSent event     │             │         │
│   │                   │───────────────────────▶│             │         │
│   │                   │                        │             │         │
│   │                   │                        │ anchor hash │         │
│   │                   │                        │────────────▶│         │
│   │                   │                        │             │         │
│   │  getL1ToL2MessageStatus() ─────────────────────────────▶│          │
│   │◀─── CLAIMABLE ──────────────────────────────────────────│          │
│   │                   │                        │             │         │
│   │  claimOnL2()      │                        │             │         │
│   │─────────────────────────────────────────────────────────▶│         │
│   │                   │                        │             │         │
│   │◀─── ETH received ───────────────────────────────────────│          │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### L2 → L1 Withdrawal

```
┌────────────────────────────────────────────────────────────────────────┐
│                     L2 → L1 WITHDRAWAL FLOW                            │
│                                                                        │
│  User                L2                   Coordinator        L1        │
│   │                   │                        │             │         │
│   │  withdraw()       │                        │             │         │
│   │──────────────────▶│                        │             │         │
│   │                   │                        │             │         │
│   │                   │  MessageSent event     │             │         │
│   │                   │  Rolling hash update   │             │         │
│   │                   │───────────────────────▶│             │         │
│   │                   │                        │             │         │
│   │                   │            finalization with proof   │         │
│   │                   │                        │────────────▶│         │
│   │                   │                        │             │         │
│   │  getL2ToL1MessageStatus() ◀─────────────────────────────────────── │
│   │◀─── CLAIMABLE                                                      │
│   │                   │                        │             │         │
│   │  getMessageProof()                                                 │
│   │──────────────────▶│                        │             │         │
│   │◀─── proof ────────│                        │             │         │
│   │                   │                        │             │         │
│   │  claimOnL1(proof)                                       │          │
│   │─────────────────────────────────────────────────────────▶│         │
│   │                   │                        │             │         │
│   │◀─── ETH received ───────────────────────────────────────│          │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Development

```bash
cd sdk
pnpm install && pnpm run build && pnpm run test
```

See individual package READMEs for detailed contribution guidelines.

## Related Documentation

- [Feature: SDK](../../features/sdk.md) — Usage guide, package overview, and integration examples
