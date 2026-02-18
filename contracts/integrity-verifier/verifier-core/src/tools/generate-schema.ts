/**
 * Contract Integrity Verifier - Schema Generator
 *
 * Generates storage schema JSON from Solidity storage layout files.
 * Uses the CryptoAdapter pattern for framework-agnostic crypto operations.
 *
 * @packageDocumentation
 */

import { getSolidityTypeSize } from "../utils/hex";
import { calculateErc7201BaseSlot } from "../utils/storage";

import type { CryptoAdapter } from "../adapter";

// Re-export calculateErc7201BaseSlot from storage.ts to maintain public API
// Import for internal use as well
export { calculateErc7201BaseSlot };

// ============================================================================
// Types
// ============================================================================

export interface FieldDef {
  slot: number;
  type: string;
  byteOffset?: number;
}

export interface StructDef {
  namespace?: string;
  baseSlot?: string;
  fields: Record<string, FieldDef>;
}

export interface Schema {
  $comment?: string;
  structs: Record<string, StructDef>;
}

export interface SchemaGeneratorOptions {
  /** Optional comment to add to the schema */
  comment?: string;
  /** Whether to validate calculated baseSlots against explicit constants */
  validateConstants?: boolean;
  /** Whether to include explicit byteOffset: 0 for first field in packed slots (default: true) */
  includeExplicitZeroOffset?: boolean;
}

