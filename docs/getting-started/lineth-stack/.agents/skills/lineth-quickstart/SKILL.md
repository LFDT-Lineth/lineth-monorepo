---
name: lineth-quickstart
description: >-
  Operating manual for the Lineth Stack quickstart — the Docker-Compose dev/demo stack at
  docs/getting-started/lineth-stack in the lineth-monorepo that boots a local Linea/Lineth L2 with
  Sepolia or local L1 finality. Use whenever you are working inside the lineth-stack quickstart and
  need to boot or run the stack, choose between local and Sepolia L1 mode, get past the Sepolia
  deployer-funding step, read the finality/success signals, run traffic or bridge smoke tests, reset
  it, or debug a failing boot (L1 RPC unreachable, deploy gas too low, unfunded deployer, port
  collision, prover out-of-memory, address mismatch, finality not advancing). Trigger even on casual
  asks — "start the stack", "run the quickstart", "boot lineth locally", "my deploy keeps failing",
  "finality isn't happening", "reset the quickstart". When editing this package's scripts or docs,
  also read rules/making-changes.md so commits pass the repo's lint and DCO gates.
license: AGPL-3.0
metadata:
  author: linea
  version: '1.0.0'
---

# Lineth Stack quickstart — agent skill

This skill helps you operate the Lineth Stack quickstart: a Docker-Compose dev/demo stack that boots
a local Linea/Lineth L2 settled by either **Sepolia** (default, real public L1) or a self-contained
**local** Besu+Teku L1. It is a dev/demo tool, not a production deployment.

The skill does **not** restate the boot procedure — that has one source of truth in the package, and
duplicating it here would let the two drift apart. Your job is to route to that source, then layer on
the failure triage and change/commit rules the skill bundles.

## Start here

Run everything from `docs/getting-started/lineth-stack/` (find it with `git rev-parse --show-toplevel`
then `cd` in). Then read these two files before acting — they are the procedure, and if anything here
ever disagrees with them, **they win**:

- `AGENTS.md` — the canonical operating runbook: preflight, L1-mode choice, the boot commands, the
  Sepolia funding stop, success signals, reset, and guardrails. Follow it step by step.
- `README.md` — the operator guide it draws on: endpoints, gas caps, prover modes, full
  troubleshooting.

Don't work from memory or from this file's summary — the stack is nonce/order-sensitive and
artifact-driven, so a 60-second read of the current `AGENTS.md` prevents a failed boot.

## Two things agents most often get wrong

These are the classic false steps; `AGENTS.md` has the detail, but keep them front of mind:

- **The Sepolia first-boot "exit" is expected, not an error.** First Sepolia boot generates the
  deployer keystore, prints the address + funding amount, and stops before any container starts. You
  cannot fund it — surface the address to the human, then rerun the same command. For unattended runs,
  choose `--l1-mode local` instead, which needs no funding.
- **Success means finality, not a blob.** Report success only on an observed `finalizeBlocks` and an
  advancing rollup `currentL2BlockNumber`. `Submit Blobs` only publishes data — it is not finalization.

## When something fails

Read `rules/debugging.md` — it has a symptom → cause → fix table (RPC unreachable, gas too low,
unfunded deployer, port collision, prover OOM, address mismatch, finality not advancing), the
logs-first diagnostics, and timing expectations so "slow" isn't mistaken for "broken". Hard STOP
conditions that need a human: an unfunded Sepolia deployer and repeated Sepolia gas/RPC failures.

## When you change scripts or docs in this package

Read `rules/making-changes.md` — read-before-change, the validation gate
(`./scripts/check-quickstart-static.sh` plus a real boot for boot-path changes), and the repo's commit
rules that reject otherwise-fine commits: scope must be from the allowed list (use `misc`;
`lineth-stack` is rejected), `git commit -s` for DCO, and a GitHub noreply author email or the push is
blocked. Never commit `.env`, `artifacts/`, or any real secret.
