# AGENTS.md

## Documentation Precedence

- `AGENTS.md` is the canonical source for repository-wide agent instructions.
- Tool-specific files should stay concise and link back to `AGENTS.md`.
- If instructions conflict, follow `AGENTS.md`.

## Agent Entry Points

- Codex: `AGENTS.md` (this file)
- Cursor: `.cursor/rules/documentation.mdc`, `.cursor/BUGBOT.md`, then `AGENTS.md`
- Claude Code: `CLAUDE.md`, then `AGENTS.md`
- GitHub Copilot: `.github/copilot-instructions.md`, then `AGENTS.md`

## Discoverability Index

- Repository overview: `README.md`
- Contribution process: `CONTRIBUTING.md`, `docs/contribute.md`
- Local setup: `docs/get-started.md`, `docs/local-development-guide.md`
- Architecture: `docs/architecture-description.md`
- Engineering guidelines: `docs/development-guidelines.md`
- Security and audits: `docs/security.md`, `docs/audits.md`
- Package-specific agent rules: `*/AGENTS.md` (`contracts/`, `coordinator/`, `prover/`, `tracer/`, `sdk/`, `bridge-ui/`, `besu-plugins/`, `transaction-exclusion-api/`, `e2e/`)

## Project Guidelines

- [contracts/AGENTS.md](contracts/AGENTS.md) — Solidity contracts, deployment artifacts
- [e2e/AGENTS.md](e2e/AGENTS.md) — E2E tests, ABI generation

## Continuous Improvement

When a task requires a significant course correction (e.g., wrong architecture choice, misunderstood repository pattern, missed convention), propose an update to the relevant `AGENTS.md`.

For each proposal, include:
- What failed or was ambiguous
- Why it caused rework or risk
- The concrete rule to add or modify
- The correct scope (`AGENTS.md` at root vs package-level `*/AGENTS.md`)

Only propose rules that are repository-specific and repeatable.

## Repository

