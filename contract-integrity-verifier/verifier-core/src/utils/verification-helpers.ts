/**
 * Contract Integrity Verifier - Shared Verification Helpers
 *
 * Common verification logic shared between verifier.ts (Node.js) and
 * verifier-browser.ts (browser-compatible).
 *
 * This module is browser-compatible - no Node.js dependencies.
 */

import { BYTECODE_MATCH_THRESHOLD_PERCENT, ADDRESS_HEX_CHARS, HEX_PREFIX_LENGTH } from "../constants";
import {
  BytecodeComparisonResult,
  NormalizedArtifact,
  ImmutableValuesResult,
  DefinitiveBytecodeResult,
  GroupedImmutableDifference,
  ViewCallResult,
  AbiElement,
} from "../types";
import {
  compareBytecode,
  validateImmutablesAgainstArgs,
  verifyImmutableValues,
  definitiveCompareBytecode,
  groupImmutableDifferences,
  formatGroupedImmutables,
} from "./bytecode";
import { formatValue, formatForDisplay, compareValues } from "./comparison";
import { formatError } from "./errors";

import type { Web3Adapter } from "../adapter";

// ============================================================================
// Types
// ============================================================================

export interface BytecodeVerificationContext {
  artifact: NormalizedArtifact;
  remoteBytecode: string;
  constructorArgs?: unknown[] | string | undefined;
  immutableValues?: Record<string, string | number | boolean | bigint> | undefined;
  verbose?: boolean | undefined;
}

export interface BytecodeVerificationResult {
  bytecodeResult: BytecodeComparisonResult;
  immutableValuesResult: ImmutableValuesResult | undefined;
  definitiveResult: DefinitiveBytecodeResult | undefined;
  groupedImmutables: GroupedImmutableDifference[] | undefined;
}

export interface StateAggregationResult {
  allPass: boolean;
  totalChecks: number;
  passedChecks: number;
  message: string;
  status: "pass" | "fail" | "warn";
}

// ============================================================================
// Bytecode Verification (Shared Logic)
// ============================================================================

/**
 * Performs complete bytecode verification with all checks.
 * This is the shared logic used by both Node.js and browser verifiers.
 */
