#!/usr/bin/env npx ts-node
/**
 * Generate viewCalls configuration from contract ABI.
 *
 * Extracts all view/pure functions and generates a template configuration
 * that users can edit to add expected values.
 *
 * Usage:
 *   npx ts-node tools/generate-viewcalls.ts <artifact.json> <output.json>
 *   npx ts-node tools/generate-viewcalls.ts <artifact.json> <output.json> --no-params
 */

import { readFileSync, writeFileSync, mkdirSync } from "fs";
import { resolve, dirname } from "path";

interface AbiInput {
  name: string;
  type: string;
  internalType?: string;
}

interface AbiElement {
  type: string;
  name?: string;
  inputs?: AbiInput[];
  outputs?: AbiInput[];
  stateMutability?: string;
}

interface ViewCallTemplate {
  $comment?: string;
  function: string;
  params?: string[];
  expected: string;
}

interface GeneratedConfig {
  $comment: string;
  viewCalls: ViewCallTemplate[];
}

/**
 * Get a placeholder value for a Solidity type.
 */
function getPlaceholder(type: string, name?: string): string {
  const displayName = name ? `_${name}` : "";

  if (type === "address") return `TODO${displayName}_address`;
  if (type === "bool") return `TODO${displayName}_bool`;
  if (type === "string") return `TODO${displayName}_string`;
  if (type.startsWith("bytes")) return `TODO${displayName}_${type}`;
  if (type.startsWith("uint") || type.startsWith("int")) return `TODO${displayName}_${type}`;
  if (type.endsWith("[]")) return `TODO${displayName}_array`;
  return `TODO${displayName}`;
}

/**
 * Generate human-readable description from function signature.
 */
function generateDescription(func: AbiElement): string {
  const name = func.name || "unknown";
  const outputs = func.outputs || [];

  // Common patterns
  if (name === "hasRole" || name.startsWith("has")) {
    return `Check if account has role`;
  }
  if (name.startsWith("get") || name.startsWith("is") || name.startsWith("has")) {
    return `Get ${name.replace(/^(get|is|has)/, "").toLowerCase()}`;
  }
  if (name.endsWith("_ROLE")) {
    return `Get ${name} hash`;
  }
  if (name.toUpperCase() === name) {
    return `Get constant ${name}`;
  }

  // Generic
  const outputDesc = outputs.length > 0 ? outputs.map((o) => o.type).join(", ") : "void";
  return `${name}() -> ${outputDesc}`;
}

/**
 * Load artifact and extract ABI.
 */
function loadArtifact(artifactPath: string): AbiElement[] {
  const content = JSON.parse(readFileSync(artifactPath, "utf-8"));

  // Handle both Hardhat and Foundry formats
  if (Array.isArray(content.abi)) {
    return content.abi;
  }
  if (Array.isArray(content)) {
    return content; // Direct ABI array
  }
  throw new Error("Could not find ABI in artifact");
}

/**
 * Extract view/pure functions from ABI.
 */
function extractViewFunctions(abi: AbiElement[]): AbiElement[] {
  return abi.filter(
    (e) =>
      e.type === "function" && (e.stateMutability === "view" || e.stateMutability === "pure") && e.name !== undefined,
  );
}

/**
 * Generate viewCalls configuration.
 */
function generateViewCalls(
  viewFunctions: AbiElement[],
  options: { includeParams: boolean; includeComments: boolean },
): ViewCallTemplate[] {
  const viewCalls: ViewCallTemplate[] = [];

  for (const func of viewFunctions) {
    const inputs = func.inputs || [];
    const outputs = func.outputs || [];

    const template: ViewCallTemplate = {
      function: func.name!,
      expected: outputs.length > 0 ? getPlaceholder(outputs[0].type, outputs[0].name) : "TODO",
    };

    if (options.includeComments) {
      template.$comment = generateDescription(func);
    }

    // Add params if function has inputs
    if (inputs.length > 0 && options.includeParams) {
      template.params = inputs.map((i) => getPlaceholder(i.type, i.name));
    }

    viewCalls.push(template);
  }

  // Sort: no-param functions first, then by name
  viewCalls.sort((a, b) => {
    const aHasParams = a.params && a.params.length > 0;
    const bHasParams = b.params && b.params.length > 0;
    if (aHasParams !== bHasParams) {
      return aHasParams ? 1 : -1;
    }
    return a.function.localeCompare(b.function);
  });

  return viewCalls;
}

function printUsage(): void {
  console.log("Usage: npx ts-node tools/generate-viewcalls.ts <artifact.json> <output.json> [options]");
  console.log("");
  console.log("Options:");
  console.log("  --no-params     Only include functions without parameters");
  console.log("  --no-comments   Exclude $comment fields");
  console.log("  --help          Show this help");
}

function main(): void {
  const args = process.argv.slice(2);

  // Parse flags
  const noParams = args.includes("--no-params");
  const noComments = args.includes("--no-comments");
  const showHelp = args.includes("--help") || args.includes("-h");
  const positionalArgs = args.filter((a) => !a.startsWith("--"));

  if (showHelp || positionalArgs.length < 2) {
    printUsage();
    process.exit(showHelp ? 0 : 1);
  }

  const artifactPath = resolve(process.cwd(), positionalArgs[0]);
  const outputPath = resolve(process.cwd(), positionalArgs[1]);

  console.log("View Calls Generator");
  console.log("=".repeat(50));
  console.log(`Artifact: ${artifactPath}`);
  console.log(`Output:   ${outputPath}`);

  try {
    const abi = loadArtifact(artifactPath);
    const viewFunctions = extractViewFunctions(abi);

    console.log(`\nFound ${viewFunctions.length} view/pure functions`);

    // Filter to no-params only if requested
    const filteredFunctions = noParams
      ? viewFunctions.filter((f) => !f.inputs || f.inputs.length === 0)
      : viewFunctions;

    if (noParams) {
      console.log(`Filtered to ${filteredFunctions.length} functions without parameters`);
    }

    const viewCalls = generateViewCalls(filteredFunctions, {
      includeParams: !noParams,
      includeComments: !noComments,
    });

    const config: GeneratedConfig = {
      $comment: `Generated view calls template. Replace TODO values with expected results. Remove functions you don't need to verify.`,
      viewCalls,
    };

    // Ensure output directory exists
    const outputDir = dirname(outputPath);
    mkdirSync(outputDir, { recursive: true });

    writeFileSync(outputPath, JSON.stringify(config, null, 2) + "\n");

    console.log(`\n✓ Generated ${viewCalls.length} view call templates`);
    console.log(`  Written to: ${outputPath}`);
    console.log("\nNext steps:");
    console.log("  1. Edit the output file to set expected values");
    console.log("  2. Remove functions you don't need to verify");
    console.log("  3. Add params for functions that need them");
    console.log("  4. Copy viewCalls array into your verification config");
  } catch (error) {
    console.error("\nError:", error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

main();
