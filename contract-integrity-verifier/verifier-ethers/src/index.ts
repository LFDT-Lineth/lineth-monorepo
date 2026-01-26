/**
 * @consensys/linea-contract-integrity-verifier-ethers
 *
 * Ethers v6 adapter for @consensys/linea-contract-integrity-verifier
 *
 * @packageDocumentation
 */

import { ethers } from "ethers";
import type { Web3Adapter, Web3AdapterOptions, AbiElement } from "@consensys/linea-contract-integrity-verifier";

/**
 * Options for creating an EthersAdapter.
 */
export interface EthersAdapterOptions extends Web3AdapterOptions {
  /** Optional pre-configured provider */
  provider?: ethers.JsonRpcProvider;
}

/**
 * Ethers v6 implementation of Web3Adapter.
 *
 * @example
 * ```typescript
 * import { EthersAdapter } from "@consensys/linea-contract-integrity-verifier-ethers";
 * import { Verifier } from "@consensys/linea-contract-integrity-verifier";
 *
 * const adapter = new EthersAdapter({ rpcUrl: "https://rpc.linea.build" });
 * const verifier = new Verifier(adapter);
 *
 * const result = await verifier.verify(config);
 * ```
 */
export class EthersAdapter implements Web3Adapter {
  private readonly provider: ethers.JsonRpcProvider;

  constructor(options: EthersAdapterOptions | string) {
    if (typeof options === "string") {
      this.provider = new ethers.JsonRpcProvider(options);
    } else if (options.provider) {
      this.provider = options.provider;
    } else {
      this.provider = new ethers.JsonRpcProvider(
        options.rpcUrl,
        options.chainId ? { chainId: options.chainId, name: `chain-${options.chainId}` } : undefined,
      );
    }
  }

  // ============================================
  // Crypto Operations
  // ============================================

  keccak256(value: string | Uint8Array): string {
    if (typeof value === "string") {
      // If it looks like a hex string, hash it directly
      if (value.startsWith("0x")) {
        return ethers.keccak256(value);
      }
      // Otherwise treat as UTF-8 string
      return ethers.keccak256(ethers.toUtf8Bytes(value));
    }
    return ethers.keccak256(value);
  }

  checksumAddress(address: string): string {
    return ethers.getAddress(address);
  }

  get zeroAddress(): string {
    return ethers.ZeroAddress;
  }

  // ============================================
  // ABI Operations
  // ============================================

  encodeAbiParameters(types: readonly string[], values: readonly unknown[]): string {
    return ethers.AbiCoder.defaultAbiCoder().encode([...types], [...values]);
  }

  encodeFunctionData(abi: readonly AbiElement[], functionName: string, args?: readonly unknown[]): string {
    const iface = new ethers.Interface(abi as ethers.InterfaceAbi);
    return iface.encodeFunctionData(functionName, args ? [...args] : []);
  }

  decodeFunctionResult(abi: readonly AbiElement[], functionName: string, data: string): readonly unknown[] {
    const iface = new ethers.Interface(abi as ethers.InterfaceAbi);
    const result = iface.decodeFunctionResult(functionName, data);
    return Array.from(result);
  }

  // ============================================
  // RPC Operations
  // ============================================

  async getCode(address: string): Promise<string> {
    const code = await this.provider.getCode(address);
    return code;
  }

  async getStorageAt(address: string, slot: string): Promise<string> {
    const value = await this.provider.getStorage(address, slot);
    return value;
  }

  async call(to: string, data: string): Promise<string> {
    const result = await this.provider.call({ to, data });
    return result;
  }
}

/**
 * Creates an EthersAdapter instance.
 * Convenience function for quick setup.
 */
export function createEthersAdapter(rpcUrl: string, chainId?: number): EthersAdapter {
  return new EthersAdapter({ rpcUrl, chainId });
}
