/**
 * Contract Integrity Verifier - Browser-Safe Verifier
 *
 * This module contains only browser-compatible verification methods.
 * It does NOT include methods that depend on Node.js 'fs' module.
 *
 * For Node.js usage with file-based verification, use the main Verifier from ./verifier.
 */

import { EIP1967_IMPLEMENTATION_SLOT } from "./constants";
import {
  ContractConfig,
  ChainConfig,
  ContractVerificationResult,
  StateVerificationConfig,
  StateVerificationResult,
  AbiElement,
  NormalizedArtifact,
  StorageSchema,
} from "./types";
import { extractSelectorsFromArtifact, compareSelectors } from "./utils/abi";
import { extractSelectorsFromBytecode } from "./utils/bytecode";
import { formatError } from "./utils/errors";
import { calculateErc7201BaseSlot, verifySlot, verifyNamespace, verifyStoragePath } from "./utils/storage";
import {
  performBytecodeVerification,
  aggregateStateResults,
  executeViewCallShared,
  extractAddressFromSlot,
  getBytecodeLength,
} from "./utils/verification-helpers";

import type { Web3Adapter } from "./adapter";

/**
 * Options for verification operations.
 */
export interface VerifyOptions {
  verbose?: boolean;
  skipBytecode?: boolean;
  skipAbi?: boolean;
  skipState?: boolean;
  contractFilter?: string;
  chainFilter?: string;
}

/**
 * Content for browser-based verification.
 * Contains pre-loaded artifact and optional schema.
 */
export interface VerificationContent {
  /** Pre-loaded and parsed artifact */
  artifact: NormalizedArtifact;
  /** Pre-loaded and parsed storage schema (if needed for state verification) */
  schema?: StorageSchema;
}

/**
 * Browser-safe Verifier class.
 * Requires a Web3Adapter for blockchain interactions.
 *
 * This version only includes methods that don't require Node.js 'fs' module.
 * Use verifyContractWithContent() instead of verifyContract() for browser usage.
 */
export class BrowserVerifier {
  constructor(private readonly adapter: Web3Adapter) {}

  /**
   * Fetches bytecode from chain at a given address.
   */
  async fetchBytecode(address: string): Promise<string> {
    const bytecode = await this.adapter.getCode(address);
    if (bytecode === "0x" || bytecode === "") {
      throw new Error(`No bytecode found at address ${address}`);
    }
    return bytecode;
  }

  /**
   * Checks if a contract is an EIP-1967 proxy and returns the implementation address.
   */
  async getImplementationAddress(address: string): Promise<string | null> {
    try {
      const implementationSlot = await this.adapter.getStorageAt(address, EIP1967_IMPLEMENTATION_SLOT);
      const rawAddress = extractAddressFromSlot(implementationSlot);
      const implementationAddress = this.adapter.checksumAddress(rawAddress);

      if (implementationAddress === this.adapter.zeroAddress) {
        return null;
      }

      return implementationAddress;
    } catch {
      return null;
    }
  }

  /**
   * Calculates ERC-7201 base slot for a namespace.
   */
  calculateErc7201Slot(namespaceId: string): string {
    return calculateErc7201BaseSlot(this.adapter, namespaceId);
  }

