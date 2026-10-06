const fs = require("node:fs");
const path = require("node:path");

function emptyDir(dir) {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    fs.rmSync(path.join(dir, entry.name), { recursive: true, force: true });
  }
}

function writeOutput(result, outputDir) {
  const generatedDir = path.join(outputDir, "_generated", "prover");
  fs.mkdirSync(outputDir, { recursive: true });
  emptyDir(generatedDir);
  fs.mkdirSync(generatedDir, { recursive: true });

  fs.writeFileSync(path.join(outputDir, "linea-prover-options.json"), result.manifestJson);
  fs.writeFileSync(path.join(outputDir, "report.json"), result.reportJson);

  for (const part of result.partials) {
    const absolutePath = path.join(generatedDir, part.relPath);
    fs.mkdirSync(path.dirname(absolutePath), { recursive: true });
    fs.writeFileSync(absolutePath, part.markdown);
  }

  return generatedDir;
}

function listGeneratedPartials(generatedDir) {
  const partials = [];
  if (!fs.existsSync(generatedDir)) return partials;

  function walk(dir, prefix) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const relativePath = prefix ? `${prefix}/${entry.name}` : entry.name;
      const absolutePath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        walk(absolutePath, relativePath);
      } else if (entry.name.endsWith(".mdx")) {
        partials.push(relativePath.replace(/\\/g, "/"));
      }
    }
  }

  walk(generatedDir, "");
  return partials.sort();
}

module.exports = {
  listGeneratedPartials,
  writeOutput,
};
