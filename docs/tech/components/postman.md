# Postman

> Cross-chain message relay service that automates message delivery between L1 (Ethereum) and L2 (Linea).

> **Diagrams:** [Postman Architecture](../diagrams/postman-architecture.mmd) | [Message Lifecycle](../diagrams/message-lifecycle.mmd)

## Overview

The Postman service implements the relay component of Linea's [Canonical Message Service](https://docs.linea.build/protocol/architecture/interoperability/canonical-message-service). It monitors `MessageSent` events on both chains, tracks when messages become claimable (anchored), and automatically submits claim transactions on the destination chain.

**Problem it solves**: When a user sends a cross-chain message (L1→L2 or L2→L1), someone must call the `claimMessage` function on the destination chain to finalize delivery. The Postman automates this process, removing the need for users to manually claim messages.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                          POSTMAN SERVICE                               │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Event Pollers                               │  │
│  │                                                                  │  │
│  │  ┌─────────────────────┐      ┌─────────────────────┐            │  │
│  │  │  L1 MessageSent     │      │  L2 MessageSent     │            │  │
│  │  │  EventPoller        │      │  EventPoller        │            │  │
│  │  └─────────┬───────────┘      └─────────┬───────────┘            │  │
│  └────────────┼────────────────────────────┼────────────────────────┘  │
│               │                            │                           │
│  ┌────────────▼────────────────────────────▼────────────────────────┐  │
│  │                      Message Processors                          │  │
│  │                                                                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐  │  │
│  │  │  Anchoring  │  │ Transaction │  │  Claiming   │  │Persister│  │  │
│  │  │  Processor  │  │ Size Proc.  │  │  Processor  │  │         │  │  │
│  │  │             │  │ (L1→L2)     │  │             │  │         │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────┘  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      Support Services                            │  │
│  │                                                                  │  │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐   │  │
│  │  │ Gas Estimation  │  │ Nonce Manager   │  │ Database Cleaner│   │  │
│  │  │ (linea_estimate │  │                 │  │                 │   │  │
│  │  │  Gas)           │  │                 │  │                 │   │  │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘   │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└───────────────────────────────┬────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐       ┌───────────────┐       ┌───────────────┐
│   L1 RPC      │       │   L2 RPC      │       │  PostgreSQL   │
│  (Ethereum)   │       │   (Linea)     │       │   Database    │
└───────────────┘       └───────────────┘       └───────────────┘
```

## Directory Structure

```
postman/
├── scripts/                    # Entry points and CLI utilities
│   ├── runPostman.ts          # Main entry point
│   ├── sendMessageOnL1.ts     # Manual testing scripts
│   └── sendMessageOnL2.ts
│
├── src/
│   ├── core/                   # Domain layer (clean architecture)
│   │   ├── entities/          # Message domain model
│   │   ├── enums/             # MessageStatus enum
│   │   ├── errors/            # Custom error types
│   │   ├── metrics/           # Metrics interfaces
│   │   ├── persistence/       # Repository interfaces
│   │   └── services/          # Service interfaces
│   │
│   ├── services/               # Business logic
│   │   ├── pollers/           # Polling schedulers
│   │   ├── processors/        # Processing logic
│   │   └── persistence/       # DB service implementations
│   │
│   ├── application/            # Application layer
│   │   └── postman/
│   │       ├── app/           # PostmanServiceClient, config
│   │       ├── api/           # Metrics API
│   │       └── persistence/   # TypeORM entities, migrations
│   │
│   └── utils/                  # Logging, error parsing
│
├── Dockerfile
├── package.json
└── .env.sample
```

## Message Lifecycle

Messages progress through a state machine tracked by `MessageStatus`:

```
SENT → ANCHORED → TRANSACTION_SIZE_COMPUTED* → PENDING → CLAIMED_SUCCESS
                                                      └→ CLAIMED_REVERTED
                                                      └→ NON_EXECUTABLE
                                                      └→ ZERO_FEE
                                                      └→ FEE_UNDERPRICED
       └→ EXCLUDED
