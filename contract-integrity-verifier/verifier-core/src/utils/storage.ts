/**
 * Contract Integrity Verifier - Storage Utilities
 *
 * Utilities for ERC-7201 slot computation and storage verification.
 * Requires Web3Adapter for hashing and RPC operations.
 */

import { readFileSync } from "fs";
import { resolve } from "path";

import {
  StorageSchema,
  StorageStructDef,
  StorageFieldDef,
  StoragePathConfig,
  StoragePathResult,
  SolidityType,
  SlotConfig,
  SlotResult,
  NamespaceConfig,
  NamespaceResult,
} from "../types";

import type { Web3Adapter } from "../adapter";

// ============================================================================
// ERC-7201 Slot Calculation
// ============================================================================

/**
 * Calculates the base storage slot for an ERC-7201 namespace.
 * Formula: keccak256(abi.encode(uint256(keccak256(id)) - 1)) & ~bytes32(uint256(0xff))
 *
 * @param adapter - Web3Adapter for hashing
 * @param namespaceId - Namespace identifier (e.g., "linea.storage.MyContract")
 */
export function calculateErc7201BaseSlot(adapter: Web3Adapter, namespaceId: string): string {
  // Step 1: keccak256(id)
  const idHash = adapter.keccak256(namespaceId);

  // Step 2: uint256(hash) - 1
  const hashBigInt = BigInt(idHash);
  const decremented = hashBigInt - 1n;

  // Step 3: keccak256(abi.encode(decremented))
  const encoded = adapter.encodeAbiParameters(["uint256"], [decremented]);
  const finalHash = adapter.keccak256(hexToBytes(encoded));

  // Step 4: Mask off the last byte (& ~0xff)
  const finalBigInt = BigInt(finalHash);
  const masked = finalBigInt & ~0xffn;

  return "0x" + masked.toString(16).padStart(64, "0");
}

// Re-export for backward compatibility
export const calculateErc7201Slot = calculateErc7201BaseSlot;

// ============================================================================
// Storage Slot Reading and Decoding
// ============================================================================

/**
 * Reads and decodes a storage slot value.
 */
export async function readStorageSlot(
  adapter: Web3Adapter,
  address: string,
  slot: string,
  type: string,
  offset: number = 0,
): Promise<unknown> {
  const rawValue = await adapter.getStorageAt(address, slot);
  return decodeSlotValue(adapter, rawValue, type, offset);
}

/**
 * Decodes a raw storage slot value based on type and offset.
 */
export function decodeSlotValue(adapter: Web3Adapter, rawValue: string, type: string, offset: number = 0): unknown {
  // Remove 0x prefix and ensure 64 chars (32 bytes)
  const normalized = rawValue.slice(2).padStart(64, "0");

  // For packed storage, extract the relevant bytes
  const typeBytes = getTypeBytes(type);
  const startByte = 32 - offset - typeBytes;
  const endByte = 32 - offset;
  const hexValue = normalized.slice(startByte * 2, endByte * 2);

  switch (type) {
    case "address":
      return adapter.checksumAddress("0x" + hexValue.slice(-40));
    case "bool":
      return hexValue !== "0".repeat(hexValue.length);
    case "uint8":
    case "uint16":
    case "uint32":
    case "uint64":
    case "uint96":
    case "uint128":
    case "uint256":
      return BigInt("0x" + hexValue).toString();
    case "int8":
    case "int16":
    case "int32":
    case "int64":
    case "int96":
    case "int128":
    case "int256":
      return decodeSignedInt(hexValue, getTypeBytes(type));
    case "bytes32":
      return "0x" + hexValue;
    default:
      return "0x" + hexValue;
  }
}

/**
 * Decodes a signed integer from hex using two's complement.
 */
function decodeSignedInt(hexValue: string, byteSize: number): string {
  const value = BigInt("0x" + hexValue);
  const bitSize = BigInt(byteSize * 8);
  const maxPositive = (1n << (bitSize - 1n)) - 1n;

  // If the value is greater than the max positive value, it's negative
  if (value > maxPositive) {
    // Two's complement: subtract 2^bitSize
    const negativeValue = value - (1n << bitSize);
    return negativeValue.toString();
  }
  return value.toString();
}

/**
 * Returns the byte size of a Solidity type.
 */
