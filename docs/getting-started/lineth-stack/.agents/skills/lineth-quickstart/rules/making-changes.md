# Changing scripts or docs in the Lineth Stack quickstart

This is the secondary workflow: editing the package, not just running it. The goal is changes that
boot correctly and land in a commit that actually pushes — this repo has lint, DCO, and email gates
that reject otherwise-fine commits, so knowing them up front saves a frustrating loop.

## Before you edit

- Read the target file **and its tests** (`scripts/internal/*.test.ts`, the assertions in
  `scripts/check-quickstart-static.sh`). Tests often pin exact error strings; changing wording breaks
  them.
- Keep the boundary: shell stays Docker orchestration glue; structured wallet/RPC/gas/JSON/funding
  logic belongs in `scripts/internal/*.ts`. Don't move logic into shell to "keep it simple".
- Don't duplicate a helper that already exists. Shell helpers live in `scripts/lib/runtime.sh`
  (~40 `lineth_*` functions); TS infra helpers in `scripts/internal/lib/`. Reuse them.

## Validate (the gate)

Cheap, deterministic, safe in normal PR CI:

```bash
find scripts -name '*.sh' -print0 | xargs -0 -n1 sh -n   # shell syntax (bash -n where a script needs bash)
./scripts/check-quickstart-static.sh                     # package invariants + forbidden-helper guards
docker compose --env-file versions.env --env-file .env --profile stack-partial-prover config
# plus the mocked TypeScript checks for .env, RPC, wallet generation, funding, address aggregation
```

`check-quickstart-static.sh` is the source of truth for which scripts must source the shared libs and
which helper names may not be redefined locally — if it fails, read its message rather than working
around it.

**Do not claim success from static checks alone for boot or runtime changes.** Those require a real
boot (`./scripts/reset.sh` → `./scripts/start.sh --tail`), and at least one bridge/traffic smoke when
the change touches deployment, funding, or signing. Do not put full Sepolia finality in normal PR CI;
it depends on live gas, RPC, and funded accounts.

## Commit conventions (these gates reject commits — get them right the first time)

The repo runs a `commit-msg` hook (commitlint) and a DCO check, and GitHub enforces email privacy.
All three have bitten real commits here:

1. **Scope must be from the allowed list.** Allowed scopes include: `coordinator`, `prover`,
   `prover-ray`, `verifier-ray`, `postman`, `tx-exclusion-api`, `linea-besu`, `contracts`,
   `sdk-core`, `sdk-ethers`, `sdk-viem`, `tracer`, `sequencer`, `state-recovery`, `jvm-libs`,
   `blob-libs`, `e2e`, `ci`, `docker`, `deps`, `misc`, `maru`. **`lineth-stack` is NOT allowed** —
   the hook rejects it. Use `misc` for quickstart changes and put the package name in the message
   text, e.g. `docs(misc): rewrite lineth-stack AGENTS.md ...`.
2. **DCO sign-off is required.** Commit with `git commit -s` so the body carries a
   `Signed-off-by:` line.
3. **Author email must be a GitHub noreply address**, or the push is rejected with `GH007: Your push
   would publish a private email address`. Match the address the branch's other commits already use —
   check it with `git log -1 --format='%an <%ae>'` on the branch and set `git config user.email`
   to that `*@users.noreply.github.com` value before committing. The DCO sign-off (`-s`) then uses the
   same address.

A clean commit + push therefore looks like:

```bash
git config user.name  "<name used on the branch>"
git config user.email "<id>+<user>@users.noreply.github.com"

git add <only the files you changed>
git commit -s -m "docs(misc): <what changed in the quickstart>" \
              -m "<why, and any caveats reviewers should verify>"
git push origin <branch>
```

If you already committed with the wrong email/scope, fix without piling up commits:
`git commit --amend --reset-author -s -m "..."` (re-author + re-sign), then push.

## Never

- Commit `.env`, anything under `artifacts/`, or any real secret/private key — all are gitignored on
  purpose.
- Stage unrelated changes into the same commit, or commit/push without the owner's explicit go-ahead.
- Leave a personal name or internal review note in a file destined for the public repo.
