/**
 * Contract Integrity Verifier - Hex Utilities
 *
 * Shared utilities for hex string manipulation and Solidity type handling.
 * Browser-compatible with no external dependencies.
 */

/**
 * Converts a hex string to a Uint8Array.
 * Handles both "0x" prefixed and unprefixed hex strings.
 *
 * @param hex - Hex string to convert (with or without 0x prefix)
 * @returns Uint8Array of bytes
 */
export function hexToBytes(hex: string): Uint8Array {
  const normalized = hex.startsWith("0x") ? hex.slice(2) : hex;
  const bytes = new Uint8Array(normalized.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(normalized.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

/**
 * Normalizes a hex string by removing 0x prefix and converting to lowercase.
 *
 * @param hex - Hex string to normalize (with or without 0x prefix)
 * @returns Normalized hex string without prefix, in lowercase
 */
export function normalizeHex(hex: string): string {
  const value = hex.toLowerCase();
  return value.startsWith("0x") ? value.slice(2) : value;
}

/**
 * Checks if a string represents a decimal number.
 *
 * @param value - String to check
 * @returns true if the string is a valid decimal number
 */
export function isDecimalString(value: string): boolean {
  return /^-?\d+$/.test(value);
}

/**
 * Returns the byte size of a Solidity primitive type.
 * Supports all Solidity primitive types:
 * - uint8 to uint256 (in 8-bit increments)
 * - int8 to int256 (in 8-bit increments)
 * - bytes1 to bytes32
 * - address, bool
 *
 * @param type - Solidity type name
 * @returns Byte size of the type (defaults to 32 for unknown types)
 */
export function getSolidityTypeSize(type: string): number {
  // Fixed types
  if (type === "address") return 20;
  if (type === "bool") return 1;

  // uint<N> - extract bit size and convert to bytes
  const uintMatch = type.match(/^uint(\d+)$/);
  if (uintMatch) {
    const bits = parseInt(uintMatch[1], 10);
    if (bits >= 8 && bits <= 256 && bits % 8 === 0) {
      return bits / 8;
    }
  }

  // int<N> - extract bit size and convert to bytes
  const intMatch = type.match(/^int(\d+)$/);
  if (intMatch) {
    const bits = parseInt(intMatch[1], 10);
    if (bits >= 8 && bits <= 256 && bits % 8 === 0) {
      return bits / 8;
    }
  }

  // bytes<N> - extract byte size directly
  const bytesMatch = type.match(/^bytes(\d+)$/);
  if (bytesMatch) {
    const bytes = parseInt(bytesMatch[1], 10);
    if (bytes >= 1 && bytes <= 32) {
      return bytes;
    }
  }

  // Default to 32 bytes (full slot) for unknown types
  return 32;
}
