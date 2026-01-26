/**
 * Contract Integrity Verifier - Web3 Adapter Interface
 *
 * Abstract interface for web3 operations. Implementations can use
 * ethers, viem, or any web3 library.
 *
 * @packageDocumentation
 */

import type { AbiElement } from "./types";

/**
 * Abstract interface for web3 operations.
 *
 * This interface abstracts all blockchain interactions, allowing the core
 * verifier to work with any web3 library (ethers, viem, etc.).
 *
 * @example
 * ```typescript
 * import { EthersAdapter } from "@consensys/linea-verifier-ethers";
 * import { Verifier } from "@consensys/linea-verifier-core";
 *
 * const adapter = new EthersAdapter("https://rpc.linea.build");
 * const verifier = new Verifier(adapter);
 * ```
 */
export interface Web3Adapter {
  // ============================================
  // Crypto Operations (synchronous)
  // ============================================

  /**
   * Compute keccak256 hash.
   * @param value - UTF-8 string or raw bytes to hash
   * @returns Hex-encoded hash with 0x prefix
   */
  keccak256(value: string | Uint8Array): string;

  /**
   * Convert address to checksummed format (EIP-55).
   * @param address - Address to checksum (with or without 0x prefix)
   * @returns Checksummed address with 0x prefix
   * @throws If address is invalid
   */
  checksumAddress(address: string): string;

  /**
   * The zero address constant.
   */
  readonly zeroAddress: string;

  // ============================================
  // ABI Operations (synchronous)
  // ============================================

  /**
   * ABI-encode values with given types.
   * @param types - Solidity type strings (e.g., ["uint256", "address"])
   * @param values - Values to encode
   * @returns Hex-encoded ABI data with 0x prefix
   */
  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string;

  /**
   * Encode function call data from ABI.
   * @param abi - Contract ABI
   * @param functionName - Name of function to call
   * @param args - Function arguments (optional)
   * @returns Hex-encoded call data with 0x prefix
   */
  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string;

  /**
   * Decode function return data from ABI.
   * @param abi - Contract ABI
   * @param functionName - Name of function that was called
   * @param data - Hex-encoded return data
   * @returns Decoded return values as array
   */
  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[];

  // ============================================
  // RPC Operations (asynchronous)
  // ============================================

  /**
   * Get deployed bytecode at address.
   * @param address - Contract address
   * @returns Hex-encoded bytecode (or "0x" if no code)
   */
  getCode(address: string): Promise<string>;

  /**
   * Read storage slot value.
   * @param address - Contract address
   * @param slot - Storage slot (hex string)
   * @returns Hex-encoded 32-byte value
   */
  getStorageAt(address: string, slot: string): Promise<string>;

  /**
   * Execute read-only contract call.
   * @param to - Contract address
   * @param data - Hex-encoded call data
   * @returns Hex-encoded return data
   */
  call(to: string, data: string): Promise<string>;
}

/**
 * Options for creating a Web3Adapter.
 */
export interface Web3AdapterOptions {
  /** JSON-RPC URL */
  rpcUrl: string;
  /** Optional chain ID for network validation */
  chainId?: number;
}
