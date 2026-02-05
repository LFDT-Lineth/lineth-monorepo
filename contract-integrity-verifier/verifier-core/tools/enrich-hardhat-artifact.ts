#!/usr/bin/env npx ts-node
/**
 * Enrich Hardhat artifact with immutableReferences from build-info.
 *
 * Hardhat standard artifacts don't include immutableReferences, but the Solidity
 * compiler outputs them. This script extracts immutableReferences from the
 * build-info files and adds them to the artifact in a Foundry-compatible format.
 *
 * This enables definitive bytecode verification (100% confidence, no ambiguity).
 *
 * Usage:
 *   # Auto-find build-info in artifacts/build-info/
 *   npx ts-node tools/enrich-hardhat-artifact.ts <artifact.json> [output.json]
 *
 *   # Specify build-info file explicitly
 *   npx ts-node tools/enrich-hardhat-artifact.ts <artifact.json> [output.json] --build-info <build-info.json>
 *
 *   # Specify contract path for multi-contract build-info
 *   npx ts-node tools/enrich-hardhat-artifact.ts <artifact.json> [output.json] --contract contracts/MyContract.sol:MyContract
 *
 * Examples:
 *   npx ts-node tools/enrich-hardhat-artifact.ts artifacts/contracts/LineaRollup.sol/LineaRollup.json
 *   npx ts-node tools/enrich-hardhat-artifact.ts LineaRollup.json enriched/LineaRollup.json --build-info build-info/abc.json
 */

import { readFileSync, writeFileSync, readdirSync, existsSync } from "fs";
import { resolve, dirname, join } from "path";

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
}

interface EnrichedArtifact extends HardhatArtifact {
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
  solcLongVersion?: string;
  input?: {
    sources: Record<string, unknown>;
  };
  output: {
    contracts: Record<string, Record<string, BuildInfoContract>>;
    sources?: Record<string, unknown>;
  };
}

/**
 * Parse command line arguments.
 */
function parseArgs(): {
  artifactPath: string;
  outputPath: string | undefined;
  buildInfoPath: string | undefined;
  contractPath: string | undefined;
  verbose: boolean;
} {
  const args = process.argv.slice(2);

  let artifactPath: string | undefined;
  let outputPath: string | undefined;
  let buildInfoPath: string | undefined;
  let contractPath: string | undefined;
  let verbose = false;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];

    if (arg === "--build-info" || arg === "-b") {
      buildInfoPath = args[++i];
    } else if (arg === "--contract" || arg === "-c") {
      contractPath = args[++i];
    } else if (arg === "--verbose" || arg === "-v") {
      verbose = true;
    } else if (arg === "--help" || arg === "-h") {
      printUsage();
      process.exit(0);
    } else if (!artifactPath) {
      artifactPath = arg;
    } else if (!outputPath) {
      outputPath = arg;
    }
  }

  if (!artifactPath) {
    printUsage();
    process.exit(1);
  }

  return {
    artifactPath: resolve(artifactPath),
    outputPath: outputPath ? resolve(outputPath) : undefined,
    buildInfoPath: buildInfoPath ? resolve(buildInfoPath) : undefined,
    contractPath,
    verbose,
  };
}

function printUsage(): void {
  console.log(`
Enrich Hardhat artifact with immutableReferences from build-info.

Usage:
  npx ts-node tools/enrich-hardhat-artifact.ts <artifact.json> [output.json] [options]

Arguments:
  artifact.json    Path to Hardhat artifact file
  output.json      Output path (optional, defaults to overwriting input)

Options:
  --build-info, -b <path>   Path to build-info JSON file
  --contract, -c <path>     Contract path (e.g., contracts/MyContract.sol:MyContract)
  --verbose, -v             Enable verbose output
  --help, -h                Show this help message

Examples:
  # Auto-find build-info (searches artifacts/build-info/)
  npx ts-node tools/enrich-hardhat-artifact.ts artifacts/contracts/LineaRollup.sol/LineaRollup.json

  # Specify build-info explicitly
  npx ts-node tools/enrich-hardhat-artifact.ts LineaRollup.json --build-info build-info/abc.json

  # Output to different file
  npx ts-node tools/enrich-hardhat-artifact.ts LineaRollup.json enriched/LineaRollup.json
`);
}

