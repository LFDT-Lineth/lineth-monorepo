# Prover config docs generator

Standalone Node tool that statically parses `prover/config/*.go` and emits
public-safe TOML config reference MDX partials for [doc.linea](https://docs.linea.build).

**Not a monorepo workspace package.** Install and run only inside this directory.

## Hard invariant

Automation only ever writes under `output/_generated/prover/`.
It never writes the human-owned wrapper (`templates/linea-prover-options.mdx` here;
`docs/stack/reference/linea-prover-options.mdx` on doc.linea).

## Partials + wrapper

| Artifact | Owner | Path |
|----------|-------|------|
| Per-section MDX partials | Generated | `output/_generated/prover/<section>.mdx` |
| Manifest + report | Generated (monorepo drift anchors) | `output/linea-prover-options.json`, `output/report.json` |
| Wrapper page | Human | seeded once into `templates/linea-prover-options.mdx` |

Each partial is neutral markdown (heading + table or note): no front matter, imports, or components.

`traces_limits` is documented as an explanatory note, not a dump of module rows.

## How to run

```bash
cd scripts/prover-options
pnpm install
pnpm run generate:seed-wrapper   # once — creates the human-owned wrapper template
pnpm run generate                # refresh output/ from Go sources
pnpm run check                   # drift + completeness gate
pnpm run test
pnpm run prettier:check
```

## New-section flow

1. Add the nested struct / fields in `prover/config/config.go` (and defaults in `config_default.go` if needed).
2. Run `pnpm run generate` — a new partial appears under `output/_generated/prover/`.
3. Update the human-owned wrapper: import the new partial and render `<Component />`.
4. Run `pnpm run check` (fails until the wrapper imports the new partial).

## Defaults and public-safety

- Defaults come only from `viper.SetDefault` in `config_default.go` (plus resolvable same-file literals).
- Never read values from `config-*.toml` (real addresses, paths, env-specific tables).
- Unresolvable defaults (e.g. cross-package consts) are left blank and listed in `report.json`.
- Missing or dev-only descriptions (`TODO`, `not serialized`, …) are blanked and listed in `report.json`.

## doc.linea PR flow + app-install gate

Workflow: `.github/workflows/prover-config-docs.yml`

1. **Dry-run (default):** `workflow_dispatch` with `dry_run=true` → `pnpm run check` only; prints partial list; no PR; no App token.
2. **Live:** `dry_run=false` on `main` (or a `prover-*` tag once enabled) → generate → write publish-only `provenance.mdx` → mint App token → copy **only** `output/_generated/prover/` → `docs/stack/reference/_generated/prover/` → refuse if the wrapper would change → open PR with `add-paths` limited to that tree.

**App-install gate:** live PRs require the `CREATE_GITHUB_PR` GitHub App installed on `Consensys/doc.linea` (`CREATE_GITHUB_PR_APP_ID` / `CREATE_GITHUB_PR_PRIVATE_KEY`). Until TechOps confirms the install, keep using dry-run and leave the push/tag trigger commented out.
