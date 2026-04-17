# AGENTS.md — sdk

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

TypeScript SDK libraries for interacting with the Linea protocol. Three packages: `sdk-core` (framework-agnostic types and utilities), `sdk-ethers` (ethers.js v6 wrapper with typechain-generated contract bindings), and `sdk-viem` (Viem wrapper).

## How to Run

```bash
# Build all SDKs
pnpm -F @consensys/linea-sdk-core run build
pnpm -F @consensys/linea-sdk run build
pnpm -F @consensys/linea-sdk-viem run build

# Test all SDKs
pnpm -F @consensys/linea-sdk-core run test
pnpm -F @consensys/linea-sdk run test
pnpm -F @consensys/linea-sdk-viem run test

# Lint
pnpm -F @consensys/linea-sdk-core run lint
pnpm -F @consensys/linea-sdk run lint
pnpm -F @consensys/linea-sdk-viem run lint
```

## SDK-Specific Conventions

### Package Differences

| Package | Build Tool | Output | Key Dependency |
|---------|-----------|--------|----------------|
| `sdk-core` | tsup | CJS + ESM + DTS | abitype |
| `sdk-ethers` | tsc (+ typechain pre-step) | CJS + DTS | ethers 6.13.7 |
| `sdk-viem` | tsup | CJS + ESM + DTS | viem (peer dep >= 2.22.0) |

### Dependency Chain

```
postman -> sdk-viem -> sdk-core
```

- `sdk-ethers` requires `pnpm -F @consensys/linea-sdk run build:pre` (typechain) before build
- `sdk-viem` declares `viem` as a peer dependency — consumers must provide it

### Testing

- Framework: Jest 29.7.0 with ts-jest preset
- `sdk-ethers` uses `--forceExit` and `jest-mock-extended`
- Coverage: HTML, LCOV, and text reporters
- Test files: `*.test.ts` pattern

### Directory Structure

```
sdk/
├── sdk-core/       Core types, utilities (framework-agnostic)
├── sdk-ethers/     Ethers.js v6 integration with typechain contract bindings
└── sdk-viem/       Viem integration
```

## npm Publication

### Published Packages

| Package | npm | Scope |
|---------|-----|-------|
| `sdk-core` | `@consensys/linea-sdk-core` | public |
| `sdk-viem` | `@consensys/linea-sdk-viem` | public |

`sdk-ethers` is **not** published to npm (out of scope).

### Versioning and Changelog

- Independent versioning — each package has its own version in `package.json`.
- Both packages start at `1.0.0`.
- Each package maintains a `CHANGELOG.md` auto-generated from git commits scoped to its directory.

### How to Release

Two workflows handle the full release cycle:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `sdk-release.yml` | Manual dispatch | Creates a release PR (version bump + changelog) |
| `sdk-publish.yml` | Manual dispatch | Builds, tests, publishes to npm |

**Step 1 — Create a release PR:**

Go to **Actions → sdk-release → Run workflow**, pick the package and bump type. The workflow creates a PR with the version bumped in `package.json` and a changelog entry auto-generated from git commits since the last release.

**Step 2 — Review and merge:**

Review the PR (changelog, version). Merge to `main`.

**Step 3 — Publish:**

Go to **Actions → sdk-publish → Run workflow**, select the package. CI builds, tests, lints, and publishes to npm.

**Dependency order:** When releasing both packages, release and publish `sdk-core` first — `sdk-viem` depends on it. The publish workflow enforces this ordering.

### workspace:* Protocol

`sdk-viem` declares `"@consensys/linea-sdk-core": "workspace:*"` in `dependencies`. `pnpm publish` automatically resolves `workspace:*` to the actual version at publish time. Do not change this to a pinned version in source.

### CI

- **PR guard:** `.github/workflows/sdk-testing.yml` runs build, test, and lint on PRs touching `sdk/**`.
- **Release:** `.github/workflows/sdk-release.yml` creates a release PR (version bump + auto-changelog) via manual dispatch.
- **Publish:** `.github/workflows/sdk-publish.yml` builds, tests, and publishes to npm via manual dispatch.

### Prerequisites for Publishing

- The `@consensys` npm scope must exist and the publishing account must have write access.
- An `NPM_TOKEN` secret must be configured in the GitHub repository settings.

## Agent Rules (Overrides)

- Changes to `sdk-core` affect both `sdk-ethers` and `sdk-viem` — test downstream packages
- `sdk-ethers` typechain types are generated from ABIs — regenerate with `build:pre` after ABI changes
- Public API changes must follow versioning rules (new versioned exports, deprecate old)
