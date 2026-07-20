# Prover options docs generator — PLAN

## Status

- [x] Branch `docs/prover-config-docs-gen` (local main; remote pull blocked by DNS)
- [x] Scaffold + static Go extract
- [x] Render / generate / check / seed-wrapper
- [x] Tests + verification loop green
- [x] Workflow + README
- [x] Final status printed (no commit/push/PR)

## Acceptance checklist

- [x] Static Go parse of config.go + config_default.go (+ types)
- [x] Per-section neutral partials under `output/_generated/prover/`
- [x] Manifest + report.json; starter wrapper via seed only
- [x] Count assertion: rendered rows == documentable keys (59)
- [x] traces_limits = note; env-only / `-` excluded; gaps in report
- [x] Drift-check + completeness + idempotency
- [x] prettier --check + node --test (incl. public-safe leak) — 13/13
- [x] Workflow dry-run path; live writes only `_generated/prover/`

## Run

```bash
cd scripts/prover-options
pnpm install
pnpm run generate:seed-wrapper
pnpm run generate
pnpm run check
pnpm run prettier:check
pnpm run test
```

## Latest generate counts

| Section | Keys |
|---------|------|
| top-level | 4 |
| controller | 14 |
| execution | 8 |
| data-availability | 6 |
| invalidity | 5 |
| aggregation | 5 |
| public-input-interconnection | 7 |
| debug | 6 |
| layer2 | 4 |
| traces-limits | note only |
| **total rows** | **59** |
| excluded | 11 |
| missing/dev-only descriptions | 20 |
| unresolved defaults | 1 (`data_availability.max_uncompressed_nb_bytes` ← `v1.MaxUncompressedBytes`) |
