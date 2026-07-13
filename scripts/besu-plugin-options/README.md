# Linea-Besu plugin options generator

Auto-generates **neutral MDX partials** for Linea-Besu plugin CLI options.

**Extract** = Java reflection (`:linea-besu:plugins:besu-plugin-options-docgen`)  
**Render** = this Node tool (MDX partials + drift/completeness)

Java `@Option` sources are the single source of truth; committed output under
`output/` must never be hand-edited.

## pnpm workspace capture (hard gate)

This tool is a **standalone pnpm project** (`pnpm-lock.yaml` + local
`pnpm-workspace.yaml` with `"."` only) and is **not** a member of the monorepo
workspace. Install and run with pnpm **inside this directory** — do not add it
to root `pnpm-workspace.yaml`.

## Output model

| Artifact | Owner | Path |
| --- | --- | --- |
| MDX partials (one per plugin) | Automation | `output/_generated/<plugin>.mdx` |
| JSON manifest + report | Automation (monorepo drift anchor; not shipped) | `output/*.json` |
| Wrapper page | Human (seeded once) | `templates/linea-besu-plugin-options.mdx` |

**Hard invariant:** normal `pnpm run generate` only writes under `output/` (and
`_generated/`). It never overwrites the wrapper. Use
`pnpm run generate:seed-wrapper` once to create the template when missing.

## Scope

- Flags starting with `--plugin-` only.
- Plugins: Sequencer, Tracer; State recovery gets a “no options” note in the
  wrapper. Forced-tx group excluded (unreleased).
- Hidden options included and marked **Advanced**.

## Prerequisites

- JDK **25+**
- `go-corset` on `PATH` (Gradle installs it via `:tracer:arithmetization:installGoCorset` when Go is available)
- Node/pnpm as in root `AGENTS.md`

## Run

```bash
cd scripts/besu-plugin-options
pnpm install
pnpm run generate:seed-wrapper   # once, if templates/ wrapper is missing
pnpm run generate                # runs Gradle extractor, then renders MDX
pnpm run check                   # re-extracts to temp + drift/completeness
pnpm test
```

Skip re-extract when iterating on MDX only:

```bash
node generate.js --skip-extract
```

Direct extractor:

```bash
./gradlew :linea-besu:plugins:besu-plugin-options-docgen:generateBesuPluginOptionsManifest
```
