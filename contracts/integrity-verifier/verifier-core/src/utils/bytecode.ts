/**
 * Contract Integrity Verifier - Bytecode Utilities
 *
 * Utilities for comparing deployed bytecode.
 * Handles CBOR metadata stripping and immutable detection for accurate comparisons.
 *
 * Note: This module is pure TypeScript with no web3 dependencies.
 */

import {
  CBOR_METADATA_MIN_LENGTH,
  CBOR_METADATA_MAX_LENGTH,
  CBOR_METADATA_ABSOLUTE_MAX,
  MIN_BYTECODE_HEX_LENGTH,
  MIN_BYTECODE_AFTER_STRIP,
  METADATA_LENGTH_HEX_CHARS,
  HEX_CHARS_PER_BYTE,
  HEX_CHARS_PER_STORAGE_SLOT,
  BYTES_PER_STORAGE_SLOT,
  ADDRESS_HEX_CHARS,
  ADDRESS_BYTES,
  PUSH4_OPCODE,
  NULL_SELECTOR,
  MAX_SELECTOR,
  SELECTOR_HEX_CHARS,
  COMMON_IMMUTABLE_SIZES,
  MAX_REGIONS_FOR_IMMUTABLE_CHECK,
  MAX_REGIONS_FOR_IMMUTABLE_CHECK_STRICT,
  MAX_DIFFERENCES_TO_RETURN,
  MAX_DIFF_POSITIONS,
  MIN_FRAGMENT_LENGTH,
} from "../constants";
import {
  BytecodeComparisonResult,
  BytecodeDifference,
  ImmutableDifference,
  ImmutableReference,
  ImmutableValuesResult,
  ImmutableValueResult,
  DefinitiveBytecodeResult,
  GroupedImmutableDifference,
} from "../types";
import { isDecimalString, normalizeHex } from "./hex";

/**
 * Strips CBOR-encoded metadata from bytecode.
 *
 * Solidity appends metadata hash at the end of bytecode in CBOR format.
 * The metadata starts with `a264` (2-item map) or `a165` (1-item map) and ends
 * with a 2-byte length indicator.
 *
 * Format: ...contract_code...a264ipfs...solc...0033
 * The last 2 bytes (0033 = 51 in decimal) indicate metadata length.
 *
 * Validation strategy:
 * 1. Parse the trailing length indicator
 * 2. Calculate where the CBOR marker should be
 * 3. Verify the CBOR marker is valid (a1/a2 prefix)
 * 4. Apply length bounds as a sanity check (not primary validation)
 *
 * This approach allows valid metadata outside typical bounds to be stripped
 * as long as the CBOR structure is valid.
 */
export function stripCborMetadata(bytecode: string): string {
  // Normalize bytecode (remove 0x prefix, lowercase)
  const normalized = normalizeHex(bytecode);

  // Minimum viable bytecode with metadata: at least 4 chars for length + some content
  if (normalized.length < MIN_BYTECODE_HEX_LENGTH) {
    return normalized;
  }

  // Get the last 2 bytes (4 hex chars) which indicate metadata length
  const lengthHex = normalized.slice(-METADATA_LENGTH_HEX_CHARS);

  // Validate length hex is valid hexadecimal
  if (!/^[0-9a-f]{4}$/.test(lengthHex)) {
    return normalized;
  }

  const metadataLength = parseInt(lengthHex, 16);

  // Absolute bounds check - metadata cannot be absurdly large or tiny
  // This prevents catastrophic stripping or processing invalid data
  if (metadataLength < CBOR_METADATA_MIN_LENGTH || metadataLength > CBOR_METADATA_ABSOLUTE_MAX) {
    return normalized;
  }

  // Metadata length is in bytes, each byte is 2 hex chars
  // Total to strip: metadata + 2 bytes for length indicator
  const totalToStrip = (metadataLength + 2) * HEX_CHARS_PER_BYTE;

  // Ensure we don't strip more than the bytecode length
  // Also ensure we leave at least some bytecode (min 10 bytes = 20 chars)
  if (totalToStrip >= normalized.length || normalized.length - totalToStrip < MIN_BYTECODE_AFTER_STRIP) {
    return normalized;
  }

  // Verify this looks like CBOR metadata by checking the marker
  const potentialMetadataStart = normalized.length - totalToStrip;
  const metadataMarker = normalized.slice(potentialMetadataStart, potentialMetadataStart + METADATA_LENGTH_HEX_CHARS);

  // Common CBOR map prefixes for Solidity metadata
  // a2 = 2-item map (ipfs + solc), a1 = 1-item map (just ipfs or bzzr)
  // a3 = 3-item map (additional experimental fields)
  // Also check for a0 (empty map) which some minimal toolchains might produce
  const isValidCborMarker =
    metadataMarker.startsWith("a0") ||
    metadataMarker.startsWith("a1") ||
    metadataMarker.startsWith("a2") ||
    metadataMarker.startsWith("a3");

  if (isValidCborMarker) {
    // For lengths outside typical bounds, apply stricter CBOR validation
    if (metadataLength > CBOR_METADATA_MAX_LENGTH) {
      // For unusually large metadata, verify it contains expected CBOR patterns
      const metadataContent = normalized.slice(potentialMetadataStart);
      // Check for common Solidity metadata patterns: ipfs, bzzr, solc
      const hasKnownPattern =
        metadataContent.includes("697066735822") || // ipfs (as text)
        metadataContent.includes("627a7a72") || // bzzr (as text)
        metadataContent.includes("736f6c63"); // solc (as text)

      if (hasKnownPattern) {
        return normalized.slice(0, potentialMetadataStart);
      }
      // Large metadata without known patterns - don't strip to be safe
      return normalized;
    }

    // Within normal bounds with valid marker - strip it
    return normalized.slice(0, potentialMetadataStart);
  }

  // If no valid CBOR marker found, return original
  return normalized;
}

