# Hardhat signer UI (browser signing)

When `HARDHAT_SIGNER_UI=true`, Hardhat deploy scripts can route transactions through this local Next.js app so you approve them in a browser wallet (MetaMask, Rabby, etc.) instead of putting `DEPLOYER_PRIVATE_KEY` in the environment.

`HARDHAT_SIGNER_UI=true` and `DEPLOYER_PRIVATE_KEY` are mutually exclusive. If both are set, Hardhat fails immediately rather than guessing between browser signing and private-key signing.

The Hardhat side (`contracts/scripts/hardhat/signer-ui-bridge.ts`) starts a **local HTTP bridge** (session API + CORS), launches **`pnpm --dir contracts/signer-ui exec next dev`** on a free port, prints/opens the UI URL, and blocks until each transaction is **verified on-chain** against the pending request.

This app is local operator tooling for the contracts package. It is not intended to be published, imported, or consumed as a standalone package.

## Prerequisites

1. **Monorepo install** (from repo root):

   ```bash
   pnpm install
   ```

2. **RPC** for the target network (local stack, devnet, or testnet).

3. **Wallet** funded on that chain.

## One session per `hardhat deploy` run

For chained tags, e.g. `--tags PlonkVerifier,LineaRollup,Timelock`, Hardhat keeps **one** browser session for the **entire** run. Deploy scripts execute in order; you reconnect/sign per transaction, but the UI and bridge are **not** restarted between files.

## UI features (operator-facing)

- **Sticky “Sign current transaction”** bar at the top while a request is pending (same action as the pending card below).
- **Submitted transaction history** with tx hash, from/to, constructor/proxy context, and raw request JSON; stored in **session storage** so it survives closing the HTTP bridge.
- **In-page anchors** `#signer-tx-<requestId>` after each successful submit; **Jump to** links and **Copy page link** for bookmarks.
- **Proxy hints** (OpenZeppelin transparent / UUPS / beacon) when deploy helpers pass metadata from `contracts/scripts/hardhat/utils.ts`.
- After a full **`hardhat deploy`** run (success or failure), Hardhat **stops the Next.js dev server by default** so the process exits cleanly; the browser tab stays open with history in session storage. Set `HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true` to leave Next running (legacy behavior). For **non-deploy** sessions (operational scripts, etc.), Next still runs unless you set `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true`.
- The bridge sends a **terminal session outcome** (`complete` / `error`) before closing so the UI **stops polling** immediately instead of only noticing a dead bridge.

## Network: `zkevm_dev` (local L1)

Hardhat network `zkevm_dev` uses `L1_RPC_URL` when set, otherwise defaults to `http://127.0.0.1:8545`. For the docker stack, point L1 at **8445**:

| Item | Value |
|------|--------|
| Hardhat `--network` | `zkevm_dev` |
| Env | `L1_RPC_URL=http://127.0.0.1:8445` |
| Chain ID | **`31648428`** for docker local L1 (`docker/config/l1-node/el/genesis.json`). Use **`59139`** only if your RPC is the hosted Linea devnet. |

Add a custom network in your wallet matching the RPC you use.

## Example: deploy `TestERC20` on local L1

From the **`contracts`** package directory:

```bash
cd contracts

L1_RPC_URL=http://127.0.0.1:8445 \
TEST_ERC20_NAME="LocalUI" \
TEST_ERC20_SYMBOL="LUI" \
TEST_ERC20_INITIAL_SUPPLY="1000000" \
HARDHAT_SIGNER_UI=true \
pnpm exec hardhat deploy --network zkevm_dev --tags TestERC20
```

1. Open the printed URL (or rely on auto-open unless `HARDHAT_SIGNER_UI_OPEN_BROWSER=false`).
2. Use the full URL including `sessionToken` — it authenticates the browser to the bridge.
3. Connect the wallet and switch chain if prompted.
4. Approve the transaction; the CLI continues when the tx matches the pending request on the RPC.

Artifacts and `deployments/<network>/` match a private-key deploy.

## Network: `l2` (local L2)

Set **`L2_RPC_URL`** (e.g. `http://127.0.0.1:8545` for the default stack). Add the same RPC/chain ID in your wallet if needed.

```bash
cd contracts

L2_RPC_URL=http://127.0.0.1:8545 \
HARDHAT_SIGNER_UI=true \
pnpm exec hardhat deploy --network l2 --tags <YourL2Tag>
```

## Environment variables

| Variable | Effect |
|----------|--------|
| `HARDHAT_SIGNER_UI=true` | Enable browser signing. When unset or not `true`, scripts use `DEPLOYER_PRIVATE_KEY` / named accounts. Must not be combined with `DEPLOYER_PRIVATE_KEY`. |
| `HARDHAT_SIGNER_UI_OPEN_BROWSER=false` | Do not auto-open a tab; open the URL from the terminal. |
| `HARDHAT_SIGNER_UI_DEBUG=true` | Forward `next dev` stdout/stderr to the terminal. |
| `HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true` | After `hardhat deploy`, **keep** the Next.js dev server running when the bridge closes (default is to stop Next). |
| `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true` | For **non-deploy** signer UI sessions, stop the Next.js child when the HTTP bridge closes. |
| `HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS` | Optional. Milliseconds to allow the UI to poll terminal `sessionOutcome` before the bridge closes after deploy (default `1500`). |
| `HARDHAT_SIGNER_UI_SHUTDOWN_GRACE_MS` | Optional. Milliseconds after the bridge closes before SIGTERM on Next when stopping it (default `2000`). |
| `EXPECTED_SIGNER_ADDRESS=0x...` | Optional safety guard. If set, deploy/sign flows fail fast unless the resolved signer address exactly matches this address. |

## Manual UI dev (optional)

You normally do **not** run the UI yourself — Hardhat starts it. For layout/CSS work only:

```bash
cd contracts/signer-ui
pnpm dev
```

A full signing round-trip still requires the bridge from a running `hardhat deploy`.

## Further reading

- [Linea deployment scripts overview](../docs/deployment/README.md) — env vars, tags, verification, upgradeable / `.openzeppelin` reuse.
- [Chained deployments](../docs/deployment/chained-deployments.md) — one UI session for the whole tag batch.
- [Removing the signer UI](../docs/deployment/signer-ui-removal.md) — maintainers only.

## Limitations

- Scripts that broadcast **fixed pre-signed** transactions (some predeploy paths) do not use the browser signer.
- **One in-flight transaction** from Hardhat’s perspective at a time per session.
- OpenZeppelin **may reuse** implementation / ProxyAdmin from `.openzeppelin/` — fewer wallet prompts than three does not imply a bug.
