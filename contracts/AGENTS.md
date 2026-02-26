# AGENTS.md — Contracts Agent Guidelines

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

Solidity smart contracts for the Linea protocol: L1 rollup (LineaRollup), messaging services (L1/L2MessageService), token bridge (TokenBridge), and supporting libraries. Built with Hardhat and Foundry.

## How to Run

```bash
# Install dependencies
pnpm install

# Build (compile contracts)
pnpm -F contracts run build

# Run tests
pnpm -F contracts run test

# Run tests with gas reporting
pnpm -F contracts run test:reportgas

# Coverage
pnpm -F contracts run coverage

# Lint Solidity
pnpm -F contracts run lint:sol
pnpm -F contracts run lint:sol:fix

# Lint TypeScript (test/deploy scripts)
pnpm -F contracts run lint:ts
pnpm -F contracts run lint:ts:fix

# Format Solidity
pnpm -F contracts run prettier:sol
pnpm -F contracts run prettier:sol:fix

# Full lint + format
pnpm -F contracts run lint:fix

# Generate Solidity docs (requires Foundry)
pnpm -F contracts run solidity:docgen
```

## Solidity-Specific Conventions

### Compiler and EVM

- Solidity version: `0.8.33` (exact for contracts, caret `^0.8.33` for interfaces/abstract/libraries)
- EVM version: osaka (Hardhat), cancun (Foundry)
- OpenZeppelin contracts: 4.9.6

### Licenses

- Interfaces: `// SPDX-License-Identifier: Apache-2.0`
- Contracts: `// SPDX-License-Identifier: AGPL-3.0`

### NatSpec Documentation

Every public/external function, event, and error MUST have NatSpec:
- `@notice` on every public/external function
- `@param` for every parameter (in signature order)
- `@return` for every return value (named)
- `@author Consensys Software Inc.` and `@custom:security-contact security-report@linea.build` on every contract/interface
- `@dev` for non-obvious implementation details
- `DEPRECATED` in NatSpec for deprecated items

### Naming

| Item | Convention | Example |
|------|-----------|---------|
| Public state | camelCase | `uint256 messageCount` |
| Private/internal state | _camelCase | `uint256 _internalCounter` |
| Constants | UPPER_SNAKE_CASE | `bytes32 DEFAULT_ADMIN_ROLE` |
| Function params | _camelCase | `function send(address _to)` |
| Return variables | camelCase (named) | `returns (bytes32 messageHash)` |
| Mappings | descriptive keys | `mapping(uint256 id => bytes32 hash)` |
| Init functions | __Contract_init | `__PauseManager_init()` |

### File Layout

**Interface:** Structs -> Enums -> Events -> Errors -> External Functions

**Contract:** Using statements -> Constants -> State variables -> Structs -> Enums -> Events -> Errors -> Modifiers -> Functions

### Imports

Named imports only. Explicit inheritance of key ancestors. Blank line after import block.

### Gas Optimization

- `external` + `calldata` for functions accepting arrays/structs
- Cache storage values read multiple times
- Custom errors over revert strings
- `unchecked` only with proven safe arithmetic (with comment)
- Short-circuit: cheap checks before expensive ones
- Explicit batch limits for loops

### Visibility

- Constants: `internal` unless explicitly needed public
- Overridable functions: `public virtual` (not `external virtual`)
- Minimize public surface area
- Explicit visibility on all state variables

## Solidity-Specific Safety Rules

- Upgradeable contracts use ERC-7201 namespaced storage (not storage gaps)
- Both `initialize` and `reinitializeVN` use `reinitializer(N)` (never `initializer`)
- Zero-value checks via `ErrorUtils.revertIfZeroAddress()` / `ErrorUtils.revertIfZeroHash()`
- Assembly: hex for memory offsets (`mstore(add(mPtr, 0x20), _var)`)
- Repeated checks extracted into modifiers
- No magic numbers — use named constants
- Never deploy without `VERIFY_CONTRACT=true` for block explorer verification
- Contract modifications from audited code require independent audit

## Testing

- Hardhat tests: `test/hardhat/` — TypeScript test files
- Foundry tests: `test/foundry/` — Solidity test files
- Coverage: `SOLIDITY_COVERAGE=true` flag with `.solcover.js` config
- CI runs coverage and uploads to Codecov with `hardhat` flag

## Agent Rules (Overrides)

- Always read the existing interface before modifying a contract
- Check storage layout compatibility before changing state variables in upgradeable contracts
- Run `pnpm -F contracts run lint:fix` before committing any Solidity changes
- For detailed rules, see `.agents/skills/developing-smart-contracts/` and `.cursor/rules/smart-contract-guidelines/`

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