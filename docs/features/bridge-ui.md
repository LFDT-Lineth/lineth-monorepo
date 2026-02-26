# Bridge UI

> Next.js 16 web application for bridging tokens between Ethereum and Linea.

## Overview

The Bridge UI is a production web application for asset transfers between Ethereum L1 and Linea L2. Built on Next.js with React, it uses wagmi/viem for wallet interaction and `@consensys/linea-sdk-viem` for on-chain operations.

## Stack

| Technology | Role |
|------------|------|
| Next.js 16 | Framework (App Router) |
| React 19 | UI library |
| wagmi + viem | Wallet connection and contract interaction |
| Zustand | State management |
| Web3Auth | Social login and wallet aggregation |
| `@consensys/linea-sdk-viem` | Linea SDK integration |
| Playwright + Synpress | E2E testing with MetaMask automation |

## Bridging Paths

| Route | Method | Description |
|-------|--------|-------------|
| `/native-bridge` | Linea contracts | Direct bridging via `MessageService` (ETH) and `TokenBridge` (ERC-20) |
| `/native-bridge` (USDC) | Circle CCTP | USDC bridging via Cross-Chain Transfer Protocol v2 |
| `/bridge-aggregator` | LiFi widget | Multi-chain bridging through aggregated routes |
| `/centralized-exchange` | LayerSwap | Transfer from CEX wallets |
| `/buy` | Onramper | Purchase crypto with fiat |

### CCTP Integration

USDC bridging uses Circle's Cross-Chain Transfer Protocol v2:
- Feature-flagged via `NEXT_PUBLIC_IS_CCTP_ENABLED`
- Supports `STANDARD` and `FAST` modes (different finality thresholds)
- Requires attestation from Circle's Iris API before claiming
- Uses separate contract ABIs (`USDCBridge`, `MessageTransmitterV2`)

## Claim Types

For L1→L2 transfers, the UI determines the appropriate claim mechanism based on estimated gas:

| Type | Direction | Description |
|------|-----------|-------------|
| `AUTO_SPONSORED` | L1→L2 | Postman sponsors the claim (free for users) |
| `AUTO_PAID` | L1→L2 | User pays for L2 claim (gas exceeds sponsorship cap) |
| `MANUAL` | L2→L1 | User manually claims on L1 with Merkle proof (all L2→L1 transfers) |

## Bridge Transaction Flow

1. **Connect wallet** — MetaMask, WalletConnect, Coinbase Wallet, or social login via Web3Auth
2. **Select token and amount** — Tokens loaded from `linea-token-list` GitHub repo, filtered by bridge compatibility
3. **Fee calculation** — Hooks estimate gas and determine claim type
4. **Transaction routing** — Based on token type:
   - ETH → `MessageService.sendMessage`
   - ERC-20 → approve + `TokenBridge.bridgeToken`
   - USDC → `depositForBurn` (CCTP)
5. **Wallet signs** → transaction confirmed → form resets

## Transaction History & Claiming

The UI queries on-chain events (`MessageSent`, `BridgingInitiatedV2`, `DepositForBurn`) for the connected address, checks message status via the SDK, and presents claimable transactions. Completed transactions are cached in localStorage.

## State Management

Zustand stores manage application state:

| Store | Scope | Purpose |
|-------|-------|---------|
| `chainStore` | Global | Source/destination chain selection |
| `formStore` | Scoped (Context) | Bridge form inputs (amount, token, recipient, fees) |
| `tokenStore` | Scoped (Context) | Available tokens from registry |
| `historyStore` | Global (localStorage) | Completed transaction cache |
| `configStore` | Global (localStorage) | User preferences (currency, testnet toggle) |

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `bridge-ui/test/e2e/bridge-l1-l2.spec.ts` | Playwright/Synpress | L1→L2 bridge flow in browser |
| `bridge-ui/test/e2e/bridge-l2-l1.spec.ts` | Playwright/Synpress | L2→L1 bridge flow in browser |
| `bridge-ui/test/e2e/general.spec.ts` | Playwright/Synpress | General UI behavior |
| Colocated unit tests (`*.spec.ts`) | Playwright | CCTP logic, utility functions |

## Deployment

Containerized via `bridge-ui/Dockerfile`. Built and published via GitHub Actions workflow.

## Related Documentation

- [Token Bridge Feature](token-bridge.md) — ERC-20 bridging contracts
- [Messaging Feature](messaging.md) — Cross-chain message protocol
- [Postman Feature](postman.md) — Automated claiming and sponsorship model
- [Tech: Bridge UI Component](../tech/components/bridge-ui.md) — Full architecture diagrams, directory structure, environment variables, development notes