/**
 * Find build-info file that contains the specified contract.
 */
function findBuildInfo(
  artifactPath: string,
  contractName: string,
  sourceName: string | undefined,
  verbose: boolean,
): string | undefined {
  // Try to find artifacts/build-info/ relative to artifact path
  const artifactDir = dirname(artifactPath);

  // Search common locations for build-info
  const searchPaths = [
    join(artifactDir, "../../../build-info"), // artifacts/contracts/X.sol/X.json -> artifacts/build-info
    join(artifactDir, "../../build-info"), // artifacts/X.sol/X.json -> artifacts/build-info
    join(artifactDir, "../build-info"), // artifacts/X.json -> artifacts/build-info
    join(artifactDir, "build-info"),
    join(process.cwd(), "artifacts/build-info"),
  ];

  for (const searchPath of searchPaths) {
    if (!existsSync(searchPath)) continue;

    if (verbose) {
      console.log(`  Searching for build-info in: ${searchPath}`);
    }

    const files = readdirSync(searchPath).filter((f) => f.endsWith(".json"));

    for (const file of files) {
      const buildInfoPath = join(searchPath, file);

      try {
        const buildInfo: BuildInfo = JSON.parse(readFileSync(buildInfoPath, "utf-8"));

        // Search for the contract in build-info
        for (const [source, contracts] of Object.entries(buildInfo.output.contracts)) {
          // Match by contract name, optionally checking source path
          if (contracts[contractName]) {
            // If sourceName is provided, verify it matches
            if (sourceName && !source.endsWith(sourceName) && !sourceName.endsWith(source)) {
              continue;
            }

            if (verbose) {
              console.log(`  Found contract ${contractName} in ${file} at ${source}`);
            }

            return buildInfoPath;
          }
        }
      } catch {
        // Skip invalid JSON files
      }
    }
  }

  return undefined;
}

/**
 * Extract immutableReferences from build-info for a specific contract.
 */
function extractImmutableReferences(
  buildInfo: BuildInfo,
  contractName: string,
  sourceName: string | undefined,
  verbose: boolean,
): Record<string, ImmutableRef[]> | undefined {
  for (const [source, contracts] of Object.entries(buildInfo.output.contracts)) {
    // Match by contract name
    if (contracts[contractName]) {
      // If sourceName is provided, verify it matches
      if (sourceName && !source.endsWith(sourceName) && !sourceName.endsWith(source)) {
        continue;
      }

      const contract = contracts[contractName];
      const immutableRefs = contract.evm?.deployedBytecode?.immutableReferences;

      if (immutableRefs && Object.keys(immutableRefs).length > 0) {
        if (verbose) {
          console.log(`  Found ${Object.keys(immutableRefs).length} immutable reference(s)`);
          for (const [astId, refs] of Object.entries(immutableRefs)) {
            console.log(`    AST ID ${astId}: ${refs.length} occurrence(s) at positions ${refs.map((r) => r.start).join(", ")}`);
          }
        }
        return immutableRefs;
      } else {
        if (verbose) {
          console.log(`  No immutableReferences found for ${contractName}`);
        }
        return undefined;
      }
    }
  }

  return undefined;
}

/**
 * Enrich a Hardhat artifact with immutableReferences.
 */
function enrichArtifact(artifact: HardhatArtifact, immutableReferences: Record<string, ImmutableRef[]>): EnrichedArtifact {
  return {
    ...artifact,
    immutableReferences,
  };
}

/**
 * Normalize immutableReferences to a flat array format for verifier consumption.
 */
