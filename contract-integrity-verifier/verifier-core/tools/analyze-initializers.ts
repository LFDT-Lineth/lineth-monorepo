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
    discoveredRoles: string[];
  };
  suggestions: Suggestion[];
  roleHashes: Array<{
    $comment: string;
    role: string;
    function: string;
    hash: string;
  }>;
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
 * Discovered role from ABI.
 */
interface DiscoveredRole {
  name: string;
  functionName: string;
  returnsBytes32: boolean;
}

/**
 * Find all role constants/getters from the ABI.
 * Looks for functions like: DEFAULT_ADMIN_ROLE(), OPERATOR_ROLE(), PAUSER_ROLE(), etc.
 */
function findRoles(abi: AbiElement[]): DiscoveredRole[] {
  const roles: DiscoveredRole[] = [];
  const seenNames = new Set<string>();

  for (const element of abi) {
    if (element.type !== "function" || !element.name) continue;

    // Look for patterns:
    // - Ends with _ROLE (e.g., OPERATOR_ROLE, PAUSER_ROLE)
    // - Is DEFAULT_ADMIN_ROLE
    // - Ends with _ROLE_ADMIN or similar
    const name = element.name;
    const isRolePattern =
      name.endsWith("_ROLE") ||
      name.endsWith("_ROLE_ADMIN") ||
      name === "DEFAULT_ADMIN_ROLE" ||
      (name.includes("ROLE") && name === name.toUpperCase());

    if (!isRolePattern) continue;

    // Should be a view/pure function with no inputs and bytes32 output
    const isViewOrPure = element.stateMutability === "view" || element.stateMutability === "pure";
    const hasNoInputs = !element.inputs || element.inputs.length === 0;
    const returnsBytes32 = element.outputs?.length === 1 && element.outputs[0].type === "bytes32";

    if (isViewOrPure && hasNoInputs && !seenNames.has(name)) {
      roles.push({
        name: name,
        functionName: name,
        returnsBytes32: returnsBytes32 ?? false,
      });
      seenNames.add(name);
    }
  }

  // Sort roles: DEFAULT_ADMIN_ROLE first, then alphabetically
  return roles.sort((a, b) => {
    if (a.name === "DEFAULT_ADMIN_ROLE") return -1;
    if (b.name === "DEFAULT_ADMIN_ROLE") return 1;
    return a.name.localeCompare(b.name);
  });
}

/**
 * Try to infer which role an address parameter might receive based on naming.
 */
function inferRoleForAddress(inputName: string, roles: DiscoveredRole[]): DiscoveredRole | undefined {
  const nameLower = inputName.toLowerCase();

  // Direct matches
  for (const role of roles) {
    const roleLower = role.name.toLowerCase();
    const roleBaseName = roleLower.replace(/_role$/, "").replace(/_/g, "");

    // Check if input name contains the role base name
    if (nameLower.includes(roleBaseName)) {
      return role;
    }
  }

  // Common mappings
  if (nameLower.includes("admin") || nameLower.includes("owner")) {
    return roles.find((r) => r.name === "DEFAULT_ADMIN_ROLE" || r.name.includes("ADMIN"));
  }
  if (nameLower.includes("operator")) {
    return roles.find((r) => r.name.includes("OPERATOR"));
  }
  if (nameLower.includes("verifier") || nameLower.includes("prover")) {
    return roles.find((r) => r.name.includes("VERIFIER") || r.name.includes("PROVER"));
  }
  if (nameLower.includes("pause") || nameLower.includes("pauser")) {
    return roles.find((r) => r.name.includes("PAUSE"));
  }

  return undefined;
}

/**
 * Analyze an input parameter and suggest verifications.
 */