export function performBytecodeVerification(ctx: BytecodeVerificationContext): BytecodeVerificationResult {
  const { artifact, remoteBytecode, constructorArgs, immutableValues, verbose } = ctx;

  // Initial bytecode comparison
  const bytecodeResult = compareBytecode(artifact.deployedBytecode, remoteBytecode, artifact.immutableReferences);

  let immutableValuesResult: ImmutableValuesResult | undefined;
  let definitiveResult: DefinitiveBytecodeResult | undefined;
  let groupedImmutables: GroupedImmutableDifference[] | undefined;

  // Validate immutables against constructor args if provided
  if (bytecodeResult.onlyImmutablesDiffer && bytecodeResult.immutableDifferences && constructorArgs) {
    const validation = validateImmutablesAgainstArgs(bytecodeResult.immutableDifferences, constructorArgs, verbose);

    if (verbose && validation.details) {
      for (const detail of validation.details) {
        console.log(`    ${detail}`);
      }
    }

    if (validation.valid) {
      bytecodeResult.message += " - constructor args validated";
    } else {
      bytecodeResult.status = "warn";
      bytecodeResult.message += ` - ${validation.message}`;
    }
  }

  // Verify named immutable values if provided
  if (
    bytecodeResult.immutableDifferences &&
    bytecodeResult.immutableDifferences.length > 0 &&
    immutableValues &&
    Object.keys(immutableValues).length > 0
  ) {
    immutableValuesResult = verifyImmutableValues(immutableValues, bytecodeResult.immutableDifferences);

    if (verbose && immutableValuesResult.results) {
      for (const immResult of immutableValuesResult.results) {
        const icon = immResult.status === "pass" ? "✓" : "✗";
        console.log(`    ${icon} ${immResult.message}`);
      }
    }

    // Update bytecode status based on immutable values verification
    if (immutableValuesResult.status === "pass") {
      if (bytecodeResult.status === "fail" && bytecodeResult.matchPercentage !== undefined) {
        if (bytecodeResult.matchPercentage >= BYTECODE_MATCH_THRESHOLD_PERCENT) {
          bytecodeResult.status = "pass";
          bytecodeResult.message = `Bytecode matches (${bytecodeResult.immutableDifferences.length} immutable region(s) verified by name)`;
          bytecodeResult.onlyImmutablesDiffer = true;
        }
      }
      if (bytecodeResult.onlyImmutablesDiffer) {
        bytecodeResult.matchPercentage = 100;
      }
      bytecodeResult.message += " - immutable values verified";
    } else {
      bytecodeResult.status = "warn";
      bytecodeResult.message += ` - ${immutableValuesResult.message}`;
    }
  }

  // Definitive bytecode comparison (100% confidence)
  if (
    artifact.immutableReferences &&
    artifact.immutableReferences.length > 0 &&
    bytecodeResult.immutableDifferences &&
    bytecodeResult.immutableDifferences.length > 0
  ) {
    definitiveResult = definitiveCompareBytecode(
      artifact.deployedBytecode,
      remoteBytecode,
      artifact.immutableReferences,
      bytecodeResult.immutableDifferences,
    );

    groupedImmutables = groupImmutableDifferences(
      bytecodeResult.immutableDifferences,
      artifact.immutableReferences,
      remoteBytecode,
    );

    if (verbose) {
      const icon = definitiveResult.exactMatch ? "✓" : "✗";
      console.log(`    ${icon} Definitive: ${definitiveResult.message}`);

      if (groupedImmutables.length > 0) {
        const fragmentedCount = groupedImmutables.filter((g) => g.isFragmented).length;
        if (fragmentedCount > 0) {
          console.log(`    Note: ${fragmentedCount} immutable(s) are fragmented due to matching bytes`);
        }
        const formattedLines = formatGroupedImmutables(groupedImmutables);
        for (const line of formattedLines) {
          console.log(`      ${line}`);
        }
      }
    }

    // Update bytecode result based on definitive check
    if (definitiveResult.exactMatch) {
      const fragmentedCount = groupedImmutables.filter((g) => g.isFragmented).length;
      const fragmentNote = fragmentedCount > 0 ? ` (${fragmentedCount} fragmented)` : "";

      if (immutableValuesResult?.status === "fail") {
        bytecodeResult.status = "warn";
        bytecodeResult.matchPercentage = 100;
        bytecodeResult.message = `Bytecode structure matches (${definitiveResult.immutablesSubstituted} immutable(s)${fragmentNote}) - ${immutableValuesResult.message}`;
      } else {
        bytecodeResult.status = "pass";
        bytecodeResult.matchPercentage = 100;
        bytecodeResult.message = `Bytecode matches exactly (${definitiveResult.immutablesSubstituted} immutable(s) verified${fragmentNote})`;
      }
    } else if (definitiveResult.status === "fail") {
      bytecodeResult.status = "fail";
      bytecodeResult.message = definitiveResult.message;
    }
  }

  return {
    bytecodeResult,
    immutableValuesResult,
    definitiveResult,
    groupedImmutables,
  };
}

// ============================================================================
// State Verification Aggregation (Shared Logic)
// ============================================================================

/**
 * Aggregates state verification results into a summary.
 * Used by both verifier implementations.
 */
