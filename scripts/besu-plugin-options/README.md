# Linea-Besu plugin options generator

Auto-generates **neutral MDX partials** for Linea-Besu plugin CLI options.

**Extract** = Java reflection (`:linea-besu:plugins:besu-plugin-options-docgen`)  
**Render** = this Node tool (MDX partials + safety/completeness validation)

Java `@Option` sources are the single source of truth. Everything under
`output/` is ephemeral, ignored by Git, and regenerated for validation or
publication.

## pnpm workspace capture (hard gate)

This tool is a **standalone pnpm project** (`pnpm-lock.yaml` + local
`pnpm-workspace.yaml` with `"."` only) and is **not** a member of the monorepo
workspace. Install and run with pnpm **inside this directory** — do not add it
to root `pnpm-workspace.yaml`.

## Output model

| Artifact                      | Owner                               | Path                                      |
| ----------------------------- | ----------------------------------- | ----------------------------------------- |
| MDX partials (one per plugin) | Automation                          | `output/_generated/besu/<plugin>.mdx`     |
| JSON manifest + report        | Automation (ephemeral; not shipped) | `output/*.json`                           |
| Wrapper page                  | Human (seeded once)                 | `templates/linea-besu-plugin-options.mdx` |

**Hard invariant:** normal `pnpm run generate` only writes under `output/` (and
the Besu-only `_generated/besu/` namespace). It never overwrites the wrapper or
the shared `_generated/` root. Use
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
pnpm run check                   # re-extracts/renders in temp + validates completeness
pnpm test
pnpm run prettier:check
```

Skip re-extract when iterating on MDX only:

```bash
node generate.js --skip-extract
```

Direct extractor:

```bash
./gradlew :linea-besu:plugins:besu-plugin-options-docgen:generateBesuPluginOptionsManifest
```

## CI and publication

`.github/workflows/besu-plugin-options-docs.yml` validates relevant pull
requests without credentials. Relevant pushes to `main` validate and publish
automatically. Manual runs validate by default; set the `publish` input to
`true` only from `main` or a `linea-besu-*` tag to publish.

Validation generates the ephemeral output, runs the fresh temporary-output
check, tests MDX safety and completeness, checks Prettier, and uploads the
output as a workflow artifact. Publication copies only
`docs/stack/reference/_generated/besu/**` into `Consensys/doc.linea`; any
changed path outside that subtree fails the job.

Publication uses the `DOC_LINEA_PR_APP_ID` and
`DOC_LINEA_PR_PRIVATE_KEY` repository secrets for the GitHub App that opens or
updates the stable `ci/besu-plugin-options-docs` branch.
