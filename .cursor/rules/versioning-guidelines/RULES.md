---
description: API and Asset Versioning - Rules for backwards compatibility
globs: "**/*.ts, **/*.js, **/*.kt, **/*.java, **/*.go, **/*.sol, **/*.py, **/*.abi, **/*.json"
alwaysApply: true
---

# API and Asset Versioning Rules

Components must maintain backwards compatibility, especially those already released beyond devnet (sepolia, external partners). This is critical for Linea Stack/Enterprise, where components (e.g. coordinator, contracts, tracing modules) must coexist across multiple versions.

Refer to [EXAMPLES.md](EXAMPLES.md) for detailed do/don't examples.

## MUST

- Create a new versioned method when introducing breaking changes to a public API consumed by another component. The existing method must remain functional so consumers can migrate on their own schedule.
- Mark the old method as deprecated with a pointer to the new version.
- Create a new versioned asset (e.g. `LineaRollupV7.abi`, `LineaRollupV8.abi`) instead of overwriting an existing one.
- Use simple incremental version suffixes per method/asset (V1, V2, V3), similar to Ethereum Engine API conventions (`engine_getPayloadV1` ... `engine_getPayloadV5`).
- Bump the version number when a breaking change is introduced to that specific method or asset - not when unrelated parts of the system change.

## MUST NOT

- Refactor, rename, or change the signature of an existing public API method in a way that breaks current consumers.
- Override or replace a versioned asset (e.g. ABI file) that is already deployed and consumed by external partners or environments beyond devnet.
- Tie method/asset version numbers to an unrelated release train (e.g. `linea-besu-package` versions) - components are independently versioned.

## Scope

These rules apply when:
- The component is released on any environment other than devnet (sepolia, mainnet, external partners).
- The public API or asset is consumed by another component or external integrator.

For internal-only, devnet-only code with no external consumers, lighter versioning practices are acceptable.

## Enforcement

When a PR modifies a public API signature or overwrites an existing versioned asset, **flag the change** and ask:
1. Is this method/asset consumed by another component or external partner?
2. If yes, has a new versioned method/asset been created instead of modifying the existing one?
3. Is the old version deprecated with a pointer to the new one?

Block the change if the answer to (1) is yes and (2) or (3) is no. Reference [EXAMPLES.md](EXAMPLES.md) for correct patterns.
