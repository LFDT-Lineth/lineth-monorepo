# contracts/deployments

This directory contains two categories of deployment artefacts:

## addresses/

PR-reviewed, per-network registry of stable deployed contract addresses. Entries may be edited manually or generated from an external source of truth, then validated before use.

See **[addresses/README.md](addresses/README.md)** for:

- How addresses are resolved during deploys (registry vs env var precedence)
- How to update an address (PR-based, auditable)
- The full registry key-to-env-var mapping

## bytecode/

Pinned, audit-signed contract bytecode snapshots used by `*Artifacts` deploy scripts (e.g. `21_deploy_YieldManagerArtifacts.ts`). Each subdirectory is named by audit date and contains the exact bytecode committed at that snapshot.

| Directory | Contents |
|-----------|----------|
| `bytecode/2024-12-03/` | LineaRollup, L2MessageService, TokenBridge (Dec 2024 audit) |
| `bytecode/2026-01-14/` | YieldManager, ValidatorContainerProofVerifier, LidoStVaultYieldProviderFactory (Jan 2026 audit) |
| `bytecode/2026-02-17/` | LineaRollup, L2MessageService, TokenBridge (Feb 2026 audit) |
| `bytecode/2025-10-27/` | RollupRevenueVault, V3DexSwapAdapter, L1LineaTokenBurner (Oct 2025 audit) |
| `bytecode/mainnet-proxy/` | TransparentUpgradeableProxy and ProxyAdmin bytecode (mainnet-verified) |
