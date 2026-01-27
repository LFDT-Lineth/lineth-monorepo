/**
 * Contract Integrity Verifier - ABI Utilities
 *
 * Utilities for parsing ABI files and comparing function selectors.
 * Supports both Hardhat and Foundry artifact formats.
 *
 * Note: Selector computation requires a Web3Adapter for hashing.
 */

import { readFileSync } from "fs";
import { basename } from "path";

import {
  AbiElement,
  AbiInput,
  AbiComparisonResult,
  NormalizedArtifact,
  ArtifactFormat,
  HardhatArtifact,
  FoundryArtifact,
  ImmutableReference,
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
 * Loads and normalizes an artifact file (Hardhat or Foundry).
 * @throws Error with descriptive message if file cannot be read or parsed
 */
export function loadArtifact(filePath: string): NormalizedArtifact {
  let content: string;
  try {
    content = readFileSync(filePath, "utf-8");
  } catch (err) {
    throw new Error(`Failed to read artifact file at ${filePath}: ${err instanceof Error ? err.message : String(err)}`);
  }

  let raw: unknown;
  try {
    raw = JSON.parse(content);
  } catch (err) {
    throw new Error(
      `Failed to parse artifact JSON at ${filePath}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }

  if (isFoundryArtifact(raw)) {
    return normalizeFoundryArtifact(raw, filePath);
  }

  return normalizeHardhatArtifact(raw as HardhatArtifact);
}

/**
 * Normalizes a Hardhat artifact to the common format.
 */
function normalizeHardhatArtifact(artifact: HardhatArtifact): NormalizedArtifact {
  return {
    format: "hardhat",
    contractName: artifact.contractName,
    abi: artifact.abi,
    bytecode: artifact.bytecode,
    deployedBytecode: artifact.deployedBytecode,
    immutableReferences: undefined,
    methodIdentifiers: undefined,
  };
}

/**
 * Normalizes a Foundry artifact to the common format.
 */
function normalizeFoundryArtifact(artifact: FoundryArtifact, filePath: string): NormalizedArtifact {
  const fileName = basename(filePath, ".json");
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

  return {
    format: "foundry",
    contractName,
    abi: artifact.abi,
    bytecode: artifact.bytecode.object,
    deployedBytecode: artifact.deployedBytecode.object,
    immutableReferences,
    methodIdentifiers,
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
      const selector = hash.slice(2, 10).toLowerCase();
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
      const selector = hash.slice(2, 10).toLowerCase();
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
    extraSelectors: extraSelectors.length > 10 ? extraSelectors.slice(0, 10) : extraSelectors,
  };
}
