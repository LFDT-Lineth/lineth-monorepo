# Postman options docs generator

Standalone tool that statically extracts Linea Postman environment variables from
`postman/src/application/postman/app/config/envLoader.ts`, correlates them with
Zod `.describe()` / types / optionality in `schema.ts`, and writes neutral MDX
partials for docs.linea.build.

This is **not** a monorepo workspace package. It has its own `pnpm-workspace.yaml`
(`packages: ["."]`) so `pnpm install` stays local.

## Hard invariant

Automation writes **only** `output/_generated/postman/`. The human-owned wrapper
(`templates/linea-postman-options.mdx`, and later the live doc.linea page) is
never modified by `generate` or the GitHub Action.

| Artifact | Owner |
|----------|--------|
| `output/_generated/postman/*.mdx` | Automation |
| `output/linea-postman-options.json` | Automation (drift anchor) |
| `output/report.json` | Automation (gaps / placeholders / secrets) |
| `templates/linea-postman-options.mdx` | Human (seed once) |

## How to run

```bash
cd scripts/postman-options
pnpm install
pnpm run generate          # refresh output/
pnpm run check             # drift + wrapper completeness
pnpm test
pnpm run prettier:check
```

First-time wrapper seed (refuses to overwrite if present):

```bash
pnpm run generate:seed-wrapper
```

## Correlation approach

1. **Inventory** — env var names, defaults, and assignment targets from `envLoader.ts`
   (including L1/L2 prefix expansion and sponsoring `${opposite}_${prefix}_…`).
2. **Schema** — field descriptions/types/optional/constraints from `schema.ts` `.describe()`.
3. **Join** on schema field name (e.g. `maxNonceDiff` ← `MAX_NONCE_DIFF`).
4. Never fabricate descriptions; passthrough DB leaves (`POSTGRES_*`) are listed in
   `report.json` with blank Description.
5. Never read `.env` / `.env.sample` for values. Placeholders (`""`, `"0x"`) → blank Default.

## New section flow

1. Add env vars to `ENV_INVENTORY` in `parse-postman.js` (and `SECTION_META` if needed).
2. `pnpm run generate`
3. Manually add `import` + `<Component />` for the new partial in the wrapper.
4. `pnpm run check` fails until the wrapper is updated.

## doc.linea PR + App-install gate

Workflow: `.github/workflows/postman-config-docs.yml`

- Default `dry_run=true`: install + `pnpm run check` only.
- Live path (`dry_run=false` on `main` or `postman-*` tag): generate → mint App token
  (`owner: Consensys`, `repositories: doc.linea`) → copy **only**
  `docs/stack/reference/_generated/postman/` → open PR.
- Wrapper-untouched guard refuses if the human page would change.
- Until TechOps installs `CREATE_GITHUB_PR` on `Consensys/doc.linea`, keep dry-run.
