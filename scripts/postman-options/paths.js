const path = require("node:path");

/** This tool directory (scripts/postman-options). */
const TOOL_ROOT = __dirname;

/** Monorepo root (two levels up from this script directory). */
const MONOREPO_ROOT = path.resolve(__dirname, "..", "..");

const OUTPUT_DIR = path.join(TOOL_ROOT, "output");
const GENERATED_DIR = path.join(OUTPUT_DIR, "_generated", "postman");
const TEMPLATES_DIR = path.join(TOOL_ROOT, "templates");

const MANIFEST_PATH = path.join(OUTPUT_DIR, "linea-postman-options.json");
const REPORT_PATH = path.join(OUTPUT_DIR, "report.json");
const WRAPPER_TEMPLATE_PATH = path.join(TEMPLATES_DIR, "linea-postman-options.mdx");

/** Parse `--monorepo <path>` / `--monorepo=<path>` from argv. */
function parseMonorepoArg(argv = process.argv.slice(2)) {
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--monorepo") return argv[i + 1];
    const m = a.match(/^--monorepo=(.+)$/);
    if (m) return m[1];
  }
  return undefined;
}

function hasFlag(flag, argv = process.argv.slice(2)) {
  return argv.includes(flag);
}

module.exports = {
  TOOL_ROOT,
  MONOREPO_ROOT,
  OUTPUT_DIR,
  GENERATED_DIR,
  TEMPLATES_DIR,
  MANIFEST_PATH,
  REPORT_PATH,
  WRAPPER_TEMPLATE_PATH,
  parseMonorepoArg,
  hasFlag,
};
