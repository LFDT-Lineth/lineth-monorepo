/**
 * Contract Integrity Verifier - Core Verification Logic
 *
 * Main verification engine that fetches on-chain bytecode and compares
 * against local artifact files.
 */

import type { Web3Adapter } from "./adapter";
import {
  VerifierConfig,
  ContractConfig,
  ChainConfig,
  ContractVerificationResult,
  VerificationSummary,
  StateVerificationConfig,
  StateVerificationResult,
  ViewCallResult,
  AbiElement,
} from "./types";
import { compareBytecode, extractSelectorsFromBytecode, validateImmutablesAgainstArgs } from "./utils/bytecode";
import { loadArtifact, extractSelectorsFromArtifact, compareSelectors } from "./utils/abi";
import {
  calculateErc7201BaseSlot,
  verifySlot,
  verifyNamespace,
  verifyStoragePath,
  loadStorageSchema,
} from "./utils/storage";
import { EIP1967_IMPLEMENTATION_SLOT } from "./constants";

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
 * Main Verifier class.
 * Requires a Web3Adapter for blockchain interactions.
 */
export class Verifier {
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
      const implementationAddress = this.adapter.checksumAddress("0x" + implementationSlot.slice(-40));

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
   * Verifies a single contract.
   */
  async verifyContract(
    contract: ContractConfig,
    chain: ChainConfig,
    options: VerifyOptions = {},
    configDir: string = ".",
  ): Promise<ContractVerificationResult> {
    const result: ContractVerificationResult = {
      contract,
      chain,
    };

    try {
      // Load artifact
      const artifact = loadArtifact(contract.artifactFile);

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

      // Bytecode comparison
      if (!options.skipBytecode) {
        result.bytecodeResult = compareBytecode(
          artifact.deployedBytecode,
          remoteBytecode,
          artifact.immutableReferences,
        );

        // Validate immutables against constructor args if provided
        if (
          result.bytecodeResult.onlyImmutablesDiffer &&
          result.bytecodeResult.immutableDifferences &&
          contract.constructorArgs
        ) {
          const validation = validateImmutablesAgainstArgs(
            result.bytecodeResult.immutableDifferences,
            contract.constructorArgs,
            options.verbose,
          );

          if (options.verbose && validation.details) {
            for (const detail of validation.details) {
              console.log(`    ${detail}`);
            }
          }

          if (validation.valid) {
            result.bytecodeResult.message += ` - constructor args validated`;
          } else {
            result.bytecodeResult.status = "warn";
            result.bytecodeResult.message += ` - ${validation.message}`;
          }
        }
      }

      // ABI comparison
      if (!options.skipAbi) {
        const abiSelectors = extractSelectorsFromArtifact(this.adapter, artifact);
        const bytecodeSelectors = extractSelectorsFromBytecode(remoteBytecode);
        result.abiResult = compareSelectors(abiSelectors, bytecodeSelectors);
      }

      // State verification
      if (!options.skipState && contract.stateVerification) {
        result.stateResult = await this.verifyState(
          contract.address,
          artifact.abi,
          contract.stateVerification,
          configDir,
        );
      }

      if (options.verbose) {
        console.log(`  Address verified: ${addressUsed}`);
        console.log(`  Remote bytecode length: ${(remoteBytecode.length - 2) / 2} bytes`);
      }
    } catch (error) {
      result.error = error instanceof Error ? error.message : String(error);
    }

    return result;
  }

