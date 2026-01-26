/**
 * @consensys/linea-contract-integrity-verifier-viem
 *
 * Viem adapter for @consensys/linea-contract-integrity-verifier
 *
 * @packageDocumentation
 */

import {
  createPublicClient,
  http,
  keccak256,
  toBytes,
  getAddress,
  zeroAddress,
  encodeAbiParameters,
  parseAbiParameters,
  encodeFunctionData,
  decodeFunctionResult,
  type PublicClient,
  type Hex,
  type Chain,
} from "viem";
import type { Web3Adapter, Web3AdapterOptions, AbiElement } from "@consensys/linea-contract-integrity-verifier";

/**
 * Options for creating a ViemAdapter.
 */
export interface ViemAdapterOptions extends Web3AdapterOptions {
  /** Optional pre-configured client */
  client?: PublicClient;
  /** Optional chain configuration */
  chain?: Chain;
}

/**
 * Viem implementation of Web3Adapter.
 *
 * @example
 * ```typescript
 * import { ViemAdapter } from "@consensys/linea-contract-integrity-verifier-viem";
 * import { Verifier } from "@consensys/linea-contract-integrity-verifier";
 *
 * const adapter = new ViemAdapter({ rpcUrl: "https://rpc.linea.build" });
 * const verifier = new Verifier(adapter);
 *
 * const result = await verifier.verify(config);
 * ```
 */
export class ViemAdapter implements Web3Adapter {
  private readonly client: PublicClient;

  constructor(options: ViemAdapterOptions | string) {
    if (typeof options === "string") {
      this.client = createPublicClient({
        transport: http(options),
      });
    } else if (options.client) {
      this.client = options.client;
    } else {
      this.client = createPublicClient({
        transport: http(options.rpcUrl),
        chain: options.chain,
      });
    }
  }

  // ============================================
  // Crypto Operations
  // ============================================

  keccak256(value: string | Uint8Array): string {
    if (typeof value === "string") {
      // If it looks like a hex string, hash it directly
      if (value.startsWith("0x")) {
        return keccak256(value as Hex);
      }
      // Otherwise treat as UTF-8 string
      return keccak256(toBytes(value));
    }
    return keccak256(value);
  }

  checksumAddress(address: string): string {
    return getAddress(address);
  }

  get zeroAddress(): string {
    return zeroAddress;
  }

  // ============================================
  // ABI Operations
  // ============================================

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    // Convert string types to viem AbiParameter format
    const params = parseAbiParameters(types.join(", "));
    return encodeAbiParameters(params, values as readonly unknown[]);
  }

  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
    return encodeFunctionData({
      abi: abi as readonly unknown[],
      functionName,
      args: args ? [...args] : [],
    });
  }

  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
    const result = decodeFunctionResult({
      abi: abi as readonly unknown[],
      functionName,
      data: data as Hex,
    });
    // Ensure we always return an array
    if (Array.isArray(result)) {
      return result;
    }
    return [result];
  }

  // ============================================
  // RPC Operations
  // ============================================

  async getCode(address: string): Promise<string> {
    const code = await this.client.getCode({ address: address as Hex });
    return code ?? "0x";
  }

  async getStorageAt(address: string, slot: string): Promise<string> {
    const value = await this.client.getStorageAt({
      address: address as Hex,
      slot: slot as Hex,
    });
    return value ?? "0x" + "0".repeat(64);
  }

  async call(to: string, data: string): Promise<string> {
    const result = await this.client.call({
      to: to as Hex,
      data: data as Hex,
    });
    return result.data ?? "0x";
  }
}

/**
 * Creates a ViemAdapter instance.
 * Convenience function for quick setup.
 */
export function createViemAdapter(rpcUrl: string, chain?: Chain): ViemAdapter {
  return new ViemAdapter({ rpcUrl, chain });
}

/**
 * Creates a ViemAdapter with a pre-configured client.
 */
export function createViemAdapterFromClient(client: PublicClient): ViemAdapter {
  return new ViemAdapter({ rpcUrl: "", client });
}
