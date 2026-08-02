const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const { buildFromManifest, checkCompleteness } = require("./lib");
const { TOOL_ROOT, MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH, parseMonorepoArg } = require("./paths");

function listGeneratedPartials(generatedDir) {
  const out = [];
  if (!fs.existsSync(generatedDir)) return out;
  function walk(dir, prefix) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full, rel);
      else if (entry.name.endsWith(".mdx")) out.push(rel.replace(/\\/g, "/"));
    }
  }
  walk(generatedDir, "");
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
  const freshGeneratedDir = path.join(tmp, "output", "_generated", "besu");

  try {
    runJavaExtractorTo(monorepoPath, freshManifest, freshReport);

    const manifest = JSON.parse(fs.readFileSync(freshManifest, "utf8"));
    const report = JSON.parse(fs.readFileSync(freshReport, "utf8"));
    const result = await buildFromManifest({ manifest, report, toolRoot: TOOL_ROOT });

    fs.mkdirSync(freshGeneratedDir, { recursive: true });
    for (const partial of result.partials) {
      const outputPath = path.join(freshGeneratedDir, partial.relPath);
      fs.mkdirSync(path.dirname(outputPath), { recursive: true });
      fs.writeFileSync(outputPath, partial.markdown);
    }

    const failures = [];
    const expectedPartials = new Map(result.partials.map((p) => [p.relPath.replace(/\\/g, "/"), p.markdown]));
    const onDisk = listGeneratedPartials(freshGeneratedDir);

    for (const rel of expectedPartials.keys()) {
      if (!onDisk.includes(rel)) failures.push(`partial: missing _generated/besu/${rel}`);
    }
    for (const rel of onDisk) {
      if (!expectedPartials.has(rel)) {
        failures.push(`partial: unexpected _generated/besu/${rel} (not produced by current sources)`);
      }
    }
    for (const [rel, expected] of expectedPartials) {
      const file = path.join(freshGeneratedDir, rel);
      if (!fs.existsSync(file)) continue;
      if (fs.readFileSync(file, "utf8") !== expected) {
        failures.push(`partial: temporary _generated/besu/${rel} does not match the fresh render`);
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
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

check()
  .then((failures) => {
    if (failures.length) {
      console.error("Besu plugin options generation/completeness check failed:");
      for (const f of failures) console.error(`- ${f}`);
      process.exit(1);
    }
    console.log("Besu plugin options check passed (fresh temporary output and wrapper completeness are valid).");
  })
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
