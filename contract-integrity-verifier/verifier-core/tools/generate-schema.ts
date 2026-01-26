#!/usr/bin/env ts-node
/**
 * Contract Integrity Verifier - Schema Generator
 *
 * Generates storage schema JSON from Solidity storage layout files.
 *
 * Usage:
 *   npx ts-node tools/generate-schema.ts <input.sol> <output.json> [--verbose]
 */

import { readFileSync, writeFileSync, mkdirSync } from "fs";
import { resolve, dirname } from "path";

// Dynamic crypto helpers - try ethers first, then viem
function keccak256(value: string | Uint8Array): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { keccak256: ethersKeccak256, toUtf8Bytes } = require("ethers");
    if (typeof value === "string") {
      if (value.startsWith("0x")) {
        return ethersKeccak256(value);
      }
      return ethersKeccak256(toUtf8Bytes(value));
    }
    return ethersKeccak256(value);
  } catch {
    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const { keccak256: viemKeccak256, stringToBytes, toHex } = require("viem");
      if (typeof value === "string") {
        if (value.startsWith("0x")) {
          return viemKeccak256(value as `0x${string}`);
        }
        return viemKeccak256(stringToBytes(value));
      }
      return viemKeccak256(toHex(value));
    } catch {
      throw new Error("Neither ethers nor viem is available. Install one of them to use this tool.");
    }
  }
}

function encodeUint256(value: bigint): string {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const { AbiCoder } = require("ethers");
    return AbiCoder.defaultAbiCoder().encode(["uint256"], [value]);
  } catch {
    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const { encodeAbiParameters } = require("viem");
      return encodeAbiParameters([{ type: "uint256" }], [value]);
    } catch {
      throw new Error("Neither ethers nor viem is available. Install one of them to use this tool.");
    }
  }
}

interface FieldDef {
  slot: number;
  type: string;
  byteOffset?: number;
}

interface StructDef {
  namespace?: string;
  baseSlot?: string;
  fields: Record<string, FieldDef>;
}

interface Schema {
  $comment?: string;
  structs: Record<string, StructDef>;
}

// Type sizes in bytes
const TYPE_SIZES: Record<string, number> = {
  bool: 1,
  uint8: 1,
  int8: 1,
  uint16: 2,
  int16: 2,
  uint32: 4,
  int32: 4,
  uint64: 8,
  int64: 8,
  uint96: 12,
  int96: 12,
  uint128: 16,
  int128: 16,
  uint256: 32,
  int256: 32,
  address: 20,
  bytes32: 32,
  bytes4: 4,
};

/**
 * Calculate ERC-7201 base slot from namespace ID.
 */
function calculateErc7201BaseSlot(namespaceId: string): string {
  const idHash = keccak256(namespaceId);
  const hashBigInt = BigInt(idHash);
  const decremented = hashBigInt - 1n;
  const encoded = encodeUint256(decremented);
  const finalHash = keccak256(encoded);
  const finalBigInt = BigInt(finalHash);
  const masked = finalBigInt & ~0xffn;
  return "0x" + masked.toString(16).padStart(64, "0");
}

/**
 * Extract namespace ID from NatSpec comment.
 * Looks for: @custom:storage-location erc7201:linea.storage.SomeName
 */
function extractNamespace(comments: string): string | undefined {
  const match = comments.match(/@custom:storage-location\s+erc7201:([^\s*]+)/);
  return match ? match[1] : undefined;
}

/**
 * Parse a Solidity type and return normalized type string.
 */
function normalizeType(solidityType: string): string {
  // Handle enums as uint8
  if (
    !solidityType.startsWith("uint") &&
    !solidityType.startsWith("int") &&
    !solidityType.startsWith("bytes") &&
    solidityType !== "address" &&
    solidityType !== "bool"
  ) {
    // Check if it looks like an enum (single word, PascalCase)
    if (/^[A-Z][a-zA-Z0-9]*$/.test(solidityType)) {
      return "uint8"; // Enums are uint8 by default
    }
  }

  // Handle arrays
  if (solidityType.endsWith("[]")) {
    const baseType = solidityType.slice(0, -2);
    return `${normalizeType(baseType)}[]`;
  }

  // Handle mappings: mapping(KeyType => ValueType)
  const mappingMatch = solidityType.match(/^mapping\s*\(\s*(\w+).*=>\s*(.+)\)$/);
  if (mappingMatch) {
    const keyType = normalizeType(mappingMatch[1]);
    const valueType = normalizeType(mappingMatch[2].trim());
    return `mapping(${keyType} => ${valueType})`;
  }

  return solidityType;
}

