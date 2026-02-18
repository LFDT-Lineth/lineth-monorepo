#!/usr/bin/env npx ts-node
/**
 * Batch enrich all Hardhat artifacts in a directory with immutableReferences.
 *
 * Walks through a directory of artifacts, finds their corresponding build-info,
 * and enriches each one with immutableReferences for definitive verification.
 *
 * Usage:
 *   npx ts-node tools/enrich-all-artifacts.ts <artifacts-dir> [output-dir]
 *
 * Examples:
 *   # Enrich in place
 *   npx ts-node tools/enrich-all-artifacts.ts artifacts/contracts
 *
 *   # Enrich to different directory
 *   npx ts-node tools/enrich-all-artifacts.ts artifacts/contracts enriched-artifacts
 *
 *   # Enrich specific contracts directory for deployment
 *   npx ts-node tools/enrich-all-artifacts.ts artifacts/contracts deployments/bytecode/2026-01-14
 */

import { readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync } from "fs";
import { resolve, dirname, basename, join, relative } from "path";

interface ImmutableRef {
  start: number;
  length: number;
}

interface HardhatArtifact {
  _format?: string;
  contractName: string;
  sourceName?: string;
  abi: unknown[];
  bytecode: string;
  deployedBytecode: string;
  linkReferences?: Record<string, unknown>;
  deployedLinkReferences?: Record<string, unknown>;
  immutableReferences?: Record<string, ImmutableRef[]>;
}

interface BuildInfoContract {
  abi: unknown[];
  evm: {
    bytecode: {
      object: string;
      linkReferences?: Record<string, unknown>;
    };
    deployedBytecode: {
      object: string;
      linkReferences?: Record<string, unknown>;
      immutableReferences?: Record<string, ImmutableRef[]>;
    };
  };
}

interface BuildInfo {
  _format?: string;
  solcVersion?: string;
  output: {
    contracts: Record<string, Record<string, BuildInfoContract>>;
  };
}

interface EnrichmentResult {
  file: string;
  contractName: string;
  status: "enriched" | "skipped" | "no-immutables" | "error";
  immutableCount?: number;
  refCount?: number;
  error?: string;
}

/**
 * Load all build-info files from artifacts/build-info directory.
 */
function loadBuildInfoFiles(artifactsDir: string): Map<string, BuildInfo> {
  const buildInfoDir = join(artifactsDir, "../build-info");
  const buildInfoMap = new Map<string, BuildInfo>();

  if (!existsSync(buildInfoDir)) {
    // Try alternative locations
    const altPaths = [
      join(artifactsDir, "build-info"),
      join(dirname(artifactsDir), "build-info"),
      join(process.cwd(), "artifacts/build-info"),
    ];

    for (const altPath of altPaths) {
      if (existsSync(altPath)) {
        return loadBuildInfoFromDir(altPath);
      }
    }

    console.warn(`Warning: Could not find build-info directory`);
    return buildInfoMap;
  }

  return loadBuildInfoFromDir(buildInfoDir);
}

function loadBuildInfoFromDir(dir: string): Map<string, BuildInfo> {
  const buildInfoMap = new Map<string, BuildInfo>();

  const files = readdirSync(dir).filter((f) => f.endsWith(".json"));

  for (const file of files) {
    try {
      const content = readFileSync(join(dir, file), "utf-8");
      const buildInfo: BuildInfo = JSON.parse(content);
      buildInfoMap.set(file, buildInfo);
    } catch {
      // Skip invalid files
    }
  }

  console.log(`Loaded ${buildInfoMap.size} build-info file(s) from ${dir}`);
  return buildInfoMap;
}

/**
 * Find immutableReferences for a contract in any build-info file.
 */
function findImmutableReferences(
  contractName: string,
  sourceName: string | undefined,
  buildInfoFiles: Map<string, BuildInfo>,
): Record<string, ImmutableRef[]> | undefined {
  for (const buildInfo of buildInfoFiles.values()) {
    for (const [source, contracts] of Object.entries(buildInfo.output.contracts)) {
      if (contracts[contractName]) {
        // Optionally match source name
        if (sourceName && !source.includes(sourceName.replace("contracts/", ""))) {
          continue;
        }

        const immutableRefs = contracts[contractName].evm?.deployedBytecode?.immutableReferences;

        if (immutableRefs && Object.keys(immutableRefs).length > 0) {
          return immutableRefs;
        }
      }
    }
  }

  return undefined;
}

/**
 * Recursively find all JSON files that look like Hardhat artifacts.
 * Uses a single read operation to avoid TOCTOU race conditions.
 */
function findArtifactFiles(dir: string, files: string[] = []): string[] {
  let entries: string[];
  try {
    entries = readdirSync(dir);
  } catch {
    // Directory may have been removed - skip it
    return files;
  }

  for (const entry of entries) {
    // Skip build-info and node_modules early
    if (entry === "build-info" || entry === "node_modules") continue;

    const fullPath = join(dir, entry);

    // Try to read as JSON file first - this handles the file case
    // and avoids race conditions between stat and read
    if (entry.endsWith(".json") && !entry.includes(".dbg.")) {
      try {
        const content = JSON.parse(readFileSync(fullPath, "utf-8"));
        if (content.contractName && content.abi && content.deployedBytecode) {
          files.push(fullPath);
        }
      } catch {
        // Not a valid JSON file or can't read - skip
      }
    } else {
      // Try as directory (recurse) - handles both files and directories gracefully
      try {
        findArtifactFiles(fullPath, files);
      } catch {
        // Not a directory or can't access - skip
      }
    }
  }

  return files;
}

