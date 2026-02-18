/**
 * Contract Integrity Verifier - Comparison Utilities
 *
 * Shared utilities for value comparison and formatting.
 * Used by both verifier.ts and storage.ts.
 */

import { DEFAULT_MAX_DISPLAY_LENGTH } from "../constants";

/**
 * Serializes a value to a JSON string, converting bigint values to strings.
 * This is a browser-compatible implementation that avoids importing Node.js dependencies.
 */
function serialize(value: unknown): string {
  return JSON.stringify(value, (_, v: unknown) => (typeof v === "bigint" ? v.toString() : v));
}

// ============================================================================
// Value Formatting
// ============================================================================

/**
 * Recursively formats values for storage and display.
 * Converts BigInts to strings and handles nested arrays/objects (tuples/structs).
 */
export function formatValue(value: unknown): unknown {
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (Array.isArray(value)) {
    return value.map(formatValue);
  }
  if (value !== null && typeof value === "object") {
    // Handle struct-like objects (named fields from ABI decoding)
    const formatted: Record<string, unknown> = {};
    for (const [key, val] of Object.entries(value)) {
      formatted[key] = formatValue(val);
    }
    return formatted;
  }
  return value;
}

/**
 * Formats a value for human-readable display.
 * Truncates long strings and serializes arrays/objects.
 * Uses BigInt-safe serialization.
 */
export function formatForDisplay(value: unknown, maxLength: number = DEFAULT_MAX_DISPLAY_LENGTH): string {
  if (typeof value === "string" && value.length > maxLength) {
    // Truncate long strings: show first half and last third
    const headLength = Math.ceil(maxLength / 2);
    const tailLength = Math.floor(maxLength * 0.4);
    return value.slice(0, headLength) + "..." + value.slice(-tailLength);
  }
  if (Array.isArray(value) || (value !== null && typeof value === "object")) {
    const json = serialize(value);
    // Truncate long JSON: 50 char threshold, show 25 head + 20 tail
    const jsonMaxLength = maxLength * 2.5;
    if (json.length > jsonMaxLength) {
      const jsonHeadLength = Math.ceil(jsonMaxLength / 2);
      const jsonTailLength = Math.floor(jsonMaxLength * 0.4);
      return json.slice(0, jsonHeadLength) + "..." + json.slice(-jsonTailLength);
    }
    return json;
  }
  return String(value);
}

// ============================================================================
// Value Normalization
// ============================================================================

/**
 * Checks if a string represents a numeric value (decimal or hex).
 */
export function isNumericString(value: string): boolean {
  return /^-?\d+$/.test(value) || /^0x[0-9a-fA-F]+$/.test(value);
}

/**
 * Normalizes a primitive value for string comparison.
 * For arrays/objects, returns a canonical JSON string.
 * Uses BigInt-safe serialization.
 */
export function normalizeForComparison(value: unknown): string {
  if (typeof value === "string") {
    // Normalize addresses to lowercase
    if (value.startsWith("0x") && value.length === 42) {
      return value.toLowerCase();
    }
    return value;
  }
  if (typeof value === "bigint" || typeof value === "number") {
    return String(value);
  }
  if (typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    // Recursively normalize array elements (tuples)
    return serialize(value.map(normalizeArrayElement));
  }
  if (value !== null && typeof value === "object") {
    // Recursively normalize object properties (structs)
    return serialize(normalizeObjectForComparison(value as Record<string, unknown>));
  }
  return String(value);
}

/**
 * Normalizes an array element for comparison.
 */
function normalizeArrayElement(value: unknown): unknown {
  if (typeof value === "string") {
    if (value.startsWith("0x") && value.length === 42) {
      return value.toLowerCase();
    }
    return value;
  }
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (Array.isArray(value)) {
    return value.map(normalizeArrayElement);
  }
  if (value !== null && typeof value === "object") {
    return normalizeObjectForComparison(value as Record<string, unknown>);
  }
  return value;
}

/**
 * Normalizes an object (struct) for comparison.
 * Handles nested structures and normalizes all values.
 */
function normalizeObjectForComparison(obj: Record<string, unknown>): Record<string, unknown> {
  const normalized: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(obj)) {
    normalized[key] = normalizeArrayElement(value);
  }
  return normalized;
}

