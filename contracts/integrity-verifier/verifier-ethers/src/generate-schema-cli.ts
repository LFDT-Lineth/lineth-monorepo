#!/usr/bin/env node
/**
 * Schema Generator CLI - Ethers
 *
 * Generates storage schema JSON from Solidity storage layout files.
 * Uses ethers for crypto operations.
 *
 * Usage:
 *   npx generate-schema-ethers <input.sol...> -o <output.json> [--verbose]
 *   npx generate-schema-ethers <input.sol> <output.json> [--verbose]  (legacy)
 *
 * Examples:
 *   # Single file (legacy mode)
 *   npx generate-schema-ethers Storage.sol schema.json
 *
 *   # Multiple files (recommended for inherited storage)
 *   npx generate-schema-ethers LineaRollupYieldExtension.sol YieldManager.sol -o schema.json
 *
 *   # With verbose output
 *   npx generate-schema-ethers *.sol -o schema.json --verbose
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync, statSync, readdirSync } from "fs";
import { resolve, dirname, basename, extname } from "path";

import { generateSchema } from "./tools";

function printUsage(): void {
  console.log("Schema Generator CLI (ethers)");
  console.log("");
  console.log("Usage:");
  console.log("  generate-schema-ethers <input.sol...> -o <output.json> [options]");
  console.log("  generate-schema-ethers <input.sol> <output.json> [options]  (legacy)");
  console.log("");
  console.log("Arguments:");
  console.log("  input.sol...   One or more Solidity files or directories containing storage structs");
  console.log("  -o, --output   Output JSON schema file path");
  console.log("");
  console.log("Options:");
  console.log("  --verbose, -v  Show detailed field-level output");
  console.log("  --help, -h     Show this help");
  console.log("");
  console.log("Examples:");
  console.log("  # Single file (legacy mode)");
  console.log("  generate-schema-ethers Storage.sol schema.json");
  console.log("");
  console.log("  # Multiple files (recommended for inherited storage)");
  console.log("  generate-schema-ethers LineaRollupYieldExtension.sol YieldManager.sol -o schema.json");
  console.log("");
  console.log("  # Process all .sol files in a directory");
  console.log("  generate-schema-ethers ./contracts/storage/ -o schema.json");
}

interface ParsedArgs {
  inputFiles: string[];
  outputPath: string;
  verbose: boolean;
  showHelp: boolean;
}

function parseArgs(args: string[]): ParsedArgs {
  const verbose = args.includes("--verbose") || args.includes("-v");
  const showHelp = args.includes("--help") || args.includes("-h");

  let outputPath = "";
  const outputIndex = args.findIndex((a) => a === "-o" || a === "--output");
  if (outputIndex !== -1 && outputIndex + 1 < args.length) {
    outputPath = args[outputIndex + 1];
  }

  const flagsAndOutput = new Set(["-v", "--verbose", "-h", "--help", "-o", "--output"]);
  const inputFiles: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (flagsAndOutput.has(arg)) {
      if (arg === "-o" || arg === "--output") {
        i++;
      }
      continue;
    }
    inputFiles.push(arg);
  }

  // Legacy mode: if no -o flag and exactly 2 positional args, treat second as output
  if (!outputPath && inputFiles.length === 2 && inputFiles[1].endsWith(".json")) {
    outputPath = inputFiles.pop()!;
  }

  return { inputFiles, outputPath, verbose, showHelp };
}

function resolveInputFiles(patterns: string[], cwd: string): string[] {
  const files: string[] = [];

  for (const pattern of patterns) {
    const resolvedPath = resolve(cwd, pattern);

    if (existsSync(resolvedPath) && statSync(resolvedPath).isDirectory()) {
      const dirFiles = readdirSync(resolvedPath)
        .filter((f) => extname(f) === ".sol")
        .map((f) => resolve(resolvedPath, f));
      files.push(...dirFiles);
    } else if (existsSync(resolvedPath)) {
      files.push(resolvedPath);
    } else {
      console.warn(`Warning: File not found: ${pattern}`);
    }
  }

  return [...new Set(files)];
}

function main(): void {
  const args = process.argv.slice(2);
  const { inputFiles, outputPath, verbose, showHelp } = parseArgs(args);

  if (showHelp || inputFiles.length === 0 || !outputPath) {
    printUsage();
    process.exit(showHelp ? 0 : 1);
  }

  console.log("Storage Schema Generator (ethers)");
  console.log("=".repeat(50));

  const resolvedInputs = resolveInputFiles(inputFiles, process.cwd());
  const resolvedOutput = resolve(process.cwd(), outputPath);

  if (resolvedInputs.length === 0) {
    console.error("Error: No valid input files found.");
    process.exit(1);
  }

  console.log(`Input files (${resolvedInputs.length}):`);
  for (const file of resolvedInputs) {
    console.log(`  - ${basename(file)}`);
  }
  console.log(`Output: ${resolvedOutput}`);

  try {
    const sources = resolvedInputs.map((inputPath) => ({
      source: readFileSync(inputPath, "utf-8"),
      fileName: basename(inputPath),
    }));

    const { schema, warnings } = generateSchema(sources);

    if (warnings.length > 0) {
      console.log("\nWarnings:");
      for (const warning of warnings) {
        console.log(`  - ${warning}`);
      }
    }

    const structCount = Object.keys(schema.structs).length;
    if (structCount === 0) {
      console.log("\nNo structs found in the input files.");
      process.exit(1);
    }

    console.log(`\nFound ${structCount} struct(s):`);
    for (const [name, def] of Object.entries(schema.structs)) {
      const fieldCount = Object.keys(def.fields).length;
      const nsInfo = def.namespace ? ` (ns: ${def.namespace})` : "";
      const slotInfo = def.baseSlot ? ` [slot: ${def.baseSlot.slice(0, 10)}...]` : "";
      console.log(`  - ${name}: ${fieldCount} fields${nsInfo}${slotInfo}`);

      if (verbose) {
        if (def.baseSlot) {
          console.log(`      baseSlot: ${def.baseSlot}`);
        }
        for (const [fieldName, fieldDef] of Object.entries(def.fields)) {
          const offset = fieldDef.byteOffset !== undefined ? ` @byte ${fieldDef.byteOffset}` : "";
          console.log(`      slot ${fieldDef.slot}: ${fieldName} (${fieldDef.type})${offset}`);
        }
      }
    }

    const missingBaseSlots = Object.entries(schema.structs)
      .filter(([, def]) => def.namespace && !def.baseSlot)
      .map(([name]) => name);

    if (missingBaseSlots.length > 0) {
      console.log("\nWarning: The following structs have a namespace but no baseSlot:");
      for (const name of missingBaseSlots) {
        console.log(`    - ${name}`);
      }
    }

    const outputDir = dirname(resolvedOutput);
    mkdirSync(outputDir, { recursive: true });

    writeFileSync(resolvedOutput, JSON.stringify(schema, null, 2) + "\n");
    console.log(`\nSchema written to ${resolvedOutput}`);
  } catch (error) {
    console.error("\nError:", error instanceof Error ? error.message : String(error));
    process.exit(2);
  }
}

main();