/**
 * Finds contiguous regions of differences in bytecode.
 * Returns regions that could be immutable values (typically 32-byte aligned).
 */
function findDifferenceRegions(
  local: string,
  remote: string,
): { start: number; end: number; localValue: string; remoteValue: string }[] {
  const regions: { start: number; end: number; localValue: string; remoteValue: string }[] = [];
  const minLength = Math.min(local.length, remote.length);

  let regionStart: number | null = null;

  for (let i = 0; i < minLength; i += HEX_CHARS_PER_BYTE) {
    const localByte = local.slice(i, i + HEX_CHARS_PER_BYTE);
    const remoteByte = remote.slice(i, i + HEX_CHARS_PER_BYTE);
    const isDifferent = localByte !== remoteByte;

    if (isDifferent && regionStart === null) {
      regionStart = i;
    } else if (!isDifferent && regionStart !== null) {
      regions.push({
        start: regionStart / HEX_CHARS_PER_BYTE,
        end: i / HEX_CHARS_PER_BYTE,
        localValue: local.slice(regionStart, i),
        remoteValue: remote.slice(regionStart, i),
      });
      regionStart = null;
    }
  }

  // Handle trailing difference
  if (regionStart !== null) {
    regions.push({
      start: regionStart / HEX_CHARS_PER_BYTE,
      end: minLength / HEX_CHARS_PER_BYTE,
      localValue: local.slice(regionStart, minLength),
      remoteValue: remote.slice(regionStart, minLength),
    });
  }

  return regions;
}

/**
 * Analyzes difference regions to identify potential immutables.
 * Immutables are typically:
 * - 32 bytes (64 hex chars) for most types
 * - 20 bytes (40 hex chars) for addresses (padded to 32)
 * - May appear as contiguous or nearby regions
 */
