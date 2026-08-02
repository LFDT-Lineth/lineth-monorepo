const path = require("node:path");

const { build } = require("./lib");
const { writeOutput } = require("./output");
const { TOOL_ROOT, MONOREPO_ROOT, OUTPUT_DIR, GENERATED_DIR, parseMonorepoArg, hasFlag } = require("./paths");

async function main() {
  const monorepoPath = parseMonorepoArg() || MONOREPO_ROOT;
  if (hasFlag("--seed-wrapper")) {
    console.warn(`--seed-wrapper is handled by seed-wrapper.js (pnpm run generate:seed-wrapper), not generate.js.`);
  }

  const result = await build({
    monorepoRoot: monorepoPath,
    toolRoot: TOOL_ROOT,
  });

  writeOutput(result, OUTPUT_DIR);

  const c = result.manifest.counts;
  console.log(
    `Generated ${c.total} TOML config keys across ${c.sections} sections ` + `(${c.excluded} excluded field(s)).`,
  );
  console.log("Per-section breakdown:");
  for (const [id, info] of Object.entries(result.manifest.perSection)) {
    if (info.noteOnly) {
      console.log(`  - ${info.title} (${id}): note only (no table rows)`);
      continue;
    }
    console.log(`  - ${info.title} (${id}): ${info.keyCount} key(s)`);
  }
  console.log(`  total rows: ${result.rowCount}`);
  console.log(`  manifest: ${path.relative(MONOREPO_ROOT, path.join(OUTPUT_DIR, "linea-prover-options.json"))}`);
  console.log(`  report:   ${path.relative(MONOREPO_ROOT, path.join(OUTPUT_DIR, "report.json"))}`);
  console.log(`  partials: ${result.partials.length} under ${path.relative(MONOREPO_ROOT, GENERATED_DIR)}`);
  for (const p of result.partials) {
    console.log(`    - _generated/prover/${p.relPath}`);
  }
  if (result.report.missingDescriptions?.length) {
    console.log(`  flagged: ${result.report.missingDescriptions.length} key(s) with missing/dev-only descriptions.`);
  }
  if (result.report.unresolvedDefaults?.length) {
    console.log(
      `  flagged: ${result.report.unresolvedDefaults.length} key(s) with an unresolved default (left blank).`,
    );
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
