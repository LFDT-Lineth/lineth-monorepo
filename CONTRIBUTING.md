# Contributing

## Prerequisites

| Tool | Version |
|------|---------|
| Node.js | >= 22.22.0 (see `.nvmrc`) |
| pnpm | >= 10.28.0 |
| JDK | 21 (for coordinator, Besu plugins) |
| Docker | 24+ with 16 GB memory, 4+ CPUs |
| Docker Compose | 2.19+ |
| Make | 3.81+ |

## Setup

```bash
git clone https://github.com/Consensys/linea-monorepo.git
cd linea-monorepo
nvm use          # or install Node 22.22.0
pnpm install     # installs all workspaces + sets up Husky hooks
```

## Development

```bash
# Smart contracts
pnpm -F contracts run build
pnpm -F contracts run test

# Bridge UI
pnpm -F bridge-ui dev

# Coordinator (Kotlin)
./gradlew :coordinator:app:build

# Full local stack (L1 + L2 + contracts)
make start-env
```

See [docs/local-development-guide.md](docs/local-development-guide.md) for detailed coordinator setup.

## Before Submitting a PR

```bash
# TypeScript/JavaScript
pnpm run lint:fix
pnpm run test

# Smart contracts
pnpm -F contracts run lint:fix
pnpm -F contracts run test

# Kotlin/Java
./gradlew spotlessApply
./gradlew test

# Go (prover)
cd prover && gofmt -w . && golangci-lint run && go test ./... -tags nocorset,fuzzlight
```

## PR Process

1. Create a branch: `type/issue#-short-description` (e.g., `feature/123-add-login-button`)
2. Implement changes with tests
3. Run lint and test commands for affected packages
4. Commit: `type(issue#): short description` (e.g., `fix(456): corrected login error`)
5. Open a PR to `main` using the PR template
6. Ensure CI passes; respond to review feedback
7. External PRs follow the standard deployment flow with Consensys engineer support

## Rules

- **pnpm only** — npm and yarn are blocked
- **No secrets** — use environment variables and `.env.template` placeholders
- **Tests required** — all new logic must have corresponding tests
- **Breaking API changes** — create new versioned methods/assets, never modify existing ones
- **Solidity NatSpec** — every public/external function, event, and error must have documentation
- **Husky pre-commit** — runs automatically on commit; do not skip with `--no-verify`

## Package-Specific Guides

- **Smart contracts:** See [contracts/AGENTS.md](contracts/AGENTS.md) and [contracts/docs/contract-style-guide.md](contracts/docs/contract-style-guide.md)
- **Bridge UI:** See [bridge-ui/AGENTS.md](bridge-ui/AGENTS.md)
- **Prover:** See [prover/AGENTS.md](prover/AGENTS.md)

## Additional Resources

- [AGENTS.md](AGENTS.md) — Complete repository guide (commands, conventions, architecture)
- [docs/contribute.md](docs/contribute.md) — Detailed contribution guidelines and release process
- [docs/security.md](docs/security.md) — Security policy and vulnerability reporting
- [docs/code-of-conduct.md](docs/code-of-conduct.md) — Community guidelines