function analyzeImmutableDifferences(
  regions: { start: number; end: number; localValue: string; remoteValue: string }[],
): { immutables: ImmutableDifference[]; isLikelyOnlyImmutables: boolean } {
  const immutables: ImmutableDifference[] = [];
  let allLookLikeImmutables = true;

  // Common immutable sizes in bytes (Solidity types)
  const commonImmutableSizes = new Set(COMMON_IMMUTABLE_SIZES);

  for (const region of regions) {
    const length = region.end - region.start;
    const localValue = region.localValue;
    const remoteValue = region.remoteValue;

    // Check if this looks like an immutable (must be a common Solidity type size)
    const isPlausibleImmutableSize = commonImmutableSizes.has(length);

    // Check if local value looks like a placeholder
    const localIsPlaceholder =
      localValue === "0".repeat(localValue.length) ||
      /^0+[0-9a-f]{1,16}$/.test(localValue) ||
      /^0{48}[0-9a-f]{16}$/.test(localValue);

    // Determine possible type based on value patterns
    // ADDRESS_BYTES (20) for addresses, BYTES_PER_STORAGE_SLOT (32) for full slots
    let possibleType: string | undefined;
    const addressPadding = "0".repeat(HEX_CHARS_PER_STORAGE_SLOT - ADDRESS_HEX_CHARS);
    if (length === ADDRESS_BYTES || (length === BYTES_PER_STORAGE_SLOT && remoteValue.startsWith(addressPadding))) {
      possibleType = "address";
    } else if (length <= 8) {
      possibleType = "uint" + length * 8;
    } else if (length === BYTES_PER_STORAGE_SLOT) {
      possibleType = "bytes32 or uint256";
    }

    immutables.push({
      position: region.start,
      length,
      localValue,
      remoteValue,
      possibleType,
    });

    if (!localIsPlaceholder && !isPlausibleImmutableSize) {
      allLookLikeImmutables = false;
    }

    if (!localIsPlaceholder && length <= 2) {
      allLookLikeImmutables = false;
    }
  }

  if (regions.length > MAX_REGIONS_FOR_IMMUTABLE_CHECK && regions.every((r) => r.end - r.start <= HEX_CHARS_PER_BYTE)) {
    allLookLikeImmutables = false;
  }

  if (regions.length > MAX_REGIONS_FOR_IMMUTABLE_CHECK_STRICT) {
    allLookLikeImmutables = false;
  }

  return { immutables, isLikelyOnlyImmutables: allLookLikeImmutables && regions.length > 0 };
}

/**
 * Checks if a difference region falls within known immutable positions.
 */
function isDifferenceAtKnownImmutable(
  region: { start: number; end: number },
  knownImmutables: ImmutableReference[],
): boolean {
  for (const imm of knownImmutables) {
    const immEnd = imm.start + imm.length;
    if (region.start >= imm.start && region.end <= immEnd) {
      return true;
    }
  }
  return false;
}

/**
 * Compares two bytecode strings after stripping metadata.
 * Identifies if differences are only due to immutable values.
 * Returns detailed comparison result.
 *
 * @param localBytecode - Local artifact bytecode
 * @param remoteBytecode - On-chain bytecode
 * @param knownImmutables - Known immutable positions from Foundry artifact (optional)
 */
