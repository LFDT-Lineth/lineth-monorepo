const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const { build, checkCompleteness } = require("./lib");
const { listGeneratedPartials, writeOutput } = require("./output");
const { TOOL_ROOT, MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH, parseMonorepoArg } = require("./paths");

async function check() {
  const monorepoPath = parseMonorepoArg();
  const result = await build({ monorepoPath, monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  const temporaryOutput = fs.mkdtempSync(path.join(os.tmpdir(), "prover-options-check-"));

  try {
    const generatedDir = writeOutput(result, temporaryOutput);
    const failures = [];
    const targets = [
      {
        label: "manifest",
        file: path.join(temporaryOutput, "linea-prover-options.json"),
        expected: result.manifestJson,
      },
      { label: "report", file: path.join(temporaryOutput, "report.json"), expected: result.reportJson },
    ];
    for (const target of targets) {
      if (!fs.existsSync(target.file)) {
        failures.push(`${target.label}: missing freshly generated output`);
        continue;
      }
      if (fs.readFileSync(target.file, "utf8") !== target.expected) {
        failures.push(`${target.label}: freshly generated output does not match the extracted source`);
      }
    }

    const expectedPartials = new Map(result.partials.map((p) => [p.relPath.replace(/\\/g, "/"), p.markdown]));
    const generatedPartials = listGeneratedPartials(generatedDir);

    for (const relativePath of expectedPartials.keys()) {
      if (!generatedPartials.includes(relativePath)) {
        failures.push(`partial: missing freshly generated _generated/prover/${relativePath}`);
      }
    }
    for (const relativePath of generatedPartials) {
      if (!expectedPartials.has(relativePath)) {
        failures.push(`partial: unexpected freshly generated _generated/prover/${relativePath}`);
      }
    }
    for (const [relativePath, expected] of expectedPartials) {
      const file = path.join(generatedDir, relativePath);
      if (fs.existsSync(file) && fs.readFileSync(file, "utf8") !== expected) {
        failures.push(`partial: freshly generated _generated/prover/${relativePath} is invalid`);
      }
    }

    if (!fs.existsSync(WRAPPER_TEMPLATE_PATH)) {
      failures.push(
        `wrapper: missing template at ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)} ` +
          `(run pnpm run generate:seed-wrapper once).`,
      );
    } else {
      const wrapper = fs.readFileSync(WRAPPER_TEMPLATE_PATH, "utf8");
      failures.push(...checkCompleteness(wrapper, generatedPartials).map((failure) => `completeness: ${failure}`));
    }

    return failures;
  } finally {
    fs.rmSync(temporaryOutput, { recursive: true, force: true });
  }
}

check()
  .then((failures) => {
    if (failures.length) {
      console.error("Prover options generation/completeness check failed:");
      for (const f of failures) console.error(`- ${f}`);
      process.exit(1);
    }
    console.log("Prover options check passed (fresh generation valid; wrapper completeness OK).");
  })
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
