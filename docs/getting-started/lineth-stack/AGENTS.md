# AGENTS.md

## Goal

Provide a fast, reproducible Lineth Stack quickstart for demos, development, and validation.

Current practical objective:
- default path: local Linea L2 + Sepolia-backed L1 finality, usable by engineers and customer-facing demos;
- fallback path: `L1_MODE=local` for development, CI, rehearsal, and periods where Sepolia gas/RPC/funding make repeated testing unreliable.

This quickstart is still a dev/demo stack, not a production deployment model.

## Current State

Completed and already integrated on this branch:
- host-backed generated artifacts under `artifacts/` replaced the older runtime handoff through shared Docker volumes;
- folder/layout refactor first pass is in place:
  - `scripts/phases/`
  - `scripts/services/`
  - `scripts/internal/`
  - `scripts/init/`
  - `scripts/lib/`
  - `config/genesis/`
  - `config/services/`
  - `profiles/`
- generated runtime wallets are created at boot by TypeScript, not committed;
- phase-03 service config rendering is split into per-service renderers under `scripts/services/`;
- encrypted ethers keystores are generated under `artifacts/accounts/runtime-keystores/`;
- Web3Signer key YAMLs are generated under `artifacts/accounts/web3signer-keys/`;
- coordinator and postman are configured to sign through Web3Signer;
- Postman no longer receives raw signer private keys in rendered runtime env;
- runtime funding was moved from serial shell sends to batched TypeScript funding;
- shared runtime shell helper exists in `scripts/lib/runtime.sh` and is used across status/links/export/smoke/traffic scripts;
- shared Sepolia/L1 policy exists in `scripts/internal/sepolia-policy.ts`;
- L2 demo traffic account lifecycle was centralized in `traffic-account.ts` / `traffic-account.sh`;
- L2-to-L1 manual claim/proof logic was centralized in `claim-l2-to-l1.ts`;
- bridge smoke scripts were revalidated after the claim-helper refactor;
- `ERC20Example` was removed from base boot and is now deployed on demand by smoke/traffic flows;
- Sepolia deployer private-key input was removed from the default tester flow:
  - first Sepolia boot generates an encrypted deployer keystore under
    `artifacts/accounts/deployer-keystore/`;
  - the host preflight prints the generated address, exact minimum funding, and rerun instruction before Docker work;
  - advanced users may provide `L1_DEPLOYER_KEYSTORE_PATH` with password env/file;
  - `L1_DEPLOYER_PRIVATE_KEY` remains only as a deprecated compatibility escape hatch;
- README was updated to describe the artifact-backed layout, TS role, Web3Signer use, and current boot path.

Current branch/repo status:
- branch: `feat/linea-stack-quickstart`
- `L1_MODE=sepolia|local` is implemented on this branch;
- local-L1 fresh-boot validation has been run successfully through first L1 finality;
- local mode ignores stale Sepolia RPC/deployer keystore/private-key config and uses the built-in local genesis deployer
  and local RPC defaults.

## Major Design Decisions

- Keep Docker Compose as the execution engine.
  Reason: service topology, health checks, profiles, and reproducibility already live there.

- Keep `./scripts/start.sh --tail` as the normal user entrypoint.
  Reason: new testers need one short command; Compose remains available for debugging.

- Keep shell as orchestration glue and move structured logic into TypeScript.
  Reason: wallets, JSON, RPC checks, policy, timing, address aggregation, and funding are easier to validate and harder to corrupt in TS than in ad hoc shell.

- Use generated runtime wallets instead of reusing the L1 deployer for long-lived services.
  Reason: separates deployer responsibility from runtime signers and makes the funding model explicit.

- Use encrypted ethers keystores plus Web3Signer for coordinator and postman.
  Reason: Postman was challenged in review for raw key env usage; current direction matches the coordinator pattern and avoids passing raw runtime signer keys through service env.

- Keep `.env` as the runtime config contract, but do not require raw private keys in the default tester path.
  Reason: testers already understand `.env`; the Sepolia deployer is now generated as a durable gitignored artifact and
  raw-key config is deprecated compatibility only.

- Keep Sepolia as the default external-finality path.
  Reason: customer/demo value comes from real public L1 settlement.

- Reintroduce local L1 as a first-class fallback mode.
  Reason: Sepolia gas, RPC rate limits, and funding make repeated demos and CI painful.

- Do not deploy demo ERC20 contracts during base boot.
  Reason: they are not required for first finality and were wasting time on fresh boot.