  /**
   * Performs complete state verification for a contract.
   * All verification types run in parallel for efficiency.
   */
  async verifyState(
    address: string,
    abi: AbiElement[],
    config: StateVerificationConfig,
    configDir: string = ".",
  ): Promise<StateVerificationResult> {
    // Run all verification types in parallel for efficiency
    const [viewCallResults, namespaceResults, slotResults, storagePathResults] = await Promise.all([
      // 1. Execute view calls in parallel
      config.viewCalls && config.viewCalls.length > 0
        ? Promise.all(config.viewCalls.map((viewCall) => this.executeViewCall(address, abi, viewCall)))
        : Promise.resolve([]),

      // 2. Verify namespaces (ERC-7201) in parallel
      config.namespaces && config.namespaces.length > 0
        ? Promise.all(config.namespaces.map((namespace) => verifyNamespace(this.adapter, address, namespace)))
        : Promise.resolve([]),

      // 3. Verify explicit slots in parallel
      config.slots && config.slots.length > 0
        ? Promise.all(config.slots.map((slot) => verifySlot(this.adapter, address, slot)))
        : Promise.resolve([]),

      // 4. Verify storage paths (schema-based) in parallel
      config.storagePaths && config.storagePaths.length > 0 && config.schemaFile && configDir
        ? (async () => {
            const schema = loadStorageSchema(config.schemaFile!, configDir);
            return Promise.all(
              config.storagePaths!.map((pathConfig) => verifyStoragePath(this.adapter, address, pathConfig, schema)),
            );
          })()
        : Promise.resolve([]),
    ]);

    // Aggregate results
    const allViewCallsPass = viewCallResults.every((r) => r.status === "pass");
    const allNamespacesPass = namespaceResults.every((r) => r.status === "pass");
    const allSlotsPass = slotResults.every((r) => r.status === "pass");
    const allStoragePathsPass = storagePathResults.every((r) => r.status === "pass");

    const totalChecks =
      viewCallResults.length + namespaceResults.length + slotResults.length + storagePathResults.length;
    const passedChecks =
      viewCallResults.filter((r) => r.status === "pass").length +
      namespaceResults.filter((r) => r.status === "pass").length +
      slotResults.filter((r) => r.status === "pass").length +
      storagePathResults.filter((r) => r.status === "pass").length;

    const allPass = allViewCallsPass && allNamespacesPass && allSlotsPass && allStoragePathsPass;

    return {
      status: allPass ? "pass" : "fail",
      message: allPass
        ? `All ${totalChecks} state checks passed`
        : `${passedChecks}/${totalChecks} state checks passed`,
      viewCallResults: viewCallResults.length > 0 ? viewCallResults : undefined,
      namespaceResults: namespaceResults.length > 0 ? namespaceResults : undefined,
      slotResults: slotResults.length > 0 ? slotResults : undefined,
      storagePathResults: storagePathResults.length > 0 ? storagePathResults : undefined,
    };
  }

  /**
   * Executes a view function call and returns the result.
   */
  async executeViewCall(
    address: string,
    abi: AbiElement[],
    config: import("./types").ViewCallConfig,
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

      const calldata = this.adapter.encodeFunctionData(abi, config.function, config.params ?? []);
      const result = await this.adapter.call(address, calldata);
      const decoded = this.adapter.decodeFunctionResult(abi, config.function, result);

      const actual = decoded.length === 1 ? formatValue(decoded[0]) : decoded.map(formatValue);

      const comparison = config.comparison ?? "eq";
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
        message: `Call failed: ${error instanceof Error ? error.message : String(error)}`,
      };
    }
  }

  /**
   * Runs verification for all contracts in a configuration.
   */
  async verify(
    config: VerifierConfig,
    options: VerifyOptions = {},
    configDir: string = ".",
  ): Promise<VerificationSummary> {
    const results: ContractVerificationResult[] = [];
    let passed = 0;
    let failed = 0;
    let warnings = 0;
    let skipped = 0;

    // Filter contracts if specified
    let contractsToVerify = config.contracts;
    if (options.contractFilter) {
      contractsToVerify = contractsToVerify.filter(
        (c) => c.name.toLowerCase() === options.contractFilter!.toLowerCase(),
      );
    }
    if (options.chainFilter) {
      contractsToVerify = contractsToVerify.filter((c) => c.chain.toLowerCase() === options.chainFilter!.toLowerCase());
    }

    if (contractsToVerify.length === 0) {
      console.log("No contracts to verify matching the specified filters.");
      return { total: 0, passed: 0, failed: 0, warnings: 0, skipped: 0, results: [] };
    }

    console.log(`\nVerifying ${contractsToVerify.length} contract(s)...\n`);

    for (const contract of contractsToVerify) {
      const chain = config.chains[contract.chain];
      if (!chain) {
        console.log(`Skipping ${contract.name}: unknown chain '${contract.chain}'`);
        skipped++;
        continue;
      }

      console.log(`Verifying ${contract.name} on ${contract.chain}...`);
      console.log(`  Address: ${contract.address}`);

      const result = await this.verifyContract(contract, chain, options, configDir);
      results.push(result);

      if (result.error) {
        console.log(`  ERROR: ${result.error}`);
        failed++;
      } else {
        // Print results
        if (result.bytecodeResult) {
          const br = result.bytecodeResult;
          const icon = br.status === "pass" ? "✓" : br.status === "fail" ? "✗" : br.status === "warn" ? "!" : "-";
          console.log(`  Bytecode: ${icon} ${br.message}`);
        }

        if (result.abiResult) {
          const ar = result.abiResult;
          const icon = ar.status === "pass" ? "✓" : ar.status === "fail" ? "✗" : ar.status === "warn" ? "!" : "-";
          console.log(`  ABI: ${icon} ${ar.message}`);
        }

        if (result.stateResult) {
          const sr = result.stateResult;
          const icon = sr.status === "pass" ? "✓" : sr.status === "fail" ? "✗" : sr.status === "warn" ? "!" : "-";
          console.log(`  State: ${icon} ${sr.message}`);
        }

        // Count results
        const bytecodeStatus = result.bytecodeResult?.status;
        const abiStatus = result.abiResult?.status;
        const stateStatus = result.stateResult?.status;

        if (bytecodeStatus === "fail" || abiStatus === "fail" || stateStatus === "fail") {
          failed++;
        } else if (bytecodeStatus === "warn" || abiStatus === "warn" || stateStatus === "warn") {
          warnings++;
        } else {
          const hasBytecodeResult = result.bytecodeResult !== undefined;
          const hasAbiResult = result.abiResult !== undefined;
          const hasStateResult = result.stateResult !== undefined;
          const hasAnyVerification = hasBytecodeResult || hasAbiResult || hasStateResult;

          if (!hasAnyVerification) {
            skipped++;
          } else if (
            (bytecodeStatus === "skip" || !hasBytecodeResult) &&
            (abiStatus === "skip" || !hasAbiResult) &&
            !hasStateResult
          ) {
            skipped++;
          } else {
            passed++;
          }
        }
      }

      console.log("");
    }

    return {
      total: contractsToVerify.length,
      passed,
      failed,
      warnings,
      skipped,
      results,
    };
  }
}

