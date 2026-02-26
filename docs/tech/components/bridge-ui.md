# Bridge UI

## Overview

The `bridge-ui` folder contains the official Linea Bridge web application, a Next.js 15 frontend that enables users to transfer assets between Ethereum and Linea networks. It provides multiple bridging options through a unified interface.

### Features

The Bridge UI enables:
- Bridge ETH and ERC-20 tokens between Ethereum L1 and Linea L2
- Bridge USDC via Circle's Cross-Chain Transfer Protocol (CCTP)
- Access third-party bridge aggregators for multi-chain transfers
- Purchase crypto via fiat on-ramps
- Transfer from centralized exchanges

### How It Fits Into the System

This is a user-facing frontend application that interacts with:
- Linea's `MessageService` contracts for native message passing
- Linea's `TokenBridge` contracts for ERC-20 bridging
- Circle's CCTP infrastructure for USDC transfers
- Third-party services (LiFi, LayerSwap, Onramper)
- The `@consensys/linea-sdk-viem` package for SDK functionality

It is deployed separately from backend services and communicates directly with blockchain RPCs and external APIs.

### Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│                            BRIDGE UI                                   │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       Next.js App Router                         │  │
│  │                                                                  │  │
│  │ ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │  │
│  │ │    /         │  │ /native-     │  │ /bridge-aggregator       │ │  │
│  │ │   Landing    │  │   bridge     │  │  (LiFi Widget)           │ │  │
│  │ └──────────────┘  └──────────────┘  └──────────────────────────┘ │  │
│  │                                                                  │  │
│  │ ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │  │
│  │ │ /centralized │  │    /buy      │  │      /faq                │ │  │
│  │ │  -exchange   │  │ (OnRamper)   │  │                          │ │  │
│  │ │ (LayerSwap)  │  │              │  │                          │ │  │
│  │ └──────────────┘  └──────────────┘  └──────────────────────────┘ │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       Components Layer                           │  │
│  │                                                                  │  │
│  │ ┌────────────────────────────────────────────────────────────┐   │  │
│  │ │                   Bridge Components                        │   │  │
│  │ │                                                            │   │  │
│  │ │ ┌────────────┐  ┌────────────┐  ┌────────────┐             │   │  │
│  │ │ │ From Chain │  │ To Chain   │  │   Amount   │             │   │  │
│  │ │ │  Selector  │  │  Selector  │  │   Input    │             │   │  │
│  │ │ └────────────┘  └────────────┘  └────────────┘             │   │  │
│  │ │                                                            │   │  │
│  │ │ ┌────────────┐  ┌────────────┐  ┌────────────┐             │   │  │
│  │ │ │   Token    │  │  Claiming  │  │   Submit   │             │   │  │
│  │ │ │   List     │  │   Mode     │  │   Button   │             │   │  │
│  │ │ └────────────┘  └────────────┘  └────────────┘             │   │  │
│  │ │                                                            │   │  │
│  │ │ ┌─────────────────────────────────────────────────────┐    │   │  │
│  │ │ │             Transaction History                     │    │   │  │
│  │ │ └─────────────────────────────────────────────────────┘    │   │  │
│  │ │                                                            │   │  │
│  │ └────────────────────────────────────────────────────────────┘   │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       State & Services                           │  │
│  │                                                                  │  │
│  │ ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐ │  │
│  │ │   Zustand    │  │   Wagmi +    │  │      Web3Auth            │ │  │
│  │ │   Stores     │  │    Viem      │  │       Modal              │ │  │
│  │ └──────────────┘  └──────────────┘  └──────────────────────────┘ │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                         External Services                              │
│                                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐ │
│  │   Ethereum   │  │    Linea     │  │   Circle     │  │   Token    │ │
│  │     RPC      │  │     RPC      │  │  Iris API    │  │   List     │ │
│  │  (Infura/    │  │  (Infura/    │  │   (CCTP)     │  │  (GitHub)  │ │
│  │   Alchemy)   │  │   Alchemy)   │  │              │  │            │ │
│  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
bridge-ui/
├── public/                    # Static assets (fonts, images, logos)
├── src/
│   ├── abis/                  # Smart contract ABIs
│   │   ├── MessageService.json
│   │   ├── MessageTransmitterV2.json
│   │   ├── TokenBridge.json
│   │   └── USDCBridge.json
│   ├── app/                   # Next.js App Router pages
│   │   ├── (main)/            # Main layout routes
│   │   │   ├── bridge-aggregator/
│   │   │   ├── buy/
│   │   │   ├── faq/
│   │   │   └── native-bridge/
│   │   ├── (layerswap)/       # Layerswap layout routes
│   │   │   └── centralized-exchange/
│   │   └── api-v1/            # API routes
│   ├── assets/                # SVG icons and fonts
│   ├── components/            # React components
│   │   ├── bridge/            # Core bridging UI components
│   │   ├── layouts/           # App layout and providers
│   │   ├── lifi/              # LiFi bridge aggregator widget
│   │   ├── layerswap/         # LayerSwap CEX integration
│   │   ├── onramper/          # Fiat on-ramp widget
│   │   └── ui/                # Shared UI primitives
│   ├── config/                # Application configuration
│   ├── constants/             # Chain definitions and constants
│   ├── contexts/              # React context providers
│   ├── hooks/                 # Custom React hooks
│   │   ├── fees/              # Fee calculation hooks
│   │   └── transaction-args/  # Transaction building hooks
│   ├── lib/                   # External integrations (PoH)
│   ├── services/              # API service functions
│   ├── stores/                # Zustand state stores
│   ├── types/                 # TypeScript type definitions
│   └── utils/                 # Utility functions
│       ├── events/            # Event parsing utilities
│       └── history/           # Transaction history utilities
├── test/
│   ├── e2e/                   # Playwright E2E tests
│   ├── utils/                 # Test utilities
│   └── wallet-setup/          # MetaMask test fixtures
├── .env.template              # Environment variable template
├── .env.production            # Production configuration
├── Dockerfile                 # Container build configuration
└── package.json
```

### Key Directories

| Directory | Purpose |
|-----------|---------|
| `src/components/bridge/` | Core bridge form, transaction history, chain selectors, fee displays |
| `src/stores/` | Zustand stores for form state, chain selection, tokens, and history |
| `src/hooks/` | Business logic hooks for bridging, claiming, fees, and allowances |
| `src/hooks/transaction-args/` | Transaction data construction for different bridge types |
| `src/utils/history/` | Fetching and caching bridge transaction history from on-chain events |
| `src/services/` | External API integrations (CCTP attestation, navigation data, token lists) |

---

## Key Concepts & Design Decisions

### Application Structure

The Bridge UI is organized around four main bridging paths, each with its own page:

| Route | Component | Description |
|-------|-----------|-------------|
| `/native-bridge` | Native bridge form | Direct bridging via Linea's official contracts |
| `/bridge-aggregator` | LiFi widget | Multi-chain bridging through aggregated routes |
| `/centralized-exchange` | LayerSwap widget | Transfer from CEX wallets |
| `/buy` | Onramper widget | Purchase crypto with fiat |

### State Management Architecture

The application uses **Zustand** for state management, organized into specialized stores:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Provider Hierarchy                           │
├─────────────────────────────────────────────────────────────────┤
│  ModalProvider                                                   │
│    └─ QueryProvider (TanStack Query for async data)             │
│         └─ Web3Provider (Web3Auth + Wagmi for wallet)           │
│              └─ TokenStoreProvider (available tokens)           │
│                   └─ FormStoreProvider (bridge form state)      │
└─────────────────────────────────────────────────────────────────┘
```

