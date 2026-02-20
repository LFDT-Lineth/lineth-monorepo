/**
 * Contract Integrity Verifier - ABI Utilities
 *
 * Utilities for parsing ABI files and comparing function selectors.
 * Supports both Hardhat and Foundry artifact formats.
 *
 * Note: Selector computation requires a Web3Adapter for hashing.
 *
 * This file is browser-compatible. Node.js-only functions (loadArtifact)
 * are in abi-node.ts.
 */

import { HEX_PREFIX_LENGTH, SELECTOR_HEX_CHARS, MAX_EXTRA_SELECTORS_TO_REPORT } from "../constants";
import {
  AbiElement,
  AbiInput,
  AbiComparisonResult,
  NormalizedArtifact,
  ArtifactFormat,
  HardhatArtifact,
  FoundryArtifact,
  ImmutableReference,
  DeployedLinkReferences,
} from "../types";

import type { Web3Adapter } from "../adapter";

// ============================================================================
// Artifact Format Detection
// ============================================================================

/**
 * Detects whether an artifact is Hardhat or Foundry format.
 */
export function detectArtifactFormat(artifact: unknown): ArtifactFormat {
  const obj = artifact as Record<string, unknown>;

  // Foundry: bytecode is an object with 'object' property
  if (obj.bytecode && typeof obj.bytecode === "object" && (obj.bytecode as Record<string, unknown>).object) {
    return "foundry";
  }

  // Hardhat: bytecode is a string
  return "hardhat";
}

/**
 * Type guard for Foundry artifacts.
 */
function isFoundryArtifact(artifact: unknown): artifact is FoundryArtifact {
  return detectArtifactFormat(artifact) === "foundry";
}

// ============================================================================
// Artifact Loading and Normalization
// ============================================================================

/**
 * Parses and normalizes an artifact from content (string or object).
 * Browser-compatible - does not use filesystem.
 *
 * @param content - JSON string or parsed object
 * @param filename - Optional filename for contract name extraction (used for Foundry artifacts)
 * @throws Error with descriptive message if content cannot be parsed
 */
export function parseArtifact(content: string | object, filename?: string): NormalizedArtifact {
  let raw: unknown;

  if (typeof content === "string") {
    try {
      raw = JSON.parse(content);
    } catch (err) {
      throw new Error(`Failed to parse artifact JSON: ${err instanceof Error ? err.message : String(err)}`);
    }
  } else {
    raw = content;
  }

  if (isFoundryArtifact(raw)) {
    return normalizeFoundryArtifact(raw, filename ?? "Contract.json");
  }

  return normalizeHardhatArtifact(raw as HardhatArtifact);
}

// loadArtifact is in abi-node.ts to avoid bundling 'fs' in browser builds

/**
 * Enriched Hardhat artifact with immutableReferences added by enrich-hardhat-artifact.ts
 */
interface EnrichedHardhatArtifact extends HardhatArtifact {
  immutableReferences?: Record<string, Array<{ start: number; length: number }>>;
}

/**
 * Normalizes a Hardhat artifact to the common format.
 * Supports enriched artifacts with immutableReferences from build-info.
 */
function normalizeHardhatArtifact(artifact: HardhatArtifact): NormalizedArtifact {
  // Check for enriched artifact with immutableReferences
  const enriched = artifact as EnrichedHardhatArtifact;
  let immutableReferences: ImmutableReference[] | undefined;

  if (enriched.immutableReferences && Object.keys(enriched.immutableReferences).length > 0) {
    immutableReferences = [];
    for (const refs of Object.values(enriched.immutableReferences)) {
      for (const ref of refs) {
        immutableReferences.push({ start: ref.start, length: ref.length });
      }
    }
  }

  let deployedLinkReferences: DeployedLinkReferences | undefined;
  if (artifact.deployedLinkReferences && Object.keys(artifact.deployedLinkReferences).length > 0) {
    deployedLinkReferences = artifact.deployedLinkReferences;
  }

  return {
    format: "hardhat",
    contractName: artifact.contractName,
    abi: artifact.abi,
    bytecode: artifact.bytecode,
    deployedBytecode: artifact.deployedBytecode,
    immutableReferences,
    methodIdentifiers: undefined,
    deployedLinkReferences,
  };
}

/**
 * Browser-safe basename implementation.
 * Extracts filename without extension from a path.
 */
function getBasename(filePath: string, ext?: string): string {
  // Handle both Unix and Windows path separators
  const lastSep = Math.max(filePath.lastIndexOf("/"), filePath.lastIndexOf("\\"));
  let name = lastSep >= 0 ? filePath.slice(lastSep + 1) : filePath;
  if (ext && name.endsWith(ext)) {
    name = name.slice(0, -ext.length);
  }
  return name;
}

/**
 * Normalizes a Foundry artifact to the common format.
 */
function normalizeFoundryArtifact(artifact: FoundryArtifact, filePath: string): NormalizedArtifact {
  const fileName = getBasename(filePath, ".json");
  const contractName = fileName;

  let immutableReferences: ImmutableReference[] | undefined;
  if (artifact.deployedBytecode.immutableReferences) {
    immutableReferences = [];
    for (const refs of Object.values(artifact.deployedBytecode.immutableReferences)) {
      for (const ref of refs as Array<{ start: number; length: number }>) {
        immutableReferences.push({ start: ref.start, length: ref.length });
      }
    }
  }

  let methodIdentifiers: Map<string, string> | undefined;
  if (artifact.methodIdentifiers) {
    methodIdentifiers = new Map();
    for (const [signature, selector] of Object.entries(artifact.methodIdentifiers)) {
      methodIdentifiers.set((selector as string).toLowerCase(), signature);
    }
  }

  let deployedLinkReferences: DeployedLinkReferences | undefined;
  if (artifact.deployedBytecode.linkReferences && Object.keys(artifact.deployedBytecode.linkReferences).length > 0) {
    deployedLinkReferences = artifact.deployedBytecode.linkReferences;
  }

  return {
    format: "foundry",
    contractName,
    abi: artifact.abi,
    bytecode: artifact.bytecode.object,
    deployedBytecode: artifact.deployedBytecode.object,
    immutableReferences,
    methodIdentifiers,
    deployedLinkReferences,
  };
}

