/**
 * Contract Integrity Verifier - Bytecode Utilities
 *
 * Utilities for comparing deployed bytecode.
 * Handles CBOR metadata stripping and immutable detection for accurate comparisons.
 *
 * Note: This module is pure TypeScript with no web3 dependencies.
 */

import { BytecodeComparisonResult, BytecodeDifference, ImmutableDifference, ImmutableReference } from "../types";

/**
 * Strips CBOR-encoded metadata from bytecode.
 *
 * Solidity appends metadata hash at the end of bytecode in CBOR format.
 * The metadata starts with `a264` (2-item map) or `a165` (1-item map) and ends
 * with a 2-byte length indicator.
 *
 * Format: ...contract_code...a264ipfs...solc...0033
 * The last 2 bytes (0033 = 51 in decimal) indicate metadata length.
 */
export function stripCborMetadata(bytecode: string): string {
  // Normalize bytecode
  const normalized = bytecode.toLowerCase().startsWith("0x") ? bytecode.slice(2).toLowerCase() : bytecode.toLowerCase();

  if (normalized.length < 4) {
    return normalized;
  }

  // Get the last 2 bytes (4 hex chars) which indicate metadata length
  const lengthHex = normalized.slice(-4);
  const metadataLength = parseInt(lengthHex, 16);

  // Metadata length is in bytes, each byte is 2 hex chars
  // Total to strip: metadata + 2 bytes for length indicator
  const totalToStrip = (metadataLength + 2) * 2;

  if (totalToStrip >= normalized.length) {
    // Something's wrong, return original
    return normalized;
  }

  // Verify this looks like CBOR metadata by checking the marker
  const potentialMetadataStart = normalized.length - totalToStrip;
  const metadataMarker = normalized.slice(potentialMetadataStart, potentialMetadataStart + 4);

  // Common CBOR map prefixes for Solidity metadata
  if (metadataMarker.startsWith("a2") || metadataMarker.startsWith("a1")) {
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

  for (let i = 0; i < minLength; i += 2) {
    const localByte = local.slice(i, i + 2);
    const remoteByte = remote.slice(i, i + 2);
    const isDifferent = localByte !== remoteByte;

    if (isDifferent && regionStart === null) {
      regionStart = i;
    } else if (!isDifferent && regionStart !== null) {
      regions.push({
        start: regionStart / 2,
        end: i / 2,
        localValue: local.slice(regionStart, i),
        remoteValue: remote.slice(regionStart, i),
      });
      regionStart = null;
    }
  }

  // Handle trailing difference
  if (regionStart !== null) {
    regions.push({
      start: regionStart / 2,
      end: minLength / 2,
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
  const commonImmutableSizes = new Set([1, 2, 4, 8, 12, 16, 20, 32]);

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
    let possibleType: string | undefined;
    if (length === 20 || (length === 32 && remoteValue.startsWith("000000000000000000000000"))) {
      possibleType = "address";
    } else if (length <= 8) {
      possibleType = "uint" + length * 8;
    } else if (length === 32) {
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

  if (regions.length > 5 && regions.every((r) => r.end - r.start <= 2)) {
    allLookLikeImmutables = false;
  }

  if (regions.length > 8) {
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

  const localBytes = strippedLocal.length / 2;
  const remoteBytes = strippedRemote.length / 2;

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
        matchPercentage,
        differences: undefined,
        immutableDifferences: immutables,
        onlyImmutablesDiffer: true,
      };
    }

    const differences: BytecodeDifference[] = regions
      .filter((r) => !isDifferenceAtKnownImmutable(r, knownImmutables))
      .slice(0, 10)
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

  const differences: BytecodeDifference[] = regions.slice(0, 10).map((r) => ({
    position: r.start,
    localByte: r.localValue.slice(0, 2),
    remoteByte: r.remoteValue.slice(0, 2),
  }));

  if (isLikelyOnlyImmutables) {
    return {
      status: "pass",
      message: `Bytecode matches (${immutables.length} immutable value(s) differ as expected)`,
      localBytecodeLength: localBytes,
      remoteBytecodeLength: remoteBytes,
      matchPercentage,
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
  const normalized = bytecode.toLowerCase().startsWith("0x") ? bytecode.slice(2).toLowerCase() : bytecode.toLowerCase();

  const selectors: Set<string> = new Set();

  if (normalized.length < 10) {
    return [];
  }

  for (let i = 0; i <= normalized.length - 10; i += 2) {
    if (normalized.slice(i, i + 2) === "63") {
      const potentialSelector = normalized.slice(i + 2, i + 10);
      if (potentialSelector !== "00000000" && potentialSelector !== "ffffffff") {
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
    const normalized = constructorArgs.toLowerCase().startsWith("0x")
      ? constructorArgs.slice(2).toLowerCase()
      : constructorArgs.toLowerCase();

    for (let i = 0; i < normalized.length; i += 64) {
      expectedValues.push(normalized.slice(i, i + 64));
    }
  } else if (Array.isArray(constructorArgs)) {
    for (const arg of constructorArgs) {
      if (typeof arg === "string" && arg.startsWith("0x")) {
        const normalized = arg.slice(2).toLowerCase().padStart(64, "0");
        expectedValues.push(normalized);
      } else if (typeof arg === "bigint" || typeof arg === "number") {
        const hex = BigInt(arg).toString(16).padStart(64, "0");
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