// ============================================================================
// Value Comparison
// ============================================================================

/**
 * Supported comparison operators.
 */
export type ComparisonOperator = "eq" | "gt" | "gte" | "lt" | "lte" | "contains";

/**
 * Compares two values using the specified comparison operator.
 * Handles primitives, arrays (tuples), and objects (structs).
 *
 * @param actual - The actual value from the chain
 * @param expected - The expected value from config
 * @param comparison - The comparison operator (default: "eq")
 * @returns true if the comparison passes
 */
export function compareValues(
  actual: unknown,
  expected: unknown,
  comparison: ComparisonOperator | string = "eq",
): boolean {
  // For deep equality, use recursive comparison
  if (comparison === "eq") {
    return deepEqual(actual, expected);
  }

  // For other comparisons, normalize to strings
  const normalizedActual = normalizeForComparison(actual);
  const normalizedExpected = normalizeForComparison(expected);

  switch (comparison) {
    case "gt":
    case "gte":
    case "lt":
    case "lte": {
      // For numeric comparisons, both values must be numeric
      if (!isNumericString(normalizedActual) || !isNumericString(normalizedExpected)) {
        // Fall back to string equality for non-numeric values
        return normalizedActual === normalizedExpected;
      }
      const actualBigInt = BigInt(normalizedActual);
      const expectedBigInt = BigInt(normalizedExpected);

      if (comparison === "gt") return actualBigInt > expectedBigInt;
      if (comparison === "gte") return actualBigInt >= expectedBigInt;
      if (comparison === "lt") return actualBigInt < expectedBigInt;
      return actualBigInt <= expectedBigInt; // lte
    }

    case "contains":
      return String(normalizedActual).includes(String(normalizedExpected));

    default:
      // Unknown comparison type, fall back to equality
      return normalizedActual === normalizedExpected;
  }
}

/**
 * Deep equality comparison for values including arrays (tuples) and objects (structs).
 * Normalizes addresses to lowercase and BigInts to strings.
 */
export function deepEqual(a: unknown, b: unknown): boolean {
  // Handle null/undefined
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (a === undefined || b === undefined) return false;

  // Handle primitives
  if (typeof a !== typeof b) {
    // Allow BigInt/string comparison
    if ((typeof a === "bigint" && typeof b === "string") || (typeof a === "string" && typeof b === "bigint")) {
      return String(a) === String(b);
    }
    // Allow number/string comparison
    if ((typeof a === "number" && typeof b === "string") || (typeof a === "string" && typeof b === "number")) {
      return String(a) === String(b);
    }
    return false;
  }

  // Handle strings (normalize addresses)
  if (typeof a === "string" && typeof b === "string") {
    const normalizedA = a.startsWith("0x") && a.length === 42 ? a.toLowerCase() : a;
    const normalizedB = b.startsWith("0x") && b.length === 42 ? b.toLowerCase() : b;
    return normalizedA === normalizedB;
  }

  // Handle BigInt
  if (typeof a === "bigint" && typeof b === "bigint") {
    return a === b;
  }

  // Handle arrays (tuples)
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    return a.every((val, idx) => deepEqual(val, b[idx]));
  }

  // Handle objects (structs)
  if (typeof a === "object" && typeof b === "object") {
    const aObj = a as Record<string, unknown>;
    const bObj = b as Record<string, unknown>;

    const aKeys = Object.keys(aObj).filter((k) => !/^\d+$/.test(k)); // Skip numeric keys (tuple indices)
    const bKeys = Object.keys(bObj).filter((k) => !/^\d+$/.test(k));

    // If both have only numeric keys, compare as arrays
    if (aKeys.length === 0 && bKeys.length === 0) {
      const aVals = Object.values(aObj);
      const bVals = Object.values(bObj);
      if (aVals.length !== bVals.length) return false;
      return aVals.every((val, idx) => deepEqual(val, bVals[idx]));
    }

    // Compare named fields
    if (aKeys.length !== bKeys.length) return false;
    return aKeys.every((key) => key in bObj && deepEqual(aObj[key], bObj[key]));
  }

  // Default comparison
  return a === b;
}
