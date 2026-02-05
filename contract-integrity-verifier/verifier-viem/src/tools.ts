/**
 * Tools with Viem crypto operations pre-configured.
 *
 * These exports provide the same functionality as the core tools,
 * but with viem's crypto implementation automatically wired in.
 *
 * @packageDocumentation
 */

import {
  generateSchema as coreGenerateSchema,
  parseSoliditySource as coreParseSoliditySource,
  mergeSchemas as coreMergeSchemas,
  calculateErc7201BaseSlotWithAdapter,
} from "@consensys/linea-contract-integrity-verifier";
import { keccak256, toBytes, encodeAbiParameters as viemEncodeAbiParameters, parseAbiParameters, type Hex } from "viem";

import type {
  CryptoAdapter,
  SchemaGeneratorOptions,
  ParseResult,
  Schema,
} from "@consensys/linea-contract-integrity-verifier";

/**
 * Viem implementation of CryptoAdapter.
 * Provides crypto operations without requiring an RPC connection.
 */
class ViemCryptoAdapter implements CryptoAdapter {
  keccak256(value: string | Uint8Array): string {
    if (typeof value === "string") {
      if (value.startsWith("0x")) {
        return keccak256(value as Hex);
      }
      return keccak256(toBytes(value));
    }
    return keccak256(value);
  }

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    const params = parseAbiParameters(types.join(", "));
    return viemEncodeAbiParameters(params, values as readonly unknown[]);
  }
}

// Singleton adapter instance
const cryptoAdapter = new ViemCryptoAdapter();

/**
 * Create a CryptoAdapter using viem.
 * Useful if you need to pass the adapter to core functions directly.
 */
export function createCryptoAdapter(): CryptoAdapter {
  return new ViemCryptoAdapter();
}

// Re-export types
export type { Schema, SchemaGeneratorOptions, ParseResult };
export type { StructDef, FieldDef } from "@consensys/linea-contract-integrity-verifier";

/**
 * Calculate ERC-7201 base slot from namespace ID using viem.
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