/**
 * Get the byte size of a type (for packing calculation).
 */
function getTypeSize(type: string): number {
  return TYPE_SIZES[type] ?? 32;
}

/**
 * Parse struct fields from Solidity source.
 */
function parseStructFields(structBody: string): Record<string, FieldDef> {
  const fields: Record<string, FieldDef> = {};
  let currentSlot = 0;
  let currentByteOffset = 0;

  const lines = structBody.split("\n");

  for (const line of lines) {
    // Skip comments
    if (line.trim().startsWith("//") || line.trim().startsWith("*")) {
      continue;
    }

    // Try mapping first
    const mappingMatch = line.match(/^\s*(mapping\s*\([^)]+\))\s+(\w+)\s*;/);
    if (mappingMatch) {
      const [, typeStr, name] = mappingMatch;
      // Mappings always start a new slot
      if (currentByteOffset > 0) {
        currentSlot++;
        currentByteOffset = 0;
      }
      fields[name] = {
        slot: currentSlot,
        type: normalizeType(typeStr),
      };
      currentSlot++;
      continue;
    }

    // Try regular field (handles types like IMessageService, uint256, address[], etc.)
    // Type pattern: word chars, optional interface prefix (I), optional array brackets
    const fieldMatch = line.match(/^\s*([A-Z]?[a-zA-Z0-9_]+(?:\[\])?)\s+(\w+)\s*;/);
    if (fieldMatch) {
      const [, typeStr, name] = fieldMatch;
      const normalizedType = normalizeType(typeStr);
      const typeSize = getTypeSize(normalizedType);

      // Dynamic types (arrays, mappings) always start a new slot
      if (normalizedType.endsWith("[]") || normalizedType.startsWith("mapping")) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        fields[name] = {
          slot: currentSlot,
          type: normalizedType,
        };
        currentSlot++;
        continue;
      }

      // Check if this field fits in the current slot
      if (currentByteOffset + typeSize > 32) {
        // Start new slot
        currentSlot++;
        currentByteOffset = 0;
      }

      fields[name] = {
        slot: currentSlot,
        type: normalizedType,
        ...(currentByteOffset > 0 ? { byteOffset: currentByteOffset } : {}),
      };

      currentByteOffset += typeSize;

      // If we filled the slot exactly, move to next
      if (currentByteOffset >= 32) {
        currentSlot++;
        currentByteOffset = 0;
      }
    }
  }

  return fields;
}

/**
 * Extract preceding comments (both multi-line and single-line styles) for a struct.
 */
function extractPrecedingComments(source: string, structIndex: number): string {
  // Look backwards from the struct to find comments
  const beforeStruct = source.slice(0, structIndex);
  const lines = beforeStruct.split("\n");

  const commentLines: string[] = [];

  // Walk backwards through lines to collect comments
  for (let i = lines.length - 1; i >= 0; i--) {
    const line = lines[i].trim();

    // Skip empty lines
    if (line === "") continue;

    // Collect /// style comments
    if (line.startsWith("///")) {
      commentLines.unshift(line);
      continue;
    }

    // Check for end of /** */ block
    if (line.endsWith("*/")) {
      // Find the start of this block
      let blockStart = i;
      while (blockStart >= 0 && !lines[blockStart].includes("/**")) {
        commentLines.unshift(lines[blockStart]);
        blockStart--;
      }
      if (blockStart >= 0) {
        commentLines.unshift(lines[blockStart]);
      }
      break;
    }

    // If we hit non-comment content, stop
    break;
  }

  return commentLines.join("\n");
}