export function compareBytecode(
  localBytecode: string,
  remoteBytecode: string,
  knownImmutables?: ImmutableReference[],
): BytecodeComparisonResult {
  const strippedLocal = stripCborMetadata(localBytecode);
  const strippedRemote = stripCborMetadata(remoteBytecode);

  const localBytes = strippedLocal.length / HEX_CHARS_PER_BYTE;
  const remoteBytes = strippedRemote.length / HEX_CHARS_PER_BYTE;

  // Exact match
  if (strippedLocal === strippedRemote) {
    return {
      status: "pass",
      message: "Bytecode matches exactly",
      localBytecodeLength: localBytes,
      remoteBytecodeLength: remoteBytes,
      matchPercentage: 100,
      differences: undefined,
      immutableDifferences: undefined,
      onlyImmutablesDiffer: undefined,
    };
  }

  // Length must match for immutable-only differences
  if (strippedLocal.length !== strippedRemote.length) {
    return {
      status: "fail",
      message: `Bytecode length mismatch: local ${localBytes} bytes, remote ${remoteBytes} bytes`,
      localBytecodeLength: localBytes,
      remoteBytecodeLength: remoteBytes,
      matchPercentage: 0,
      differences: undefined,
      immutableDifferences: undefined,
      onlyImmutablesDiffer: false,
    };
  }

  // Find difference regions
  const regions = findDifferenceRegions(strippedLocal, strippedRemote);

  // Calculate match percentage
  let diffBytes = 0;
  for (const region of regions) {
    diffBytes += region.end - region.start;
  }
  const matchPercentage = localBytes > 0 ? Math.round(((localBytes - diffBytes) / localBytes) * 100) : 0;

  // If we have known immutable positions from Foundry, use them for precise matching
  if (knownImmutables && knownImmutables.length > 0) {
    const immutables: ImmutableDifference[] = [];
    let allAtKnownPositions = true;

    for (const region of regions) {
      const isKnownImmutable = isDifferenceAtKnownImmutable(region, knownImmutables);

      if (isKnownImmutable) {
        immutables.push({
          position: region.start,
          length: region.end - region.start,
          localValue: region.localValue,
          remoteValue: region.remoteValue,
          possibleType: "immutable (verified by artifact)",
        });
      } else {
        allAtKnownPositions = false;
        immutables.push({
          position: region.start,
          length: region.end - region.start,
          localValue: region.localValue,
          remoteValue: region.remoteValue,
          possibleType: "unknown (not at immutable position)",
        });
      }
    }

    if (allAtKnownPositions) {
      return {
        status: "pass",
        message: `Bytecode matches (${immutables.length} immutable value(s) differ at known positions)`,
        localBytecodeLength: localBytes,
        remoteBytecodeLength: remoteBytes,
        matchPercentage: 100, // Effective match is 100% when only immutables differ
        differences: undefined,
        immutableDifferences: immutables,
        onlyImmutablesDiffer: true,
      };
    }

    const differences: BytecodeDifference[] = regions
      .filter((r) => !isDifferenceAtKnownImmutable(r, knownImmutables))
      .slice(0, MAX_DIFFERENCES_TO_RETURN)
      .map((r) => ({
        position: r.start,
        localByte: r.localValue.slice(0, 2),
        remoteByte: r.remoteValue.slice(0, 2),
      }));

    return {
      status: "fail",
      message: `Bytecode mismatch: ${regions.length - immutables.filter((i) => i.possibleType?.includes("verified")).length} unexpected difference(s)`,
      localBytecodeLength: localBytes,
      remoteBytecodeLength: remoteBytes,
      matchPercentage,
      differences,
      immutableDifferences: immutables,
      onlyImmutablesDiffer: false,
    };
  }

  // No known immutables - use heuristic analysis
  const { immutables, isLikelyOnlyImmutables } = analyzeImmutableDifferences(regions);

  const differences: BytecodeDifference[] = regions.slice(0, MAX_DIFFERENCES_TO_RETURN).map((r) => ({
    position: r.start,
    localByte: r.localValue.slice(0, HEX_CHARS_PER_BYTE),
    remoteByte: r.remoteValue.slice(0, 2),
  }));

  if (isLikelyOnlyImmutables) {
    return {
      status: "pass",
      message: `Bytecode matches (${immutables.length} immutable value(s) differ as expected)`,
      localBytecodeLength: localBytes,
      remoteBytecodeLength: remoteBytes,
      matchPercentage: 100, // Effective match is 100% when only immutables differ
      differences: undefined,
      immutableDifferences: immutables,
      onlyImmutablesDiffer: true,
    };
  }

  return {
    status: "fail",
    message: `Bytecode mismatch: ${matchPercentage}% match, ${regions.length} difference region(s)`,
    localBytecodeLength: localBytes,
    remoteBytecodeLength: remoteBytes,
    matchPercentage,
    differences,
    immutableDifferences: immutables.length > 0 ? immutables : undefined,
    onlyImmutablesDiffer: false,
  };
}

/**
 * Extracts function selectors from deployed bytecode.
 * Looks for PUSH4 (0x63) instructions followed by 4 bytes.
 * This is a heuristic and may not catch all selectors.
 */
export function extractSelectorsFromBytecode(bytecode: string): string[] {
  const normalized = normalizeHex(bytecode);

  const selectors: Set<string> = new Set();

  // PUSH4 opcode (63) + 4-byte selector = 10 hex chars minimum
  const minLengthForSelector = HEX_CHARS_PER_BYTE + SELECTOR_HEX_CHARS;
  if (normalized.length < minLengthForSelector) {
    return [];
  }

  for (let i = 0; i <= normalized.length - minLengthForSelector; i += HEX_CHARS_PER_BYTE) {
    if (normalized.slice(i, i + HEX_CHARS_PER_BYTE) === PUSH4_OPCODE) {
      const potentialSelector = normalized.slice(i + HEX_CHARS_PER_BYTE, i + HEX_CHARS_PER_BYTE + SELECTOR_HEX_CHARS);
      if (potentialSelector !== NULL_SELECTOR && potentialSelector !== MAX_SELECTOR) {
        selectors.add(potentialSelector);
      }
    }
  }

  return Array.from(selectors).sort();
}

/**
 * Validates that immutable differences match expected constructor args.
 * Returns validation result with details.
 */
