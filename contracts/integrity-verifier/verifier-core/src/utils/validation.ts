/**
 * Contract Integrity Verifier - Input Validation Utilities
 *
 * Centralized input validation helpers for consistent null checks
 * and format validation across the codebase.
 */

// ============================================================================
// Constants for validation
// ============================================================================

/** Valid Ethereum address regex (40 hex chars with 0x prefix) */
const ADDRESS_REGEX = /^0x[a-fA-F0-9]{40}$/;

/** Valid hex string regex (with 0x prefix) */
const HEX_STRING_REGEX = /^0x[a-fA-F0-9]*$/;

/** Valid slot hex regex (64 hex chars with 0x prefix) */
const SLOT_REGEX = /^0x[a-fA-F0-9]{1,64}$/;

// ============================================================================
// Null/Undefined Checks
// ============================================================================

/**
 * Asserts that a value is not null or undefined.
 * Throws a descriptive error if the value is nullish.
 *
 * @param value - The value to check
 * @param name - Name of the parameter (for error messages)
 * @throws Error if value is null or undefined
 */
export function assertNonNullish<T>(value: T | null | undefined, name: string): asserts value is T {
  if (value === null || value === undefined) {
    throw new Error(`${name} is required but was ${value === null ? "null" : "undefined"}`);
  }
}

/**
 * Asserts that a string is not empty.
 *
 * @param value - The string to check
 * @param name - Name of the parameter (for error messages)
 * @throws Error if value is empty string
 */
export function assertNonEmpty(value: string, name: string): void {
  if (value === "") {
    throw new Error(`${name} cannot be empty`);
  }
}

// ============================================================================
// Format Validation
// ============================================================================

/**
 * Validates that a string is a valid Ethereum address.
 *
 * @param address - The address to validate
 * @returns true if valid
 */
export function isValidAddress(address: string): boolean {
  return ADDRESS_REGEX.test(address);
}

/**
 * Asserts that a string is a valid Ethereum address.
 *
 * @param address - The address to validate
 * @param name - Name of the parameter (for error messages)
 * @throws Error if address format is invalid
 */
export function assertValidAddress(address: string, name: string = "address"): void {
  if (!isValidAddress(address)) {
    throw new Error(`${name} must be a valid Ethereum address (0x + 40 hex chars), got: ${address}`);
  }
}

/**
 * Validates that a string is a valid hex string (with 0x prefix).
 *
 * @param hex - The hex string to validate
 * @returns true if valid
 */
export function isValidHexString(hex: string): boolean {
  return HEX_STRING_REGEX.test(hex);
}

/**
 * Asserts that a string is a valid hex string.
 *
 * @param hex - The hex string to validate
 * @param name - Name of the parameter (for error messages)
 * @throws Error if hex format is invalid
 */
export function assertValidHex(hex: string, name: string = "value"): void {
  if (!isValidHexString(hex)) {
    throw new Error(`${name} must be a valid hex string (0x prefix + hex chars), got: ${hex?.slice(0, 20)}...`);
  }
}

/**
 * Validates that a string is a valid storage slot.
 *
 * @param slot - The slot to validate
 * @returns true if valid
 */
export function isValidSlot(slot: string): boolean {
  return SLOT_REGEX.test(slot);
}

/**
 * Asserts that a string is a valid storage slot.
 *
 * @param slot - The slot to validate
 * @param name - Name of the parameter (for error messages)
 * @throws Error if slot format is invalid
 */
export function assertValidSlot(slot: string, name: string = "slot"): void {
  if (!isValidSlot(slot)) {
    throw new Error(`${name} must be a valid storage slot (0x + 1-64 hex chars), got: ${slot}`);
  }
}

// ============================================================================
// Type Guards
// ============================================================================

/**
 * Type guard for checking if a value is an object (not null).
 */
export function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Type guard for checking if a value is a non-empty array.
 */
export function isNonEmptyArray<T>(value: T[] | undefined | null): value is T[] {
  return Array.isArray(value) && value.length > 0;
}
