const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const { buildFromManifest, checkCompleteness } = require("./lib");
const {
  TOOL_ROOT,
  MONOREPO_ROOT,
  GENERATED_DIR,
  MANIFEST_PATH,
  REPORT_PATH,
  WRAPPER_TEMPLATE_PATH,
  parseMonorepoArg,
} = require("./paths");

function listGeneratedPartials() {
  const out = [];
  if (!fs.existsSync(GENERATED_DIR)) return out;
  function walk(dir, prefix) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full, rel);
      else if (entry.name.endsWith(".mdx")) out.push(rel.replace(/\\/g, "/"));
    }
  }
  walk(GENERATED_DIR, "");
  return out.sort();
}

function runJavaExtractorTo(monorepoPath, manifestOut, reportOut) {
  const gradlew = path.join(monorepoPath, "gradlew");
  const result = spawnSync(
    gradlew,
    [
      ":linea-besu:plugins:besu-plugin-options-docgen:generateBesuPluginOptionsManifest",
      `-PbesuPluginOptionsManifest=${manifestOut}`,
      `-PbesuPluginOptionsReport=${reportOut}`,
      "--quiet",
    ],
    { cwd: monorepoPath, stdio: "inherit", env: process.env },
  );
  if (result.status !== 0) {
    throw new Error(`Java extractor failed (exit ${result.status}).`);
  }
}

async function check() {
  const monorepoPath = parseMonorepoArg() || MONOREPO_ROOT;
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "besu-plugin-options-"));
  const freshManifest = path.join(tmp, "linea-besu-plugin-options.json");
  const freshReport = path.join(tmp, "report.json");

  runJavaExtractorTo(monorepoPath, freshManifest, freshReport);

  const manifest = JSON.parse(fs.readFileSync(freshManifest, "utf8"));
  const report = JSON.parse(fs.readFileSync(freshReport, "utf8"));
  const result = await buildFromManifest({ manifest, report, toolRoot: TOOL_ROOT });

  const failures = [];
  const targets = [
    { label: "manifest", file: MANIFEST_PATH, expected: result.manifestJson },
    { label: "report", file: REPORT_PATH, expected: result.reportJson },
  ];
  for (const t of targets) {
    if (!fs.existsSync(t.file)) {
      failures.push(`${t.label}: missing committed output at ${path.relative(MONOREPO_ROOT, t.file)}`);
      continue;
    }
    const actual = fs.readFileSync(t.file, "utf8");
    if (actual !== t.expected) {
      failures.push(`${t.label}: ${path.relative(MONOREPO_ROOT, t.file)} is out of date (run pnpm run generate).`);
    }
  }

  const expectedPartials = new Map(result.partials.map((p) => [p.relPath.replace(/\\/g, "/"), p.markdown]));
  const onDisk = listGeneratedPartials();

  for (const rel of expectedPartials.keys()) {
    if (!onDisk.includes(rel)) failures.push(`partial: missing _generated/${rel}`);
  }
  for (const rel of onDisk) {
    if (!expectedPartials.has(rel)) {
      failures.push(`partial: unexpected _generated/${rel} (not produced by current sources)`);
    }
  }
  for (const [rel, expected] of expectedPartials) {
    const file = path.join(GENERATED_DIR, rel);
    if (!fs.existsSync(file)) continue;
    if (fs.readFileSync(file, "utf8") !== expected) {
      failures.push(`partial: _generated/${rel} is out of date (run pnpm run generate).`);
    }
  }

  if (!fs.existsSync(WRAPPER_TEMPLATE_PATH)) {
    failures.push(
      `wrapper: missing template at ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)} ` +
        `(run pnpm run generate:seed-wrapper once).`,
    );
  } else {
    const wrapper = fs.readFileSync(WRAPPER_TEMPLATE_PATH, "utf8");
    failures.push(...checkCompleteness(wrapper, [...expectedPartials.keys()]).map((f) => `completeness: ${f}`));
  }

  return failures;
}

check()
  .then((failures) => {
    if (failures.length) {
      console.error("Besu plugin options drift/completeness check failed:");
      for (const f of failures) console.error(`- ${f}`);
      process.exit(1);
    }
    console.log("Besu plugin options check passed (committed output matches source; wrapper completeness OK).");
  })
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
