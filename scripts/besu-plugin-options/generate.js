const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

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

function runJavaExtractor(monorepoPath) {
  if (hasFlag("--skip-extract")) return;
  const gradlew = path.join(monorepoPath, "gradlew");
  if (!fs.existsSync(gradlew)) {
    throw new Error(`gradlew not found at ${gradlew}`);
  }
  const result = spawnSync(
    gradlew,
    [":linea-besu:plugins:besu-plugin-options-docgen:generateBesuPluginOptionsManifest", "--quiet"],
    {
      cwd: monorepoPath,
      stdio: "inherit",
      env: process.env,
    },
  );
  if (result.status !== 0) {
    throw new Error(
      `Java extractor failed (exit ${result.status}). Ensure JDK 25+ and go-corset are available.`,
    );
  }
}

async function main() {
  const monorepoPath = parseMonorepoArg() || MONOREPO_ROOT;
  if (hasFlag("--seed-wrapper")) {
    console.warn(
      `--seed-wrapper is handled by seed-wrapper.js (pnpm run generate:seed-wrapper), not generate.js.`,
    );
  }

  runJavaExtractor(monorepoPath);

  const result = await build({
    toolRoot: TOOL_ROOT,
  });

  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
  emptyDir(GENERATED_DIR);
  fs.mkdirSync(GENERATED_DIR, { recursive: true });

  // Manifest/report are written by the Java extractor; re-canonicalize via Prettier.
  fs.writeFileSync(MANIFEST_PATH, result.manifestJson);
  fs.writeFileSync(REPORT_PATH, result.reportJson);

  for (const part of result.partials) {
    const abs = path.join(GENERATED_DIR, part.relPath);
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, part.markdown);
  }

  const c = result.manifest.counts;
  console.log(
    `Generated ${c.total} plugin options across ${c.plugins} plugins / ${c.groups} groups ` +
      `(${c.standard} standard, ${c.advanced} advanced); ` +
      `${c.excludedOptions} option(s) in ${c.excludedGroups} excluded group(s).`,
  );
  console.log("Per-plugin breakdown:");
  for (const p of result.manifest.perPlugin) {
    if (!p.hasOptions) {
      console.log(`  - ${p.title}: no plugin-specific CLI options`);
      continue;
    }
    console.log(
      `  - ${p.title}: ${p.total} total (${p.standard} standard, ${p.advanced} advanced), ${p.classes} group(s)`,
    );
  }
  console.log(`  manifest: ${path.relative(MONOREPO_ROOT, MANIFEST_PATH)}`);
  console.log(`  report:   ${path.relative(MONOREPO_ROOT, REPORT_PATH)}`);
  console.log(`  partials: ${result.partials.length} under ${path.relative(MONOREPO_ROOT, GENERATED_DIR)}`);
  for (const p of result.partials) {
    console.log(`    - _generated/${p.relPath}`);
  }
  if (result.report.unresolvedDefaults?.length) {
    console.log(
      `  flagged: ${result.report.unresolvedDefaults.length} option(s) with an unresolved default (left blank).`,
    );
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
