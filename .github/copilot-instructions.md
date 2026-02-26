# GitHub Copilot Instructions

## Repository

Linea zkEVM monorepo — Layer 2 zero-knowledge rollup scaling Ethereum. Smart contracts, ZK prover, coordinator, postman, bridge UI, and SDKs.

## Key Commands

```bash
pnpm install                              # Install dependencies (pnpm only)
pnpm run build                            # Build all TS/JS packages
pnpm run test                             # Test all TS/JS packages
pnpm run lint:fix                         # Lint and auto-fix all
pnpm -F contracts run test                # Test smart contracts
pnpm -F contracts run lint:fix            # Lint contracts (Solidity + TS)
./gradlew :coordinator:app:test           # Test coordinator (Kotlin)
make start-env                            # Start full local stack
```

## Conventions

- **TypeScript:** strict mode, ES2022, nodenext modules, double quotes, trailing commas, 120-char lines
- **Solidity 0.8.33:** NatSpec required on all public items, named imports only, AGPL-3.0 license
- **Kotlin:** ktlint via Spotless, JUnit 5 tests
- **Go 1.24.6:** gofmt, golangci-lint
- **Formatting:** Prettier 3.7.4 (TS/JS/Sol), Spotless (Kotlin), gofmt (Go)

## Rules

- pnpm only — npm and yarn are blocked
- Never commit secrets, API keys, or private keys
- Solidity: custom errors over revert strings, `calldata` for read-only inputs
- Breaking API changes require new versioned methods (V1 -> V2)
- Branch naming: `type/issue#-short-description`
- Commit format: `type(issue#): description`
- Tests required for all new logic
- Run package-specific lint before committing

## Packages

| Name | Stack | Purpose |
|------|-------|---------|
| contracts | Solidity, Hardhat, Foundry | Core protocol smart contracts |
| coordinator | Kotlin, Gradle, Vertx | Proof/blob submission orchestrator |
| prover | Go, gnark | ZK proof generation |
| postman | TypeScript, Express, TypeORM | Bridge message executor |
| bridge-ui | Next.js 16, React 19, Wagmi | Bridge frontend |
| sdk/sdk-core | TypeScript, tsup | Core SDK types |
| sdk/sdk-ethers | TypeScript, ethers.js 6 | Ethers.js SDK |
| sdk/sdk-viem | TypeScript, tsup, Viem | Viem SDK |
| e2e | TypeScript, Jest | Protocol E2E tests |
| operations | TypeScript, oclif | Operations CLI |
| ts-libs/* | TypeScript | Shared libs (eslint-config, native-libs, shared-utils) |
| besu-plugins | Kotlin, Gradle | Besu client plugins (sequencer, state recovery) |
| tracer | Java/Go Corset, Gradle | EVM trace generation and arithmetization |
| transaction-exclusion-api | Kotlin, Gradle, Vertx | Transaction exclusion tracking API |
| corset | Rust, Cargo | Constraint system compiler |

## See Also

- [AGENTS.md](../AGENTS.md) — Complete repository guide
- [CONTRIBUTING.md](../CONTRIBUTING.md) — Contribution workflow
- [docs/contribute.md](../docs/contribute.md) — Detailed contribution and release process