export function validateImmutablesAgainstArgs(
  immutables: ImmutableDifference[],
  constructorArgs: unknown[] | string,
  verbose: boolean = false,
): { valid: boolean; message: string; details?: string[] } {
  if (immutables.length === 0) {
    return { valid: true, message: "No immutable differences to validate" };
  }

  const expectedValues: string[] = [];

  if (typeof constructorArgs === "string") {
    const normalized = normalizeHex(constructorArgs);

    for (let i = 0; i < normalized.length; i += 64) {
      expectedValues.push(normalized.slice(i, i + 64));
    }
  } else if (Array.isArray(constructorArgs)) {
    for (const arg of constructorArgs) {
      if (typeof arg === "string" && arg.startsWith("0x")) {
        const normalized = arg.slice(2).toLowerCase().padStart(HEX_CHARS_PER_STORAGE_SLOT, "0");
        expectedValues.push(normalized);
      } else if (typeof arg === "bigint" || typeof arg === "number") {
        const hex = BigInt(arg).toString(16).padStart(HEX_CHARS_PER_STORAGE_SLOT, "0");
        expectedValues.push(hex);
      } else if (typeof arg === "boolean") {
        expectedValues.push(arg ? "0".repeat(63) + "1" : "0".repeat(64));
      } else {
        expectedValues.push(String(arg));
      }
    }
  }

  const details: string[] = [];
  let allMatched = true;
  let matchedCount = 0;

  for (const imm of immutables) {
    const remoteValue = imm.remoteValue.toLowerCase().padStart(64, "0");

    let found = false;
    for (const expected of expectedValues) {
      const expectedStripped = expected.replace(/^0+/, "") || "0";
      const remoteStripped = remoteValue.replace(/^0+/, "") || "0";

      const isExactMatch = remoteValue === expected;
      const isStrippedMatch = remoteStripped === expectedStripped;
      const isAddressMatch =
        imm.possibleType === "address" &&
        remoteValue.slice(-40) === expected.slice(-40) &&
        expected.slice(-40) !== "0".repeat(40);

      if (isExactMatch || isStrippedMatch || isAddressMatch) {
        found = true;
        matchedCount++;
        if (verbose) {
          details.push(`✓ Immutable at position ${imm.position}: ${imm.remoteValue} matches expected arg`);
        }
        break;
      }
    }

    if (!found) {
      allMatched = false;
      details.push(
        `✗ Immutable at position ${imm.position}: ${imm.remoteValue} (${imm.possibleType || "unknown type"}) - no matching constructor arg found`,
      );
    }
  }

  if (allMatched) {
    return verbose
      ? { valid: true, message: `All ${immutables.length} immutable value(s) match constructor args`, details }
      : { valid: true, message: `All ${immutables.length} immutable value(s) match constructor args` };
  }

  return {
    valid: false,
    message: `${matchedCount}/${immutables.length} immutable values matched constructor args`,
    details,
  };
}

/**
 * Normalizes a value to a lowercase hex string (without 0x prefix).
 * Handles addresses, numbers, booleans, and bigints.
 * All values are padded to 64 hex chars (32 bytes) for EVM storage comparison.
 *
 * String handling:
 * - "0x..." prefix: treated as hex
 * - Decimal string (e.g., "1000000"): converted to hex via BigInt
 * - Other strings: treated as hex (legacy behavior for backwards compatibility)
 *
 * Negative numbers are converted to two's complement representation.
 */
function normalizeValueToHex(value: string | number | boolean | bigint): string {
  if (typeof value === "string") {
    // Check for explicit hex prefix first
    if (value.toLowerCase().startsWith("0x")) {
      const normalized = value.slice(2).toLowerCase();
      return normalized.padStart(64, "0");
    }

    // Check if it's a decimal number string (including negative numbers)
    if (isDecimalString(value)) {
      // Convert decimal string to bigint, then to hex
      const bigintValue = BigInt(value);
      if (bigintValue < 0n) {
        // Two's complement for negative values
        const twoComplement = (1n << 256n) + bigintValue;
        return twoComplement.toString(16).padStart(64, "0");
      }
      return bigintValue.toString(16).padStart(64, "0");
    }

    // Treat as hex string (legacy behavior for backwards compatibility)
    // This handles cases where users pass hex without 0x prefix
    return value.toLowerCase().padStart(64, "0");
  } else if (typeof value === "bigint") {
    // Handle negative numbers with two's complement
    if (value < 0n) {
      // Two's complement for 256-bit signed integers
      // For negative values: 2^256 + value (since value is negative, this gives the correct representation)
      const twoComplement = (1n << 256n) + value;
      return twoComplement.toString(16).padStart(64, "0");
    }
    return value.toString(16).padStart(64, "0");
  } else if (typeof value === "number") {
    // Convert to bigint to handle potential negative numbers
    const bigintValue = BigInt(value);
    if (bigintValue < 0n) {
      const twoComplement = (1n << 256n) + bigintValue;
      return twoComplement.toString(16).padStart(64, "0");
    }
    return bigintValue.toString(16).padStart(64, "0");
  } else if (typeof value === "boolean") {
    return value ? "0".repeat(63) + "1" : "0".repeat(64);
  }
  return String(value);
}

