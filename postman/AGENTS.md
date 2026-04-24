# AGENTS.md — postman

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

TypeScript backend service that facilitates cross-chain message delivery between L1 (Ethereum) and L2 (Linea). Monitors `MessageSent` events, checks message anchoring readiness, and auto-claims messages with configurable retry/gas-bumping logic. Built with Viem for blockchain interactions, TypeORM + PostgreSQL for persistence, Express for metrics/health API, and Zod for configuration validation.

## How to Run

```bash
# Build dependency projects first
NATIVE_LIBS_RELEASE_TAG=blob-libs-v1.2.0 pnpm run -F linea-native-libs build && \
pnpm run -F linea-shared-utils build && \
pnpm run -F "./sdk/*" build

# Build the postman
pnpm -F @consensys/linea-postman run build

# Run locally (requires local stack + PostgreSQL)
pnpm exec ts-node src/main.ts

# Run as part of full stack
make start-env-with-tracing-v2
```

### Test

```bash
# Unit tests
pnpm -F @consensys/linea-postman run test
```

### Lint

```bash
pnpm -F @consensys/linea-postman run lint
pnpm -F @consensys/linea-postman run lint:fix
```

## TypeScript-Specific Conventions

- **Build tool:** tsc (`tsconfig.build.json`)
- **Runtime:** Node.js (entry point: `src/main.ts`, production: `dist/src/main.js`)
- **Blockchain library:** Viem
- **ORM:** TypeORM 0.3 with PostgreSQL
- **Configuration:** Zod schema validation (`src/application/postman/app/config/schema.ts`)
- **Logging:** Winston via `@consensys/linea-shared-utils`
- **Metrics:** Prometheus via `prom-client`

### Directory Structure

```
postman/
├── src/
│   ├── main.ts                          Entry point
│   ├── application/postman/app/         Application layer
│   │   ├── PostmanApp.ts                Lifecycle orchestrator (start/stop)
│   │   ├── PostmanContainer.ts          Dependency wiring / service builder
│   │   ├── L1ToL2App.ts                 L1→L2 message processing pipeline
│   │   ├── L2ToL1App.ts                 L2→L1 message processing pipeline
│   │   └── config/                      Env loading, Zod schemas, defaults
│   ├── core/                            Domain interfaces and types
│   │   ├── clients/blockchain/          Blockchain client interfaces
│   │   ├── services/                    Service interfaces (signing, retries, nonce)
│   │   ├── persistence/                 Repository interfaces
│   │   ├── metrics/                     Metrics interfaces
│   │   ├── enums/                       Direction, MessageStatus, etc.
│   │   ├── errors/                      Error types
│   │   ├── constants/                   Default values
│   │   └── types/                       Shared types (hex, log, receipt)
│   ├── infrastructure/                  Implementations
│   │   ├── blockchain/viem/             Viem-based blockchain clients
│   │   │   ├── clients/                 LineaRollup, L2MessageService clients
│   │   │   ├── providers/               Viem/Linea providers
│   │   │   ├── gas/                     Gas estimation (Ethereum + Linea)
│   │   │   ├── signers/                 Private key + Web3Signer support
│   │   │   └── mappers/                 Viem type mappers
│   │   ├── persistence/                 TypeORM entities, repos, migrations
│   │   ├── metrics/                     Prometheus metrics updaters
│   │   └── api/                         Express API (metrics, health)
│   ├── services/                        Business logic processors and pollers
│   │   ├── processors/                  Event processing, claiming, anchoring
│   │   └── pollers/                     Interval-based polling
│   └── utils/testing/                   Test helpers, fixtures, mocks
├── scripts/                             CLI scripts (send messages for testing)
└── Dockerfile                           Multi-stage production Docker image
```

### Key Dependencies

| Dependency | Purpose |
|------------|---------|
| `viem` | Blockchain interactions (public/wallet clients, ABI encoding) |
| `@consensys/linea-sdk-core` | Core SDK types and utilities |
| `@consensys/linea-sdk-viem` | Viem-based SDK integration |
| `@consensys/linea-native-libs` | Native library bindings (blob compressor) |
| `@consensys/linea-shared-utils` | Shared utilities (logging, metrics, server) |
| `typeorm` | PostgreSQL ORM with migrations |
| `zod` | Configuration schema validation |
| `filtrex` | Calldata filter expression parsing |
| `winston` | Structured logging |
| `prom-client` | Prometheus metrics |

### Testing

- Framework: Jest 29.7.0 with ts-jest preset
- Uses `jest-mock-extended` for mock generation
- `--forceExit` and `--detectOpenHandles` flags enabled
- Test files: `**/__tests__/*.test.ts` pattern

### Database Migrations

Migrations live in `src/infrastructure/persistence/migrations/`. Create new ones with:

```bash
MIGRATION_NAME=<NAME> pnpm run migration:create
```

Migrations are written manually — TypeORM CLI auto-generation is not used due to incompatibility.

## Postman-Specific Safety Rules

- Never log private keys, signer secrets, or database passwords
- Signer configuration supports `private-key`, `web3signer`, and `aws-kms` types — test all three paths when modifying signer logic. The `aws-kms` adapter requires an async `init()` (performed inside `createSignerClient`), so `createSignerClient` is `async` and must be awaited.
- Nonce management is critical for transaction ordering — changes to `NonceManager` require careful review
- Gas bumping (`MAX_BUMPS_PER_CYCLE`, `MAX_RETRY_CYCLES`) affects transaction cost — validate defaults in `core/constants/common.ts`
- Database migration files are append-only — never modify existing migrations

## Agent Rules (Overrides)

- Run `pnpm -F @consensys/linea-postman run lint` before proposing TypeScript changes
- Configuration changes must be reflected in both `config/schema.ts` (Zod) and `config/envLoader.ts`
- New environment variables must be added to `.env.sample` with sensible defaults
- Changes to blockchain client interfaces (`core/clients/`) affect both L1→L2 and L2→L1 paths — test both directions
- TypeORM entity changes may require a new migration — check `infrastructure/persistence/entities/`
