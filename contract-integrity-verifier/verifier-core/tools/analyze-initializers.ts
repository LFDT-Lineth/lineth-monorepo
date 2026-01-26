#!/usr/bin/env npx ts-node
/**
 * Analyze contract initializers and reinitializers to suggest state verification.
 *
 * This tool examines constructor and initializer function signatures to suggest
 * what state should be verified after deployment/upgrade.
 *
 * LIMITATIONS:
 * - ABI only contains function signatures, not implementation
 * - Cannot determine exact storage locations or role hashes
 * - Cannot know which mappings are populated
 * - User must fill in expected values based on deployment script
 *
 * Usage:
 *   npx ts-node tools/analyze-initializers.ts <artifact.json>
 *   npx ts-node tools/analyze-initializers.ts <artifact.json> <output.json>
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

interface Suggestion {
  source: string;
  input: string;
  inputType: string;
  suggestedVerification: string;
  notes: string[];
}

interface AnalysisResult {
  $comment: string;
  contractInfo: {
    hasConstructor: boolean;
    initializerFunctions: string[];
    reinitializerFunctions: string[];
  };
  suggestions: Suggestion[];
  templateViewCalls: Array<{
    $comment: string;
    function: string;
    params?: string[];
    expected: string;
  }>;
  templateSlots: Array<{
    $comment: string;
    slot: string;
    type: string;
    name: string;
    expected: string;
  }>;
}

/**
 * Load artifact and extract ABI.
 */
function loadArtifact(artifactPath: string): AbiElement[] {
  const content = JSON.parse(readFileSync(artifactPath, "utf-8"));
  
  if (Array.isArray(content.abi)) {
    return content.abi;
  }
  if (Array.isArray(content)) {
    return content;
  }
  throw new Error("Could not find ABI in artifact");
}

/**
 * Find constructor in ABI.
 */
function findConstructor(abi: AbiElement[]): AbiElement | undefined {
  return abi.find((e) => e.type === "constructor");
}

/**
 * Find initializer/reinitializer functions.
 */
function findInitializers(abi: AbiElement[]): { initializers: AbiElement[]; reinitializers: AbiElement[] } {
  const initializers: AbiElement[] = [];
  const reinitializers: AbiElement[] = [];
  
  for (const element of abi) {
    if (element.type !== "function" || !element.name) continue;
    
    const name = element.name.toLowerCase();
    
    // Reinitializers typically have version in name
    if (name.includes("reinit") || /reinitialize.*v\d+/i.test(element.name)) {
      reinitializers.push(element);
    }
    // General initializers
    else if (name === "initialize" || name.startsWith("init") || name.includes("_init")) {
      initializers.push(element);
    }
  }
  
  return { initializers, reinitializers };
}

/**
 * Analyze an input parameter and suggest verifications.
 */