/**
 * Verifies named immutable values against detected bytecode differences.
 *
 * This function matches expected immutable values (by name) against the actual
 * values found in the deployed bytecode. It supports:
 * - Exact value matching (for addresses, uints, bytes32)
 * - Address matching (comparing last 40 hex chars for left-padded addresses)
 * - Fragment matching (addresses split into multiple regions due to matching bytes)
 *
 * @param immutableValues - Map of variable names to expected values
 * @param immutableDifferences - Detected immutable differences from bytecode comparison
 * @returns Verification result with details for each named immutable
 */
export function verifyImmutableValues(
  immutableValues: Record<string, string | number | boolean | bigint>,
  immutableDifferences: ImmutableDifference[],
): ImmutableValuesResult {
  const results: ImmutableValueResult[] = [];
  let allPassed = true;

  // Extract all remote values from the bytecode (keep raw values for fragment matching)
  const remoteValues = immutableDifferences.map((diff) => ({
    position: diff.position,
    rawValue: diff.remoteValue.toLowerCase(),
    value: diff.remoteValue.toLowerCase().padStart(HEX_CHARS_PER_STORAGE_SLOT, "0"),
    length: diff.length,
    possibleType: diff.possibleType,
  }));

  // Track which positions have been consumed to prevent matching same region twice
  const consumedPositions = new Set<number>();

  for (const [name, expectedValue] of Object.entries(immutableValues)) {
    const expectedHex = normalizeValueToHex(expectedValue);
    const expectedStripped = expectedHex.replace(/^0+/, "") || "0";

    // Try to find a matching remote value (excluding already consumed positions)
    let matchedRemote: (typeof remoteValues)[0] | undefined;
    let matchType: "exact" | "stripped" | "address" | "fragment" | undefined;

    for (const remote of remoteValues) {
      // Skip already consumed positions to prevent matching same region twice
      if (consumedPositions.has(remote.position)) {
        continue;
      }

      const remoteStripped = remote.value.replace(/^0+/, "") || "0";

      // Check exact match
      if (remote.value === expectedHex) {
        matchedRemote = remote;
        matchType = "exact";
        break;
      }

      // Check stripped match (ignoring leading zeros)
      if (remoteStripped === expectedStripped) {
        matchedRemote = remote;
        matchType = "stripped";
        break;
      }

      // Check address match (last 40 chars)
      if (
        expectedHex.length >= ADDRESS_HEX_CHARS &&
        remote.value.slice(-ADDRESS_HEX_CHARS) === expectedHex.slice(-ADDRESS_HEX_CHARS)
      ) {
        matchedRemote = remote;
        matchType = "address";
        break;
      }
    }

    // If no direct match, check for fragment matches
    // Addresses can be split when they have matching bytes (like 00) with the placeholder
    if (!matchedRemote) {
      // For addresses, check if the expected value contains any of the raw fragments
      const expectedLower = expectedHex.toLowerCase();

      for (const remote of remoteValues) {
        // Skip already consumed positions
        if (consumedPositions.has(remote.position)) {
          continue;
        }

        // Check if this fragment is part of the expected value
        if (remote.rawValue.length >= MIN_FRAGMENT_LENGTH && expectedLower.includes(remote.rawValue)) {
          matchedRemote = remote;
          matchType = "fragment";
          break;
        }

        // Also check if the raw remote value (without padding) matches the end of expected
        // This handles cases like "7ba269a03eed86f2f54cb04ca3b4b7626636df4e" matching an address
        if (remote.rawValue.length === ADDRESS_HEX_CHARS && expectedLower.endsWith(remote.rawValue)) {
          matchedRemote = remote;
          matchType = "address";
          break;
        }
      }
    }

    if (matchedRemote) {
      // Mark this position as consumed so it cannot match another immutable name
      consumedPositions.add(matchedRemote.position);

      const displayValue =
        matchedRemote.rawValue.length <= ADDRESS_HEX_CHARS
          ? matchedRemote.rawValue
          : matchedRemote.rawValue.slice(-ADDRESS_HEX_CHARS);
      results.push({
        name,
        expected: "0x" + expectedHex,
        actual: "0x" + matchedRemote.value,
        status: "pass",
        message: `${name} = 0x${displayValue}... (${matchType} match at position ${matchedRemote.position})`,
      });
    } else {
      allPassed = false;
      results.push({
        name,
        expected: "0x" + expectedHex,
        actual: undefined,
        status: "fail",
        message: `${name}: expected 0x${expectedHex.slice(-ADDRESS_HEX_CHARS)}... not found in bytecode immutables`,
      });
    }
  }

  // Check for unmatched immutable differences (remote values not in config)
  const matchedPositions = new Set(
    results.filter((r) => r.status === "pass").map((r) => r.message.match(/position (\d+)/)?.[1]),
  );
  const unmatchedRemotes = remoteValues.filter((r) => !matchedPositions.has(String(r.position)));

  const passedCount = results.filter((r) => r.status === "pass").length;
  const totalCount = results.length;

  let message: string;
  if (allPassed) {
    if (unmatchedRemotes.length > 0) {
      message = `All ${totalCount} named immutables verified (${unmatchedRemotes.length} additional region(s) in bytecode - likely duplicates or fragments)`;
    } else {
      message = `All ${totalCount} named immutables verified`;
    }
  } else {
    message = `${passedCount}/${totalCount} named immutables verified`;
  }

  return {
    status: allPassed ? "pass" : "fail",
    message,
    results,
  };
}