- Keep host-backed `artifacts/` even though it adds some file-management complexity.
  Reason: it makes runtime state inspectable, reusable across sessions, and easier to export/debug than opaque Docker volumes.

## Current Architecture

### Docker Compose
- Owns the service graph, profiles, volumes, health checks, and container startup.
- Profiles currently separate bootstrap/render steps from the main stack.
- Compose is the runtime engine for:
  - account setup
  - service config rendering
  - L2 genesis generation
  - contract deployment
  - coordinator/postman/prover/sequencer/shomei/Besu/Blockscout runtime
- In local-L1 mode, Compose also starts Besu + Teku L1 services.

### Shell scripts
- User-facing shell commands stay in `scripts/`:
  - `start.sh`
  - `bootstrap-artifacts.sh`
  - `reset.sh`
  - `check-ports.sh`
  - `watch.sh`
  - `status.sh`
  - `links.sh`
  - `export-output.sh`
- Phase scripts in `scripts/phases/` orchestrate boot substeps.
- Service scripts in `scripts/services/` render generated config/genesis.
- Init scripts in `scripts/init/` are for long-lived service entrypoints only.
- Shared shell helpers live in `scripts/lib/`.

### TypeScript tooling
- `deployer-wallet.ts`: shared L1 mode/deployer resolver for local genesis, generated Sepolia keystore, advanced
  keystore override, and deprecated raw-key compatibility.
- `quickstart-preflight.ts`: host/container L1 preflight entrypoint; in Sepolia mode it generates/resolves the deployer
  before Docker work and stops for funding when needed.
- `sepolia-policy.ts`: shared Sepolia checks, fee defaults, funding defaults, and L1 policy validation.
- `account-setup.ts`: runtime wallet generation, keystores, Web3Signer YAMLs, precomputed addresses.
- `quickstart-invariants.ts`: tested genesis shnarf formula and boot-critical deterministic address precompute.
- `fund-runtime-accounts.ts`: batch funding of generated runtime accounts.
- `aggregate-addresses.ts`: writes deployed-address handoff file.
- `deploy-timing.ts`: writes timing evidence.
- `ensure-demo-erc20.ts`: deploy/reuse demo ERC20 only on demand.
- `traffic-account.ts`: disposable traffic account lifecycle.
- `claim-l2-to-l1.ts`: shared SDK proof/claim helper for L2-to-L1 smoke flows.

### Generated artifacts
Generated, gitignored state is host-backed under `artifacts/`:
- `artifacts/accounts/`
  - runtime keystores
  - Web3Signer YAMLs
  - compatibility env files
  - precomputed addresses
  - optional demo traffic env
- `artifacts/genesis/`
  - rendered L2 genesis
  - fork timestamp
- `artifacts/config/`
  - rendered coordinator/postman/sequencer/Besu/maru/prover config
- `artifacts/deployments/`
  - `addresses.json`
  - `deploy-runtime.env`
  - step logs
  - `deploy-timing.jsonl`
- `artifacts/reports/`
  - reserved for exported runtime reports; current exported bundle still goes to `lineth-output/`

### Runtime accounts
- User-funded deployer in Sepolia mode:
  - default: generated encrypted keystore under `artifacts/accounts/deployer-keystore/`;
  - advanced override: `L1_DEPLOYER_KEYSTORE_PATH` plus `L1_DEPLOYER_KEYSTORE_PASSWORD` or
    `L1_DEPLOYER_KEYSTORE_PASSWORD_FILE`;
  - deprecated compatibility only: `L1_DEPLOYER_PRIVATE_KEY`.
- Generated accounts:
  - L1 blob submitter
  - L1 finalization submitter
  - L1 postman
  - generated L2 deployer
  - L2 message anchorer
  - L2 postman
- The deployer deploys and funds; runtime services should not reuse it.
- Demo traffic uses a separate disposable account created only when needed.

### Sepolia integration
- Sepolia remains the default path with real public L1 finality.
- The default tester path must not ask for a raw Sepolia deployer private key in `.env`.
- If the generated/provided deployer is unfunded, host preflight should print the deployer address, exact minimum
  funding, current balance when known, artifact path when relevant, and the same rerun command.
- Shared policy checks:
  - chain ID
  - deployer balance
  - deployer nonce / pending state
  - current execution fee
  - current blob base fee
  - configured fee caps