  /**
   * Verifies a contract using pre-loaded content (browser-compatible).
   * Use this method in browser environments where artifacts are loaded via fetch/upload.
   */
  async verifyContractWithContent(
    contract: ContractConfig,
    chain: ChainConfig,
    options: VerifyOptions = {},
    content: VerificationContent,
  ): Promise<ContractVerificationResult> {
    const result: ContractVerificationResult = {
      contract,
      chain,
    };

    try {
      const artifact = content.artifact;

      if (options.verbose) {
        console.log(`  Artifact format: ${artifact.format}`);
        if (artifact.immutableReferences && artifact.immutableReferences.length > 0) {
          console.log(`  Known immutable positions: ${artifact.immutableReferences.length}`);
        }
      }

      // Fetch on-chain bytecode
      let remoteBytecode = await this.fetchBytecode(contract.address);
      let addressUsed = contract.address;

      // If marked as proxy, get implementation bytecode
      if (contract.isProxy) {
        const implAddress = await this.getImplementationAddress(contract.address);
        if (implAddress) {
          if (options.verbose) {
            console.log(`  Proxy detected, fetching implementation at ${implAddress}`);
          }
          remoteBytecode = await this.fetchBytecode(implAddress);
          addressUsed = implAddress;
        } else {
          console.warn(
            `  Warning: Contract marked as proxy but no EIP-1967 implementation found at ${contract.address}`,
          );
        }
      }

      // Bytecode verification using shared logic
      if (!options.skipBytecode) {
        const bytecodeVerification = performBytecodeVerification({
          artifact,
          remoteBytecode,
          constructorArgs: contract.constructorArgs,
          immutableValues: contract.immutableValues,
          verbose: options.verbose,
        });

        result.bytecodeResult = bytecodeVerification.bytecodeResult;
        if (bytecodeVerification.immutableValuesResult) {
          result.immutableValuesResult = bytecodeVerification.immutableValuesResult;
        }
        if (bytecodeVerification.definitiveResult) {
          result.definitiveResult = bytecodeVerification.definitiveResult;
        }
        if (bytecodeVerification.groupedImmutables) {
          result.groupedImmutables = bytecodeVerification.groupedImmutables;
        }
      }

      // ABI verification
      if (!options.skipAbi) {
        const abiSelectors = extractSelectorsFromArtifact(this.adapter, artifact);
        const bytecodeSelectors = extractSelectorsFromBytecode(remoteBytecode);
        result.abiResult = compareSelectors(abiSelectors, bytecodeSelectors);
      }

      // State verification (browser-compatible version)
      if (!options.skipState && contract.stateVerification) {
        result.stateResult = await this.verifyStateWithContent(
          contract.address,
          artifact.abi,
          contract.stateVerification,
          content.schema,
        );
      }

      if (options.verbose) {
        console.log(`  Address verified: ${addressUsed}`);
        console.log(`  Remote bytecode length: ${getBytecodeLength(remoteBytecode)} bytes`);
      }
    } catch (error) {
      result.error = formatError(error);
    }

    return result;
  }

  /**
   * Verifies state using pre-loaded schema (browser-compatible).
   */
  async verifyStateWithContent(
    address: string,
    abi: AbiElement[],
    config: StateVerificationConfig,
    schema?: StorageSchema,
  ): Promise<StateVerificationResult> {
    // Warn if storage paths are configured but schema is missing
    const storagePathsSkipped = !!(config.storagePaths && config.storagePaths.length > 0 && !schema);
    const skippedCount = storagePathsSkipped ? config.storagePaths!.length : 0;

    const [viewCallResults, namespaceResults, slotResults, storagePathResults] = await Promise.all([
      // 1. Verify view calls in parallel
      config.viewCalls && config.viewCalls.length > 0
        ? Promise.all(config.viewCalls.map((call) => executeViewCallShared(this.adapter, address, abi, call)))
        : Promise.resolve([]),

      // 2. Verify ERC-7201 namespaces in parallel
      config.namespaces && config.namespaces.length > 0
        ? Promise.all(config.namespaces.map((ns) => verifyNamespace(this.adapter, address, ns)))
        : Promise.resolve([]),

      // 3. Verify explicit slots in parallel
      config.slots && config.slots.length > 0
        ? Promise.all(config.slots.map((slot) => verifySlot(this.adapter, address, slot)))
        : Promise.resolve([]),

      // 4. Verify storage paths (schema-based) in parallel
      config.storagePaths && config.storagePaths.length > 0 && schema
        ? Promise.all(
            config.storagePaths.map((pathConfig) => verifyStoragePath(this.adapter, address, pathConfig, schema)),
          )
        : Promise.resolve([]),
    ]);

    // Aggregate results using shared logic
    const aggregation = aggregateStateResults(
      viewCallResults,
      namespaceResults,
      slotResults,
      storagePathResults,
      storagePathsSkipped,
      skippedCount,
    );

    return {
      status: aggregation.status,
      message: aggregation.message,
      viewCallResults: viewCallResults.length > 0 ? viewCallResults : undefined,
      namespaceResults: namespaceResults.length > 0 ? namespaceResults : undefined,
      slotResults: slotResults.length > 0 ? slotResults : undefined,
      storagePathResults: storagePathResults.length > 0 ? storagePathResults : undefined,
    };
  }
}

// Export Verifier as alias for backwards compatibility
export { BrowserVerifier as Verifier };