/**
 * Performs definitive bytecode comparison by substituting immutable values.
 *
 * This provides 100% confidence by:
 * 1. Taking the local artifact bytecode
 * 2. Reading actual immutable values directly from remote bytecode at known positions
 * 3. Substituting those values into local bytecode
 * 4. Comparing byte-for-byte
 *
 * If they match exactly, there is zero ambiguity - the bytecode is identical.
 *
 * Note: This does NOT use immutableDifferences because those can be fragmented
 * when addresses contain bytes that match the placeholder (e.g., 0x73bf00ad...
 * splits into "73bf" and "ad..." because "00" matches). Instead, we read the
 * full 32 bytes directly from the remote bytecode at each known position.
 *
 * @param localBytecode - Local artifact deployed bytecode
 * @param remoteBytecode - On-chain deployed bytecode
 * @param immutableReferences - Exact byte positions of immutables (from artifact)
 * @param _immutableDifferences - Unused, kept for API compatibility
 * @returns Definitive verification result
 */
export function definitiveCompareBytecode(
  localBytecode: string,
  remoteBytecode: string,
  immutableReferences: ImmutableReference[],
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  _immutableDifferences: ImmutableDifference[],
): DefinitiveBytecodeResult {
  // Strip CBOR metadata from both
  const strippedLocal = stripCborMetadata(localBytecode);
  const strippedRemote = stripCborMetadata(remoteBytecode);

  // Length must match
  if (strippedLocal.length !== strippedRemote.length) {
    return {
      exactMatch: false,
      status: "fail",
      message: `Bytecode length mismatch: local ${strippedLocal.length / HEX_CHARS_PER_BYTE} bytes, remote ${strippedRemote.length / HEX_CHARS_PER_BYTE} bytes`,
      immutablesSubstituted: 0,
    };
  }

  // Substitute immutable values into local bytecode by reading directly from remote
  let substitutedLocal = strippedLocal.toLowerCase();
  const normalizedRemote = strippedRemote.toLowerCase();
  let substitutionsApplied = 0;

  for (const ref of immutableReferences) {
    const bytePos = ref.start;
    const hexPos = bytePos * 2;
    const hexLen = ref.length * 2;

    // Validate position is within bounds
    if (hexPos + hexLen > normalizedRemote.length) {
      continue;
    }

    // Read the full immutable value directly from remote bytecode
    const remoteValue = normalizedRemote.slice(hexPos, hexPos + hexLen);

    // Substitute into local bytecode
    substitutedLocal = substitutedLocal.slice(0, hexPos) + remoteValue + substitutedLocal.slice(hexPos + hexLen);
    substitutionsApplied++;
  }

  // Now compare byte-for-byte
  if (substitutedLocal === normalizedRemote) {
    return {
      exactMatch: true,
      status: "pass",
      message: `Bytecode matches exactly after substituting ${substitutionsApplied} immutable(s) - NO alterations detected`,
      immutablesSubstituted: substitutionsApplied,
    };
  }

  // Find where the differences are
  let diffCount = 0;
  const diffPositions: number[] = [];
  for (let i = 0; i < substitutedLocal.length; i += 2) {
    if (substitutedLocal.slice(i, i + 2) !== normalizedRemote.slice(i, i + 2)) {
      diffCount++;
      if (diffPositions.length < MAX_DIFF_POSITIONS) {
        diffPositions.push(i / HEX_CHARS_PER_BYTE);
      }
    }
  }

  return {
    exactMatch: false,
    status: "fail",
    message: `Bytecode mismatch after immutable substitution: ${diffCount} bytes differ at positions ${diffPositions.join(", ")}${diffCount > 10 ? "..." : ""}`,
    immutablesSubstituted: substitutionsApplied,
  };
}