/**
 * Enrich a single artifact file.
 */
function enrichArtifact(
  artifactPath: string,
  outputPath: string,
  buildInfoFiles: Map<string, BuildInfo>,
): EnrichmentResult {
  const fileName = basename(artifactPath);

  try {
    const artifact: HardhatArtifact = JSON.parse(readFileSync(artifactPath, "utf-8"));

    // Already enriched?
    if (artifact.immutableReferences && Object.keys(artifact.immutableReferences).length > 0) {
      return {
        file: fileName,
        contractName: artifact.contractName,
        status: "skipped",
        immutableCount: Object.keys(artifact.immutableReferences).length,
      };
    }

    // Find immutableReferences
    const immutableRefs = findImmutableReferences(artifact.contractName, artifact.sourceName, buildInfoFiles);

    if (!immutableRefs || Object.keys(immutableRefs).length === 0) {
      // Copy as-is if no immutables
      if (outputPath !== artifactPath) {
        const outputDir = dirname(outputPath);
        if (!existsSync(outputDir)) {
          mkdirSync(outputDir, { recursive: true });
        }
        writeFileSync(outputPath, JSON.stringify(artifact, null, 2));
      }

      return {
        file: fileName,
        contractName: artifact.contractName,
        status: "no-immutables",
      };
    }

    // Enrich and write
    const enriched = { ...artifact, immutableReferences: immutableRefs };
    const refCount = Object.values(immutableRefs).reduce((sum, refs) => sum + refs.length, 0);

    const outputDir = dirname(outputPath);
    if (!existsSync(outputDir)) {
      mkdirSync(outputDir, { recursive: true });
    }
    writeFileSync(outputPath, JSON.stringify(enriched, null, 2));

    return {
      file: fileName,
      contractName: artifact.contractName,
      status: "enriched",
      immutableCount: Object.keys(immutableRefs).length,
      refCount,
    };
  } catch (err) {
    return {
      file: fileName,
      contractName: "unknown",
      status: "error",
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

function main(): void {
  const args = process.argv.slice(2).filter((a) => !a.startsWith("--"));

  if (args.length < 1) {
    console.log(`
Batch enrich Hardhat artifacts with immutableReferences.

Usage:
  npx ts-node tools/enrich-all-artifacts.ts <artifacts-dir> [output-dir]

Arguments:
  artifacts-dir    Directory containing Hardhat artifacts
  output-dir       Output directory (optional, defaults to in-place)

Examples:
  npx ts-node tools/enrich-all-artifacts.ts artifacts/contracts
  npx ts-node tools/enrich-all-artifacts.ts artifacts/contracts enriched/
`);
    process.exit(1);
  }

  const artifactsDir = resolve(args[0]);
  const outputDir = args[1] ? resolve(args[1]) : artifactsDir;
  const inPlace = outputDir === artifactsDir;

  console.log("Batch Enriching Hardhat Artifacts");
  console.log("=".repeat(50));
  console.log(`Input directory:  ${artifactsDir}`);
  console.log(`Output directory: ${outputDir}`);
  console.log(`Mode: ${inPlace ? "in-place" : "copy"}`);
  console.log();

  // Load build-info files
  const buildInfoFiles = loadBuildInfoFiles(artifactsDir);

  if (buildInfoFiles.size === 0) {
    console.error("Error: No build-info files found. Run `npx hardhat compile` first.");
    process.exit(1);
  }

  // Find all artifact files
  console.log(`\nScanning for artifacts in ${artifactsDir}...`);
  const artifactFiles = findArtifactFiles(artifactsDir);
  console.log(`Found ${artifactFiles.length} artifact file(s)`);

  if (artifactFiles.length === 0) {
    console.log("No artifacts to process.");
    process.exit(0);
  }

  // Process each artifact
  console.log("\nProcessing artifacts...\n");

  const results: EnrichmentResult[] = [];

  for (const artifactPath of artifactFiles) {
    const relativePath = relative(artifactsDir, artifactPath);
    const outputPath = join(outputDir, relativePath);

    const result = enrichArtifact(artifactPath, outputPath, buildInfoFiles);
    results.push(result);

    const icon =
      result.status === "enriched" ? "✓" : result.status === "skipped" ? "○" : result.status === "error" ? "✗" : "·";

    let detail = "";
    if (result.status === "enriched") {
      detail = ` (${result.immutableCount} immutable(s), ${result.refCount} ref(s))`;
    } else if (result.status === "skipped") {
      detail = ` (already enriched)`;
    } else if (result.status === "error") {
      detail = ` - ${result.error}`;
    }

    console.log(`  ${icon} ${result.contractName}${detail}`);
  }

  // Summary
  console.log("\n" + "=".repeat(50));
  console.log("Summary:");
  console.log(`  Enriched:      ${results.filter((r) => r.status === "enriched").length}`);
  console.log(`  Skipped:       ${results.filter((r) => r.status === "skipped").length}`);
  console.log(`  No immutables: ${results.filter((r) => r.status === "no-immutables").length}`);
  console.log(`  Errors:        ${results.filter((r) => r.status === "error").length}`);
  console.log("=".repeat(50));

  const errors = results.filter((r) => r.status === "error");
  if (errors.length > 0) {
    console.log("\nErrors:");
    for (const e of errors) {
      console.log(`  - ${e.file}: ${e.error}`);
    }
    process.exit(1);
  }

  console.log("\nDone! Artifacts are ready for definitive bytecode verification.");
}

main();
