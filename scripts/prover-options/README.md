# Prover config docs generator

Standalone Node tool that statically parses `prover/config/*.go` and emits
public-safe TOML config reference MDX partials for [doc.linea](https://docs.linea.build).

**Not a monorepo workspace package.** Install and run only inside this directory.

## Hard invariant

Generated artifacts are ephemeral and ignored by Git. Automation writes them only under `output/`,
and publishing copies only `output/_generated/prover/`.
It never writes the human-owned wrapper (`templates/linea-prover-options.mdx` here;
`docs/stack/reference/linea-prover-options.mdx` on doc.linea).

## Partials + wrapper

| Artifact                 | Owner                | Path                                                     |
| ------------------------ | -------------------- | -------------------------------------------------------- |
| Per-section MDX partials | Generated, ephemeral | `output/_generated/prover/<section>.mdx`                 |
| Manifest + report        | Generated, ephemeral | `output/linea-prover-options.json`, `output/report.json` |
| Wrapper page             | Human                | seeded once into `templates/linea-prover-options.mdx`    |

Each partial is neutral markdown (heading + table or note): no front matter, imports, or components.

`traces_limits` is documented as an explanatory note, not a dump of module rows.

## How to run

```bash
cd scripts/prover-options
pnpm install --frozen-lockfile
pnpm run generate:seed-wrapper   # once — creates the human-owned wrapper template
pnpm run generate                # create ignored review/publish artifacts under output/
pnpm run check                   # validate fresh temporary output + wrapper completeness
pnpm run test
pnpm run prettier:check
```

`check` does not read or compare against `output/`; it generates into a temporary directory and
removes that directory after validation. Run `generate` first when you want local review artifacts.

## New-section flow

1. Add the nested struct / fields in `prover/config/config.go` (and defaults in `config_default.go` if needed).
2. Run `pnpm run generate` — a new partial appears under `output/_generated/prover/`.
3. Update the human-owned wrapper: import the new partial and render `<Component />`.
4. Run `pnpm run check` (fails until the wrapper imports the new partial).

## Defaults and public-safety

- Defaults come only from `viper.SetDefault` in `config_default.go` (plus resolvable same-file literals).
- Never read values from `config-*.toml` (real addresses, paths, env-specific tables).
- Unresolvable defaults (e.g. cross-package consts) are left blank and listed in the ephemeral `report.json`.
- Missing or dev-only descriptions (`TODO`, `not serialized`, …) are blanked and listed in the ephemeral `report.json`.

## Validation and doc.linea publishing

Workflow: `.github/workflows/prover-config-docs.yml`

1. Pull requests that change Prover config sources, generator inputs, or the workflow validate without credentials.
   Validation generates artifacts, checks fresh temporary output and wrapper completeness, runs unit tests and
   Prettier, then uploads `output/` for review.
2. Relevant pushes to `main` validate and publish automatically. Publishing adds provenance, copies only
   `output/_generated/prover/` to `docs/stack/reference/_generated/prover/`, enforces that no other doc.linea
   path changed, and updates the stable `ci/prover-config-docs` pull-request branch.
3. `workflow_dispatch` is the recovery path. Its `publish` input defaults to `false`; `publish=true` is accepted
   only from `main` or a `prover-*` tag.

Publishing uses the `DOC_LINEA_PR_APP_ID` and `DOC_LINEA_PR_PRIVATE_KEY` repository secrets. Validation never
accesses those credentials.