export interface ParseResult {
  schema: Schema;
  warnings: string[];
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Extract namespace ID from NatSpec comment.
 * Looks for: @custom:storage-location erc7201:linea.storage.SomeName
 */
function extractNamespace(comments: string): string | undefined {
  const match = comments.match(/@custom:storage-location\s+erc7201:([^\s*]+)/);
  return match ? match[1] : undefined;
}

/**
 * Parser context holding state for a single parsing operation.
 * This is passed through all parsing functions to avoid shared mutable state.
 */
interface ParserContext {
  /** Known struct names discovered during parsing */
  knownStructNames: Set<string>;
  /** Known enum names and their byte sizes */
  knownEnums: Map<string, number>;
  /** Parsed struct bodies for calculating nested struct sizes */
  structBodies: Map<string, string>;
}

/**
 * Create a new parser context for a parsing operation.
 * Each parsing operation should create its own context to avoid concurrency issues.
 */
function createParserContext(): ParserContext {
  return {
    knownStructNames: new Set(),
    knownEnums: new Map(),
    structBodies: new Map(),
  };
}

/**
 * Register a struct name as known within a parsing context.
 */
function registerStructName(ctx: ParserContext, name: string): void {
  ctx.knownStructNames.add(name);
}

interface StructDefinition {
  structName: string;
  structBody: string;
  structIndex: number;
}

/**
 * Parse struct definitions from source code.
 * Uses manual parsing to avoid ReDoS vulnerabilities.
 */
function parseStructDefinitions(source: string): StructDefinition[] {
  const results: StructDefinition[] = [];
  let pos = 0;

  while (pos < source.length) {
    const structIndex = source.indexOf("struct ", pos);
    if (structIndex === -1) break;

    // Skip if not at word boundary (check char before)
    if (structIndex > 0 && /\w/.test(source[structIndex - 1])) {
      pos = structIndex + 7;
      continue;
    }

    // Find the name (skip whitespace after "struct ")
    let nameStart = structIndex + 7;
    while (nameStart < source.length && /\s/.test(source[nameStart])) {
      nameStart++;
    }

    // Extract the name
    let nameEnd = nameStart;
    while (nameEnd < source.length && /\w/.test(source[nameEnd])) {
      nameEnd++;
    }

    if (nameEnd === nameStart) {
      pos = nameEnd;
      continue;
    }

    const structName = source.slice(nameStart, nameEnd);

    // Find opening brace (skip whitespace)
    let braceStart = nameEnd;
    while (braceStart < source.length && source[braceStart] !== "{") {
      if (!/\s/.test(source[braceStart])) {
        // Non-whitespace before brace - not a valid struct definition
        break;
      }
      braceStart++;
    }

    if (braceStart >= source.length || source[braceStart] !== "{") {
      pos = braceStart + 1;
      continue;
    }

    // Find closing brace (handle nested braces)
    let depth = 1;
    let braceEnd = braceStart + 1;
    while (braceEnd < source.length && depth > 0) {
      if (source[braceEnd] === "{") depth++;
      if (source[braceEnd] === "}") depth--;
      braceEnd++;
    }

    if (depth !== 0) {
      pos = braceStart + 1;
      continue;
    }

    const structBody = source.slice(braceStart + 1, braceEnd - 1);
    results.push({ structName, structBody, structIndex });

    pos = braceEnd;
  }

  return results;
}

/**
 * Discover all struct names in source code.
 * Reuses parseStructDefinitions to avoid code duplication.
 */
function discoverStructNames(ctx: ParserContext, source: string): void {
  const structDefs = parseStructDefinitions(source);
  for (const { structName, structBody } of structDefs) {
    registerStructName(ctx, structName);
    // Store struct body for calculating nested struct sizes
    ctx.structBodies.set(structName, structBody);
  }
}

/**
 * Calculate the byte size needed for an enum based on value count.
 * Solidity uses the smallest uint type that can hold all enum indices.
 *
 * Boundaries are based on max representable values:
 * - uint8:  2^8  = 256 values (indices 0-255)
 * - uint16: 2^16 = 65536 values (indices 0-65535)
 * - uint24: 2^24 = 16777216 values (indices 0-16777215)
 * - uint32: 2^32 = 4294967296 values (indices 0-4294967295)
 *
 * Note: Solidity rejects enums with more than 256 values in practice,
 * but we handle larger cases for completeness.
 */
function calculateEnumSize(valueCount: number): number {
  if (valueCount <= 256) return 1; // uint8: 0-255
  if (valueCount <= 65536) return 2; // uint16: 0-65535
  if (valueCount <= 16777216) return 3; // uint24: 0-16777215
  if (valueCount <= 4294967296) return 4; // uint32: 0-4294967295
  return 32; // fallback to uint256 for impossibly large enums
}

/**
 * Parse enum definitions from Solidity source and register them.
 * Uses manual parsing to avoid ReDoS vulnerabilities.
 */
function parseEnums(ctx: ParserContext, source: string): void {
  // Find enum definitions manually to avoid ReDoS
  let pos = 0;
  while (pos < source.length) {
    const enumIndex = source.indexOf("enum ", pos);
    if (enumIndex === -1) break;

    // Skip if not at word boundary (check char before)
    if (enumIndex > 0 && /\w/.test(source[enumIndex - 1])) {
      pos = enumIndex + 5;
      continue;
    }

    // Find the name (skip whitespace after "enum ")
    let nameStart = enumIndex + 5;
    while (nameStart < source.length && /\s/.test(source[nameStart])) {
      nameStart++;
    }

    // Extract the name
    let nameEnd = nameStart;
    while (nameEnd < source.length && /\w/.test(source[nameEnd])) {
      nameEnd++;
    }

    if (nameEnd === nameStart) {
      pos = nameEnd;
      continue;
    }

    const enumName = source.slice(nameStart, nameEnd);

    // Find opening brace
    let braceStart = nameEnd;
    while (braceStart < source.length && source[braceStart] !== "{") {
      if (!/\s/.test(source[braceStart])) {
        // Non-whitespace before brace - not a valid enum
        break;
      }
      braceStart++;
    }

    if (braceStart >= source.length || source[braceStart] !== "{") {
      pos = braceStart + 1;
      continue;
    }

    // Find closing brace
    const braceEnd = source.indexOf("}", braceStart);
    if (braceEnd === -1) {
      pos = braceStart + 1;
      continue;
    }

    const valuesStr = source.slice(braceStart + 1, braceEnd);
    const values = valuesStr.split(",").filter((v) => v.trim().length > 0);
    const valueCount = values.length;
    const byteSize = calculateEnumSize(valueCount);
    ctx.knownEnums.set(enumName, byteSize);

    pos = braceEnd + 1;
  }
}

/**
 * Parse a Solidity type and return normalized type string.
 * Converts enums to their equivalent uint type, preserves struct names.
 */
function normalizeType(ctx: ParserContext, solidityType: string): string {
  const trimmed = solidityType.trim();

  // Handle arrays first
  if (trimmed.endsWith("[]")) {
    const baseType = trimmed.slice(0, -2);
    return `${normalizeType(ctx, baseType)}[]`;
  }

  // Handle nested mappings: mapping(KeyType => mapping(...))
  // Need to handle balanced parentheses for nested mappings
  // Pre-normalize whitespace to avoid ReDoS from multiple whitespace quantifiers
  if (trimmed.startsWith("mapping")) {
    // Normalize whitespace to single spaces to prevent ReDoS
    const normalized = trimmed.replace(/\s+/g, " ");
    // Now use a simple pattern on normalized input
    const innerMatch = normalized.match(/^mapping ?\( ?([^=> ]+) ?=> ?(.+)\)$/);
    if (innerMatch) {
      const keyType = normalizeType(ctx, innerMatch[1]);
      const valueType = normalizeType(ctx, innerMatch[2].trim());
      return `mapping(${keyType} => ${valueType})`;
    }
  }

  // Check if it's a known struct type (preserve it)
  if (ctx.knownStructNames.has(trimmed)) {
    return trimmed;
  }

  // Check if it's a known enum type (convert to uintN)
  if (ctx.knownEnums.has(trimmed)) {
    const byteSize = ctx.knownEnums.get(trimmed)!;
    return `uint${byteSize * 8}`;
  }

  // Handle primitive types
  if (
    trimmed.startsWith("uint") ||
    trimmed.startsWith("int") ||
    trimmed.startsWith("bytes") ||
    trimmed === "address" ||
    trimmed === "bool" ||
    trimmed === "string"
  ) {
    return trimmed;
  }

  // PascalCase that's not a known struct or enum - could be interface or unknown type
  if (/^[A-Z][a-zA-Z0-9_]*$/.test(trimmed)) {
    // Check if it looks like an interface (starts with I followed by uppercase)
    if (/^I[A-Z]/.test(trimmed)) {
      return "address"; // Interfaces are addresses
    }
    // Unknown type - preserve the name but it will be treated as full slot
    return trimmed;
  }

  return trimmed;
}

/**
 * Get the byte size of a type (for packing calculation).
 * Delegates to shared getSolidityTypeSize utility.
 */
function getTypeSize(type: string): number {
  return getSolidityTypeSize(type);
}

/**
 * Calculate how many storage slots a struct occupies when stored inline.
 * Solidity stores structs inline with their fields consuming sequential slots.
 *
 * @param ctx - Parser context with known types and struct bodies
 * @param structName - Name of the struct to calculate
 * @param visited - Set of struct names being calculated (for cycle detection)
 * @returns Number of 32-byte slots the struct occupies
 */
function calculateStructStorageSlots(ctx: ParserContext, structName: string, visited: Set<string> = new Set()): number {
  // Cycle detection - if we've already started calculating this struct, assume 1 slot
  if (visited.has(structName)) {
    return 1;
  }

  const structBody = ctx.structBodies.get(structName);
  if (!structBody) {
    // Unknown struct, assume 1 slot (conservative fallback)
    return 1;
  }

  visited.add(structName);

  const lines = structBody.split("\n");
  let currentSlot = 0;
  let currentByteOffset = 0;

  for (const line of lines) {
    // Skip comments
    if (line.trim().startsWith("//") || line.trim().startsWith("*")) {
      continue;
    }

    // Check for mapping
    if (line.includes("mapping")) {
      // Mappings always start a new slot and take 1 slot
      if (currentByteOffset > 0) {
        currentSlot++;
        currentByteOffset = 0;
      }
      currentSlot++;
      continue;
    }

    // Try regular field
    const fieldMatch = line.match(/^\s*([A-Z]?[a-zA-Z0-9_]+(?:\[\])?)\s+\w+\s*;/);
    if (fieldMatch) {
      const [, typeStr] = fieldMatch;
      const normalizedType = normalizeType(ctx, typeStr);

      // Check if it's a dynamic array
      if (normalizedType.endsWith("[]")) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        currentSlot++;
        continue;
      }

      // Check if it's a nested struct
      if (ctx.knownStructNames.has(normalizedType)) {
        // Nested structs start at a new slot boundary
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        // Add the nested struct's slot count
        const nestedSlots = calculateStructStorageSlots(ctx, normalizedType, visited);
        currentSlot += nestedSlots;
        continue;
      }

      // Check if it's a mapping
      if (normalizedType.startsWith("mapping")) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        currentSlot++;
        continue;
      }

      // Primitive type - apply packing rules
      const typeSize = getTypeSize(normalizedType);

      if (currentByteOffset + typeSize > 32) {
        // Start new slot
        currentSlot++;
        currentByteOffset = 0;
      }

      currentByteOffset += typeSize;

      if (currentByteOffset >= 32) {
        currentSlot++;
        currentByteOffset = 0;
      }
    }
  }

  // Account for any remaining partial slot
  if (currentByteOffset > 0) {
    currentSlot++;
  }

  visited.delete(structName);

  // Minimum 1 slot for any struct
  return Math.max(1, currentSlot);
}

