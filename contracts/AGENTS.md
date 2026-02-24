# Contracts Agent Guidelines

## `local-deployments-artifacts/`

Hardhat compilation artifacts consumed by the E2E test pipeline (`e2e/scripts/generateAbi.ts`).

| Subdirectory | Contents |
|---|---|
| `deployed-artifacts/` | Artifacts for production on-chain deployments |
| `static-artifacts/` | Artifacts for non-upgradeable contracts used in E2E tests |
| `dynamic-artifacts/` | Artifacts for upgradeable contracts |

### Producing an artifact

1. Run `pnpm hardhat compile` in `/contracts` (requires `pnpm i` first).
2. Find the JSON build artifact in `contracts/build/src/`.
3. Copy it to the appropriate subdirectory above.