**Store Responsibilities:**

| Store | Scope | Persistence | Purpose |
|-------|-------|-------------|---------|
| `chainStore` | Global | Memory | Source and destination chain selection |
| `formStore` | Scoped via Context | Memory | Bridge form inputs (amount, token, recipient, fees) |
| `tokenStore` | Scoped via Context | Memory | Available tokens list fetched from token registry |
| `historyStore` | Global | localStorage | Cache of completed transactions |
| `configStore` | Global | localStorage | User preferences (currency, testnet toggle, visited modals) |
| `nativeBridgeNavigationStore` | Global | Memory | Toggle between bridge form and transaction history views |

### Native Bridge Component Structure

The native bridge page (`/native-bridge`) toggles between two views controlled by `nativeBridgeNavigationStore`:

```
BridgeLayout
├─ [isBridgeOpen = true]
│   └─ BridgeForm
│       ├─ FromChain (source chain selector)
│       ├─ SwapChain (swap source/destination)
│       ├─ ToChain (destination chain selector)
│       ├─ Amount (input with balance)
│       ├─ TokenList (token dropdown)
│       ├─ Claiming (fee display, claim mode)
│       ├─ DestinationAddress (optional custom recipient)
│       └─ Submit (connect/approve/bridge button)
│
└─ [isTransactionHistoryOpen = true]
    └─ TransactionHistory
        └─ ListTransaction
            └─ TransactionItem (click opens details modal with claim action)
```