function analyzeInput(input: AbiInput, functionName: string): Suggestion {
  const suggestion: Suggestion = {
    source: functionName,
    input: input.name,
    inputType: input.type,
    suggestedVerification: "",
    notes: [],
  };
  
  const nameLower = input.name.toLowerCase();
  
  // Address inputs - likely set in storage
  if (input.type === "address") {
    if (nameLower.includes("admin") || nameLower.includes("owner")) {
      suggestion.suggestedVerification = `viewCall: owner() or hasRole(DEFAULT_ADMIN_ROLE, ${input.name})`;
      suggestion.notes.push("Check if address has admin/owner role after init");
    } else if (nameLower.includes("operator")) {
      suggestion.suggestedVerification = `viewCall: hasRole(OPERATOR_ROLE, ${input.name})`;
      suggestion.notes.push("Check OPERATOR_ROLE is granted to this address");
      suggestion.notes.push("You need the OPERATOR_ROLE hash (keccak256('OPERATOR_ROLE'))");
    } else if (nameLower.includes("verifier")) {
      suggestion.suggestedVerification = `viewCall: hasRole(VERIFIER_ROLE, ${input.name}) or storagePath`;
      suggestion.notes.push("Check verifier role or storage slot");
    } else if (nameLower.includes("manager") || nameLower.includes("service")) {
      suggestion.suggestedVerification = `viewCall: getter for ${input.name} or storagePath`;
      suggestion.notes.push("Look for a getter function or verify via storage path");
    } else {
      suggestion.suggestedVerification = `viewCall or storagePath for ${input.name}`;
      suggestion.notes.push("Check if there's a getter function for this address");
    }
  }
  // Address arrays - likely role grants
  else if (input.type === "address[]") {
    if (nameLower.includes("operator")) {
      suggestion.suggestedVerification = `viewCall: hasRole(OPERATOR_ROLE, addr) for each address`;
      suggestion.notes.push("Create one viewCall per address in the array");
      suggestion.notes.push("You need the OPERATOR_ROLE hash from the contract");
    } else if (nameLower.includes("admin")) {
      suggestion.suggestedVerification = `viewCall: hasRole(DEFAULT_ADMIN_ROLE, addr) for each`;
      suggestion.notes.push("DEFAULT_ADMIN_ROLE is typically 0x00...00");
    } else {
      suggestion.suggestedVerification = `viewCall: check role for each address in ${input.name}`;
      suggestion.notes.push("Determine which role is granted to these addresses");
    }
  }
  // Uint values - likely config values
  else if (input.type.startsWith("uint")) {
    suggestion.suggestedVerification = `viewCall: getter for ${input.name} or slot`;
    suggestion.notes.push("Look for a getter function or determine storage slot");
  }
  // Bool values
  else if (input.type === "bool") {
    suggestion.suggestedVerification = `viewCall or slot for ${input.name}`;
    suggestion.notes.push("Check if stored in a getter or direct storage");
  }
  // Bytes32 - could be role hash or other identifier
  else if (input.type === "bytes32") {
    suggestion.suggestedVerification = `Depends on usage - may be role hash or identifier`;
    suggestion.notes.push("Check contract code to understand how this is used");
  }
  else {
    suggestion.suggestedVerification = `Manual analysis needed for ${input.type}`;
    suggestion.notes.push("Complex type requires understanding contract implementation");
  }
  
  return suggestion;
}

/**
 * Generate template viewCalls for common patterns.
 */
function generateTemplates(suggestions: Suggestion[]): AnalysisResult["templateViewCalls"] {
  const templates: AnalysisResult["templateViewCalls"] = [];
  const seen = new Set<string>();
  
  for (const s of suggestions) {
    // Role checks
    if (s.inputType === "address[]" && s.input.toLowerCase().includes("operator")) {
      if (!seen.has("operator_role")) {
        templates.push({
          $comment: `Check OPERATOR_ROLE for each address in ${s.input}`,
          function: "hasRole",
          params: ["TODO_OPERATOR_ROLE_HASH", "TODO_OPERATOR_ADDRESS"],
          expected: "true",
        });
        seen.add("operator_role");
      }
    }
    // Single address storage
    else if (s.inputType === "address" && (s.input.includes("manager") || s.input.includes("service"))) {
      templates.push({
        $comment: `Verify ${s.input} is set correctly`,
        function: `get${s.input.charAt(0).toUpperCase()}${s.input.slice(1).replace(/^_/, "")}`,
        expected: "TODO_ADDRESS",
      });
    }
  }
  
  return templates;
}

function printUsage(): void {
  console.log("Usage: npx ts-node tools/analyze-initializers.ts <artifact.json> [output.json]");
  console.log("");
  console.log("Arguments:");
  console.log("  artifact.json   Path to contract artifact (Hardhat or Foundry format)");
  console.log("  output.json     Optional output file for JSON analysis");
  console.log("");
  console.log("Options:");
  console.log("  --help          Show this help");
}