function analyzeInput(input: AbiInput, functionName: string, roles: DiscoveredRole[]): Suggestion {
  const suggestion: Suggestion = {
    source: functionName,
    input: input.name,
    inputType: input.type,
    suggestedVerification: "",
    notes: [],
  };

  const nameLower = input.name.toLowerCase();

  // Address inputs - likely set in storage or granted a role
  if (input.type === "address") {
    const inferredRole = inferRoleForAddress(input.name, roles);

    if (inferredRole) {
      suggestion.suggestedVerification = `viewCall: hasRole(${inferredRole.name}(), ${input.name})`;
      suggestion.notes.push(`Check ${inferredRole.name} is granted to this address`);
      suggestion.notes.push(`Get role hash: contract.${inferredRole.name}()`);
    } else if (nameLower.includes("admin") || nameLower.includes("owner")) {
      const adminRole = roles.find((r) => r.name === "DEFAULT_ADMIN_ROLE" || r.name.includes("ADMIN"));
      if (adminRole) {
        suggestion.suggestedVerification = `viewCall: hasRole(${adminRole.name}(), ${input.name})`;
        suggestion.notes.push(`Get role hash: contract.${adminRole.name}()`);
      } else {
        suggestion.suggestedVerification = `viewCall: owner() or hasRole check`;
        suggestion.notes.push("Check if address has admin/owner role after init");
      }
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
    const inferredRole = inferRoleForAddress(input.name, roles);

    if (inferredRole) {
      suggestion.suggestedVerification = `viewCall: hasRole(${inferredRole.name}(), addr) for each address`;
      suggestion.notes.push("Create one viewCall per address in the array");
      suggestion.notes.push(`Get role hash: contract.${inferredRole.name}()`);
    } else if (nameLower.includes("admin")) {
      const adminRole = roles.find((r) => r.name === "DEFAULT_ADMIN_ROLE");
      if (adminRole) {
        suggestion.suggestedVerification = `viewCall: hasRole(${adminRole.name}(), addr) for each`;
        suggestion.notes.push("DEFAULT_ADMIN_ROLE is typically 0x00...00");
      } else {
        suggestion.suggestedVerification = `viewCall: hasRole check for each admin address`;
      }
    } else {
      suggestion.suggestedVerification = `viewCall: check role for each address in ${input.name}`;
      suggestion.notes.push("Determine which role is granted to these addresses");
      if (roles.length > 0) {
        suggestion.notes.push(`Available roles: ${roles.map((r) => r.name).join(", ")}`);
      }
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
  } else {
    suggestion.suggestedVerification = `Manual analysis needed for ${input.type}`;
    suggestion.notes.push("Complex type requires understanding contract implementation");
  }

  return suggestion;
}

/**
 * Generate template viewCalls for common patterns.
 */
function generateTemplates(suggestions: Suggestion[], roles: DiscoveredRole[]): AnalysisResult["templateViewCalls"] {
  const templates: AnalysisResult["templateViewCalls"] = [];
  const seenRoles = new Set<string>();

  // Generate hasRole templates for discovered roles based on suggestions
  for (const s of suggestions) {
    if (s.inputType === "address" || s.inputType === "address[]") {
      const inferredRole = inferRoleForAddress(s.input, roles);
      if (inferredRole && !seenRoles.has(inferredRole.name)) {
        templates.push({
          $comment: `Check ${inferredRole.name} is granted (for ${s.input})`,
          function: "hasRole",
          params: [`contract.${inferredRole.name}()`, "TODO_ADDRESS"],
          expected: "true",
        });
        seenRoles.add(inferredRole.name);
      }
    }

    // Single address storage
    if (s.inputType === "address" && (s.input.includes("manager") || s.input.includes("service"))) {
      const cleanName = s.input.replace(/^_/, "");
      templates.push({
        $comment: `Verify ${s.input} is set correctly`,
        function: `get${cleanName.charAt(0).toUpperCase()}${cleanName.slice(1)}`,
        expected: "TODO_ADDRESS",
      });
    }
  }

  return templates;
}

/**
 * Generate role hash templates for output.
 */
function generateRoleTemplates(
  roles: DiscoveredRole[],
): Array<{ $comment: string; role: string; function: string; hash: string }> {
  return roles.map((role) => ({
    $comment: `Call contract.${role.name}() to get this hash`,
    role: role.name,
    function: role.functionName,
    hash: "TODO_CALL_CONTRACT_TO_GET_HASH",
  }));
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
    const roles = findRoles(abi);

    console.log("Contract Analysis:");
    console.log(`  Constructor: ${constructor ? "Yes" : "No"}`);
    console.log(`  Initializers: ${initializers.map((i) => i.name).join(", ") || "None"}`);
    console.log(`  Reinitializers: ${reinitializers.map((i) => i.name).join(", ") || "None"}`);

    // Display discovered roles
    if (roles.length > 0) {
      console.log(`\n--- Discovered Roles (${roles.length}) ---`);
      for (const role of roles) {
        console.log(`  ${role.name}() -> bytes32`);
      }
      console.log("\n  Tip: Call these functions on-chain to get the actual role hashes");
    } else {
      console.log(`\n  No role constants found in ABI`);
    }

    const suggestions: Suggestion[] = [];

    // Analyze constructor
    if (constructor && constructor.inputs) {
      console.log(`\n--- Constructor Inputs ---`);
      for (const input of constructor.inputs) {
        console.log(`  ${input.name}: ${input.type}`);
        suggestions.push(analyzeInput(input, "constructor", roles));
      }
    }

    // Analyze initializers
    for (const init of initializers) {
      if (init.inputs && init.inputs.length > 0) {
        console.log(`\n--- ${init.name}() Inputs ---`);
        for (const input of init.inputs) {
          console.log(`  ${input.name}: ${input.type}`);
          suggestions.push(analyzeInput(input, init.name!, roles));
        }
      }
    }

    // Analyze reinitializers
    for (const reinit of reinitializers) {
      if (reinit.inputs && reinit.inputs.length > 0) {
        console.log(`\n--- ${reinit.name}() Inputs ---`);
        for (const input of reinit.inputs) {
          console.log(`  ${input.name}: ${input.type}`);
          suggestions.push(analyzeInput(input, reinit.name!, roles));
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
   { "function": "hasRole", "params": ["<role_hash>", "0x..."], "expected": true }
`);

    // Print role hashes section if roles were discovered
    if (roles.length > 0) {
      console.log("=".repeat(60));
      console.log("ROLE HASHES (call these to get actual values)");
      console.log("=".repeat(60));
      for (const role of roles) {
        console.log(`  ${role.name}() -> call contract to get bytes32 hash`);
      }
      console.log("");
    }

    // Output to file if requested
    if (outputPath) {
      const result: AnalysisResult = {
        $comment: "Generated by analyze-initializers. Fill in TODO values and remove unused entries.",
        contractInfo: {
          hasConstructor: constructor !== undefined,
          initializerFunctions: initializers.map((i) => i.name!),
          reinitializerFunctions: reinitializers.map((i) => i.name!),
          discoveredRoles: roles.map((r) => r.name),
        },
        suggestions,
        roleHashes: generateRoleTemplates(roles),
        templateViewCalls: generateTemplates(suggestions, roles),
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
  - Exact role hashes (call the role functions on-chain)
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