/**
 * Prints a summary of verification results.
 */
export function printSummary(summary: VerificationSummary): void {
  console.log("=".repeat(50));
  console.log("VERIFICATION SUMMARY");
  console.log("=".repeat(50));
  console.log(`Total contracts: ${summary.total}`);
  console.log(`  Passed:   ${summary.passed}`);
  console.log(`  Failed:   ${summary.failed}`);
  console.log(`  Warnings: ${summary.warnings}`);
  console.log(`  Skipped:  ${summary.skipped}`);
  console.log("=".repeat(50));

  if (summary.failed > 0) {
    console.log("\nFailed contracts:");
    for (const result of summary.results) {
      const hasBytecodeFailure = result.bytecodeResult?.status === "fail";
      const hasAbiFailure = result.abiResult?.status === "fail";
      const hasStateFailure = result.stateResult?.status === "fail";
      const hasError = result.error;

      if (hasBytecodeFailure || hasAbiFailure || hasStateFailure || hasError) {
        console.log(`  - ${result.contract.name} (${result.contract.chain})`);
        if (hasError) {
          console.log(`    Error: ${result.error}`);
        }
        if (hasBytecodeFailure) {
          console.log(`    Bytecode: ${result.bytecodeResult!.message}`);
        }
        if (hasAbiFailure) {
          console.log(`    ABI: ${result.abiResult!.message}`);
        }
        if (hasStateFailure) {
          console.log(`    State: ${result.stateResult!.message}`);
        }
      }
    }
  }
}

// ============================================================================
// Helper Functions
// ============================================================================

function formatValue(value: unknown): unknown {
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (Array.isArray(value)) {
    return value.map(formatValue);
  }
  return value;
}

function formatForDisplay(value: unknown): string {
  if (typeof value === "string" && value.length > 20) {
    return value.slice(0, 10) + "..." + value.slice(-8);
  }
  return String(value);
}

function isNumericString(value: string): boolean {
  return /^-?\d+$/.test(value) || /^0x[0-9a-fA-F]+$/.test(value);
}

function normalizeForComparison(value: unknown): string {
  if (typeof value === "string") {
    if (value.startsWith("0x") && value.length === 42) {
      return value.toLowerCase();
    }
    return value;
  }
  if (typeof value === "bigint" || typeof value === "number") {
    return String(value);
  }
  if (typeof value === "boolean") {
    return String(value);
  }
  return String(value);
}

function compareValues(actual: unknown, expected: unknown, comparison: string): boolean {
  const normalizedActual = normalizeForComparison(actual);
  const normalizedExpected = normalizeForComparison(expected);

  switch (comparison) {
    case "eq":
      return normalizedActual === normalizedExpected;
    case "gt":
    case "gte":
    case "lt":
    case "lte": {
      if (!isNumericString(normalizedActual) || !isNumericString(normalizedExpected)) {
        return normalizedActual === normalizedExpected;
      }
      const actualBigInt = BigInt(normalizedActual);
      const expectedBigInt = BigInt(normalizedExpected);
      if (comparison === "gt") return actualBigInt > expectedBigInt;
      if (comparison === "gte") return actualBigInt >= expectedBigInt;
      if (comparison === "lt") return actualBigInt < expectedBigInt;
      return actualBigInt <= expectedBigInt;
    }
    case "contains":
      return String(normalizedActual).includes(String(normalizedExpected));
    default:
      return normalizedActual === normalizedExpected;
  }
}
