const fs = require("node:fs");
const path = require("node:path");

const { build } = require("./lib");
const { TOOL_ROOT, MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH } = require("./paths");

/**
 * One-time seed of the human-owned wrapper template.
 * Uses exclusive create (O_EXCL / wx) so we never overwrite an existing file.
 */
async function main() {
  const result = await build({ toolRoot: TOOL_ROOT });
  fs.mkdirSync(path.dirname(WRAPPER_TEMPLATE_PATH), { recursive: true });

  let fd;
  try {
    fd = fs.openSync(WRAPPER_TEMPLATE_PATH, "wx");
  } catch (err) {
    if (err && err.code === "EEXIST") {
      console.warn(
        `Refusing to overwrite existing wrapper at ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)}. ` +
          "Delete it first if you intentionally want to re-seed.",
      );
      process.exit(0);
    }
    throw err;
  }

  try {
    fs.writeFileSync(fd, result.wrapperMarkdown);
  } finally {
    fs.closeSync(fd);
  }

  console.log(`Seeded ${path.relative(MONOREPO_ROOT, WRAPPER_TEMPLATE_PATH)}`);
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
