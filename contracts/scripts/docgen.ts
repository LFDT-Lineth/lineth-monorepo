/**
 * Generates the Solidity API pages in `docs/api/` (published on docs.linea.build) from the Hardhat 3 build-info files.
 * Replaces the Hardhat 2 `hardhat docgen` task: `solidity-docgen` has no Hardhat 3 plugin, but its `docgen()` export
 * only needs the solc inputs and outputs. Run `hardhat build` first (the `solidity:docgen` script does).
 */
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { docgen } from "solidity-docgen";

type Build = Parameters<typeof docgen>[0][number];

const BUILD_INFO_DIR = path.join(import.meta.dirname, "..", "build", "build-info");

async function readBuilds(): Promise<Build[]> {
  const files = await readdir(BUILD_INFO_DIR);
  const inputFiles = files.filter((file) => file.endsWith(".json") && !file.endsWith(".output.json"));
  if (inputFiles.length === 0) {
    throw new Error(`No build-info files in ${BUILD_INFO_DIR}; run \`hardhat build\` first.`);
  }

  return Promise.all(
    inputFiles.map(async (file) => {
      const buildInfo: { input: Build["input"] } = JSON.parse(await readFile(path.join(BUILD_INFO_DIR, file), "utf8"));
      const outputFile = file.replace(/\.json$/, ".output.json");
      const buildOutput: { output: Build["output"] } = JSON.parse(
        await readFile(path.join(BUILD_INFO_DIR, outputFile), "utf8"),
      );
      return { input: buildInfo.input, output: buildOutput.output };
    }),
  );
}

async function main(): Promise<void> {
  // Sources shared by several builds are documented once (solidity-docgen dedupes by source path).
  await docgen(await readBuilds(), {
    root: path.join(import.meta.dirname, ".."),
    // Hardhat 3 source names: project files are `project/<path>`, so pages stay relative to `src/` as before.
    sourcesDir: "project/src",
    exclude: [
      "_testing",
      "bridging/token/utils/StorageFiller39.sol",
      "bridging/token/CustomBridgedToken.sol",
      "governance/TimeLock.sol",
      "security/access/PermissionsManager.sol",
      "security/reentrancy/TransientStorageReentrancyGuardUpgradeable.sol",
      "tokens",
      "verifiers",
      // Never published; solidity-docgen also rejects `@return $` (ERC-7201 storage getters), as `hardhat docgen` did.
      "yield",
    ],
    pages: "files",
    outputDir: "docs/api/",
    // For compatibility with docs.linea.build
    pageExtension: ".mdx",
    templates: "docs/docgen-templates",
  });
}

main().catch((error: unknown) => {
  console.error(error);
  process.exitCode = 1;
});
