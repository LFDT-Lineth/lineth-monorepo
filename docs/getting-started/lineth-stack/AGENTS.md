# AGENTS.md — Lineth Stack quickstart

> Inherits all rules from the [root AGENTS.md](../../../AGENTS.md). Only package-specific
> additions below.
>
> This file is an operating manual for an automated agent. It tells you how to boot, verify, and
> tear down the Lineth Stack quickstart on your own. Rationale, endpoints, gas tuning, and
> troubleshooting prose live in [`README.md`](./README.md) — this file is the procedure. When the
> two disagree, `README.md` wins.

## What this is

A dev/demo quickstart that boots a local Lineth L2 with either **Sepolia** L1 finality (default,
real public settlement) or a self-contained **local** Besu+Teku L1. It is not a production
deployment pattern. Dev-only identity and mTLS material is committed on purpose
(`config/DEV-KEYS-INVENTORY.md`); never reuse anything here outside a local quickstart.

Run all commands from `docs/getting-started/lineth-stack/`.

## Step 0 — preflight (stop if a check fails)

```bash
docker --version            # need Docker v24+
docker compose version      # need Docker Compose v2.19+
./scripts/check-ports.sh    # required host ports must be free; override HOST_PORT_* on collision
```

Resources: ~30 GB free disk; ~8 GB Docker RAM for dev-proof mode (default); 30–32 GB for partial
proving. On Apple Silicon the prover image is `linux/amd64` (runs under Rosetta) — keep the default
`PROVER_DEV_OVERRIDE=true` unless you are explicitly validating partial proofs.

## Step 1 — choose the L1 mode

| Mode | External inputs | Choose when |
|------|-----------------|-------------|
| `local` (best for unattended/CI runs) | none | Self-contained run, no funding step. Starts Besu+Teku L1 inside the stack. |
| `sepolia` (default product path) | `L1_RPC_URL` + a **funded** deployer | You need real public L1 settlement. Includes a human funding step (Step 3). |

If you are running without a human present, prefer `local`: no secrets, no faucet, no funding gate.

## Step 2 — configure and boot

Drive the wizard non-interactively so the run is deterministic. Do not hand-edit `.env` unless you
are debugging.

Local L1 (fully autonomous):

```bash
./scripts/start.sh --wizard --non-interactive --l1-mode local --prover dev
./scripts/start.sh --tail --no-pull
```

Sepolia:

```bash
./scripts/start.sh --wizard --non-interactive \
  --l1-mode sepolia \
  --l1-rpc-url "https://sepolia.<provider>/v3/<key>" \
  --prover dev
./scripts/start.sh --tail
```

Wizard input precedence: command flag > `WIZARD_*` env var (`WIZARD_L1_MODE`, `WIZARD_L1_RPC_URL`,
`WIZARD_PROVER`) > existing `.env` > `.env.example`. `--non-interactive` requires an explicit L1
mode and fails fast if Sepolia mode has no RPC URL. When the wizard overwrites an existing `.env` it
first backs it up under `artifacts/env-backups/`.

## Step 3 — Sepolia funding STOP point (Sepolia mode only)

On a clean Sepolia checkout, the first boot generates
`artifacts/accounts/deployer-keystore/l1-deployer.json`, prints the deployer address and the exact
minimum funding, then **exits before any runtime container starts**. This is expected, not a
failure.

When you reach it:

1. Stop. Surface the printed address and amount to the human — the agent cannot fund it. Minimum is
   ~2 ETH on Sepolia, ~3 ETH safer during congestion.
2. After the human confirms the address is funded, rerun the same `start.sh` command.

There is no sweep-back in v1; funded Sepolia ETH stays in that generated keystore until reused.

## Step 4 — confirm success (assert, do not assume)

Keep the terminal open until `start.sh --tail` prints `first L1 finalization observed`. Then:

```bash
./scripts/status.sh
./scripts/links.sh
```

The run succeeded only when there is a confirmed `finalizeBlocks` transaction **and** rollup
`currentL2BlockNumber` has advanced. `Submit Blobs` only publishes data — it is **not** finalization.
Local L1 mode has no Etherscan or public settlement but exercises the same finality code paths.

To reattach to the guided timeline of a running stack: `./scripts/watch.sh` (or `--once`). Use
`./scripts/export-output.sh` only when you need a support bundle to share a run.

## Step 5 — optional verification

```bash
# local L2 activity
./scripts/traffic-generation/send-l2-test-tx.sh
./scripts/traffic-generation/generate-l2-erc20-traffic.sh start   # ... logs | stop

# bridge smokes — Sepolia mode spends real Sepolia gas; local mode uses the local L1
./scripts/smoke-test/smoke-bridge-message.sh
./scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh
./scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh
./scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh
```

## Stop and reset

```bash
# stop but keep state
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover stop

# wipe generated artifacts + Docker state (preserves the Sepolia deployer keystore by default)
./scripts/reset.sh
./scripts/reset.sh --forget-deployer   # also discard the generated deployer
```

In `L1_MODE=local`, `reset.sh` also removes the local L1 data volume. If you ever run
`docker compose down -v` by hand, run `reset.sh` before the next boot so preserved deploy artifacts
do not point at missing local L1 contracts.

## When you change files in this package — validate

Cheap, deterministic checks (safe in normal PR CI):

```bash
find scripts -name '*.sh' -print0 | xargs -0 -n1 sh -n   # shell syntax (use bash -n where needed)
./scripts/check-quickstart-static.sh                     # package invariants + helper-usage guards
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover config
# plus the mocked TypeScript checks for .env, RPC, wallet generation, funding, and address logic
```

Do not claim success from static checks alone for boot or runtime changes — those need a real boot,
and at least one bridge/traffic smoke when deployment, funding, or signing behavior changed. Do not
put full Sepolia finality in normal PR CI; it depends on live gas, RPC availability, and funded
testnet accounts.

## Guardrails (do not)

- Do not introduce real secrets or production keys. `.env` and `artifacts/` are gitignored — keep it
  that way.
- Data availability is `ROLLUP` only. Any other `LINEA_COORDINATOR_DATA_AVAILABILITY` fails at init.
- Do not reintroduce raw runtime-signer private keys into rendered service env. Coordinator and
  Postman sign through Web3Signer.
- Do not log deployer private keys, keystore passwords, or keystore JSON.
- Do not regress the single-command tester path (`start.sh`). Keep shell as Docker glue; structured
  wallet/RPC/JSON/gas/timing logic stays in `scripts/internal/*.ts`.
- Do not reuse the committed dev identity/mTLS material anywhere real.
- The stack is monorepo-bound: deploy tooling bind-mounts repo contracts.

## Failure handling

Use the Troubleshooting table in [`README.md`](./README.md#troubleshooting). Hard STOP conditions for
an agent: unfunded Sepolia deployer (needs a human), port collision (run `check-ports.sh`, override
`HOST_PORT_*`), and prover OOM / exit 137 (use dev-proof mode or raise Docker memory).

## Files to read first

- [`README.md`](./README.md) — full operator guide: endpoints, gas caps, prover modes, troubleshooting.
- [`.env.example`](./.env.example) — every configuration key, required vs optional.
- [`scripts/README.md`](./scripts/README.md) — what each script directory is for.
- `config/DEV-KEYS-INVENTORY.md` — why dev keys are committed and how far their scope extends.
- Entry points: `scripts/start.sh` (boot), `scripts/watch.sh` (reattach), `scripts/status.sh` /
  `scripts/links.sh` (inspect), `scripts/reset.sh` (teardown).
