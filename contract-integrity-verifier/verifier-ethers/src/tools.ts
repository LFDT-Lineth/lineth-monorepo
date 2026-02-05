/**
 * Tools with Ethers crypto operations pre-configured.
 *
 * These exports provide the same functionality as the core tools,
 * but with ethers' crypto implementation automatically wired in.
 *
 * @packageDocumentation
 */

import {
  generateSchema as coreGenerateSchema,
  parseSoliditySource as coreParseSoliditySource,
  mergeSchemas as coreMergeSchemas,
  calculateErc7201BaseSlotWithAdapter,
} from "@consensys/linea-contract-integrity-verifier";
import { ethers } from "ethers";

import type {
  CryptoAdapter,
  SchemaGeneratorOptions,
  ParseResult,
  Schema,
} from "@consensys/linea-contract-integrity-verifier";

/**
 * Ethers implementation of CryptoAdapter.
 * Provides crypto operations without requiring an RPC connection.
 */
class EthersCryptoAdapter implements CryptoAdapter {
  keccak256(value: string | Uint8Array): string {
    if (typeof value === "string") {
      if (value.startsWith("0x")) {
        return ethers.keccak256(value);
      }
      return ethers.keccak256(ethers.toUtf8Bytes(value));
    }
    return ethers.keccak256(value);
  }

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    return ethers.AbiCoder.defaultAbiCoder().encode([...types], [...values]);
  }
}

// Singleton adapter instance
const cryptoAdapter = new EthersCryptoAdapter();

/**
 * Create a CryptoAdapter using ethers.
 * Useful if you need to pass the adapter to core functions directly.
 */
export function createCryptoAdapter(): CryptoAdapter {
  return new EthersCryptoAdapter();
}

// Re-export types
export type { Schema, SchemaGeneratorOptions, ParseResult };
export type { StructDef, FieldDef } from "@consensys/linea-contract-integrity-verifier";

/**
 * Calculate ERC-7201 base slot from namespace ID using ethers.
 *
 * @param namespaceId - The namespace identifier (e.g., "linea.storage.MyContract")
 * @returns The computed base slot as a hex string
 */
export function calculateErc7201BaseSlot(namespaceId: string): string {
  return calculateErc7201BaseSlotWithAdapter(cryptoAdapter, namespaceId);
}

/**
 * Parse a Solidity source file and extract storage schema.
 *
 * @param source - Solidity source code
 * @param fileName - Optional filename for error messages
 * @param options - Parser options
 */
export function parseSoliditySource(
  source: string,
  fileName?: string,
  options: SchemaGeneratorOptions = {},
): ParseResult {
  return coreParseSoliditySource(cryptoAdapter, source, fileName, options);
}

/**
 * Generate a storage schema from multiple Solidity sources.
 *
 * @param sources - Array of { source, fileName } objects
 * @param options - Generator options
 */
export function generateSchema(
  sources: Array<{ source: string; fileName?: string }>,
  options: SchemaGeneratorOptions = {},
): ParseResult {
  return coreGenerateSchema(cryptoAdapter, sources, options);
}

/**
 * Merge multiple schemas into one.
 * Re-exported directly from core (no crypto needed).
 */
export const mergeSchemas = coreMergeSchemas;