function main(): void {
  const args = process.argv.slice(2);
  
  const showHelp = args.includes("--help") || args.includes("-h");
  const positionalArgs = args.filter((a) => !a.startsWith("--"));
  
  if (showHelp || positionalArgs.length < 1) {
    printUsage();
    process.exit(showHelp ? 0 : 1);
  }

  const artifactPath = resolve(process.cwd(), positionalArgs[0]);
  const outputPath = positionalArgs[1] ? resolve(process.cwd(), positionalArgs[1]) : undefined;

  console.log("Initializer Analyzer");
  console.log("=".repeat(60));
  console.log(`Artifact: ${artifactPath}`);
  console.log("");

  try {
    const abi = loadArtifact(artifactPath);
    const constructor = findConstructor(abi);
    const { initializers, reinitializers } = findInitializers(abi);
    
    console.log("Contract Analysis:");
    console.log(`  Constructor: ${constructor ? "Yes" : "No"}`);
    console.log(`  Initializers: ${initializers.map((i) => i.name).join(", ") || "None"}`);
    console.log(`  Reinitializers: ${reinitializers.map((i) => i.name).join(", ") || "None"}`);
    
    const suggestions: Suggestion[] = [];
    
    // Analyze constructor
    if (constructor && constructor.inputs) {
      console.log(`\n--- Constructor Inputs ---`);
      for (const input of constructor.inputs) {
        console.log(`  ${input.name}: ${input.type}`);
        suggestions.push(analyzeInput(input, "constructor"));
      }
    }
    
    // Analyze initializers
    for (const init of initializers) {
      if (init.inputs && init.inputs.length > 0) {
        console.log(`\n--- ${init.name}() Inputs ---`);
        for (const input of init.inputs) {
          console.log(`  ${input.name}: ${input.type}`);
          suggestions.push(analyzeInput(input, init.name!));
        }
      }
    }
    
    // Analyze reinitializers
    for (const reinit of reinitializers) {
      if (reinit.inputs && reinit.inputs.length > 0) {
        console.log(`\n--- ${reinit.name}() Inputs ---`);
        for (const input of reinit.inputs) {
          console.log(`  ${input.name}: ${input.type}`);
          suggestions.push(analyzeInput(input, reinit.name!));
        }
      }
    }
    
    // Print suggestions
    console.log("\n" + "=".repeat(60));
    console.log("VERIFICATION SUGGESTIONS");
    console.log("=".repeat(60));
    
    for (const s of suggestions) {
      console.log(`\n[${s.source}] ${s.input} (${s.inputType})`);
      console.log(`  Suggested: ${s.suggestedVerification}`);
      for (const note of s.notes) {
        console.log(`  Note: ${note}`);
      }
    }
    
    // Standard checks
    console.log("\n" + "=".repeat(60));
    console.log("STANDARD CHECKS (always recommended)");
    console.log("=".repeat(60));
    console.log(`
1. _initialized slot (OZ Initializable):
   { "slot": "0x0", "type": "uint8", "name": "_initialized", "expected": "N" }
   
2. CONTRACT_VERSION (if present):
   { "function": "CONTRACT_VERSION", "expected": "X.Y" }
   
3. Owner/Admin check:
   { "function": "owner", "expected": "0x..." } or
   { "function": "hasRole", "params": ["0x00..00", "0x..."], "expected": true }
`);
    
    // Output to file if requested
    if (outputPath) {
      const result: AnalysisResult = {
        $comment: "Generated by analyze-initializers. Fill in TODO values and remove unused entries.",
        contractInfo: {
          hasConstructor: constructor !== undefined,
          initializerFunctions: initializers.map((i) => i.name!),
          reinitializerFunctions: reinitializers.map((i) => i.name!),
        },
        suggestions,
        templateViewCalls: generateTemplates(suggestions),
        templateSlots: [
          {
            $comment: "OZ Initializable._initialized (verify init version)",
            slot: "0x0",
            type: "uint8",
            name: "_initialized",
            expected: "TODO_VERSION_NUMBER",
          },
        ],
      };
      
      const outputDir = dirname(outputPath);
      mkdirSync(outputDir, { recursive: true });
      
      writeFileSync(outputPath, JSON.stringify(result, null, 2) + "\n");
      console.log(`\n✓ Analysis written to: ${outputPath}`);
    }
    
    console.log("\n" + "=".repeat(60));
    console.log("LIMITATIONS");
    console.log("=".repeat(60));
    console.log(`
This tool analyzes function SIGNATURES only, not implementation.
You must manually determine:
  - Exact role hashes (keccak256 of role name)
  - Which storage paths to verify
  - Expected values from your deployment script
  - Which addresses receive which roles
`);
    
  } catch (error) {
    console.error("\nError:", error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

main();