### Claim Types

For L1→L2 transfers, the application determines the appropriate claim mechanism:

| Type | Direction | Description |
|------|-----------|-------------|
| `AUTO_SPONSORED` | L1→L2 | Claim sponsored by the Postman service (free for users) |
| `AUTO_PAID` | L1→L2 | User pays for L2 claim (when estimated gas exceeds sponsorship cap) |
| `MANUAL` | L2→L1 | User must manually claim on L1 (required for all L2→L1 transfers) |

The claim type is automatically determined based on estimated gas costs. If the estimated gas limit is below the sponsorship cap, Postman sponsors it; otherwise, the user pays.

### CCTP Integration

USDC bridging uses Circle's Cross-Chain Transfer Protocol v2:
- Feature-flagged via `NEXT_PUBLIC_IS_CCTP_ENABLED`
- Supports `STANDARD` and `FAST` modes (different finality thresholds)
- Requires attestation from Circle's Iris API before claiming
- Uses separate contract ABIs (`USDCBridge`, `MessageTransmitterV2`)

### Web3 Provider Architecture

Wallet connectivity is managed via Web3Auth which wraps wagmi:
- Supports MetaMask, Coinbase Wallet, WalletConnect, and social logins
- Coinbase connector is EOA-only (smart wallets not supported on Linea)
- Chain switching is handled automatically when user changes source chain

---

## How It Works

### Bridge Transaction Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Connect   │────▶│   Select    │────▶│   Enter     │────▶│  Calculate  │
│   Wallet    │     │   Token     │     │   Amount    │     │    Fees     │
└─────────────┘     └─────────────┘     └─────────────┘     └──────┬──────┘
                                                                   │
                    ┌──────────────────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    Transaction Args Router                              │
│                                                                         │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────┐   │
│   │     ETH     │     │   ERC-20    │     │         USDC            │   │
│   │             │     │             │     │        (CCTP)           │   │
│   │ sendMessage │     │ bridgeToken │     │    depositForBurn       │   │
│   └─────────────┘     └──────┬──────┘     └─────────────────────────┘   │
│                              │                                          │
│                    ┌─────────┴─────────┐                                │
│                    ▼                   ▼                                │
│            ┌─────────────┐     ┌─────────────┐                          │
│            │  Allowance  │     │  Allowance  │                          │
│            │ Sufficient  │     │ Insufficient│                          │
│            │  → Bridge   │     │  → Approve  │                          │
│            └─────────────┘     └─────────────┘                          │
└─────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
                              ┌─────────────────┐
                              │  Wallet Signs   │
                              │   Transaction   │
                              └────────┬────────┘
                                       │
                                       ▼
                              ┌─────────────────┐
                              │   Confirmed!    │
                              │   Reset Form    │
                              └─────────────────┘
