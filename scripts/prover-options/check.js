const fs = require("node:fs");
const path = require("node:path");

const { build, checkCompleteness } = require("./lib");
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

async function check() {
  const monorepoPath = parseMonorepoArg() || MONOREPO_ROOT;
  const result = await build({ monorepoRoot: monorepoPath, toolRoot: TOOL_ROOT });

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
    if (!onDisk.includes(rel)) failures.push(`partial: missing _generated/prover/${rel}`);
  }
  for (const rel of onDisk) {
    if (!expectedPartials.has(rel)) {
      failures.push(`partial: unexpected _generated/prover/${rel} (not produced by current sources)`);
    }
  }
  for (const [rel, expected] of expectedPartials) {
    const file = path.join(GENERATED_DIR, rel);
    if (!fs.existsSync(file)) continue;
    if (fs.readFileSync(file, "utf8") !== expected) {
      failures.push(`partial: _generated/prover/${rel} is out of date (run pnpm run generate).`);
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
      console.error("Prover options drift/completeness check failed:");
      for (const f of failures) console.error(`- ${f}`);
      process.exit(1);
    }
    console.log("Prover options check passed (committed output matches source; wrapper completeness OK).");
  })
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