/**
 * Extract a mapping type from a line, handling nested mappings with balanced parentheses.
 */
function extractMappingType(line: string): { typeStr: string; name: string } | null {
  // Find "mapping(" at the start (after whitespace)
  const mappingStart = line.search(/\bmapping\s*\(/);
  if (mappingStart === -1) return null;

  // Count parentheses to find the end of the mapping type
  let depth = 0;
  let mappingEnd = -1;

  for (let i = mappingStart; i < line.length; i++) {
    if (line[i] === "(") depth++;
    if (line[i] === ")") {
      depth--;
      if (depth === 0) {
        mappingEnd = i + 1;
        break;
      }
    }
  }

  if (mappingEnd === -1) return null;

  const typeStr = line.slice(mappingStart, mappingEnd);
  const rest = line.slice(mappingEnd).trim();

  // Extract the variable name
  const nameMatch = rest.match(/^(\w+)\s*;/);
  if (!nameMatch) return null;

  return { typeStr, name: nameMatch[1] };
}

/**
 * Parse struct fields from Solidity source.
 * @param ctx - Parser context with known types
 * @param structBody - The content inside the struct braces
 * @param includeExplicitZeroOffset - Whether to include byteOffset: 0 for first packed field
 */
function parseStructFields(
  ctx: ParserContext,
  structBody: string,
  includeExplicitZeroOffset: boolean = true,
): Record<string, FieldDef> {
  const fields: Record<string, FieldDef> = {};
  let currentSlot = 0;
  let currentByteOffset = 0;
  let isSlotPacked = false; // Track if current slot has multiple fields

  const lines = structBody.split("\n");

  // First pass: determine which slots will have packed fields
  const slotFieldCounts: Map<number, number> = new Map();
  let tempSlot = 0;
  let tempOffset = 0;

  for (const line of lines) {
    if (line.trim().startsWith("//") || line.trim().startsWith("*")) continue;

    const mappingResult = extractMappingType(line);
    if (mappingResult) {
      if (tempOffset > 0) {
        tempSlot++;
        tempOffset = 0;
      }
      tempSlot++;
      continue;
    }

    const fieldMatch = line.match(/^\s*([A-Z]?[a-zA-Z0-9_]+(?:\[\])?)\s+(\w+)\s*;/);
    if (fieldMatch) {
      const [, typeStr] = fieldMatch;
      const normalizedType = normalizeType(ctx, typeStr);
      const typeSize = getTypeSize(normalizedType);

      // Dynamic arrays always take 1 slot (stores length, data at keccak256(slot))
      if (normalizedType.endsWith("[]")) {
        if (tempOffset > 0) {
          tempSlot++;
          tempOffset = 0;
        }
        tempSlot++;
        continue;
      }

      // Mappings always take 1 slot
      if (normalizedType.startsWith("mapping")) {
        if (tempOffset > 0) {
          tempSlot++;
          tempOffset = 0;
        }
        tempSlot++;
        continue;
      }

      // Nested structs are stored inline and take their calculated slot count
      if (ctx.knownStructNames.has(normalizedType)) {
        if (tempOffset > 0) {
          tempSlot++;
          tempOffset = 0;
        }
        const structSlots = calculateStructStorageSlots(ctx, normalizedType);
        tempSlot += structSlots;
        continue;
      }

      // Primitive types - apply packing rules
      if (tempOffset + typeSize > 32) {
        tempSlot++;
        tempOffset = 0;
      }

      slotFieldCounts.set(tempSlot, (slotFieldCounts.get(tempSlot) || 0) + 1);
      tempOffset += typeSize;

      if (tempOffset >= 32) {
        tempSlot++;
        tempOffset = 0;
      }
    }
  }

  // Second pass: actually build the fields
  for (const line of lines) {
    // Skip comments
    if (line.trim().startsWith("//") || line.trim().startsWith("*")) {
      continue;
    }

    // Try mapping first (with proper nested mapping support)
    const mappingResult = extractMappingType(line);
    if (mappingResult) {
      const { typeStr, name } = mappingResult;
      // Mappings always start a new slot
      if (currentByteOffset > 0) {
        currentSlot++;
        currentByteOffset = 0;
      }
      fields[name] = {
        slot: currentSlot,
        type: normalizeType(ctx, typeStr),
      };
      currentSlot++;
      isSlotPacked = false;
      continue;
    }

    // Try regular field (handles types like IMessageService, uint256, address[], etc.)
    const fieldMatch = line.match(/^\s*([A-Z]?[a-zA-Z0-9_]+(?:\[\])?)\s+(\w+)\s*;/);
    if (fieldMatch) {
      const [, typeStr, name] = fieldMatch;
      const normalizedType = normalizeType(ctx, typeStr);
      const typeSize = getTypeSize(normalizedType);

      // Dynamic arrays always start a new slot and take 1 slot
      if (normalizedType.endsWith("[]")) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        fields[name] = {
          slot: currentSlot,
          type: normalizedType,
        };
        currentSlot++;
        isSlotPacked = false;
        continue;
      }

      // Mappings always start a new slot and take 1 slot
      if (normalizedType.startsWith("mapping")) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        fields[name] = {
          slot: currentSlot,
          type: normalizedType,
        };
        currentSlot++;
        isSlotPacked = false;
        continue;
      }

      // Nested structs are stored inline - calculate their actual slot count
      if (ctx.knownStructNames.has(normalizedType)) {
        if (currentByteOffset > 0) {
          currentSlot++;
          currentByteOffset = 0;
        }
        fields[name] = {
          slot: currentSlot,
          type: normalizedType,
        };
        // Advance by the struct's actual slot count
        const structSlots = calculateStructStorageSlots(ctx, normalizedType);
        currentSlot += structSlots;
        isSlotPacked = false;
        continue;
      }

      // Primitive types - check if this field fits in the current slot
      if (currentByteOffset + typeSize > 32) {
        // Start new slot
        currentSlot++;
        currentByteOffset = 0;
      }

      // Determine if this slot is packed (has multiple fields)
      const slotFieldCount = slotFieldCounts.get(currentSlot) || 1;
      isSlotPacked = slotFieldCount > 1;

      // Include byteOffset if not at offset 0, or if slot is packed and includeExplicitZeroOffset is true
      const shouldIncludeOffset = currentByteOffset > 0 || (isSlotPacked && includeExplicitZeroOffset && typeSize < 32);

      fields[name] = {
        slot: currentSlot,
        type: normalizedType,
        ...(shouldIncludeOffset ? { byteOffset: currentByteOffset } : {}),
      };

      currentByteOffset += typeSize;

      // If we filled the slot exactly, move to next
      if (currentByteOffset >= 32) {
        currentSlot++;
        currentByteOffset = 0;
        isSlotPacked = false;
      }
    }
  }

  return fields;
}

