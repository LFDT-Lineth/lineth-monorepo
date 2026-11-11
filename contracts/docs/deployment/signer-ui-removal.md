# Removing the optional Hardhat signer UI (`HARDHAT_SIGNER_UI`)

[← Back to deployment index](README.md)

This document is for **maintainers** who want to delete the browser-signing stack entirely and return to **private-key / named-account** flows only. It lists integration points as of `HARDHAT_SIGNER_UI` / `signer-ui`.

Do **not** follow this guide for day-to-day use. Operators should leave `HARDHAT_SIGNER_UI` unset.

## What gets removed

| Area | Path / symbol |
|------|----------------|
| Plugin (flag, task-scoped hook, bridge) | `contracts/hardhat-plugins/signer-ui/`: `index.ts` (`--signer-ui`, `deploy:recover`), `session.ts` (`withSignerUi`), `bridge.ts` (session + network hook), `http.ts`, `next-dev.ts`, `transactions.ts`, `broadcast-log.ts`, `recover.ts`, `signer-mode.ts` (incl. `resolveDeployerAccounts`), `type-extensions.ts` |
| Next.js app | `contracts/signer-ui/` (local private Next.js app) |
| Config | `resolveDeployerAccounts()` in `contracts/hardhat.config.ts`; the deploy plugin (`hardhat-plugins/deploy/index.ts`) lists the signer-UI plugin as a dependency |
| Deploy plugin | `withSignerUi` around `ignition deploy` in `hardhat-plugins/deploy/deploy-action.ts`; `getUiWalletClient` in `ignition/modules/rollup/AddressFilter.deploy.ts` |
| Operational scripts | `withSignerUi`, `getUiWalletClient`, `setUiTransactionContext` in `scripts/operational/**`, `scripts/upgrades/**`, `scripts/testEIP7702/**` |
| Shared helpers | `common/helpers/signerUiWorkflowStatus.ts` |
| ESLint ignore | `signer-ui/**` in `contracts/eslint.config.mjs` |
| Documentation | Sections in `contracts/docs/deployment/README.md`, `chained-deployments.md`, `contracts/signer-ui/README.md`, this file; cross-links from `contracts/AGENTS.md`, `docs/tech/components/contracts.md` |

## 1. `hardhat.config.ts`

1. Remove `import("../signer-ui/index.js")` from the deploy plugin's `dependencies`.
2. Replace **`resolveDeployerAccounts()`** with the `DEPLOYER_PRIVATE_KEY` account (move the key validation next to `hardhat.config.ts`).

## 2. Delete UI and bridge implementation

- Delete **`contracts/hardhat-plugins/signer-ui/`** (this also removes `--signer-ui` and the `deploy:recover` task).
- Delete the directory **`contracts/signer-ui/`**.

## 3. Deploy plugin

Remove the `withSignerUi(...)` wrapper in `hardhat-plugins/deploy/deploy-action.ts` (call its body directly).

## 4. Deploy tasks and operational scripts

- Replace `withSignerUi(hre, label, fn)` with a direct `fn()` call.
- Replace `getUiWalletClient(connection)` with `(await connection.viem.getWalletClients())[0]`.
- Remove `setUiTransactionContext` / `setUiWorkflowStatus` calls.

## 5. Lockfile and verification

1. Root **`pnpm install`** — refreshes the lockfile after removing `contracts/signer-ui`.
2. Search the repo:

   ```bash
   rg "HARDHAT_SIGNER_UI|signer-ui|signerUi|withSignerUi|contract-signer-ui" --glob '!**/node_modules/**' --glob '!**/pnpm-lock.yaml'
   ```

## 6. Documentation

- Update `contracts/docs/deployment/README.md`, `chained-deployments.md`, `contracts/AGENTS.md`, `docs/tech/components/contracts.md`, `.env.template`.
- Delete **`contracts/docs/deployment/signer-ui-removal.md`** last.
