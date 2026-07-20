# PLAN — Postman config docs generator

## Status

- [x] Standalone scaffold (`package.json`, `pnpm-workspace.yaml`, `.npmrc`, `.gitignore`)
- [x] Static envLoader ↔ schema correlator (`parse-postman.js`)
- [x] Neutral MDX partials + manifest + report under `output/`
- [x] Human wrapper seed (`templates/linea-postman-options.mdx`)
- [x] Drift + completeness check (`check.js`)
- [x] Workflow `postman-config-docs.yml` (dry-run default; App-token live path)
- [x] Tests incl. public-safe leak scan
- [ ] Live doc.linea page seed (separate; human)
- [ ] App install on Consensys/doc.linea (TechOps gate)

## Acceptance criteria

- Static parse only (no Postman compile/runtime)
- Columns: Env var | Description | Default | Type | Required
- L1/L2 prefix + sponsoring expansion; shared vars once
- Placeholder defaults blank; secrets value-less
- Idempotent generate; drift-check; completeness; public-safe

## Run

```bash
cd scripts/postman-options && pnpm install && pnpm run generate && pnpm run check && pnpm test && pnpm run prettier:check
```

## Latest counts (from generate)

- **Total env vars:** 81 across **9** sections
- General 3 · L1 network 4 · L2 network 6 · Listener 19 · Claiming 17 · Signer 20 · Database 8 · Database cleaner 3 · API 1
- Flagged: 8 missing descriptions (POSTGRES_* passthrough leaves), 18 placeholder defaults, 32 secret/endpoint vars (names only)
- See `output/linea-postman-options.json` → `counts` / `perSection` and `output/report.json`
