# Lido Governance Monitor

## Overview

The Lido Governance Monitor is a cronjob that checks Lido governance proposals and alerts the Security Council when high-risk proposals may impact Linea's Native Yield system.

Each run executes three sequential steps:

1. **ProposalFetcher** - Fetches new proposals from two sources: Lido's Discourse forum and on-chain LDO voting contract
2. **ProposalProcessor** - Performs AI-assisted risk analysis using Claude API, scoring proposals 0-100
3. **NotificationService** - Sends Slack alerts for proposals exceeding the risk threshold

Proposals flow through a state machine: `NEW` -> `ANALYZED` -> `NOTIFIED` or `NOT_NOTIFIED`. Failed operations are automatically retried on subsequent runs.

See [Architecture](./docs/architecture.md) for detailed system design, data flows, state machine, and risk assessment model.

## Folder Structure

```
lido-governance-monitor/
├── prisma/
│   └── schema.prisma         # Database schema for proposal state tracking
├── src/
│   ├── application/          # Application bootstrap and configuration
│   │   └── main/
│   │       ├── config/       # Zod-validated configuration schema
│   │       └── LidoGovernanceMonitorBootstrap.ts
│   ├── clients/              # External service clients
│   │   ├── db/               # Prisma repository implementation
│   │   ├── ClaudeAIClient.ts # Anthropic Claude API client
│   │   ├── DiscourseClient.ts # Lido forum API client
│   │   └── SlackClient.ts    # Slack webhook client
│   ├── core/                 # Interfaces and domain entities
│   │   ├── clients/          # Client interfaces (IAIClient, IDiscourseClient, ISlackClient)
│   │   ├── entities/         # Domain entities (Proposal, Assessment, ProposalState)
│   │   ├── repositories/     # Repository interfaces
│   │   └── services/         # Service interfaces
│   ├── prompts/              # AI system prompts for risk assessment
│   │   └── risk-assessment-system.md
│   ├── services/             # Business logic services
│   │   ├── fetchers/
│   │   │   ├── DiscourseFetcher.ts          # Fetches proposals from Lido Discourse forum
│   │   │   └── LdoVotingContractFetcher.ts  # Fetches proposals from on-chain LDO voting contract
│   │   ├── NormalizationService.ts  # Converts raw Discourse data to domain entities
│   │   ├── NotificationService.ts   # Sends Slack alerts for high-risk proposals
│   │   ├── ProposalFetcher.ts       # Orchestrates all fetcher sources
│   │   └── ProposalProcessor.ts     # AI-assisted risk analysis
│   ├── utils/
│   │   └── logging/           # Structured logging utilities
│   └── __tests__/
│       └── integration/      # Integration tests for proposal lifecycle
└── run.ts                    # Service entry point
```

## Configuration

All environment variables, defaults, and validation rules are defined in the [configuration schema](./src/application/main/config/index.ts). Copy `.env.sample` to `.env` and fill in the required values.

## Development

### Prerequisites

- Node.js 18+
- PostgreSQL database
- Anthropic API key
- Slack incoming webhook

### Setup

1. Install dependencies:
   ```bash
   pnpm install
   ```

2. Generate Prisma client:
   ```bash
   pnpm db:generate
   ```

3. Create database tables:
   ```bash
   pnpm db:push
   ```

4. Create `.env` file with required configuration (see Configuration section)

### Running

```bash
pnpm --filter @consensys/lido-governance-monitor exec tsx run.ts
```

### E2E Local Test

Run the full pipeline locally against the test database and live APIs:

1. Start the test database and apply migrations:
   ```bash
   make test-db
   ```

2. Run the app with env vars (use the test DB credentials; set `INITIAL_LDO_VOTING_CONTRACT_VOTEID` to skip historical on-chain votes):
   ```bash
   DATABASE_URL="postgresql://testuser:testpass@localhost:5433/lido_governance_monitor_test" \
   INITIAL_LDO_VOTING_CONTRACT_VOTEID="150" \
   DISCOURSE_PROPOSALS_URL="https://research.lido.fi/c/proposals/9/l/latest.json" \
   ANTHROPIC_API_KEY="your-key" \
   SLACK_WEBHOOK_URL="https://hooks.slack.com/services/xxx" \
   ETHEREUM_RPC_URL="https://mainnet.infura.io/v3/xxx" \
   LDO_VOTING_CONTRACT_ADDRESS="0x2e59a20f205bb85a89c53f1936454680651e618e" \
   pnpm --filter @consensys/lido-governance-monitor exec tsx run.ts
   ```

   Or add these to `.env` and run:
   ```bash
   pnpm --filter @consensys/lido-governance-monitor exec tsx run.ts
   ```

3. Clean up when done:
   ```bash
   make clean
   ```

### Build

```bash
pnpm --filter @consensys/lido-governance-monitor build
```

### Unit Tests

```bash
pnpm --filter @consensys/lido-governance-monitor test
```

### Lint

```bash
pnpm --filter @consensys/lido-governance-monitor lint
```

## License

This package is licensed under the [Apache 2.0](../../LICENSE-APACHE) and the [MIT](../../LICENSE-MIT) licenses.
