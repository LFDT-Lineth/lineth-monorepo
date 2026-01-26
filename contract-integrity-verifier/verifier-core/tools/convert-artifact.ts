#!/usr/bin/env npx ts-node
/**
 * Convert between Hardhat and Foundry artifact formats.
 *
 * Auto-detects input format and converts to the other.
 *
 * Usage:
 *   npx ts-node tools/convert-artifact.ts <input.json> <output.json>
 *   npx ts-node tools/convert-artifact.ts <input.json> <output.json> --to-hardhat
 *   npx ts-node tools/convert-artifact.ts <input.json> <output.json> --to-foundry
 */

import { readFileSync, writeFileSync } from "fs";
import { resolve } from "path";

interface HardhatArtifact {
  _format: string;
  contractName: string;
  sourceName: string;
  abi: unknown[];
  bytecode: string;
  deployedBytecode: string;
  linkReferences?: Record<string, unknown>;
  deployedLinkReferences?: Record<string, unknown>;
}

interface FoundryArtifact {
  abi: unknown[];
  bytecode: {
    object: string;
    sourceMap?: string;
    linkReferences: Record<string, unknown>;
  };
  deployedBytecode: {
    object: string;
    sourceMap?: string;
    linkReferences: Record<string, unknown>;
    immutableReferences?: Record<string, Array<{ start: number; length: number }>>;
  };
  methodIdentifiers: Record<string, string>;
  rawMetadata?: string;
  metadata?: unknown;
  ast?: unknown;
}

type ArtifactFormat = "hardhat" | "foundry";

/**
 * Compute keccak256 hash and return first 4 bytes (8 hex chars) as function selector.
 */
function computeSelector(signature: string): string {
  const hash = keccak256(signature);
  return hash.slice(0, 8);
}

/**
 * Compute keccak256 using ethers or viem (whichever is available).
 */
function keccak256(input: string): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { keccak256: ethersKeccak256, toUtf8Bytes } = require("ethers");
    return ethersKeccak256(toUtf8Bytes(input)).slice(2);
  } catch {
    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const { keccak256: viemKeccak256, stringToBytes } = require("viem");
      return viemKeccak256(stringToBytes(input)).slice(2);
    } catch {
      console.warn(`  Warning: Could not compute keccak256 for ${input}, using placeholder`);
      return "00000000";
    }
  }
}

/**
 * Detect artifact format from JSON structure.
 */
function detectFormat(artifact: unknown): ArtifactFormat {
  const obj = artifact as Record<string, unknown>;

  // Hardhat artifacts have _format field
  if (obj._format && typeof obj._format === "string") {
    return "hardhat";
  }

  // Foundry artifacts have nested bytecode.object
  if (obj.bytecode && typeof obj.bytecode === "object") {
    const bytecode = obj.bytecode as Record<string, unknown>;
    if ("object" in bytecode) {
      return "foundry";
    }
  }

  // Fallback: check if bytecode is a string (Hardhat) or object (Foundry)
  if (typeof obj.bytecode === "string") {
    return "hardhat";
  }

  throw new Error("Unable to detect artifact format");
}

/**
 * Extract method identifiers from ABI with proper selector computation.
 */
function extractMethodIdentifiers(abi: unknown[]): Record<string, string> {
  const identifiers: Record<string, string> = {};

  for (const item of abi) {
    const entry = item as { type: string; name?: string; inputs?: Array<{ type: string }> };
    if (entry.type === "function" && entry.name && entry.inputs) {
      const signature = `${entry.name}(${entry.inputs.map((i) => i.type).join(",")})`;
      identifiers[signature] = computeSelector(signature);
    }
  }

  return identifiers;
}

/**
 * Extract contract name from ABI (uses first constructor or event).
 */