- Early failure before expensive boot is intentional.
- Boot/finality timing is materially affected by Sepolia congestion and RPC quality.

### Local L2 stack
- Local Linea L2 remains the core runtime surface:
  - sequencer
  - shomei
  - l2-node-besu
  - coordinator
  - prover
  - postman
  - Blockscout
- Default proving mode for demos is dev-proof mode.
- Partial proving remains optional and heavy.

## Remaining Work

Priority order:

1. Improve startup/watch UX for demos.
   - Current boot logs are still considered too mixed and unclear by review.
   - Labels like “preflight”, “artifacts”, and “compose” need simpler wording and cleaner phase separation.
   - Docker startup, deploy progress, useful links, and finality should be visibly separate.

2. Validate clean-machine prerequisites.
   - Need a crisp answer for minimum dependencies on a clean machine.
   - Current likely answer is Docker + Compose + shell; host Node may be optional if bootstrap stays containerized, but this should be verified explicitly.

3. Continue boot-speed work.
   - Runtime funding improved already.
   - Remaining major bottleneck is Docker-side dependency/tooling install on a cold reset.
   - Next likely direction: tooling image or better cache retention.

4. Define CI maintenance path.
   - Keep normal CI deterministic and cheap.
   - Do not require full Sepolia finality in standard PR CI.

5. Decide whether quickstart can safely remove or disable ForcedTransactionGateway deployment.
   - Still unresolved and needs upstream/protocol confirmation.

6. README polish pass.
   - Current README is useful but too long.
   - It needs compression and removal of stale or duplicated explanation.

7. Decide how prominently to expose local-L1 mode in the default tester story.
   - Sepolia remains the default public-finality path.
   - Product/docs guidance is still needed on when to recommend local-L1 first.

## Active Workstreams

### Default deployer wallet generation flow
- Implemented as the default Sepolia tester flow.
- The shared resolver is `scripts/internal/deployer-wallet.ts`; do not duplicate deployer-source selection in shell.
- Default generated artifact location is `artifacts/accounts/deployer-keystore/`.
- `./scripts/reset.sh` preserves the generated Sepolia deployer by default; only `--forget-deployer` removes it.
- Keep `L1_DEPLOYER_PRIVATE_KEY` available only as deprecated compatibility and do not restore it to `.env.example`.

### Victorien restructure items
Already completed:
- first-pass folder/layout refactor;
- host-backed `artifacts/`;
- split into `phases/services/internal/init/lib`;
- per-service config rendering instead of one large phase-03 renderer;
- explicit Compose mount for service renderers;
- static checks for renderer existence/executability/call sites;
- stale deploy-log reuse checks verify expected addresses and on-chain code before skipping.

Still pending from Victorien feedback:
- no known Victorien restructure blocker remains; keep watching for follow-up review on this branch.

### Clean-machine dependency validation
- Not yet fully documented/validated.
- Need explicit answer for:
  - Docker / Compose minimums
  - shell assumptions
  - whether host Node is optional or required in practice
  - how much the flow degrades when host TS preflight cannot run

### Boot-speed / tooling image improvements
Already completed:
- batched runtime funding;
- removal of base-boot demo ERC20 deployment.

Still pending:
- reduce cold Docker-side `pnpm install` cost;
- decide between a prebuilt quickstart tooling image vs. improved cache strategy;
- keep evidence-driven timing updates, not anecdotal estimates.

## Known Constraints

- This is a dev/demo quickstart, not a production deployment pattern.
- Do not introduce real secrets or production keys into the repo.
- `.env` is still the runtime config contract for v0.
- Compose remains the runtime engine; no big CLI rewrite for now.
- Do not regress the single-command tester path.
- Keep Sepolia as the default public-finality path unless explicitly changed by product direction.
- Local L1 must remain a fallback mode, not a silent behavior change to Sepolia mode.
- Local mode must keep ignoring Sepolia deployer keystore and raw-key config.
- Postman and coordinator should keep signing through Web3Signer.
- Do not reintroduce raw runtime signer private keys into rendered service env.
- Do not log deployer private keys, keystore passwords, or keystore JSON; transient private-key env output is only for
  upstream deploy tools that still require it.
- Avoid broad refactors outside this quickstart package unless required.
- Current implementation is monorepo-bound because deploy tooling bind-mounts monorepo contracts.

## Files And Directories Of Interest

- `README.md`
  Current operator-facing guide; useful but needs a cleanup pass.