```

*`TRANSACTION_SIZE_COMPUTED` applies only to L1→L2 messages. For L2→L1, the flow is `ANCHORED` → `PENDING`.

| Status | Meaning |
|--------|---------|
| `SENT` | Event detected on source chain, inserted into DB |
| `ANCHORED` | Message is claimable on destination chain (verified via `getMessageStatus()`) |
| `TRANSACTION_SIZE_COMPUTED` | (L1→L2 only) Compressed transaction size calculated for gas estimation |
| `PENDING` | Claim transaction submitted, awaiting confirmation |
| `CLAIMED_SUCCESS` | Claim transaction confirmed successfully |
| `CLAIMED_REVERTED` | Claim transaction reverted (may be retried if due to rate limiting) |
| `NON_EXECUTABLE` | Gas estimation failed or exceeds `MAX_CLAIM_GAS_LIMIT`; will not be retried |
| `ZERO_FEE` | Message has zero fee attached (see Postman Sponsorship below) |
| `FEE_UNDERPRICED` | Fee is lower than estimated gas cost × profit margin (see Postman Sponsorship below) |
| `EXCLUDED` | Filtered out by event filters or EOA/calldata rules; will not be processed |

### Postman Sponsorship

By default, the Postman only claims messages where the attached fee covers the gas cost plus a profit margin (`PROFIT_MARGIN`). Messages that don't meet this threshold are marked `ZERO_FEE` or `FEE_UNDERPRICED`.

**When Postman Sponsorship is enabled** (`L1_L2_ENABLE_POSTMAN_SPONSORING` or `L2_L1_ENABLE_POSTMAN_SPONSORING`), the service will pay gas costs out of its own funds:

| Condition | Without Sponsorship | With Sponsorship |
|-----------|---------------------|------------------|
| `fee = 0` | → `ZERO_FEE` (terminal) | → Claimed if gas ≤ `MAX_POSTMAN_SPONSOR_GAS_LIMIT` |
| `fee < gasCost` | → `FEE_UNDERPRICED` (retry later) | → Claimed if gas ≤ `MAX_POSTMAN_SPONSOR_GAS_LIMIT` |
| `fee ≥ gasCost` | → Claimed normally | → Claimed normally |

The `MAX_POSTMAN_SPONSOR_GAS_LIMIT` configuration caps the gas the Postman will subsidize per message. Messages exceeding this limit still fall back to `ZERO_FEE` or `FEE_UNDERPRICED` behavior.

### Why TRANSACTION_SIZE_COMPUTED? (L1→L2 only)

Linea uses a [variable gas pricing model](https://docs.linea.build/network/how-to/gas-fees#variable-cost-and-linea_estimategas) where the priority fee depends on the **compressed transaction size**:

```
priorityFeePerGas = MINIMUM_MARGIN * (min-gas-price * L2_compressed_tx_size_in_bytes / L2_tx_gas_used + fixed_cost)
```

For L1→L2 claims, the Postman must pre-compute the compressed size to accurately estimate gas costs. This is handled by `L2ClaimMessageTransactionSizeProcessor` using `@consensys/linea-native-libs`.

L2→L1 claims use standard EIP-1559 gas estimation, so this step is not required.

## Processing Flow

### L1→L2 Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ L1 Message  │    │  Anchoring  │    │ Transaction │    │  Claiming   │    │ Persisting  │
│ SentEvent   │───▶│  Processor  │───▶│ Size Proc.  │───▶│  Processor  │───▶│  Processor  │
│  Poller     │    │             │    │             │    │             │    │             │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
      │                  │                  │                  │                  │
      ▼                  ▼                  ▼                  ▼                  ▼
   INSERT            UPDATE             UPDATE             UPDATE             UPDATE
   (SENT)          (ANCHORED)    (TX_SIZE_COMPUTED)      (PENDING)      (CLAIMED_SUCCESS)
```

### L2→L1 Flow

Same as above but **without** the Transaction Size Processor step.

## Bidirectional Pollers

| Flow | Pollers | Description |
|------|---------|-------------|
| L1→L2 | `L1MessageSentEventPoller`, `L2MessageAnchoringPoller`, `L2ClaimMessageTransactionSizePoller`, `L2MessageClaimingPoller`, `L2MessagePersistingPoller` | Listens on L1, claims on L2 |
| L2→L1 | `L2MessageSentEventPoller`, `L1MessageAnchoringPoller`, `L1MessageClaimingPoller`, `L1MessagePersistingPoller` | Listens on L2, claims on L1 |

Each flow can be enabled/disabled via `L1_L2_AUTO_CLAIM_ENABLED` and `L2_L1_AUTO_CLAIM_ENABLED`.

## Retry Logic

- **Rate limit errors**: If a claim transaction reverts with `RateLimitExceeded`, the message is reset to `SENT` and retried later
- **`FEE_UNDERPRICED`**: Re-evaluated periodically; becomes claimable if gas prices drop
- **Transaction timeout**: If a `PENDING` transaction isn't mined within `MESSAGE_SUBMISSION_TIMEOUT`, retried with higher gas (up to `MAX_TX_RETRIES` attempts)

## Database Cleaning

The `DatabaseCleaningPoller` periodically deletes messages older than `DB_DAYS_BEFORE_NOW_TO_DELETE` in final states: `CLAIMED_SUCCESS`, `CLAIMED_REVERTED`, `EXCLUDED`, `ZERO_FEE`.

## Database Schema

The Postman uses a single `message` table in PostgreSQL to track all cross-chain messages.

### Message Table