function extractContractName(abi: unknown[], sourcePath?: string): string {
  // Try to extract from source path if available
  if (sourcePath) {
    const match = sourcePath.match(/([^/]+)\.sol$/);
    if (match) return match[1];
  }

  // Look for constructor or named items
  for (const item of abi) {
    const entry = item as { type: string; name?: string };
    if (entry.type === "constructor") {
      return "Contract";
    }
    if (entry.name) {
      // Use first named item as hint
      return entry.name.replace(/^(get|set|is|has)/, "");
    }
  }

  return "Contract";
}

/**
 * Convert Hardhat artifact to Foundry format.
 */
function convertToFoundry(hardhat: HardhatArtifact): FoundryArtifact {
  return {
    abi: hardhat.abi,
    bytecode: {
      object: hardhat.bytecode,
      linkReferences: (hardhat.linkReferences as Record<string, unknown>) ?? {},
    },
    deployedBytecode: {
      object: hardhat.deployedBytecode,
      linkReferences: (hardhat.deployedLinkReferences as Record<string, unknown>) ?? {},
    },
    methodIdentifiers: extractMethodIdentifiers(hardhat.abi),
  };
}

/**
 * Convert Foundry artifact to Hardhat format.
 */
function convertToHardhat(foundry: FoundryArtifact, inputPath?: string): HardhatArtifact {
  const contractName = extractContractName(foundry.abi, inputPath);

  return {
    _format: "hh-sol-artifact-1",
    contractName: contractName,
    sourceName: `contracts/${contractName}.sol`,
    abi: foundry.abi,
    bytecode: foundry.bytecode.object,
    deployedBytecode: foundry.deployedBytecode.object,
    linkReferences: foundry.bytecode.linkReferences,
    deployedLinkReferences: foundry.deployedBytecode.linkReferences,
  };
}

function main(): void {
  const args = process.argv.slice(2);

  // Parse flags
  const toHardhat = args.includes("--to-hardhat");
  const toFoundry = args.includes("--to-foundry");
  const filteredArgs = args.filter((a) => !a.startsWith("--"));

  if (filteredArgs.length < 2) {
    console.log("Usage: npx ts-node tools/convert-artifact.ts <input.json> <output.json> [--to-hardhat|--to-foundry]");
    console.log("");
    console.log("Options:");
    console.log("  --to-hardhat  Force conversion to Hardhat format");
    console.log("  --to-foundry  Force conversion to Foundry format");
    console.log("");
    console.log("If no flag is provided, auto-detects input format and converts to the other.");
    process.exit(1);
  }

  const inputPath = resolve(filteredArgs[0]);
  const outputPath = resolve(filteredArgs[1]);

  console.log(`Input:  ${inputPath}`);

  const rawArtifact = JSON.parse(readFileSync(inputPath, "utf-8"));
  const detectedFormat = detectFormat(rawArtifact);

  console.log(`Detected format: ${detectedFormat}`);

  let outputArtifact: HardhatArtifact | FoundryArtifact;
  let outputFormat: ArtifactFormat;

  if (toHardhat) {
    if (detectedFormat === "hardhat") {
      console.log("Input is already Hardhat format, copying as-is.");
      outputArtifact = rawArtifact;
    } else {
      outputArtifact = convertToHardhat(rawArtifact as FoundryArtifact, inputPath);
    }
    outputFormat = "hardhat";
  } else if (toFoundry) {
    if (detectedFormat === "foundry") {
      console.log("Input is already Foundry format, copying as-is.");
      outputArtifact = rawArtifact;
    } else {
      outputArtifact = convertToFoundry(rawArtifact as HardhatArtifact);
    }
    outputFormat = "foundry";
  } else {
    // Auto-convert to opposite format
    if (detectedFormat === "hardhat") {
      outputArtifact = convertToFoundry(rawArtifact as HardhatArtifact);
      outputFormat = "foundry";
    } else {
      outputArtifact = convertToHardhat(rawArtifact as FoundryArtifact, inputPath);
      outputFormat = "hardhat";
    }
  }

  console.log(`Output: ${outputPath} (${outputFormat})`);

  writeFileSync(outputPath, JSON.stringify(outputArtifact, null, 2));
  console.log("Conversion complete!");
}

main();