```

1. **User Input**: User connects wallet, selects token, and enters amount
2. **Fee Calculation**: Hooks estimate gas fees and determine claim type
3. **Transaction Building**: Based on token type, the appropriate hook builds transaction data:
   - ETH → `useEthBridgeTxArgs` (calls `MessageService.sendMessage`)
   - ERC-20 → `useApproveTxArgs` then `useERC20BridgeTxArgs` (calls `TokenBridge.bridgeToken`)
   - USDC → `useDepositForBurnTxArgs` (calls CCTP `depositForBurn`)
4. **Transaction Submission**: wagmi's `sendTransaction` prompts wallet signature
5. **Confirmation**: Transaction receipt is monitored, form resets on success

### Transaction History & Claiming Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Transaction History Loading                         │
│                                                                         │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐               │
│  │  Query ETH  │     │ Query ERC20 │     │ Query USDC  │               │
│  │   Events    │     │   Events    │     │   Events    │               │
│  │ MessageSent │     │ Bridging-   │     │ DepositFor- │               │
│  │             │     │ InitiatedV2 │     │    Burn     │               │
│  └──────┬──────┘     └──────┬──────┘     └──────┬──────┘               │
│         │                   │                   │                       │
│         └───────────────────┼───────────────────┘                       │
│                             ▼                                           │
│                   ┌─────────────────┐                                   │
│                   │  Check Cache    │                                   │
│                   │ (localStorage)  │                                   │
│                   └────────┬────────┘                                   │
│                            │                                            │
│              ┌─────────────┼─────────────┐                              │
│              ▼             ▼             ▼                              │
│        ┌──────────┐  ┌──────────┐  ┌──────────┐                        │
│        │ PENDING  │  │  READY   │  │ COMPLETE │                        │
│        │          │  │ TO CLAIM │  │          │                        │
│        └──────────┘  └────┬─────┘  └──────────┘                        │
│                           │                                             │
└───────────────────────────┼─────────────────────────────────────────────┘
                            │
                            ▼
              ┌─────────────────────────────┐
              │      User Clicks Claim      │
              └──────────────┬──────────────┘
                             │
       ┌─────────────────────┼─────────────────────┐
       ▼                     ▼                     ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   ETH / ERC20   │  │   ETH / ERC20   │  │      USDC       │
│    (L1→L2)      │  │    (L2→L1)      │  │                 │
│                 │  │                 │  │ Get Attestation │
│  Direct claim   │  │ Get Merkle Proof│  │ from Circle API │
│  (no proof)     │  │  from Rollup    │  │                 │
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              ▼
                    ┌─────────────────┐
                    │ Claim on Dest.  │
                    │    Chain        │
                    └─────────────────┘
```

1. **Event Fetching**: On-chain events are queried for user's address
2. **Status Resolution**: Each transaction's message status is checked via SDK
3. **Caching**: Completed transactions are cached in localStorage to avoid re-fetching
4. **Claiming**: For `READY_TO_CLAIM` transactions, user can trigger claim from the details modal

### Token List Loading

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    Server-Side Initialization                           │
│                                                                         │
│  ┌─────────────────┐                                                    │
│  │  Fetch Token    │                                                    │
│  │  List (GitHub)  │                                                    │
│  └────────┬────────┘                                                    │
│           │                                                             │
│           ▼                                                             │
│  ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐   │
│  │ Filter by Type  │────▶│ Filter USDC by  │────▶│ Sort Priority   │   │
│  │ canonical-bridge│     │  CCTP Flag      │     │ Tokens First    │   │
│  │ or native       │     │                 │     │                 │   │
│  └─────────────────┘     └─────────────────┘     └────────┬────────┘   │
│                                                           │             │
│                                                           ▼             │
│                                                  ┌─────────────────┐    │
│                                                  │ TokenStore      │    │
│                                                  │ Provider        │    │
│                                                  └─────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

Tokens are loaded server-side during app initialization:
1. Token lists are fetched from GitHub (`linea-token-list` repository)
2. Tokens are filtered by bridge compatibility (`canonical-bridge` or `native` type)
3. USDC is conditionally included based on CCTP feature flag
4. Token list is passed to `TokenStoreProvider` for client-side access