| Column | Type | Description |
|--------|------|-------------|
| `id` | `integer` | Primary key (auto-increment) |
| `message_sender` | `varchar` | Address that sent the message |
| `destination` | `varchar` | Target address on destination chain |
| `fee` | `varchar` | Fee attached to message (stored as string for precision) |
| `value` | `varchar` | ETH value transferred (stored as string for precision) |
| `message_nonce` | `integer` | Message nonce from the contract |
| `calldata` | `varchar` | Message calldata |
| `message_hash` | `varchar` | Unique hash identifying the message |
| `message_contract_address` | `varchar` | Origin contract address (LineaRollup or L2MessageService) |
| `sent_block_number` | `integer` | Block number where MessageSent event was emitted |
| `direction` | `enum` | `L1_TO_L2` or `L2_TO_L1` |
| `status` | `enum` | Current message status (see Message Lifecycle) |
| `claim_tx_creation_date` | `timestamp` | When the claim transaction was created |
| `claim_tx_gas_limit` | `integer` | Gas limit used for claim transaction |
| `claim_tx_max_fee_per_gas` | `bigint` | Max fee per gas for claim transaction |
| `claim_tx_max_priority_fee_per_gas` | `bigint` | Max priority fee for claim transaction |
| `claim_tx_nonce` | `integer` | Nonce used for claim transaction |
| `claim_tx_hash` | `varchar` | Hash of the claim transaction |
| `claim_number_of_retry` | `integer` | Number of claim retry attempts |
| `claim_last_retried_at` | `timestamp` | When the last retry occurred |
| `claim_gas_estimation_threshold` | `decimal` | Gas price threshold for profitability |
| `compressed_transaction_size` | `integer` | Compressed calldata size (L1→L2 only) |
| `is_for_sponsorship` | `boolean` | Whether this message is being sponsored |
| `created_at` | `timestamp` | Record creation time |
| `updated_at` | `timestamp` | Record last update time |

### Indexes

| Index Name | Columns | Purpose |
|------------|---------|---------|
| `message_hash_index` | `message_hash` | Fast lookup by message hash |
| `claim_tx_hash_index` | `claim_tx_hash` | Fast lookup by transaction hash |
| `direction_index` | `direction` | Filter by L1→L2 or L2→L1 |
| `status_index` | `status` | Filter by message status |
| `claim_tx_nonce_index` | `claim_tx_nonce` | Nonce management queries |
| `created_at_index` | `created_at` | Time-based queries and cleanup |
| `message_contract_address_index` | `message_contract_address` | Filter by origin contract |

### Migrations

Migrations are located in `src/application/postman/persistence/migrations/` and run automatically on startup. To create a new migration:

```bash
MIGRATION_NAME=YourMigrationName pnpm run migration:create
```

TypeORM auto-generation does not work reliably for this schema, so migrations must be implemented manually.

## Running

### Local Development

```bash
# Build dependencies
NATIVE_LIBS_RELEASE_TAG=blob-libs-v1.2.0 pnpm run -F linea-native-libs build
pnpm run -F linea-shared-utils build
pnpm run -F "./sdk/*" build

# Configure
cd postman
cp .env.sample .env

# Run
ts-node scripts/runPostman.ts
```

### Docker

```bash
# Build
docker build -t postman --build-arg NATIVE_LIBS_RELEASE_TAG=blob-libs-v1.2.0 -f postman/Dockerfile .

# Run
docker run -e L1_RPC_URL=... -e L2_RPC_URL=... postman
```

## Testing

```bash
cd postman
pnpm run test
```

Tests are colocated with implementations in `__tests__/` subdirectories, using `jest-mock-extended` for mocking.

## Configuration

See the [README.md](../../postman/README.md) for complete environment variable documentation.

| Category | Key Variables |
|----------|---------------|
| L1 Config | `L1_RPC_URL`, `L1_CONTRACT_ADDRESS`, `L1_SIGNER_PRIVATE_KEY` |
| L2 Config | `L2_RPC_URL`, `L2_CONTRACT_ADDRESS`, `L2_SIGNER_PRIVATE_KEY` |
| Listener | `*_LISTENER_INTERVAL`, `*_INITIAL_FROM_BLOCK`, `*_BLOCK_CONFIRMATION` |
| Claiming | `PROFIT_MARGIN`, `MAX_CLAIM_GAS_LIMIT`, `MAX_NUMBER_OF_RETRIES` |
| Feature Flags | `L1_L2_AUTO_CLAIM_ENABLED`, `*_EOA_ENABLED`, `*_ENABLE_POSTMAN_SPONSORING` |
| Database | `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` |

## Dependencies

- **@consensys/linea-sdk**: Contract clients and gas providers
- **@consensys/linea-shared-utils**: Shared utilities (logger, metrics, Express API)
- **@consensys/linea-native-libs**: Native blob compression library
- **TypeORM**: Database ORM with PostgreSQL
- **ethers**: Blockchain interactions

## Further Reading

- [Canonical Message Service](https://docs.linea.build/protocol/architecture/interoperability/canonical-message-service) - Architecture overview
- [Linea Gas Estimation](https://docs.linea.build/network/how-to/gas-fees#variable-cost-and-linea_estimategas) - Variable gas pricing
