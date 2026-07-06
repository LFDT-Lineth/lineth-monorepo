# Debugging a Lineth Stack boot

Work from evidence, not guesses. The stack is layered (Compose → shell phases → TypeScript →
contracts → coordinator/prover finality), so a failure almost always has a specific layer. Find the
layer first, then apply the fix. The README's own Troubleshooting table is the source of truth; this
file is the agent-facing triage on top of it.

## First move: look at the right logs

```bash
# what's actually running / crashed
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover ps

# the services that matter, together
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover logs -f --tail=120 \
  deploy-contracts coordinator prover postman sequencer shomei l2-node-besu

# deployed addresses + per-step deploy logs
cat artifacts/deployments/addresses.json
for f in artifacts/deployments/deploy-logs/*.log; do echo "=== $f ==="; cat "$f"; done
```

If finality is the question, watch whether finalized L2 block advances over a minute — that single
signal separates "stuck" from "slow".

## Symptom → likely cause → fix

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `L1 RPC not reachable` | Bad or rate-limited Sepolia RPC | Test with `cast chain-id --rpc-url "$L1_RPC_URL"` (expect `11155111`); switch providers if it fails or is throttled. |
| Deploy gas below current fee / deploy stalls in Sepolia | Sepolia base fee spiked above the configured deploy gas | Raise `L1_DEPLOY_GAS_PRICE_WEI` in `.env` before boot. For blob/finalization stalls after boot, raise the `L1_BLOB_*` / `L1_FINALIZATION_*` caps (see README "L1 gas caps"). |
| Deployer has insufficient funds | Sepolia balance too low for deploy + runtime top-ups | Fund the deployer address (printed at preflight) and rerun the same command. **Human-in-the-loop — you cannot do this yourself.** Minimum ~2 ETH, ~3 ETH safer. |
| First Sepolia boot "exits" before containers start | Expected funding STOP point, not an error | Surface the printed address + amount to the human, fund, rerun. Do not retry blindly. |
| Missing Linux-native npm module / TS tooling errors | Stale Docker dependency volume | Rerun once; if it persists, `./scripts/reset.sh` to rebuild the cold dependency cache. |
| `ADDRESS MISMATCH` | Deploy script nonce/order changed, or preserved artifacts point at a wiped L1 | If contracts/order changed, update the tested precompute in `quickstart-invariants.ts`. If you ran `docker compose down -v`, run `./scripts/reset.sh` before the next boot. |
| Coordinator retry noise (`already known`, `nonce too low`, `replacement transaction underpriced`, `StartingRootHashDoesNotMatch`, `ShnarfAlreadySubmitted`) | Normal transient retry path while catching up | Not a blocker on its own. Only worry if **finalized L2 block stops advancing**. |
| Prover exits with code 137 | Out of memory — partial proving needs far more RAM than allocated | Use dev-proof mode (`PROVER_DEV_OVERRIDE=true`) or raise Docker Desktop memory (30–32 GB for partial). |
| Port collision on boot | A local service already holds a required port | `./scripts/check-ports.sh`, then override the relevant `HOST_PORT_*` in `.env`. |
| Boots but no finality | `Submit Blobs` happened but `finalizeBlocks` hasn't | Watch coordinator logs and `status.sh`; confirm gas caps are above current Sepolia fees; give it time — first finality is minutes, not seconds. |
| `init` fails immediately on DA | `LINEA_COORDINATOR_DATA_AVAILABILITY` set to anything but `ROLLUP` | Set it back to `ROLLUP` (or unset it). Only `ROLLUP` is supported. |

## Reading success vs. stuck

`./scripts/status.sh` should show: deployed addresses, coordinator ports listening, prover
request/response counts, a blob tx, and a **separate** finalization tx that advanced rollup
`currentL2BlockNumber`. A blob tx without an advancing finalized block is *in progress*, not done.

## Timing expectations (so "slow" isn't mistaken for "broken")

On a verified dev-proof fresh boot: contract deployment ~4 minutes, runtime funding ~20–25 seconds,
first Sepolia finality ~5–6 minutes from the Compose timeline. The largest cold-boot cost is the
Docker-side dependency install after a `reset.sh` (the npm/pnpm cache was wiped) — expect extra
minutes there, and don't kill the boot thinking it hung.

## When to stop and escalate to the human

- Unfunded or under-funded Sepolia deployer.
- Repeated Sepolia RPC/gas failures that survive a provider switch and cap raise.
- Any situation where the fix would require committing a real secret or disabling a safety check —
  never do that; surface it instead.
