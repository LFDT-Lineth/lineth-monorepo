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
  WRAPPER_TEMPLATE_PATH,
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
  const seedWrapper = hasFlag("--seed-wrapper");
  const result = await build({
    monorepoPath,
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

  let seeded = false;
  if (seedWrapper) {
    fs.mkdirSync(path.dirname(WRAPPER_TEMPLATE_PATH), { recursive: true });
    try {
      // Exclusive create avoids a TOCTOU race with existsSync + writeFileSync.
      fs.writeFileSync(WRAPPER_TEMPLATE_PATH, result.wrapperMarkdown, { flag: "wx" });
      seeded = true;
    } catch (err) {
      if (err && err.code === "EEXIST") {
        console.warn(
          `Refusing to overwrite existing wrapper at ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)}. ` +
            "Delete it first if you intentionally want to re-seed.",
        );
      } else {
        throw err;
      }
    }
  }

  const c = result.manifest.counts;
  console.log(
    `Generated ${c.total} plugin options across ${c.plugins} plugins / ${c.groups} groups ` +
      `(${c.standard} standard, ${c.advanced} advanced); ` +
      `${c.excludedOptions} option(s) in ${c.excludedGroups} excluded group(s).`,
  );
  console.log("Per-plugin breakdown:");
  for (const p of result.manifest.perPlugin) {
    const detail = p.hasOptions
      ? `${p.total} total (${p.standard} standard, ${p.advanced} advanced), ${p.classes} group(s)`
      : "no plugin-specific CLI options";
    console.log(`  - ${p.title}: ${detail}`);
  }
  console.log(`  manifest: ${path.relative(MONOREPO_ROOT, MANIFEST_PATH)}`);
  console.log(`  report:   ${path.relative(MONOREPO_ROOT, REPORT_PATH)}`);
  console.log(`  partials: ${result.partials.length} under ${path.relative(MONOREPO_ROOT, GENERATED_DIR)}`);
  for (const part of result.partials) {
    console.log(`    - _generated/${part.relPath}`);
  }
  if (seeded) {
    console.log(`  seeded wrapper: ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)}`);
  } else if (!fs.existsSync(WRAPPER_TEMPLATE_PATH)) {
    console.warn("  wrapper template missing — run with --seed-wrapper once to create it.");
  }

  const r = result.report;
  if (r.missingDescriptions.length) {
    console.warn(`  flagged: ${r.missingDescriptions.length} option(s) missing a description.`);
  }
  if (r.unresolvedDefaults.length) {
    console.warn(`  flagged: ${r.unresolvedDefaults.length} option(s) with an unresolved default (left blank).`);
  }
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