/**
 * Extract preceding comments (both multi-line and single-line styles) for a struct.
 */
function extractPrecedingComments(source: string, structIndex: number): string {
  const beforeStruct = source.slice(0, structIndex);
  const lines = beforeStruct.split("\n");
  const commentLines: string[] = [];

  for (let i = lines.length - 1; i >= 0; i--) {
    const line = lines[i].trim();

    if (line === "") continue;

    if (line.startsWith("///")) {
      commentLines.unshift(line);
      continue;
    }

    if (line.endsWith("*/")) {
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

    break;
  }

  return commentLines.join("\n");
}

/**
 * Extract explicit storage location constants from source.
 * Returns a map of struct name -> explicit slot value.
 */
function extractExplicitConstants(source: string): Map<string, string> {
  const constants = new Map<string, string>();
  const constantPattern = /bytes32\s+(?:private\s+)?constant\s+(\w+StorageLocation)\s*=\s*(0x[a-fA-F0-9]+)/g;
  let match;
  while ((match = constantPattern.exec(source)) !== null) {
    const [, constantName, slotValue] = match;
    const structName = constantName.replace("StorageLocation", "");
    constants.set(structName, slotValue.toLowerCase());
  }
  return constants;
}

// ============================================================================
// Main Functions
// ============================================================================

/**
 * Parse a single Solidity file and extract storage schema.
 *
 * @param adapter - CryptoAdapter for hashing operations
 * @param source - Solidity source code
 * @param fileName - Optional filename for error messages
 * @param options - Parser options
 */
export function parseSoliditySource(
  adapter: CryptoAdapter,
  source: string,
  fileName?: string,
  options: SchemaGeneratorOptions = {},
): ParseResult {
  // Create a new context for this parsing session (thread-safe)
  const ctx = createParserContext();

  // First pass: discover all struct names in this file
  // Use manual parsing to avoid ReDoS
  discoverStructNames(ctx, source);

  // First pass: discover all enum definitions in this file
  parseEnums(ctx, source);

  // Parse the source
  return parseSoliditySourceInternal(ctx, adapter, source, fileName, options);
}

/**
 * Merge multiple schemas into one.
 * Later schemas override earlier ones for conflicting struct names.
 */
export function mergeSchemas(schemas: Schema[]): Schema {
  const merged: Schema = {
    $comment: "Auto-generated storage schema",
    structs: {},
  };

  for (const schema of schemas) {
    if (schema.$comment && schema.$comment !== "Auto-generated storage schema") {
      merged.$comment = schema.$comment;
    }
    for (const [name, def] of Object.entries(schema.structs)) {
      if (merged.structs[name]) {
        // Merge fields, prefer newer baseSlot/namespace
        merged.structs[name] = {
          ...merged.structs[name],
          ...def,
          fields: { ...merged.structs[name].fields, ...def.fields },
        };
      } else {
        merged.structs[name] = def;
      }
    }
  }

  return merged;
}

/**
 * Generate a storage schema from multiple Solidity sources.
 *
 * @param adapter - CryptoAdapter for hashing operations
 * @param sources - Array of { source, fileName } objects
 * @param options - Generator options
 */
export function generateSchema(
  adapter: CryptoAdapter,
  sources: Array<{ source: string; fileName?: string }>,
  options: SchemaGeneratorOptions = {},
): ParseResult {
  const allWarnings: string[] = [];
  const schemas: Schema[] = [];

  // Create a new context for this parsing session (thread-safe)
  const ctx = createParserContext();

  // First pass: discover all struct names across ALL sources
  // Uses manual parsing to avoid ReDoS
  for (const { source } of sources) {
    discoverStructNames(ctx, source);
  }

  // First pass: discover all enum definitions across ALL sources
  for (const { source } of sources) {
    parseEnums(ctx, source);
  }

  // Second pass: parse each source (without resetting type names)
  for (const { source, fileName } of sources) {
    const { schema, warnings } = parseSoliditySourceInternal(ctx, adapter, source, fileName, options);
    schemas.push(schema);
    allWarnings.push(...warnings);
  }

  const finalSchema = mergeSchemas(schemas);
  if (options.comment) {
    finalSchema.$comment = options.comment;
  }

  return { schema: finalSchema, warnings: allWarnings };
}

/**
 * Internal parsing function that uses an existing parser context.
 * Used by generateSchema for multi-file parsing.
 */
function parseSoliditySourceInternal(
  ctx: ParserContext,
  adapter: CryptoAdapter,
  source: string,
  fileName?: string,
  options: SchemaGeneratorOptions = {},
): ParseResult {
  const schema: Schema = {
    $comment: options.comment ?? "Auto-generated storage schema",
    structs: {},
  };
  const warnings: string[] = [];

  // Default to including explicit zero offset for consistency
  const includeExplicitZeroOffset = options.includeExplicitZeroOffset !== false;

  // Extract all explicit storage location constants
  const explicitConstants = extractExplicitConstants(source);

  // Parse struct definitions with field details using manual parsing to avoid ReDoS
  const structDefs = parseStructDefinitions(source);

  for (const { structName, structBody, structIndex } of structDefs) {
    // Extract comments that precede this struct
    const comments = extractPrecedingComments(source, structIndex);

    const namespace = extractNamespace(comments);
    const fields = parseStructFields(ctx, structBody, includeExplicitZeroOffset);

    const structDef: StructDef = { fields };

    if (namespace) {
      structDef.namespace = namespace;
      const calculatedSlot = calculateErc7201BaseSlot(adapter, namespace);
      structDef.baseSlot = calculatedSlot;

      // Validate against explicit constant if present
      const explicitSlot = explicitConstants.get(structName);
      if (explicitSlot && options.validateConstants !== false) {
        if (explicitSlot !== calculatedSlot) {
          const fileInfo = fileName ? ` in ${fileName}` : "";
          warnings.push(
            `Calculated baseSlot for ${structName} (${calculatedSlot}) does not match ` +
              `explicit constant${fileInfo} (${explicitSlot}). Using explicit value.`,
          );
          structDef.baseSlot = explicitSlot;
        }
      }
    } else {
      // No namespace annotation, but check for explicit constant
      const explicitSlot = explicitConstants.get(structName);
      if (explicitSlot) {
        structDef.baseSlot = explicitSlot;
      }
    }

    schema.structs[structName] = structDef;
  }

  return { schema, warnings };
}
