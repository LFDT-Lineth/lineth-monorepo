# Linea-Besu plugin options generator

Auto-generates **neutral MDX partials** for Linea-Besu plugin CLI options from
Java `@Option` sources in this monorepo. The Java source is the single source of
truth; committed output under `output/` must never be hand-edited.

## pnpm workspace capture (hard gate)

This tool is a **standalone pnpm project** (`pnpm-lock.yaml` + local
`pnpm-workspace.yaml` with `"."` only) and is **not** a member of the monorepo
workspace. Install and run with pnpm **inside this directory** — do not add it
to root `pnpm-workspace.yaml`.

Current root `pnpm-workspace.yaml` globs (`contracts/**`, `e2e/**`, …) do **not**
match `scripts/**`. Before moving this tool or adding a `package.json` under a
workspace glob:

1. Re-read `pnpm-workspace.yaml`.
2. If the new path would match a package glob, either add an explicit negation
   (e.g. `!scripts/besu-plugin-options`) or relocate outside workspace packages.

Do **not** add this directory to the workspace — doing so can break root
`pnpm install`.

## Output model

| Artifact | Owner | Path |
| --- | --- | --- |
| MDX partials (one per plugin) | Automation | `output/_generated/<plugin>.mdx` |
| JSON manifest + report | Automation (monorepo drift anchor; not shipped) | `output/*.json` |
| Wrapper page | Human (seeded once) | `templates/linea-besu-plugin-options.mdx` |

**Hard invariant:** normal `pnpm run generate` only writes under `output/` (and
`_generated/`). It never overwrites the wrapper. Use
`pnpm run generate:seed-wrapper` once to create the template when missing.

Each partial is plain markdown (`##` plugin heading + unique `### Plugin — Group`
sections + tables) — no front matter, no imports, no custom React. doc.linea’s
theme styles it. Headings are prefixed with the plugin name so the right-side
TOC anchors stay unique.

The starter wrapper imports partials with the Docusaurus 3.10 pattern:

```mdx
import Sequencer from './_generated/sequencer.mdx';

<Sequencer />
```

`_generated/` matches doc.linea’s `**/_*/**` docs exclude (not routed; still
importable).

### Fallback ladder (if doc.linea build rejects imports)

1. Preferred: `_generated/<plugin>.mdx` + import/`<X/>` (this tool).
2. Flatten to sibling `_*.mdx` files next to the wrapper.
3. Single `_generated-body.mdx` imported once.
4. One consolidated generated page (pilot model).

## Scope

- Flags starting with `--plugin-` only.
- Plugins: Sequencer, Tracer; State recovery gets a “no options” note in the
  wrapper. `sequencer-interfaces` skipped. Forced-tx group excluded (unreleased).
- Hidden options included and marked **Advanced**.

## Run

```bash
cd scripts/besu-plugin-options
pnpm install
pnpm run generate:seed-wrapper   # once, if templates/ wrapper is missing
pnpm run generate
pnpm run check                   # drift + completeness; exit 1 if stale
pnpm test
pnpm run prettier:check
```

Optional: `--monorepo /path/to/linea-monorepo` or `LINEA_MONOREPO_PATH`
(defaults to this checkout).

## How a new group is picked up

1. New `*CliOptions.java` / `@Option` under an **existing** plugin root → regenerate
   updates that plugin’s `_generated/<plugin>.mdx` (no wrapper change).
2. A **brand-new plugin** with options → generate emits a new `_generated/<plugin>.mdx`;
   `pnpm run check` fails completeness until a human adds the import + `<X />` to
   the wrapper (in this template and eventually in doc.linea).
3. Content (table rows) updates are fully automatic on regenerate.

## Delivery to doc.linea

GitHub Actions workflow `.github/workflows/besu-plugin-options-docs.yml`:

- Default **dry-run**: generate + check; no PR.
- Live path: App token PR into `Consensys/doc.linea` writing **only**
  `docs/stack/reference/_generated/` (never the wrapper).

Live PR requires the `CREATE_GITHUB_PR` GitHub App installed on doc.linea
(TechOps). Until then, dry-run is fully testable.
