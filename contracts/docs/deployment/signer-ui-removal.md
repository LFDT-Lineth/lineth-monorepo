# Removing the optional Hardhat signer UI (`HARDHAT_SIGNER_UI`)

[← Back to deployment index](README.md)

This document is for **maintainers** who want to delete the browser-signing stack entirely and return to **private-key / named-account** flows only. It lists integration points as of `HARDHAT_SIGNER_UI` / `signer-ui`.

Do **not** follow this guide for day-to-day use. Operators should leave `HARDHAT_SIGNER_UI` unset.

## What gets removed

| Area | Path / symbol |
|------|----------------|
| Bridge + session logic | `contracts/scripts/hardhat/signer-ui-bridge.ts` |
| Shared signer-mode helper | `contracts/scripts/hardhat/signer-mode.ts` |
| Next.js app | `contracts/signer-ui/` (local private Next.js app) |
| Hardhat hook | `subtask(TASK_DEPLOY_RUN_DEPLOY)` in `contracts/hardhat.config.ts` |
| Config branch | `deployerAccounts()` UI branch in `contracts/hardhat.config.ts` |
| Deploy script wrappers | `withSignerUiSession`, `getUiSigner`, `setUiTransactionContext` across `contracts/deploy/*.ts` |
| Operational tasks | `runWithSignerUiSession`, `getUiSigner` in `scripts/operational/**` |
| Shared helpers | `resolveUiRunner` / UI context in `contracts/scripts/hardhat/utils.ts` |
| ESLint ignore | `signer-ui/**` in `contracts/eslint.config.mjs` |
| Documentation | Sections in `contracts/docs/deployment/README.md`, `chained-deployments.md`, `contracts/signer-ui/README.md`, this file; cross-links from `contracts/AGENTS.md`, `docs/tech/components/contracts.md` |

## 1. `hardhat.config.ts`

1. Remove the `subtask(TASK_DEPLOY_RUN_DEPLOY).setAction(...)` block that `require()`s `./scripts/hardhat/signer-ui-bridge.ts`.
2. Remove the shared signer-mode validation/helper import (`./scripts/hardhat/signer-mode.ts`) if you no longer need browser-signing exclusivity checks.
3. Simplify **`deployerAccounts()`**: delete the `HARDHAT_SIGNER_UI` branch and the non-zero key validation; always return `[normalizedPrivateKey]` from env (or your preferred account wiring).

## 2. Delete UI and bridge implementation

- Delete **`contracts/scripts/hardhat/signer-ui-bridge.ts`**.
- Delete the directory **`contracts/signer-ui/`**.

## 3. `contracts/scripts/hardhat/utils.ts`

1. Remove imports from `./signer-ui-bridge` (`resolveUiRunner`, `setUiTransactionContext`, `isSignerUiEnabled`).
2. Remove **`pushUiDeployContext`** / **`setUiTransactionContext`** usage and reimplement **`resolveUiRunner`** as a local helper that only handles non-UI signers (see previous git history for the non-UI branches).

## 4. Deploy scripts and operational tasks

- Remove `withSignerUiSession` wrappers and `getUiSigner` / `runWithSignerUiSession` from `contracts/deploy/*.ts` and `scripts/operational/**/*.ts`; restore explicit `hre.ethers.getSigner` / named accounts as needed.

## 5. Lockfile and verification

1. Root **`pnpm install`** — refreshes the lockfile after removing `contracts/signer-ui`.
2. Search the repo:

   ```bash
   rg "HARDHAT_SIGNER_UI|signer-ui-bridge|linea-contract-signer-ui|signer-ui/" --glob '!**/node_modules/**' --glob '!**/pnpm-lock.yaml'
   ```

## 6. Documentation

- Update `contracts/docs/deployment/README.md`, `chained-deployments.md`, `contracts/AGENTS.md`, `docs/tech/components/contracts.md`, `.env.template`.
- Delete **`contracts/docs/deployment/signer-ui-removal.md`** last.