/**
 * Groups detected immutable differences by their parent immutable reference.
 *
 * This helps display fragmented immutables clearly. When an address contains
 * bytes that match the placeholder (e.g., 0x73bf00ad... has "00" which matches),
 * the difference detection sees multiple fragments. This function groups them
 * back together based on the immutable reference positions from the artifact.
 *
 * @param immutableDifferences - Detected differences (may be fragmented)
 * @param immutableReferences - Known immutable positions from artifact
 * @param remoteBytecode - The on-chain bytecode (to read full values)
 * @returns Grouped immutable differences with fragment information
 */
export function groupImmutableDifferences(
  immutableDifferences: ImmutableDifference[],
  immutableReferences: ImmutableReference[],
  remoteBytecode: string,
): GroupedImmutableDifference[] {
  const strippedRemote = stripCborMetadata(remoteBytecode).toLowerCase();
  const grouped: GroupedImmutableDifference[] = [];

  // Sort references by position
  const sortedRefs = [...immutableReferences].sort((a, b) => a.start - b.start);

  // For each immutable reference, find which differences fall within its range
  let index = 1;
  for (const ref of sortedRefs) {
    const refEnd = ref.start + ref.length;

    // Find all differences that fall within this reference's range
    const fragments = immutableDifferences.filter((diff) => {
      // Check if diff starts within the reference range
      return diff.position >= ref.start && diff.position < refEnd;
    });

    if (fragments.length === 0) {
      // No detected differences for this reference (values might be identical or all zeros)
      continue;
    }

    // Read the full value from remote bytecode
    const hexPos = ref.start * 2;
    const hexLen = ref.length * 2;
    const fullValue = hexPos + hexLen <= strippedRemote.length ? strippedRemote.slice(hexPos, hexPos + hexLen) : "";

    grouped.push({
      index,
      refStart: ref.start,
      refLength: ref.length,
      fullValue,
      isFragmented: fragments.length > 1,
      fragments: fragments.sort((a, b) => a.position - b.position),
    });

    index++;
  }

  return grouped;
}

/**
 * Formats grouped immutable differences for display.
 *
 * @param grouped - Grouped immutable differences
 * @returns Array of formatted strings for display
 */
export function formatGroupedImmutables(grouped: GroupedImmutableDifference[]): string[] {
  const lines: string[] = [];

  for (const group of grouped) {
    // Format the full value for display (strip leading zeros for addresses)
    const displayValue = group.fullValue.replace(/^0+/, "") || "0";
    const isAddress = group.refLength === 32 && displayValue.length <= 40;
    const typeHint = isAddress ? "address" : group.refLength === 32 ? "bytes32/uint256" : `${group.refLength} bytes`;

    if (group.isFragmented) {
      lines.push(`${group.index}) Fragmented immutable at position ${group.refStart} (${typeHint}):`);
      lines.push(`   Full value: 0x${displayValue}`);

      for (let i = 0; i < group.fragments.length; i++) {
        const frag = group.fragments[i];
        lines.push(`   ${group.index}.${i + 1}) Position ${frag.position}: ${frag.remoteValue}`);
      }
    } else {
      // Single fragment (not split)
      const frag = group.fragments[0];
      lines.push(`${group.index}) Position ${frag.position}: 0x${displayValue} (${typeHint})`);
    }
  }

  return lines;
}