- `docker-compose.yml`
  Canonical runtime topology. Local-L1 work is currently here and is the most review-sensitive area.

- `scripts/start.sh`
  Main tester entrypoint and the current focus for demo UX.

- `scripts/watch.sh`
  Guided runtime/finality log stream; still needs UX cleanup and clearer phase reporting.

- `scripts/bootstrap-artifacts.sh`
  Host-backed artifact initialization and migration bridge from old runtime layout.

- `scripts/phases/04-deploy-contracts.sh`
  Most failure-sensitive shell path; deploy-log reuse logic and useful-link timing live here.

- `scripts/internal/account-setup.ts`
  Runtime wallet generation, keystore output, Web3Signer YAML generation, precomputed addresses, and container fallback
  deployer funding gate.

- `scripts/internal/sepolia-policy.ts`
  Sepolia policy source of truth for balance, nonce, fee, blob-fee, and chain checks.

- `scripts/internal/deployer-wallet.ts`
  Shared deployer resolver and shell-env emitter. This owns local-vs-Sepolia deployer selection.

- `scripts/internal/fund-runtime-accounts.ts`
  Current funding optimization path.

- `scripts/internal/traffic-account.ts`
  Shared demo traffic account behavior.

- `scripts/internal/claim-l2-to-l1.ts`
  Shared manual proof/claim helper for L2-to-L1 smokes.

- `scripts/lib/runtime.sh`
  Shared runtime/artifact/env shell helper used across many scripts.

- `config/DEV-KEYS-INVENTORY.md`
  Canonical explanation of committed dev-only identity material.

- `profiles/local-l1.env.example`
  Copy-paste recipe for local-L1 mode.

- `artifacts/`
  Runtime-generated state. Future sessions must treat it as live state, not just scratch output.

- `artifacts/accounts/deployer-keystore/`
  Durable generated Sepolia deployer keystore/password. Preserve by default across reset.

## Validation Process

Minimum fast validation after changes:
- shell syntax:
  - `sh -n` across quickstart shell scripts
  - `bash -n` where needed
- TypeScript unit/mocked tests for quickstart helpers
- `./scripts/check-quickstart-static.sh`
- `docker compose --env-file versions.env --env-file .env --profile stack-partial-prover config`
- `git diff --check`

Behavior validation depends on change class:
- Boot-path changes:
  - `./scripts/reset.sh`
  - `./scripts/start.sh --tail`
  - `./scripts/status.sh`
  - `./scripts/links.sh`
  - `./scripts/export-output.sh`
- Bridge/traffic changes:
  - targeted smoke or traffic scripts against a live stack
- Local-L1 changes:
  - must include a real local-L1 boot, not only compose config rendering
  - should include at least one bridge or traffic smoke when the change affects deployment, funding, or signing behavior

Do not claim success from static checks alone for boot/runtime changes.

## Risks

- The deploy path is nonce/order sensitive; stale assumptions can silently corrupt address wiring.
- Reused deploy logs are dangerous if current expected addresses are not re-verified.
- `start.sh` / `watch.sh` UX changes can easily become shell spaghetti if they mix Docker state, deploy parsing, and user narration carelessly.
- Sepolia mode is vulnerable to public gas spikes, rate limits, and funded-account exhaustion.
- The generated Sepolia deployer is durable tester state; deleting it loses any leftover Sepolia ETH unless the user has
  separately retained the keystore/password.
- No sweep-back exists in v1; users must manage leftover Sepolia ETH manually if they care about recovering funds.
- Local-L1 mode can look “working” even if EL is up but CL/finality is not healthy.
- Quickstart scripts now depend heavily on the artifact layout; path drift will break multiple commands at once.
- Bind-mounted deploy-tooling changes can diverge from upstream contract deploy behavior.
- README drift is a real problem: future contributors can easily add new text without removing obsolete guidance.

## Open Questions

- Should local L1 remain a hidden advanced mode or become a first-class documented option for all testers?
- Should exported reports move fully into `artifacts/reports/`, or should `lineth-output/` remain the operator-facing export bundle?
- Can ForcedTransactionGateway be removed or disabled safely in quickstart deploys?
- What exact clean-machine dependency story do we want to support:
  - Docker/Compose only
  - Docker/Compose + host Node fallback
  - something stricter
- How far should startup log cleanup go before considering a real CLI/wizard?
- Is a small boot-time wizard desired before v1, or is better wording/log structure enough?
