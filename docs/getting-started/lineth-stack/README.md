# Lineth Stack quickstart

Run a local Lineth L2 stack using either Sepolia L1 finality or a quickstart-local
Besu+Teku L1. Sepolia remains the default public/customer demo path. Local L1 is
for development, CI, rehearsal, and unreliable-Sepolia periods.

> **Dev only**
> This stack uses committed local-dev identity and mTLS material. In Sepolia
> mode it generates a local encrypted deployer keystore and asks you to fund the
> generated address. Do not reuse any of this material for production.
> Generated wallets are written under gitignored `artifacts/accounts/`;
> Web3Signer loads the generated encrypted runtime keystores.
> Full inventory: `config/DEV-KEYS-INVENTORY.md`.

> **Apple Silicon**
> The Lineth prover image is `linux/amd64` only and runs under Rosetta on M-series, 3–5× slower than native x86_64.
> For day-to-day work, keep `PROVER_DEV_OVERRIDE=true` in `.env` (see [Prover mode](#prover-mode)) so the prover serves dummy proofs in seconds regardless of architecture.
> The slowdown only matters when you turn the override off for real partial-mode validation: the first proof takes roughly 30 minutes on M-series vs. 5–10 minutes on native x86_64.
> Everything else in the stack is multi-arch.

## First test path

Use the helper scripts first. Use raw Compose only when debugging.

```bash
cd docs/getting-started/lineth-stack
cp .env.example .env
$EDITOR .env
./scripts/start.sh --tail
```

On a clean Sepolia checkout, the first run generates
`artifacts/accounts/deployer-keystore/l1-deployer.json`, prints the deployer
address and exact minimum funding requirement, then exits before runtime
containers are created. If host TypeScript dependencies are installed, this
happens before Docker pull/start; otherwise the same check runs inside Docker
account setup. Fund that address on Sepolia and rerun the same command.

Keep the terminal open until it prints `first L1 finalization observed`.

After that:

```bash
./scripts/status.sh
./scripts/links.sh
```

Run `./scripts/export-output.sh` only when you need a local support bundle to
share run evidence or debug a failing run.

The important success signal is a successful Sepolia `finalizeBlocks`
transaction and an advanced rollup `currentL2BlockNumber`.

## Safety model

Dev-only material is committed for local identity, mTLS, and service wiring. Do
not reuse it outside this quickstart.

Committed local-dev material:

- `config/services/sequencer/key`
- `config/services/maru/private-key`
- Web3Signer, coordinator, postman, and sequencer mTLS files

User-supplied material in `L1_MODE=sepolia`:

- `L1_MODE`: `sepolia` (default) or `local`
- `L1_RPC_URL`: Sepolia HTTPS RPC URL

Generated boot material:

- The Sepolia deployer keystore is generated under
  `artifacts/accounts/deployer-keystore/` if no explicit deployer override is
  configured.
- Fresh runtime wallets are generated before Compose starts.
- Encrypted ethers keystores, Web3Signer key YAML files, compatibility env
  files, rendered config, and deployment output are written under
  `artifacts/`.
- `./scripts/reset.sh` deletes generated artifacts and Docker state but
  preserves the generated Sepolia deployer by default. Use
  `./scripts/reset.sh --forget-deployer` to remove it.

Generated runtime keystores, their keystore password file, the compatibility
`runtime-keys.env`, and the Web3Signer YAMLs under `artifacts/accounts/` are
written world-readable (`0o644`) on the host. This is a deliberate dev-only
trade-off: the host `artifacts/` tree is bind-mounted into containers that do
not share the host user, and the non-root Web3Signer container must be able to
read these files. They hold throwaway local keys only, are gitignored, and must
never be reused or copied outside a local quickstart. The funded L1 deployer
keystore is the exception and stays `0o600` because only root-running deploy
tooling reads it.

Full key inventory: `config/DEV-KEYS-INVENTORY.md`.

## Requirements

| Requirement | Minimum |
|-------------|---------|
| Docker | v24+ |
| Docker Compose | v2.19+ |
| Sepolia RPC | HTTPS endpoint |
| Sepolia ETH | 2 ETH minimum on the deployer, 3 ETH safer during congestion |
| RAM, dev-proof mode | 8 GB Docker Desktop minimum |
| RAM, partial-proof mode | 30-32 GB assigned to Docker; 128 GB recommended |
| Disk | About 30 GB free |

Apple Silicon note: the Lineth prover image is `linux/amd64`. Keep
`PROVER_DEV_OVERRIDE=true` for normal testing and demos. Partial proving on
M-series machines is much slower.

## Setup

Required `.env` values:

```bash
L1_MODE=sepolia
L1_RPC_URL=https://sepolia.infura.io/v3/<your-project-id>
```

The first Sepolia run creates the deployer keystore and prints the address to fund.
If host TypeScript dependencies are installed, this happens before Docker pull/start;
otherwise the same check runs inside Docker account setup. There is no sweep-back in
v1; keep track of any Sepolia ETH you fund into the generated deployer.

Useful optional values:

```bash
# fail early if the Sepolia deployer cannot cover deploy gas plus runtime top-ups
# L1_DEPLOYER_MIN_BALANCE_WEI=2000000000000000000

# local L2 chain ID; keep 1337 unless intentionally testing another value
# L2_CHAIN_ID=1337
```

### Config model

Keep `.env` as the single runtime config file. The files below are copy-paste
recipes, not extra Compose env files:

- `profiles/ports.env.example`
- `profiles/local-l1.env.example`
- `profiles/gas-sepolia.env.example`
- `profiles/prover-partial.env.example`

Copy only the values you need into `.env`.

Check host ports before boot:

```bash
./scripts/check-ports.sh
```

## Local L1 mode

Local mode starts Besu+Teku L1 services inside this quickstart Compose stack,
uses the pre-funded local genesis deployer by default, and avoids Sepolia RPC,
gas, and funding failures. It has no Etherscan, no public settlement, and no
real Sepolia gas spend. Bridge and finality flows still exercise the same local
stack code paths against contracts deployed on the local L1.

This mode is intended for development, CI, rehearsals, and repeated demos when
Sepolia gas or RPC conditions are unstable.

```bash
cp .env.example .env
printf 'L1_MODE=local\nPROVER_DEV_OVERRIDE=true\n' >> .env
./scripts/start.sh --tail --no-pull
```

Local L1 defaults:

| Setting | Value |
|---------|-------|
| Host RPC | `http://localhost:8445` |
| Container RPC | `http://l1-el-node:8545` |
| Chain ID | `31648428` |

### Accounts and funding model

The only user-funded account in Sepolia mode is the L1 deployer resolved by the
shared deployer resolver. By default that is the generated encrypted keystore in
`artifacts/accounts/deployer-keystore/`. In local mode, the quickstart always
uses the pre-funded local L1 genesis deployer and local L1 RPC defaults.

Advanced Sepolia users can provide an existing encrypted ethers deployer
keystore:

```bash
L1_DEPLOYER_KEYSTORE_PATH=artifacts/accounts/deployer-keystore/l1-deployer.json
L1_DEPLOYER_KEYSTORE_PASSWORD=...
# or
L1_DEPLOYER_KEYSTORE_PASSWORD_FILE=...
```

`L1_DEPLOYER_PRIVATE_KEY` remains as a temporary deprecated compatibility escape
hatch only. It is not the default tester flow.

At boot, `account-setup.ts` generates independent random runtime wallets for:

- L1 blob submitter
- L1 finalization submitter
- L1 postman
- generated L2 deployer
- L2 message anchorer
- L2 postman

The L1 deployer is not reused by runtime services. It deploys contracts and
funds generated runtime accounts.

L2 ETH is local genesis ETH, not bridged Sepolia ETH. The generated L2 deployer
is funded in genesis so it can deploy local L2 contracts and pay local gas. The
precomputed `L2MessageService` address is also funded in genesis so L1-to-L2
claims can pay out value locally.

`ERC20Example` is a demo artifact, not part of base boot. The ERC20 traffic and
ERC20 bridge smoke scripts deploy or reuse it on demand. The generated L2
deployer is the bootstrap/admin account for local L2 demo contracts. Traffic
and L2-to-L1 smoke scripts use a shared helper to create or reuse a disposable
demo traffic account. This disposable demo traffic account is funded with small
local L2 ETH and ERC20 only when needed.

### TypeScript role

Shell stays as Docker glue. Structured wallet, RPC, gas, timing, and JSON logic
lives in TypeScript.

- `deployer-wallet.ts`: owns L1 deployer resolution for local genesis,
  generated Sepolia keystore, advanced keystore override, and deprecated raw-key
  compatibility.
- `sepolia-policy.ts`: owns Sepolia chain/balance/nonce/gas policy and
  runtime-funding defaults used by host preflight and container fallback.
- `quickstart-preflight.ts`: validates `.env`, L1 RPC, deployer balance, nonce,
  and Sepolia gas settings through the shared L1 policy before Compose boot.
- `account-setup.ts`: generates ethers wallets, encrypted keystores,
  Web3Signer key config, compatibility runtime env files, and precomputed
  boot-critical addresses. It also runs the same L1 policy inside Docker
  when host TypeScript preflight is unavailable.
- `quickstart-invariants.ts`: centralizes and tests the genesis shnarf formula
  and boot-critical deterministic address precompute.
- `fund-runtime-accounts.ts`: batches generated runtime account funding.
- `traffic-account.ts`: owns disposable L2 demo account creation, reuse, ETH
  top-up, and optional ERC20 top-up for traffic and L2-to-L1 smoke scripts.
- `claim-l2-to-l1.ts`: centralizes SDK proof and manual L1 claim logic for
  L2-to-L1 smoke scripts.
- `deploy-timing.ts`: records deploy phase timing.
- `aggregate-addresses.ts`: writes the deployed address handoff.
- `ensure-demo-erc20.ts`: deploys or reuses demo ERC20 only when a traffic or
  smoke script needs it.

Keystore encryption defaults to a committed local-dev password because
Web3Signer rejects empty keystore passwords. Override with
`LINETH_KEYSTORE_PASSWORD` or `LINETH_KEYSTORE_PASSWORD_FILE`.

Coordinator and Postman both sign through Web3Signer. Postman does not receive
raw signer private keys in its runtime env.

## Boot and logs

Normal boot:

```bash
./scripts/start.sh --tail
```

`start.sh` prints a guided boot flow: check ports, check the L1 network,
generate accounts and configs, pull images, start services, deploy contracts,
show links, wait for finality, then print the result. Docker Compose remains
the engine.

Use verbose mode when you want deploy transaction hashes, install details, and
retry-noise detail in the terminal:

```bash
./scripts/start.sh --tail --verbose
./scripts/watch.sh --verbose
```

If you bypass `start.sh`, prepare generated files first:

```bash
./scripts/bootstrap-artifacts.sh
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover up -d
```

Reattach to the guided timeline:

```bash
./scripts/watch.sh
./scripts/watch.sh --once
```

Raw service logs for debugging:

```bash
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover logs -f --tail=120 \
  deploy-contracts coordinator prover postman sequencer shomei l2-node-besu
```

`start.sh --tail` prints useful local and Sepolia links as soon as
`addresses.json` exists, before first L1 finality. First finality appears later
when the coordinator submits and confirms `finalizeBlocks`.

The guided watcher classifies common transient coordinator retry noise such as
`already known`, `nonce too low`, `replacement transaction underpriced`,
`StartingRootHashDoesNotMatch`, and `ShnarfAlreadySubmitted`. These are only a
blocker if finalized L2 block stops advancing.

### Timing and current bottleneck

`deploy-contracts` writes timing evidence to `artifacts/deployments/deploy-timing.jsonl`.
For sharing or debugging a run, `./scripts/export-output.sh` collects a fresh
support bundle under `lineth-output/`.

Latest verified dev-proof fresh boot (2026-05-31, clean local state):

| Milestone | Observed |
|-----------|----------|
| Contract deployment complete | about 4m12s from Compose timeline |
| Runtime signer funding | about 24s |
| First Sepolia finality | about 5m30s from Compose timeline |
| Rollup finalized L2 block | 8 |

Largest remaining bottleneck in simple terms: after `./scripts/reset.sh`, the
Docker-side Node dependency cache is gone. The next boot must download and
rebuild the npm packages used by the TypeScript account/deploy tooling before
it can generate artifacts or deploy contracts. That is the cold Docker-side
`pnpm install` cost. A prebuilt tooling image would ship those dependencies
already installed; a better cache strategy would avoid deleting or rebuilding
them unnecessarily.

Runtime funding is already batched in TypeScript. The previous serial funding
path measured about 77s; the batched path measured about 22s in the verified
run.

## L1 gas caps and Sepolia congestion

The shared Sepolia policy fails early if deploy gas settings are below current
Sepolia fees. The check includes RPC chain ID, deployer balance, deployer nonce,
current Sepolia execution fee, current Sepolia blob base fee, and configured gas
caps. `start.sh` runs it before generated-file preparation when host TypeScript
deps are available; `account-setup` runs the same policy inside Docker as the
fallback. If Sepolia gas spikes after boot, coordinator may retry blob or
finalization submission until configured caps are high enough.

Default L1 submission caps:

| Path | Default cap |
|------|-------------|
| Blob max fee per gas | 100 gwei |
| Blob max fee per blob gas | 100 gwei |
| Blob priority fee | 20 gwei |
| Finalization max fee per gas | 200 gwei |
| Finalization priority fee | 40 gwei |

Override in `.env` when Sepolia is congested:

```bash
L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI=150000000000
L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI=150000000000
L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=30000000000
L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI=300000000000
L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=60000000000
```

Raising caps can spend more Sepolia ETH from generated L1 runtime signers.

## Endpoints

| Service | URL |
|---------|-----|
| L2 RPC HTTP | http://localhost:8745 |
| L2 RPC WebSocket | ws://localhost:8746 |
| L2 Blockscout UI | http://localhost:4001 |
| L2 Blockscout API | http://localhost:4000/api/v2/blocks |
| Coordinator | http://localhost:9545 |
| Postman | http://localhost:9090 |
| Maru | http://localhost:8080 |
| Sepolia explorer | https://sepolia.etherscan.io |

Sequencer RPC on `:8645` is internal by convention. Use `:8745` for wallets and
SDKs.

## Verify and demo

Status and links:

```bash
./scripts/status.sh
./scripts/links.sh
```

`status.sh` should show deployed addresses, coordinator ports listening,
prover request/response counts, a blob tx, and a separate finalization tx that
advanced rollup `currentL2BlockNumber`.

Do not treat `Submit Blobs` as finalization. Blob submission publishes data.
Only `finalizeBlocks` advances the Sepolia rollup state.

Local L2 traffic:

```bash
./scripts/traffic-generation/send-l2-test-tx.sh
./scripts/traffic-generation/send-l2-erc20-transfer.sh
./scripts/traffic-generation/generate-l2-erc20-traffic.sh start
./scripts/traffic-generation/generate-l2-erc20-traffic.sh logs
./scripts/traffic-generation/generate-l2-erc20-traffic.sh stop
```

Latest verified local traffic checks (2026-05-28): one L2 ETH transfer, one L2
`ERC20Example.transfer(...)`, and a bounded three-transfer continuous traffic
run. Blockscout reported local L2 fees in the `10^12` wei range with
`L2_GAS_PRICE_WEI=100000000`.

Bridge smoke tests spend real Sepolia gas in Sepolia mode (local L1 mode uses the local L1 instead):

```bash
./scripts/smoke-test/smoke-bridge-message.sh
./scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh
./scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh
./scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh
```

Coverage:

- `scripts/smoke-test/smoke-bridge-message.sh`: L1-to-L2 generic message relay through Postman.
- `scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh`: ERC20 TokenBridge
  deposit to local L2.
- `scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh`: ERC20 TokenBridge
  withdrawal and L1 claim.
- `scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh`: L2-to-L1 generic
  message finality and claim.

These scripts prove generic message relay in both directions and ERC20 bridge
deposit/withdrawal. They are not ETH withdrawal tests.

## Stop or reset

Stop but keep state:

```bash
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover stop
```

Wipe generated artifacts, Docker volumes, chaindata, and deploy caches:

```bash
./scripts/reset.sh
```

`./scripts/reset.sh` preserves the generated Sepolia deployer keystore so funded
test ETH is not stranded by routine local resets. Use
`./scripts/reset.sh --forget-deployer` only when you intentionally want a new
generated Sepolia deployer.

In `L1_MODE=local`, `./scripts/reset.sh` also removes the quickstart local L1
data volume. If you manually run `docker compose down -v`, local L1 state is
deleted; run `./scripts/reset.sh` before the next boot so preserved deploy
artifacts do not point at missing local L1 contracts.

## Prover mode

Default mode is dev proofs:

```bash
PROVER_DEV_OVERRIDE=true
```

Partial validation mode is heavier:

```bash
PROVER_DEV_OVERRIDE=false
PROVER_GOMEMLIMIT=24GiB
```

Use partial mode only for validation runs with enough Docker memory. For normal
testing, demos, Blockscout, traffic, and bridge smokes, use dev-proof mode.

This quickstart supports `LINEA_COORDINATOR_DATA_AVAILABILITY=ROLLUP` only.
Other values fail during init.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `L1 RPC not reachable` | Bad or rate-limited Sepolia RPC | Try another RPC; test with `cast chain-id --rpc-url "$L1_RPC_URL"` |
| Deploy gas below current fee | Sepolia gas spiked | Raise `L1_DEPLOY_GAS_PRICE_WEI` before boot |
| Deployer insufficient funds | Sepolia balance too low for deploy plus runtime top-ups | Fund the deployer and rerun |
| Missing Linux native npm module | Stale Docker dependency volume | Rerun once; if still broken, `./scripts/reset.sh` |
| `ADDRESS MISMATCH` | Deploy script nonce/order changed | Update the tested precompute logic in `quickstart-invariants.ts` |
| Coordinator retry noise | Normal retry path while catching up | Watch whether finalized L2 block advances |
| Prover exits 137 | Not enough Docker memory for partial mode | Use dev-proof mode or assign more memory |
| Port collision | Local service already uses a required port | Run `./scripts/check-ports.sh` and override `HOST_PORT_*` |

Useful inspection commands:

```bash
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover ps

docker compose --env-file versions.env --env-file .env --profile stack-partial-prover logs -f --tail=120 \
  deploy-contracts coordinator prover postman sequencer shomei l2-node-besu

cat artifacts/deployments/addresses.json

for f in artifacts/deployments/deploy-logs/*.log; do
  echo "=== $f ==="
  cat "$f"
done | less
```

## CI maintenance path

Normal PR CI should stay deterministic and cheap:

- shell syntax checks for quickstart scripts;
- `./scripts/check-quickstart-static.sh`;
- `docker compose --env-file versions.env --env-file .env --profile stack-partial-prover config`;
- mocked TypeScript checks for `.env`, Sepolia RPC responses, wallet
  generation, Web3Signer key rendering, gas validation, funding, and address
  aggregation.

Do not put full Sepolia finality in normal PR CI. A real Sepolia boot/finality
run should be scheduled or manual because it depends on Sepolia gas, RPC
availability, and funded testnet accounts.

## Known limitations

- Monorepo-bound: deploy tooling bind-mounts the repository contracts.
- Sepolia-backed: another L1 requires config and timing review.
- No Timelock or Security Council: deployed L1 contracts are owned by the
  deployer EOA.
- No full prover path: full proving needs far more hardware.
- Partial proving is opt-in and resource-heavy.
- Dev-proof mode is for fast evaluation only.
- Validium mode is not supported by this quickstart.
- Verifier setup is quickstart-only.
- ETH withdrawal smoke is still missing.
- There is no one-shot verification orchestrator yet.

## File layout

```text
docs/getting-started/lineth-stack/
|-- README.md
|-- docker-compose.yml
|-- versions.env
|-- .env.example
|-- profiles/                 # copy-paste config recipes
|-- artifacts/                # generated runtime state, gitignored
|   |-- accounts/             # generated wallets, keystores, Web3Signer keys
|   |-- genesis/              # rendered L2 genesis and fork timestamp
|   |-- config/               # rendered service config
|   |-- deployments/          # deployed addresses, runtime env, deploy logs
|   `-- reports/              # exported reports
|-- lineth-output/            # exported evidence bundle, gitignored
|-- config/
|   |-- DEV-KEYS-INVENTORY.md
|   |-- genesis/
|   |-- services/
|   |-- explorer/
|   `-- web3signer/
`-- scripts/
    |-- phases/               # one-shot Compose boot phases
    |-- services/             # config renderers
    |-- internal/             # TypeScript helpers and deploy implementation
    |-- init/                 # long-lived service entrypoints
    |-- lib/                  # shared logging and runtime/artifact helpers
    |-- traffic-generation/
    |-- smoke-test/
    |-- start.sh
    |-- bootstrap-artifacts.sh
    |-- reset.sh
    |-- check-ports.sh
    |-- status.sh
    |-- links.sh
    |-- watch.sh
    `-- export-output.sh
```

Runtime-generated files are host-backed under `artifacts/`. Containers still
mount them at stable internal paths such as `/accounts`, `/deployments`,
`/rendered`, and `/initialization`.
