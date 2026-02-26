# CLAUDE.md

This file provides context for Claude Code. All conventions, commands, and rules
are maintained in a single source of truth.

## Primary Reference

Read [AGENTS.md](AGENTS.md) for the complete repository guide.

## Package-Specific Rules

- [contracts/AGENTS.md](contracts/AGENTS.md) — Solidity smart contract conventions, safety rules, NatSpec requirements
- [coordinator/AGENTS.md](coordinator/AGENTS.md) — Kotlin coordinator service conventions
- [prover/AGENTS.md](prover/AGENTS.md) — Go ZK prover conventions
- [corset/AGENTS.md](corset/AGENTS.md) — Rust constraint compiler conventions
- [tracer/AGENTS.md](tracer/AGENTS.md) — EVM tracer (Java/Go Corset) conventions
- [besu-plugins/AGENTS.md](besu-plugins/AGENTS.md) — Besu plugin conventions
- [transaction-exclusion-api/AGENTS.md](transaction-exclusion-api/AGENTS.md) — Transaction exclusion API conventions
- [sdk/AGENTS.md](sdk/AGENTS.md) — TypeScript SDK conventions (core, ethers, viem)
- [bridge-ui/AGENTS.md](bridge-ui/AGENTS.md) — Next.js/React frontend conventions

## Quick Commands

```bash
pnpm install                              # Install all dependencies
pnpm run build                            # Build all pnpm packages
pnpm run test                             # Test all pnpm packages
pnpm run lint                             # Lint all pnpm packages
pnpm run lint:fix                         # Auto-fix lint issues
pnpm -F contracts run test                # Test smart contracts
pnpm -F contracts run lint:fix            # Lint + format contracts
./gradlew :coordinator:app:test           # Test coordinator (Kotlin)
make start-env                            # Start full local stack (Docker)
```

## Key Constraints

- **pnpm only** — enforced via `preinstall`; never use npm or yarn
- **Node >= 22.22.0** — see `.nvmrc`
- **Strict TypeScript** — `strict: true`, ES2022 target, nodenext modules
- **No secrets in code** — load from env vars; use placeholders in templates
- **Solidity: NatSpec required** — every public/external function, event, and error
- **Solidity: named imports only** — `import { X } from "./X.sol"`
- **API versioning** — never break existing public APIs; create versioned alternatives
- **Double quotes, trailing commas, semicolons** — Prettier config enforced