function normalizeImmutableReferences(immutableRefs: Record<string, ImmutableRef[]>): ImmutableRef[] {
  const refs: ImmutableRef[] = [];

  for (const positions of Object.values(immutableRefs)) {
    refs.push(...positions);
  }

  // Sort by position for consistent ordering
  refs.sort((a, b) => a.start - b.start);

  return refs;
}

function main(): void {
  const { artifactPath, outputPath, buildInfoPath, contractPath, verbose } = parseArgs();

  console.log("Enriching Hardhat artifact with immutableReferences");
  console.log("=".repeat(50));
  console.log(`Input artifact: ${artifactPath}`);

  // Load artifact - read directly and handle errors to avoid TOCTOU race condition
  let artifact: HardhatArtifact;
  try {
    artifact = JSON.parse(readFileSync(artifactPath, "utf-8"));
  } catch (error) {
    console.error(`Error: Failed to read artifact file: ${artifactPath}`);
    console.error(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
  const contractName = contractPath?.split(":")[1] ?? artifact.contractName;
  const sourceName = contractPath?.split(":")[0] ?? artifact.sourceName;

  console.log(`Contract: ${contractName}`);
  if (sourceName) {
    console.log(`Source: ${sourceName}`);
  }

  // Check if already enriched
  if ((artifact as EnrichedArtifact).immutableReferences) {
    console.log("\nArtifact already has immutableReferences!");
    const existing = (artifact as EnrichedArtifact).immutableReferences!;
    const refCount = Object.values(existing).reduce((sum, refs) => sum + refs.length, 0);
    console.log(`  ${Object.keys(existing).length} immutable variable(s), ${refCount} total reference(s)`);
    process.exit(0);
  }

  // Find or load build-info
  let buildInfo: BuildInfo;

  if (buildInfoPath) {
    console.log(`\nUsing build-info: ${buildInfoPath}`);
    // Read directly and handle errors to avoid TOCTOU race condition
    try {
      buildInfo = JSON.parse(readFileSync(buildInfoPath, "utf-8"));
    } catch (error) {
      console.error(`Error: Failed to read build-info file: ${buildInfoPath}`);
      console.error(error instanceof Error ? error.message : String(error));
      process.exit(1);
    }
  } else {
    console.log("\nSearching for build-info...");
    const foundPath = findBuildInfo(artifactPath, contractName, sourceName, verbose);

    if (!foundPath) {
      console.error("\nError: Could not find build-info file.");
      console.error("Try specifying it explicitly with --build-info <path>");
      process.exit(1);
    }

    console.log(`Found build-info: ${foundPath}`);
    buildInfo = JSON.parse(readFileSync(foundPath, "utf-8"));
  }

  // Extract immutableReferences
  console.log("\nExtracting immutableReferences...");
  const immutableRefs = extractImmutableReferences(buildInfo, contractName, sourceName, verbose);

  if (!immutableRefs || Object.keys(immutableRefs).length === 0) {
    console.log("\nNo immutableReferences found in build-info.");
    console.log("This contract may not have any immutable variables.");
    process.exit(0);
  }

  // Enrich artifact
  const enriched = enrichArtifact(artifact, immutableRefs);

  // Calculate stats
  const flatRefs = normalizeImmutableReferences(immutableRefs);
  console.log(`\nAdded ${Object.keys(immutableRefs).length} immutable variable(s)`);
  console.log(`  ${flatRefs.length} total bytecode reference(s)`);
  console.log(`  Positions: ${flatRefs.map((r) => r.start).join(", ")}`);

  // Write output
  const finalOutputPath = outputPath ?? artifactPath;
  console.log(`\nWriting enriched artifact: ${finalOutputPath}`);
  writeFileSync(finalOutputPath, JSON.stringify(enriched, null, 2));

  console.log("\nDone! Artifact is now ready for definitive bytecode verification.");
}

main();