---

## How to Use / Integrate

### Running Locally

```bash
# Copy environment template
cp .env.template .env

# Install dependencies
pnpm install

# Start development server
pnpm run dev
```

Frontend available at `http://localhost:3000`

### Environment Variables

Key configuration variables (see `.env.template` for full list):

| Variable | Purpose |
|----------|---------|
| `NEXT_PUBLIC_WALLET_CONNECT_ID` | WalletConnect project ID |
| `NEXT_PUBLIC_INFURA_ID` | Infura API key for RPC |
| `NEXT_PUBLIC_ALCHEMY_API_KEY` | Alchemy API key for RPC |
| `NEXT_PUBLIC_IS_CCTP_ENABLED` | Feature flag for USDC CCTP bridging |
| `NEXT_PUBLIC_MAINNET_*` | Contract addresses for mainnet |
| `NEXT_PUBLIC_SEPOLIA_*` | Contract addresses for Sepolia testnet |
| `NEXT_PUBLIC_STORAGE_MIN_VERSION` | Storage version for cache invalidation |

### Integration Points

This application does not expose APIs for other monorepo components. It is a standalone frontend that:
- Consumes the `@consensys/linea-sdk-viem` package for message status and proofs
- Reads contract addresses from environment configuration
- Fetches token lists from `linea-token-list` repository

---

## Development Notes

### Common Pitfalls

1. **Chain Switching**: L2→L1 transfers always require `MANUAL` claim type. The form auto-sets this based on the source chain's layer.

2. **Token Allowance**: ERC-20 bridges require approval before bridging. The `useTransactionArgs` hook automatically returns approval args when allowance is insufficient.

3. **BigInt Serialization**: History store uses custom JSON serialization for BigInt values (`{ __type: "bigint", value: "..." }`).

4. **Storage Versioning**: Incrementing `NEXT_PUBLIC_STORAGE_MIN_VERSION` clears all cached history. Use this for breaking storage format changes.

5. **CCTP Feature Flag**: USDC transactions are filtered out when `NEXT_PUBLIC_IS_CCTP_ENABLED` is false.

6. **E2E Test Mode**: `NEXT_PUBLIC_E2E_TEST_MODE=true` enables local chain configurations. Do not enable in production.

### Confusing Areas

- **Multiple ABIs**: `TokenBridge` for ERC-20, `MessageService` for ETH. USDC uses `USDCBridge` and `MessageTransmitterV2`.

- **Form vs Token Store**: Both stores have token state. `formStore.token` is the source of truth for the current bridge operation.

- **Two Token Filtering Layers**: Tokens are filtered server-side in `getTokenConfig()` and client-side in `useTokens()` based on current chain.

---

## Testing

### Test Location

```
test/
├── e2e/                       # Playwright E2E tests
│   ├── bridge-l1-l2.spec.ts   # L1→L2 bridging tests
│   ├── bridge-l2-l1.spec.ts   # L2→L1 bridging tests
│   └── general.spec.ts        # General UI tests
├── utils/                     # Test helpers
├── wallet-setup/              # MetaMask setup via Synpress
└── global.setup.ts            # Global test setup
```

Unit tests are colocated with source files (e.g., `cctp.spec.ts`, `isBlockTooOld.spec.ts`).

### Running Tests

```bash
# Build wallet cache (required for E2E)
pnpm run build:cache

# Run E2E tests (headful)
pnpm run test:e2e:headful

# Run E2E tests (headless)
pnpm run test:e2e:headless

# Run unit tests
pnpm run test:unit
```

### E2E Test Requirements

1. Set `NEXT_PUBLIC_E2E_TEST_MODE=true` in `.env`
2. Build the application: `pnpm run build`
3. Start the local docker stack from monorepo root: `make start-env-with-tracing-v2-ci`
4. Run tests

E2E tests use Synpress for MetaMask automation with Playwright.