Linea zkEVM monorepo — the principal repository for [Linea](https://linea.build), a Layer 2 zero-knowledge rollup scaling Ethereum. Contains smart contracts, ZK prover, coordinator, postman (bridge message executor), bridge UI, SDKs, and supporting tooling. Licensed under Apache-2.0 and MIT.

## How to Run

### Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Node.js | >= 22.22.0 | See `.nvmrc` |
| pnpm | >= 10.28.0 | Enforced via `preinstall` |
| JDK | 21 | Coordinator, Besu plugins, transaction-exclusion-api |
| Gradle | 8.5+ | JVM service builds |
| Go | 1.24.6 | Prover |
| Docker | 24+ | Local stack, CI |
| Docker Compose | 2.19+ | Multi-service orchestration |
| Make | 3.81+ | Environment management |
| Foundry | stable | Solidity testing and docgen |

### Install

```bash
pnpm install
```

This triggers `husky` setup via the `prepare` script. Only `pnpm` is allowed (enforced by `preinstall`).

### Dev

| Package | Command |
|---------|---------|
| Bridge UI | `pnpm -F bridge-ui dev` |
| Full local stack | `make start-env` |
| L1 + L2 with tracing | `make start-env-with-tracing-v2` |

### Build

```bash
# All pnpm packages
pnpm run build

# Coordinator (Kotlin)
./gradlew :coordinator:app:build

# Prover (Go)
cd prover && make build

# Contracts (Solidity)
pnpm -F contracts run build

# Bridge UI (Next.js)
pnpm -F bridge-ui run build
```

### Test

| Scope | Command |
|-------|---------|
| All pnpm packages | `pnpm run test` |
| Contracts (Hardhat) | `pnpm -F contracts run test` |
| Contracts (coverage) | `pnpm -F contracts run coverage` |
| SDK core | `pnpm -F @consensys/linea-sdk-core run test` |
| SDK ethers | `pnpm -F @consensys/linea-sdk run test` |
| SDK viem | `pnpm -F @consensys/linea-sdk-viem run test` |
| Postman | `pnpm -F @consensys/linea-postman run test` |
| E2E (requires local stack) | `pnpm -F e2e run test:local` |
| Bridge UI unit | `pnpm -F bridge-ui run test:unit` |
| Bridge UI E2E | `pnpm -F bridge-ui run test:e2e:headless` |
| Coordinator (Kotlin) | `./gradlew :coordinator:app:test` |
| Prover (Go) | `cd prover && go test ./... -tags nocorset,fuzzlight -timeout 30m` |
| Native libs | `pnpm -F @consensys/linea-native-libs run test` |
| Shared utils | `pnpm -F @consensys/linea-shared-utils run test` |
| Automation service | `pnpm -F @consensys/linea-native-yield-automation-service run test` |
| Lido governance monitor | `pnpm -F @consensys/lido-governance-monitor run test` |

### Lint and Format

| Scope | Check | Fix |
|-------|-------|-----|
| All pnpm packages | `pnpm run lint` | `pnpm run lint:fix` |
| Contracts — Solidity | `pnpm -F contracts run lint:sol` | `pnpm -F contracts run lint:sol:fix` |
| Contracts — TypeScript | `pnpm -F contracts run lint:ts` | `pnpm -F contracts run lint:ts:fix` |
| Contracts — Prettier | `pnpm -F contracts run prettier:sol` | `pnpm -F contracts run prettier:sol:fix` |
| JVM — Spotless | `./gradlew spotlessCheck` | `./gradlew spotlessApply` |
| Prover (Go) | `cd prover && gofmt -l . && golangci-lint run` | `cd prover && gofmt -w .` |

Prettier config: `prettier.config.mjs`. Formatting is integrated into `lint:fix`.

### Typecheck

- Bridge UI: `pnpm -F bridge-ui run check-types`
- Other TS packages: typecheck is part of build (`tsc`)

## Code Conventions

### Style

- **Formatter:** Prettier 3.7.4 — `prettier.config.mjs`
- **Linter (TS/JS):** ESLint 9.39.2 flat config — `ts-libs/eslint-config/`
- **Linter (Solidity):** Solhint 6.0.3 + Prettier plugin
- **Linter (Kotlin/Java):** Spotless with ktlint + Google Java Format
- **Linter (Go):** gofmt + golangci-lint
- **Line length:** 120 characters
- **Indentation:** 2 spaces (4 for Go tabs)
- **Trailing commas:** always (TS/JS)
- **Semicolons:** always (TS/JS)
- **Quotes:** double (TS/JS)
- **Line endings:** LF (enforced via `.gitattributes` and `.editorconfig`)
- **TypeScript:** strict mode, ES2022 target, nodenext module resolution

### Structure

- **Monorepo tool:** pnpm workspaces (`pnpm-workspace.yaml`)
- **Dual build systems:** pnpm for TypeScript/JavaScript, Gradle for Kotlin/Java, Make+Go for prover
- **Dependency catalog:** Shared versions in `pnpm-workspace.yaml` catalog section
- **Shared ESLint config:** `@consensys/eslint-config` with exports for default, `./nextjs`, and `./node`

### Naming

| Context | Convention | Example |
|---------|-----------|---------|
| TS/JS files | kebab-case | `message-service.ts` |
| React components | PascalCase | `BridgeForm.tsx` |
| Solidity files | PascalCase | `LineaRollup.sol` |
| Solidity interfaces | `I` prefix + PascalCase | `ILineaRollup.sol` |
| Kotlin files | PascalCase | `CoordinatorApp.kt` |
| Go files | snake_case | `blob_compressor.go` |
| Branch names | `type/issue#-short-description` | `feature/123-add-login-button` |
| Commit messages | `type(issue#): description` | `fix(456): corrected login error` |

### Imports

- **TypeScript:** ESLint import ordering enforced via `eslint-plugin-import`
- **Solidity:** Named imports only (`import { X } from "./X.sol"`)
- **Kotlin:** Star imports disabled (see `.editorconfig`)

### Testing Guidelines

| Area | Framework | Notes |
|------|----------|-------|
| Contracts (Hardhat) | Hardhat + ethers.js | `pnpm -F contracts run test` |
| Contracts (Foundry) | Forge | `test/foundry/*` |
| TypeScript packages | Jest 29.7.0 + ts-jest | `pnpm -F <pkg> run test` |
| Bridge UI unit | Playwright | `pnpm -F bridge-ui run test:unit` |
| Bridge UI E2E | Playwright + Synpress | `pnpm -F bridge-ui run test:e2e:headless` |
| Coordinator | JUnit 5 + Mockito + WireMock | `./gradlew :coordinator:app:test` |
| Prover | Go test | `go test ./... -tags nocorset,fuzzlight` |
| E2E (protocol) | Jest | `pnpm -F e2e run test:local` |

- Coverage: Codecov with flags for `hardhat` and `kotlin`
- `linea-shared-utils` enforces strict coverage thresholds (branches: 85.71%, functions: 100%, lines: 95.23%)
- Protocol E2E tests require a running local stack (`make start-env`)

## Contribution Workflow

### Branches and Commits

- **Strategy:** Trunk-based development on `main`
- **Branch naming:** `type/issue#-short-description` (e.g., `feature/123-add-login-button`, `bugfix/456-fix-login-error`)
- **Commit format:** `type(issue#): short description` (e.g., `fix(456): corrected login error`)
- **Base branch:** `main`

### Pull Requests

- PR template: `.github/pull_request_template.md`
- CI must pass: tests, linting, security analysis, coverage
- PRs are reviewed by maintainers
- External PRs follow the standard deployment flow
- CODEOWNERS: `/contracts/` requires review from `@Consensys/linea-contract-team`

### Checklist Before PR

- [ ] Tests pass locally (`pnpm run test` or package-specific test command)
- [ ] Linting passes (`pnpm run lint`)
- [ ] Solidity: NatSpec complete for all public/external items
- [ ] Solidity: Named imports used, linting passes (`pnpm -F contracts run lint:fix`)
- [ ] Kotlin/Java: Spotless passes (`./gradlew spotlessCheck`)
- [ ] Go: `gofmt` and `golangci-lint run` pass
- [ ] No secrets, credentials, or real private keys in code
- [ ] Breaking API changes: new versioned method/asset created (see versioning rules)
- [ ] Environment variable changes reflected in `.env.example` / `.env.template` files
- [ ] Relevant issue linked in PR description

## Safety and Quality

### Secrets

- Load secrets from environment variables or secure vaults exclusively
- `.env` files are in `.gitignore`; only `.env.template` / `.env.example` files committed with placeholders
- Never log secrets, API keys, private keys, or credentials (see `.cursor/rules/security-logging-guidelines/`)
- Test fixtures use obviously fake credentials (e.g., Hardhat default keys for local-only dev)

### Security

- Security contact: `security-report@linea.build`
- Bug bounty: [Immunefi program](https://immunefi.com/bounty/linea/)
- CodeQL runs on all PRs (Go, Java-Kotlin, JavaScript-TypeScript, Python, GitHub Actions)
- Weekly Dockerfile security scan via KICS
- Dependency security overrides managed in `pnpm-workspace.yaml` (`pnpm.overrides`)
- Smart contracts: See `contracts/AGENTS.md` for Solidity-specific security rules

### Dependencies

- **pnpm catalog:** Shared dependency versions in `pnpm-workspace.yaml`
- **Gradle version catalog:** `gradle/libs.versions.toml`
- **Security overrides:** Explicit in `pnpm-workspace.yaml` for known CVEs
- **Dependabot:** Configured for GitHub Actions dependencies (weekly, Monday 03:00 UTC)
- **Engine strict:** `engine-strict=true` in `.npmrc`

### Irreversible Operations

These require human approval and follow the release process:

- Smart contract deployments to testnet/mainnet
- Database migrations on shared environments
- Docker image pushes to production registries
- Maven Central releases (`maven-release.yml`)
- Linea Besu package releases
- Any changes to production configuration

## Agent Rules

### When to Ask vs Assume

**Ask** when:
- The change affects a public API consumed by another component or external partner
- The change touches smart contract storage layout or upgrade logic
- The change modifies CI/CD workflows or deployment pipelines
- The change introduces a new dependency
- The scope is ambiguous or crosses multiple packages

**Assume** when:
- Adding tests for existing code
- Fixing a clear bug with an obvious solution
- Updating documentation to match existing behavior
- Formatting or linting fixes

### Proposing Changes

1. Read existing code in the affected area
2. Check for existing tests, conventions, and patterns in that package
3. Plan the minimal change needed
4. Implement with tests
5. Run the package-specific lint and test commands
6. Verify no secrets or credentials are exposed

### Limiting Change Surface

- Change only what is directly requested
- Do not refactor surrounding code unless asked
- Do not add features beyond the scope
- Do not modify files outside the affected package unless necessary for the change
- Preserve existing code style in the file being edited

## Repository Map

### Packages

| Name | Type | Stack | Purpose |
|------|------|-------|---------|
| `contracts` | Smart contracts | Solidity 0.8.33, Hardhat, Foundry | Core protocol contracts (rollup, messaging, bridge, tokens) |
| `coordinator` | Backend service | Kotlin 2.3.0, Gradle, Vertx | Orchestrates proof submission, blob submission, finalization |
| `prover` | Backend service | Go 1.24.6 | ZK proof generation (gnark, gnark-crypto) |
| `postman` | Backend service | TypeScript, Express, TypeORM | Bridge message execution service |
| `bridge-ui` | Frontend | Next.js 16.1.5, React 19, Wagmi, Viem | Bridge user interface |
| `sdk/sdk-core` | Library | TypeScript, tsup | Core SDK utilities and types |
| `sdk/sdk-ethers` | Library | TypeScript, ethers.js 6 | SDK for ethers.js integration |
| `sdk/sdk-viem` | Library | TypeScript, tsup, Viem | SDK for Viem integration |
| `e2e` | Tests | TypeScript, Jest | Protocol-level end-to-end tests |
| `operations` | CLI tool | TypeScript, oclif | Operations management CLI |
| `ts-libs/eslint-config` | Config | ESLint 9 flat config | Shared ESLint configuration |
| `ts-libs/linea-native-libs` | Library | TypeScript, Koffi (FFI) | Native library bindings |
| `ts-libs/linea-shared-utils` | Library | TypeScript, Express, Viem | Shared utilities (server, metrics, logging) |
| `native-yield-operations/automation-service` | Backend service | TypeScript, Apollo | Automated native yield operations |
| `native-yield-operations/lido-governance-monitor` | Backend service | TypeScript, Prisma | Lido governance proposal monitoring |
| `besu-plugins` | Plugins | Kotlin, Gradle | Besu blockchain client plugins (sequencer, state recovery) |
| `jvm-libs` | Libraries | Kotlin, Gradle | Shared JVM libraries (JSON-RPC, HTTP, persistence, metrics) |
| `transaction-exclusion-api` | Backend service | Kotlin, Gradle, Vertx | Transaction exclusion tracking API |
| `tracer` | Backend service | Java/Go Corset, Gradle | EVM trace generation and arithmetization |
| `corset` | Compiler | Rust 2021 (1.70.0+), Cargo | Constraint system compiler (cdylib + CLI) |

### Key Directories

```
contracts/               Solidity smart contracts (Hardhat + Foundry)
coordinator/             Kotlin coordinator service
prover/                  Go ZK prover
corset/                  Rust constraint compiler
postman/                 TypeScript bridge message executor
bridge-ui/               Next.js bridge frontend
sdk/                     TypeScript SDKs (core, ethers, viem)
e2e/                     Protocol E2E tests
operations/              Operations CLI tool
ts-libs/                 Shared TypeScript libraries
native-yield-operations/ Native yield services
besu-plugins/            Besu client plugins
jvm-libs/                Shared Kotlin/Java libraries
transaction-exclusion-api/ Transaction exclusion API
tracer/                  EVM tracer
config/                  Service configuration files (TOML, JSON, XML)
docker/                  Docker Compose files for local stack
docs/                    Project documentation
.github/workflows/       CI/CD workflows (78 files)
.github/actions/         Custom GitHub Actions
.cursor/rules/           Cursor IDE rules
.agents/skills/          Agent skills (smart contract development)
```

### CI/CD

- **Platform:** GitHub Actions
- **Main workflow:** `.github/workflows/main.yml` — triggers on PR and push to `main`
- **Path filtering:** `dorny/paths-filter` detects which components changed
- **Pipeline:** Filter changed paths -> Run component tests -> Build Docker images -> E2E tests -> Publish
- **Coverage:** Codecov with Jacoco (JVM) and Hardhat coverage (Solidity)
- **Security:** CodeQL analysis, KICS Dockerfile scanning (weekly)
- **Runners:** Custom scale-set runners (small, med, large, xl) on Ubuntu 22.04
- **Concurrency:** Cancel in-progress runs on PRs; serial on `main`
- **Notifications:** Slack alerts on workflow failures; external contribution notifications

### Cross-Package Dependencies

```
bridge-ui -> @consensys/linea-sdk-viem -> @consensys/linea-sdk-core
postman -> @consensys/linea-sdk, @consensys/linea-native-libs, @consensys/linea-shared-utils
e2e -> @consensys/linea-shared-utils
operations -> (standalone, uses ethers + viem)
native-yield-operations/* -> @consensys/linea-shared-utils
coordinator -> jvm-libs/*
transaction-exclusion-api -> jvm-libs/*
besu-plugins -> jvm-libs/*
```

### Internal Docs

- [README](README.md) — Repository overview
- [Get Started](docs/get-started.md) — Quick start guide
- [Local Development Guide](docs/local-development-guide.md) — Coordinator local setup
- [Contributing](docs/contribute.md) — Contribution guidelines and release process
- [Security Policy](docs/security.md) — Vulnerability reporting
- [Code of Conduct](docs/code-of-conduct.md) — Community guidelines
- [Contract Style Guide](contracts/docs/contract-style-guide.md) — Solidity conventions
- [Contract Deployment](contracts/docs/deployment/README.md) — Deployment parameters