function getTypeBytes(type: string): number {
  switch (type) {
    case "address":
      return 20;
    case "bool":
    case "uint8":
    case "int8":
      return 1;
    case "uint16":
    case "int16":
      return 2;
    case "uint32":
    case "int32":
      return 4;
    case "uint64":
    case "int64":
      return 8;
    case "uint96":
    case "int96":
      return 12;
    case "uint128":
    case "int128":
      return 16;
    case "uint256":
    case "int256":
    case "bytes32":
      return 32;
    default:
      return 32;
  }
}

// ============================================================================
// Slot Verification
// ============================================================================

/**
 * Verifies a single storage slot.
 */
export async function verifySlot(adapter: Web3Adapter, address: string, config: SlotConfig): Promise<SlotResult> {
  try {
    const actual = await readStorageSlot(adapter, address, config.slot, config.type, config.offset ?? 0);
    const pass = compareValues(actual, config.expected, "eq");

    return {
      slot: config.slot,
      name: config.name,
      expected: config.expected,
      actual,
      status: pass ? "pass" : "fail",
      message: pass
        ? `${config.name} = ${formatForDisplay(actual)}`
        : `${config.name}: expected ${formatForDisplay(config.expected)}, got ${formatForDisplay(actual)}`,
    };
  } catch (error) {
    return {
      slot: config.slot,
      name: config.name,
      expected: config.expected,
      actual: undefined,
      status: "fail",
      message: `Failed to read slot: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
}

/**
 * Verifies variables within an ERC-7201 namespace.
 */
export async function verifyNamespace(
  adapter: Web3Adapter,
  address: string,
  config: NamespaceConfig,
): Promise<NamespaceResult> {
  const baseSlot = calculateErc7201BaseSlot(adapter, config.id);
  const baseSlotBigInt = BigInt(baseSlot);

  const variableResults: SlotResult[] = [];

  for (const variable of config.variables) {
    const actualSlotBigInt = baseSlotBigInt + BigInt(variable.offset);
    const actualSlot = "0x" + actualSlotBigInt.toString(16).padStart(64, "0");

    const result = await verifySlot(adapter, address, {
      slot: actualSlot,
      type: variable.type,
      name: variable.name,
      expected: variable.expected,
    });

    variableResults.push(result);
  }

  const allPass = variableResults.every((r) => r.status === "pass");

  return {
    namespaceId: config.id,
    baseSlot,
    variables: variableResults,
    status: allPass ? "pass" : "fail",
  };
}

// ============================================================================
// Schema Loading and Path Computation
// ============================================================================

/**
 * Validates a storage schema structure.
 */
function validateStorageSchema(schema: unknown, path: string): asserts schema is StorageSchema {
  if (!schema || typeof schema !== "object") {
    throw new Error(`Invalid schema at ${path}: expected object`);
  }

  const obj = schema as Record<string, unknown>;
  if (!obj.structs || typeof obj.structs !== "object") {
    throw new Error(`Invalid schema at ${path}: missing 'structs' object`);
  }

  for (const [structName, structDef] of Object.entries(obj.structs)) {
    if (!structDef || typeof structDef !== "object") {
      throw new Error(`Invalid schema at ${path}: struct '${structName}' must be an object`);
    }

    const struct = structDef as Record<string, unknown>;
    if (!struct.fields || typeof struct.fields !== "object") {
      throw new Error(`Invalid schema at ${path}: struct '${structName}' missing 'fields' object`);
    }

    for (const [fieldName, fieldDef] of Object.entries(struct.fields as Record<string, unknown>)) {
      if (!fieldDef || typeof fieldDef !== "object") {
        throw new Error(`Invalid schema at ${path}: field '${structName}.${fieldName}' must be an object`);
      }

      const field = fieldDef as Record<string, unknown>;
      if (typeof field.slot !== "number") {
        throw new Error(`Invalid schema at ${path}: field '${structName}.${fieldName}' missing numeric 'slot'`);
      }
      if (typeof field.type !== "string") {
        throw new Error(`Invalid schema at ${path}: field '${structName}.${fieldName}' missing string 'type'`);
      }
    }
  }
}

/**
 * Loads a storage schema from a JSON file.
 */
export function loadStorageSchema(schemaPath: string, configDir: string): StorageSchema {
  const resolvedPath = resolve(configDir, schemaPath);
  let content: string;
  try {
    content = readFileSync(resolvedPath, "utf-8");
  } catch (err) {
    throw new Error(
      `Failed to read schema file at ${resolvedPath}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(content);
  } catch (err) {
    throw new Error(
      `Failed to parse schema JSON at ${resolvedPath}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  validateStorageSchema(parsed, resolvedPath);
  return parsed;
}

// ============================================================================
// Path Parsing
// ============================================================================

interface ParsedPath {
  structName: string;
  segments: PathSegment[];
}

type PathSegment =
  | { type: "field"; name: string }
  | { type: "arrayIndex"; index: number }
  | { type: "arrayLength" }
  | { type: "mappingKey"; key: string };

/**
 * Parses a storage path into components.
 * Format: "StructName:field.subfield[key].nested"
 */
export function parsePath(path: string): ParsedPath {
  if (!path || typeof path !== "string") {
    throw new Error("Invalid path: path must be a non-empty string");
  }

  const trimmedPath = path.trim();
  if (trimmedPath.length === 0) {
    throw new Error("Invalid path: path cannot be empty or whitespace");
  }

  const colonIndex = trimmedPath.indexOf(":");
  if (colonIndex === -1) {
    throw new Error(`Invalid path format: ${path}. Expected "StructName:path.to.field"`);
  }

  if (colonIndex === 0) {
    throw new Error(`Invalid path format: ${path}. Struct name cannot be empty`);
  }

  const structName = trimmedPath.slice(0, colonIndex);
  const pathPart = trimmedPath.slice(colonIndex + 1);

  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(structName)) {
    throw new Error(`Invalid struct name: ${structName}. Must be a valid identifier`);
  }

  if (pathPart.length === 0) {
    throw new Error(`Invalid path format: ${path}. Field path cannot be empty`);
  }

  const segments: PathSegment[] = [];
  let current = "";
  let i = 0;

  while (i < pathPart.length) {
    const char = pathPart[i];

    if (char === ".") {
      if (current) {
        segments.push({ type: "field", name: current });
        current = "";
      }
      i++;
    } else if (char === "[") {
      if (current) {
        segments.push({ type: "field", name: current });
        current = "";
      }

      const closeIndex = pathPart.indexOf("]", i);
      if (closeIndex === -1) {
        throw new Error(`Unclosed bracket in path: ${path}`);
      }

      const bracketContent = pathPart.slice(i + 1, closeIndex);

      if (bracketContent === "length") {
        segments.push({ type: "arrayLength" });
      } else if (/^\d+$/.test(bracketContent)) {
        segments.push({ type: "arrayIndex", index: parseInt(bracketContent, 10) });
      } else {
        segments.push({ type: "mappingKey", key: bracketContent });
      }

      i = closeIndex + 1;
    } else {
      current += char;
      i++;
    }
  }

  if (current) {
    if (current === "length") {
      segments.push({ type: "arrayLength" });
    } else {
      segments.push({ type: "field", name: current });
    }
  }

  return { structName, segments };
}

// ============================================================================
// Slot Computation
// ============================================================================

interface ComputedSlot {
  slot: string;
  type: SolidityType;
  byteOffset: number;
}

/**
 * Computes the storage slot for a parsed path.
 */
export function computeSlot(adapter: Web3Adapter, parsedPath: ParsedPath, schema: StorageSchema): ComputedSlot {
  const structDef = schema.structs[parsedPath.structName];
  if (!structDef) {
    throw new Error(`Unknown struct: ${parsedPath.structName}`);
  }

  let baseSlot: bigint;
  if (structDef.baseSlot) {
    baseSlot = BigInt(structDef.baseSlot);
  } else if (structDef.namespace) {
    baseSlot = BigInt(calculateErc7201BaseSlot(adapter, structDef.namespace));
  } else {
    throw new Error(`Struct ${parsedPath.structName} has no baseSlot or namespace`);
  }

  let currentSlot = baseSlot;
  let currentType: SolidityType = "uint256";
  let currentByteOffset = 0;
  let currentStructDef: StorageStructDef | undefined = structDef;

  for (const segment of parsedPath.segments) {
    if (segment.type === "field") {
      if (!currentStructDef) {
        throw new Error(`Cannot access field ${segment.name} on non-struct type`);
      }

      const field: StorageFieldDef | undefined = currentStructDef.fields[segment.name];
      if (!field) {
        throw new Error(`Unknown field: ${segment.name} in struct`);
      }

      currentSlot = currentSlot + BigInt(field.slot);
      currentType = field.type;
      currentByteOffset = field.byteOffset ?? 0;

      const fieldType: SolidityType = field.type;
      if (fieldType.startsWith("mapping(")) {
        // Keep currentStructDef for mapping value lookup
      } else if (!isPrimitiveType(fieldType)) {
        currentStructDef = schema.structs[fieldType];
      } else {
        currentStructDef = undefined;
      }
    } else if (segment.type === "arrayLength") {
      currentType = "uint256";
      currentByteOffset = 0;
    } else if (segment.type === "arrayIndex") {
      const dataSlot = adapter.keccak256(hexToBytes(adapter.encodeAbiParameters(["uint256"], [currentSlot])));
      currentSlot = BigInt(dataSlot) + BigInt(segment.index);
      currentType = extractArrayElementType(currentType);
      currentByteOffset = 0;
    } else if (segment.type === "mappingKey") {
      const keyType = extractMappingKeyType(currentType);
      const valueType = extractMappingValueType(currentType);

      const encodedKey = encodeKey(segment.key, keyType);
      const combined = adapter.encodeAbiParameters([keyType, "uint256"], [encodedKey, currentSlot]);
      const mappingSlot = adapter.keccak256(hexToBytes(combined));

      currentSlot = BigInt(mappingSlot);
      currentType = valueType as SolidityType;
      currentByteOffset = 0;

      if (!isPrimitiveType(valueType)) {
        currentStructDef = schema.structs[valueType];
      }
    }
  }

  return {
    slot: "0x" + currentSlot.toString(16).padStart(64, "0"),
    type: currentType,
    byteOffset: currentByteOffset,
  };
}

function isPrimitiveType(type: string): boolean {
  return (
    type.startsWith("uint") ||
    type.startsWith("int") ||
    type.startsWith("bytes") ||
    type === "address" ||
    type === "bool"
  );
}

function extractArrayElementType(arrayType: SolidityType): SolidityType {
  if (arrayType.endsWith("[]")) {
    return arrayType.slice(0, -2) as SolidityType;
  }
  return "uint256";
}

function extractMappingKeyType(mappingType: SolidityType): string {
  const match = mappingType.match(/^mapping\((\w+)\s*=>/);
  return match ? match[1] : "address";
}

function extractMappingValueType(mappingType: SolidityType): string {
  // Use string operations instead of regex to avoid ReDoS
  // Format: mapping(keyType => valueType)
  const arrowIndex = mappingType.lastIndexOf("=>");
  if (arrowIndex === -1) return "uint256";

  const afterArrow = mappingType.slice(arrowIndex + 2);
  const closingParen = afterArrow.lastIndexOf(")");
  if (closingParen === -1) return "uint256";

  return afterArrow.slice(0, closingParen).trim() || "uint256";
}

function encodeKey(key: string, keyType: string): unknown {
  if (keyType === "address") {
    if (!/^0x[a-fA-F0-9]{40}$/.test(key)) {
      throw new Error(`Invalid address key: ${key}. Expected 0x followed by 40 hex characters.`);
    }
    return key;
  }
  if (keyType.startsWith("uint") || keyType.startsWith("int")) {
    if (!/^-?\d+$/.test(key) && !/^0x[a-fA-F0-9]+$/.test(key)) {
      throw new Error(`Invalid numeric key: ${key}. Expected decimal or hex number.`);
    }
    try {
      return BigInt(key);
    } catch {
      throw new Error(`Failed to convert key to BigInt: ${key}`);
    }
  }
  if (keyType === "bytes32") {
    if (!/^0x[a-fA-F0-9]{64}$/.test(key)) {
      throw new Error(`Invalid bytes32 key: ${key}. Expected 0x followed by 64 hex characters.`);
    }
    return key;
  }
  return key;
}

// ============================================================================
// Storage Path Verification
// ============================================================================

/**
 * Verifies a storage path against expected value.
 */
export async function verifyStoragePath(
  adapter: Web3Adapter,
  address: string,
  config: StoragePathConfig,
  schema: StorageSchema,
): Promise<StoragePathResult> {
  try {
    const parsed = parsePath(config.path);
    const computed = computeSlot(adapter, parsed, schema);
    const actual = await readStorageAtPath(adapter, address, computed);

    const pass = compareValues(actual, config.expected, config.comparison ?? "eq");

    return {
      path: config.path,
      computedSlot: computed.slot,
      type: computed.type,
      expected: config.expected,
      actual,
      status: pass ? "pass" : "fail",
      message: pass
        ? `${config.path} = ${formatValue(actual)}`
        : `${config.path}: expected ${formatValue(config.expected)}, got ${formatValue(actual)}`,
    };
  } catch (error) {
    return {
      path: config.path,
      computedSlot: "error",
      type: "unknown",
      expected: config.expected,
      actual: undefined,
      status: "fail",
      message: `Error: ${error instanceof Error ? error.message : String(error)}`,
    };
  }
}

/**
 * Reads and decodes a storage value at a computed slot.
 */
async function readStorageAtPath(adapter: Web3Adapter, address: string, computedSlot: ComputedSlot): Promise<unknown> {
  const rawValue = await adapter.getStorageAt(address, computedSlot.slot);
  return decodeStorageValue(adapter, rawValue, computedSlot.type, computedSlot.byteOffset);
}

function decodeStorageValue(adapter: Web3Adapter, rawValue: string, type: SolidityType, byteOffset: number): unknown {
  const normalized = rawValue.slice(2).padStart(64, "0");
  const typeBytes = getTypeBytesForSolidityType(type);

  const startByte = 32 - byteOffset - typeBytes;
  const endByte = 32 - byteOffset;
  const hexValue = normalized.slice(startByte * 2, endByte * 2);

  if (type === "address") {
    return adapter.checksumAddress("0x" + hexValue.slice(-40));
  }
  if (type === "bool") {
    return hexValue !== "0".repeat(hexValue.length);
  }
  if (type.startsWith("uint")) {
    return BigInt("0x" + hexValue).toString();
  }
  if (type.startsWith("int")) {
    return decodeSignedInt(hexValue, typeBytes);
  }
  if (type.startsWith("bytes")) {
    return "0x" + hexValue;
  }
  return "0x" + hexValue;
}

function getTypeBytesForSolidityType(type: SolidityType): number {
  if (type === "address") return 20;
  if (type === "bool") return 1;
  if (type === "uint8" || type === "int8") return 1;
  if (type === "uint16" || type === "int16") return 2;
  if (type === "uint32" || type === "int32") return 4;
  if (type === "uint64" || type === "int64") return 8;
  if (type === "uint96" || type === "int96") return 12;
  if (type === "uint128" || type === "int128") return 16;
  if (type === "uint256" || type === "int256") return 32;
  if (type === "bytes32") return 32;
  if (type === "bytes4") return 4;
  return 32;
}

// ============================================================================
// Helper Functions
// ============================================================================

function hexToBytes(hex: string): Uint8Array {
  const normalized = hex.startsWith("0x") ? hex.slice(2) : hex;
  const bytes = new Uint8Array(normalized.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(normalized.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

function isNumericString(value: string): boolean {
  return /^-?\d+$/.test(value) || /^0x[0-9a-fA-F]+$/.test(value);
}

function compareValues(actual: unknown, expected: unknown, comparison: string): boolean {
  const normalizedActual = normalizeValue(actual);
  const normalizedExpected = normalizeValue(expected);

  switch (comparison) {
    case "eq":
      return normalizedActual === normalizedExpected;
    case "gt":
    case "gte":
    case "lt":
    case "lte": {
      if (!isNumericString(normalizedActual) || !isNumericString(normalizedExpected)) {
        return normalizedActual === normalizedExpected;
      }
      const actualBigInt = BigInt(normalizedActual);
      const expectedBigInt = BigInt(normalizedExpected);
      if (comparison === "gt") return actualBigInt > expectedBigInt;
      if (comparison === "gte") return actualBigInt >= expectedBigInt;
      if (comparison === "lt") return actualBigInt < expectedBigInt;
      return actualBigInt <= expectedBigInt;
    }
    default:
      return normalizedActual === normalizedExpected;
  }
}

function normalizeValue(value: unknown): string {
  if (typeof value === "string") {
    if (value.startsWith("0x") && value.length === 42) {
      return value.toLowerCase();
    }
    return value;
  }
  return String(value);
}

function formatValue(value: unknown): string {
  if (typeof value === "string" && value.length > 20) {
    return value.slice(0, 10) + "..." + value.slice(-6);
  }
  return String(value);
}

function formatForDisplay(value: unknown): string {
  if (typeof value === "string" && value.length > 20) {
    return value.slice(0, 10) + "..." + value.slice(-8);
  }
  return String(value);
}