/**
 * Parse structs and their storage locations from Solidity source.
 */
function parseSolidityFile(source: string): Schema {
  const schema: Schema = {
    $comment: "Auto-generated storage schema",
    structs: {},
  };

  // Find struct definitions
  const structPattern = /struct\s+(\w+)\s*\{([^}]+)\}/g;

  let match;
  while ((match = structPattern.exec(source)) !== null) {
    const [, structName, structBody] = match;
    const structIndex = match.index;

    // Extract comments that precede this struct
    const comments = extractPrecedingComments(source, structIndex);

    const namespace = extractNamespace(comments);
    const fields = parseStructFields(structBody);

    const structDef: StructDef = { fields };

    if (namespace) {
      structDef.namespace = namespace;
      structDef.baseSlot = calculateErc7201BaseSlot(namespace);
    }

    schema.structs[structName] = structDef;
  }

  // Also look for explicit storage location constants
  // Pattern: bytes32 private constant SomeStorageLocation = 0x...;
  const constantPattern = /bytes32\s+(?:private\s+)?constant\s+(\w+StorageLocation)\s*=\s*(0x[a-fA-F0-9]+)/g;
  while ((match = constantPattern.exec(source)) !== null) {
    const [, constantName, slotValue] = match;
    // Try to associate with a struct
    const structName = constantName.replace("StorageLocation", "");
    if (schema.structs[structName] && !schema.structs[structName].baseSlot) {
      schema.structs[structName].baseSlot = slotValue.toLowerCase();
    }
  }

  return schema;
}

function printUsage(): void {
  console.log("Usage: npx ts-node tools/generate-schema.ts <input.sol> <output.json> [--verbose]");
  console.log("");
  console.log("Arguments:");
  console.log("  input.sol     Input Solidity file containing storage layout structs");
  console.log("  output.json   Output JSON schema file path");
  console.log("");
  console.log("Options:");
  console.log("  --verbose, -v  Show detailed field-level output");
  console.log("  --help         Show this help");
}

function main(): void {
  const args = process.argv.slice(2);

  const verbose = args.includes("--verbose") || args.includes("-v");
  const showHelp = args.includes("--help") || args.includes("-h");
  const positionalArgs = args.filter((a) => !a.startsWith("-"));

  if (showHelp || positionalArgs.length < 2) {
    printUsage();
    process.exit(showHelp ? 0 : 1);
  }

  const inputPath = resolve(process.cwd(), positionalArgs[0]);
  const outputPath = resolve(process.cwd(), positionalArgs[1]);

  console.log("Storage Schema Generator");
  console.log("=".repeat(50));
  console.log(`Input:  ${inputPath}`);
  console.log(`Output: ${outputPath}`);

  try {
    const source = readFileSync(inputPath, "utf-8");
    const schema = parseSolidityFile(source);

    const structCount = Object.keys(schema.structs).length;
    if (structCount === 0) {
      console.log("\n⚠️  No structs found in the input file.");
      process.exit(1);
    }

    console.log(`\nFound ${structCount} struct(s):`);
    for (const [name, def] of Object.entries(schema.structs)) {
      const fieldCount = Object.keys(def.fields).length;
      const nsInfo = def.namespace ? ` (ns: ${def.namespace})` : "";
      console.log(`  - ${name}: ${fieldCount} fields${nsInfo}`);

      if (verbose) {
        for (const [fieldName, fieldDef] of Object.entries(def.fields)) {
          const offset = fieldDef.byteOffset !== undefined ? ` @byte ${fieldDef.byteOffset}` : "";
          console.log(`      slot ${fieldDef.slot}: ${fieldName} (${fieldDef.type})${offset}`);
        }
      }
    }

    // Ensure output directory exists
    const outputDir = dirname(outputPath);
    mkdirSync(outputDir, { recursive: true });

    // Write schema
    writeFileSync(outputPath, JSON.stringify(schema, null, 2) + "\n");
    console.log(`\n✓ Schema written to ${outputPath}`);
  } catch (error) {
    console.error("\nError:", error instanceof Error ? error.message : String(error));
    process.exit(2);
  }
}

main();
