import { readdirSync, readFileSync, writeFileSync, mkdirSync, statSync, rmSync, existsSync } from "fs";
import { join, basename } from "path";
import { format } from "prettier";

const ABI_INPUT_DIRS = [
  "../contracts/local-deployments-artifacts/static-artifacts",
  "../contracts/local-deployments-artifacts/dynamic-artifacts",
  "../contracts/local-deployments-artifacts/deployed-artifacts",
];

const INCLUDE_FILES: string[] = [
  "LineaRollupV6",
  "L2MessageServiceV1",
  "TokenBridgeV1_1",
  "ProxyAdmin",
  "TransparentUpgradeableProxy",
  "TestERC20",
  "DummyContract",
  "TestContract",
  "BridgedToken",
  "SparseMerkleProof",
  "LineaSequencerUptimeFeed",
  "OpcodeTester",
  "Mimc",
];

async function main() {
  const OUT_DIR = join(process.cwd(), "src/generated");

  if (existsSync(OUT_DIR)) {
    rmSync(OUT_DIR, { recursive: true, force: true });
  }

  mkdirSync(OUT_DIR, { recursive: true });

  const indexExports: string[] = [];

  for (const inputDir of ABI_INPUT_DIRS) {
    const absPath = join(process.cwd(), inputDir);

    for (const { contractName, abi, bytecode, file, linkReferences } of walk(absPath)) {
      const name = basename(file).replace(".json", "Abi");
      const ts = await jsonToTsConst(name, abi, bytecode, linkReferences);
      const outFile = join(OUT_DIR, `${contractName}Abi.ts`);

      writeFileSync(outFile, ts);

      indexExports.push(`export * from "./${contractName}Abi";`);
    }
  }

  const formattedIndexExports = await format(indexExports.join("\n"), { parser: "typescript" });
  writeFileSync(join(OUT_DIR, "index.ts"), formattedIndexExports);
}

function walk(dir: string): {
  file: string;
  bytecode: string;
  abi: unknown;
  contractName: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  raw: any;
  linkReferences: Record<string, Record<string, { start: number; length: number }[]>> | undefined;
}[] {
  const results: {
    file: string;
    bytecode: string;
    abi: unknown;
    contractName: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    raw: any;
    linkReferences: Record<string, Record<string, { start: number; length: number }[]>> | undefined;
  }[] = [];

  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const stat = statSync(full);

    if (stat.isDirectory()) {
      results.push(...walk(full));
      continue;
    }

    if (!entry.endsWith(".json")) continue;

    if (INCLUDE_FILES.length > 0 && !INCLUDE_FILES.includes(basename(full).replace(".json", ""))) continue;

    const raw = JSON.parse(readFile(full));

    const contractName = raw.contractName ?? null;

    if (!contractName) {
      throw new Error(`No contractName found in JSON file: ${full}`);
    }

    const abi = raw.abi;
    if (!abi) {
      throw new Error(`No ABI found in JSON file: ${full}`);
    }

    const bytecode = raw.bytecode?.object ?? raw.bytecode ?? undefined;

    const linkReferences: Record<string, Record<string, { start: number; length: number }[]>> | undefined =
      raw?.linkReferences && Object.keys(raw.linkReferences).length !== 0 ? raw.linkReferences : undefined;

    results.push({ file: full, bytecode, abi, contractName, raw, linkReferences });
  }

  return results;
}

export function readFile(path: string) {
  return readFileSync(path, "utf8");
}

async function jsonToTsConst(
  name: string,
  abi: unknown,
  bytecode?: string,
  linkReferences?: Record<string, Record<string, { start: number; length: number }[]>>,
) {
  const ts = `// AUTO-GENERATED – DO NOT EDIT
import type { Abi } from "viem";

export const ${name} = ${JSON.stringify(abi, null, 2)} as const satisfies Abi;

${bytecode ? `export const ${name}Bytecode = "${bytecode}" as const;` : ""}

${linkReferences ? `export const ${name}LinkReferences = ${JSON.stringify(linkReferences, null, 2)};` : ""}
  `;
  return await format(ts, { parser: "typescript" });
}

main()
  .then(() => {
    process.exit(0);
  })
  .catch((error) => {
    console.error("Error during ABI generation:", error);
    process.exit(1);
  });