// ============================================================================
// Function Signature and Selector Computation
// ============================================================================

/**
 * Generates a function signature from ABI element.
 * e.g., "transfer(address,uint256)"
 */
function getFunctionSignature(element: AbiElement): string {
  if (!element.name) {
    return "";
  }

  const inputs = element.inputs || [];
  const paramTypes = inputs.map((input) => getParamType(input)).join(",");
  return `${element.name}(${paramTypes})`;
}

/**
 * Recursively gets the parameter type string for tuple types.
 */
function getParamType(input: AbiInput): string {
  if (input.type === "tuple" || input.type === "tuple[]") {
    const components = input.components || [];
    const innerTypes = components.map((c: AbiInput) => getParamType(c)).join(",");
    const tupleType = `(${innerTypes})`;
    return input.type === "tuple[]" ? `${tupleType}[]` : tupleType;
  }
  return input.type;
}

/**
 * Extracts all function selectors from an ABI.
 * Returns a map of selector -> function name for debugging.
 *
 * @param abi - Contract ABI
 * @param adapter - Web3Adapter for keccak256 hashing
 */
export function extractSelectorsFromAbi(adapter: Web3Adapter, abi: AbiElement[]): Map<string, string> {
  const selectorMap = new Map<string, string>();

  for (const element of abi) {
    if (element.type === "function" && element.name) {
      const signature = getFunctionSignature(element);
      const hash = adapter.keccak256(signature);
      // Extract 4-byte selector (8 hex chars) after 0x prefix
      const selector = hash.slice(HEX_PREFIX_LENGTH, HEX_PREFIX_LENGTH + SELECTOR_HEX_CHARS).toLowerCase();
      selectorMap.set(selector, element.name);
    }
  }

  return selectorMap;
}

/**
 * Extracts selectors from a normalized artifact.
 * Uses pre-computed methodIdentifiers for Foundry artifacts.
 *
 * @param adapter - Web3Adapter for keccak256 hashing (only used if no pre-computed selectors)
 * @param artifact - Normalized artifact
 */
export function extractSelectorsFromArtifact(adapter: Web3Adapter, artifact: NormalizedArtifact): Map<string, string> {
  // Foundry provides pre-computed selectors
  if (artifact.methodIdentifiers && artifact.methodIdentifiers.size > 0) {
    return artifact.methodIdentifiers;
  }

  // Fall back to computing from ABI
  return extractSelectorsFromAbi(adapter, artifact.abi);
}

/**
 * Extracts error selectors from an ABI.
 */
export function extractErrorSelectorsFromAbi(adapter: Web3Adapter, abi: AbiElement[]): Map<string, string> {
  const selectorMap = new Map<string, string>();

  for (const element of abi) {
    if (element.type === "error" && element.name) {
      const signature = getFunctionSignature(element);
      const hash = adapter.keccak256(signature);
      // Extract 4-byte selector (8 hex chars) after 0x prefix
      const selector = hash.slice(HEX_PREFIX_LENGTH, HEX_PREFIX_LENGTH + SELECTOR_HEX_CHARS).toLowerCase();
      selectorMap.set(selector, element.name);
    }
  }

  return selectorMap;
}

/**
 * Compares ABI selectors against bytecode-extracted selectors.
 */
export function compareSelectors(abiSelectors: Map<string, string>, bytecodeSelectors: string[]): AbiComparisonResult {
  const abiSelectorSet = new Set(abiSelectors.keys());
  const bytecodeSelectorSet = new Set(bytecodeSelectors);

  const missingSelectors: string[] = [];
  const extraSelectors: string[] = [];

  // Find selectors in ABI but not in bytecode
  for (const entry of Array.from(abiSelectors.entries())) {
    const [selector, name] = entry;
    if (!bytecodeSelectorSet.has(selector)) {
      missingSelectors.push(`${selector} (${name})`);
    }
  }

  // Find selectors in bytecode but not in ABI
  for (const selector of bytecodeSelectors) {
    if (!abiSelectorSet.has(selector)) {
      extraSelectors.push(selector);
    }
  }

  const localSelectors = Array.from(abiSelectors.keys()).sort();
  const remoteSelectors = [...bytecodeSelectors].sort();

  if (missingSelectors.length === 0) {
    return {
      status: "pass",
      message: `All ${abiSelectors.size} ABI function selectors found in bytecode`,
      localSelectors,
      remoteSelectors,
    };
  }

  return {
    status: "fail",
    message: `${missingSelectors.length} ABI function selectors not found in bytecode`,
    localSelectors,
    remoteSelectors,
    missingSelectors,
    extraSelectors:
      extraSelectors.length > MAX_EXTRA_SELECTORS_TO_REPORT
        ? extraSelectors.slice(0, MAX_EXTRA_SELECTORS_TO_REPORT)
        : extraSelectors,
  };
}
