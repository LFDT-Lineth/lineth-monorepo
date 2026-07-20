const fs = require("node:fs");
const path = require("node:path");

const { build } = require("./lib");
const {
  TOOL_ROOT,
  MONOREPO_ROOT,
  OUTPUT_DIR,
  GENERATED_DIR,
  MANIFEST_PATH,
  REPORT_PATH,
  parseMonorepoArg,
  hasFlag,
} = require("./paths");

/** Recursively remove files under dir but keep the directory. */
function emptyDir(dir) {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    fs.rmSync(full, { recursive: true, force: true });
  }
}

async function main() {
  const monorepoPath = parseMonorepoArg() || MONOREPO_ROOT;
  if (hasFlag("--seed-wrapper")) {
    console.warn(`--seed-wrapper is handled by seed-wrapper.js (pnpm run generate:seed-wrapper), not generate.js.`);
  }

  const result = await build({
    monorepoRoot: monorepoPath,
    toolRoot: TOOL_ROOT,
  });

  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  emptyDir(GENERATED_DIR);
  fs.mkdirSync(GENERATED_DIR, { recursive: true });

  fs.writeFileSync(MANIFEST_PATH, result.manifestJson);
  fs.writeFileSync(REPORT_PATH, result.reportJson);

  for (const part of result.partials) {
    const abs = path.join(GENERATED_DIR, part.relPath);
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, part.markdown);
  }

  const c = result.manifest.counts;
  console.log(`Generated ${c.total} Postman env var(s) across ${c.sections} sections.`);
  console.log("Per-section breakdown:");
  for (const [id, info] of Object.entries(result.manifest.perSection)) {
    console.log(`  - ${info.title} (${id}): ${info.keyCount} env var(s)`);
  }
  console.log(`  total rows: ${result.rowCount}`);
  console.log(`  manifest: ${path.relative(MONOREPO_ROOT, MANIFEST_PATH)}`);
  console.log(`  report:   ${path.relative(MONOREPO_ROOT, REPORT_PATH)}`);
  console.log(`  partials: ${result.partials.length} under ${path.relative(MONOREPO_ROOT, GENERATED_DIR)}`);
  for (const p of result.partials) {
    console.log(`    - _generated/postman/${p.relPath}`);
  }
  if (result.report.missingDescriptions?.length) {
    console.log(`  flagged: ${result.report.missingDescriptions.length} env var(s) with missing/blank descriptions.`);
  }
  if (result.report.placeholderDefaults?.length) {
    console.log(
      `  flagged: ${result.report.placeholderDefaults.length} env var(s) with placeholder defaults (left blank).`,
    );
  }
  if (result.report.secretVars?.length) {
    console.log(`  flagged: ${result.report.secretVars.length} secret/endpoint env var(s) (values never published).`);
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