export function aggregateStateResults(
  viewCallResults: ViewCallResult[],
  namespaceResults: { status: string }[],
  slotResults: { status: string }[],
  storagePathResults: { status: string }[],
  storagePathsSkipped: boolean,
  skippedCount: number,
): StateAggregationResult {
  const allViewCallsPass = viewCallResults.every((r) => r.status === "pass");
  const allNamespacesPass = namespaceResults.every((r) => r.status === "pass");
  const allSlotsPass = slotResults.every((r) => r.status === "pass");
  const allStoragePathsPass = storagePathResults.every((r) => r.status === "pass");

  const totalChecks = viewCallResults.length + namespaceResults.length + slotResults.length + storagePathResults.length;
  const passedChecks =
    viewCallResults.filter((r) => r.status === "pass").length +
    namespaceResults.filter((r) => r.status === "pass").length +
    slotResults.filter((r) => r.status === "pass").length +
    storagePathResults.filter((r) => r.status === "pass").length;

  const allPass = allViewCallsPass && allNamespacesPass && allSlotsPass && allStoragePathsPass && !storagePathsSkipped;

  let message: string;
  if (storagePathsSkipped) {
    message =
      allViewCallsPass && allNamespacesPass && allSlotsPass
        ? `${totalChecks} state checks passed, but ${skippedCount} storage path(s) SKIPPED (schema missing)`
        : `${passedChecks}/${totalChecks} state checks passed, ${skippedCount} storage path(s) SKIPPED (schema missing)`;
  } else {
    message = allPass ? `All ${totalChecks} state checks passed` : `${passedChecks}/${totalChecks} state checks passed`;
  }

  return {
    allPass,
    totalChecks,
    passedChecks,
    message,
    status: storagePathsSkipped ? "warn" : allPass ? "pass" : "fail",
  };
}

// ============================================================================
// View Call Execution (Shared Logic)
// ============================================================================

export interface ViewCallConfig {
  function: string;
  params?: unknown[];
  expected: unknown;
  comparison?: string;
}

/**
 * Executes a view call and compares result with expected value.
 * Shared implementation for both verifier classes.
 */
export async function executeViewCallShared(
  adapter: Web3Adapter,
  address: string,
  abi: AbiElement[],
  config: ViewCallConfig,
): Promise<ViewCallResult> {
  try {
    const funcAbi = abi.find((e) => e.type === "function" && e.name === config.function);
    if (!funcAbi) {
      return {
        function: config.function,
        params: config.params,
        expected: config.expected,
        actual: undefined,
        status: "fail",
        message: `Function '${config.function}' not found in ABI`,
      };
    }

    const calldata = adapter.encodeFunctionData(abi, config.function, config.params ?? []);
    const result = await adapter.call(address, calldata);
    const decoded = adapter.decodeFunctionResult(abi, config.function, result);

    const actual = decoded.length === 1 ? formatValue(decoded[0]) : decoded.map(formatValue);
    const comparison = (config.comparison ?? "eq") as "eq" | "gt" | "gte" | "lt" | "lte" | "contains";
    const pass = compareValues(actual, config.expected, comparison);

    return {
      function: config.function,
      params: config.params,
      expected: config.expected,
      actual,
      status: pass ? "pass" : "fail",
      message: pass
        ? `${config.function}() = ${formatForDisplay(actual)}`
        : `Expected ${formatForDisplay(config.expected)}, got ${formatForDisplay(actual)}`,
    };
  } catch (error) {
    return {
      function: config.function,
      params: config.params,
      expected: config.expected,
      actual: undefined,
      status: "fail",
      message: `Call failed: ${formatError(error)}`,
    };
  }
}

// ============================================================================
// Implementation Address Extraction (Shared Logic)
// ============================================================================

/**
 * Extracts implementation address from storage slot value.
 * Shared logic for parsing EIP-1967 implementation slot.
 *
 * @param slotValue - The 32-byte storage slot value
 * @returns Address string (lowercase, without checksum)
 */
export function extractAddressFromSlot(slotValue: string): string {
  // Take last 40 hex chars (20 bytes) from the slot value
  return "0x" + slotValue.slice(-ADDRESS_HEX_CHARS);
}

// ============================================================================
// Bytecode Length Calculation (Shared Logic)
// ============================================================================

/**
 * Calculates bytecode length in bytes from hex string.
 *
 * @param bytecode - Hex-encoded bytecode (with or without 0x prefix)
 * @returns Length in bytes
 */
export function getBytecodeLength(bytecode: string): number {
  const hex = bytecode.startsWith("0x") ? bytecode.slice(HEX_PREFIX_LENGTH) : bytecode;
  return hex.length / 2;
}
