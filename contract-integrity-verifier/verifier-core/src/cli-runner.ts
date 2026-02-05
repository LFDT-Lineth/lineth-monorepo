#!/usr/bin/env node
/**
 * Contract Integrity Verifier - Shared CLI Runner
 *
 * Common CLI logic shared between ethers and viem adapter packages.
 * Each adapter package provides the adapter factory and package metadata.
 */

import { resolve, dirname } from "path";

import { loadConfig } from "./config";
import { Verifier, printSummary } from "./verifier";

import type { Web3Adapter } from "./adapter";
import type { ContractVerificationResult, ChainConfig } from "./types";

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Truncate long values for display (e.g., addresses, hashes)
 */
export function truncateValue(value: string, maxLength: number = 12): string {
  if (value.length <= maxLength) return value;
  if (value.startsWith("0x") && value.length > maxLength + 4) {
    const halfLen = Math.floor((maxLength - 2) / 2);
    return `${value.slice(0, 2 + halfLen)}...${value.slice(-halfLen)}`;
  }
  return value.slice(0, maxLength) + "...";
}

// ============================================================================
// CLI Options
// ============================================================================

export interface CliOptions {
  config: string;
  verbose: boolean;
  contract?: string;
  chain?: string;
  skipBytecode: boolean;
  skipAbi: boolean;
  skipState: boolean;
}

export function parseCliArgs(argv: string[]): CliOptions {
  const args = argv.slice(2);
  const options: Partial<CliOptions> = {
    verbose: false,
    skipBytecode: false,
    skipAbi: false,
    skipState: false,
  };

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    const value = args[i + 1];

    switch (arg) {
      case "-c":
      case "--config":
        options.config = value;
        i++;
        break;
      case "-v":
      case "--verbose":
        options.verbose = true;
        break;
      case "--contract":
        options.contract = value;
        i++;
        break;
      case "--chain":
        options.chain = value;
        i++;
        break;
      case "--skip-bytecode":
        options.skipBytecode = true;
        break;
      case "--skip-abi":
        options.skipAbi = true;
        break;
      case "--skip-state":
        options.skipState = true;
        break;
      case "-h":
      case "--help":
        return { ...options, config: "__HELP__" } as CliOptions;
    }
  }

  if (!options.config) {
    return { ...options, config: "__MISSING__" } as CliOptions;
  }

  return options as CliOptions;
}

// ============================================================================
// CLI Runner Configuration
// ============================================================================

export interface CliRunnerConfig {
  /** Package name for display (e.g., "Ethers", "Viem") */
  adapterName: string;
  /** NPX package name */
  packageName: string;
  /** Factory function to create adapter from chain config */
  createAdapter: (chainConfig: ChainConfig) => Web3Adapter;
}

// ============================================================================
// Usage Printer
// ============================================================================

export function printUsage(config: CliRunnerConfig): void {
  console.log(`
Contract Integrity Verifier (${config.adapterName})

Verify smart contract bytecode, ABI, and state against local artifacts.
Uses ${config.adapterName.toLowerCase()} for blockchain interactions.

Usage:
  npx ${config.packageName} -c <config> [options]

Options:
  -c, --config <path>    Path to configuration file (required)
  -v, --verbose          Show detailed output
  --contract <name>      Filter to specific contract
  --chain <name>         Filter to specific chain
  --skip-bytecode        Skip bytecode verification
  --skip-abi             Skip ABI verification
  --skip-state           Skip state verification
  -h, --help             Show this help

Examples:
  npx ${config.packageName} -c ./config.json -v
  npx ${config.packageName} -c ./config.md --contract MyProxy
`);
}

// ============================================================================
// Main CLI Runner
// ============================================================================

export async function runCli(runnerConfig: CliRunnerConfig): Promise<void> {
  const options = parseCliArgs(process.argv);

  // Handle help request
  if (options.config === "__HELP__") {
    printUsage(runnerConfig);
    process.exit(0);
  }

  // Handle missing config
  if (options.config === "__MISSING__") {
    console.error("Error: --config is required\n");
    printUsage(runnerConfig);
    process.exit(1);
  }

  console.log("=".repeat(60));
  console.log(`Contract Integrity Verifier (${runnerConfig.adapterName})`);
  console.log("=".repeat(60));

  // Load config
  const configPath = resolve(options.config);
  console.log(`\nLoading config: ${configPath}`);

  let config;
  try {
    config = loadConfig(configPath);
  } catch (error) {
    console.error(`\nError loading config: ${error instanceof Error ? error.message : error}`);
    process.exit(1);
  }

  console.log(`  Chains: ${Object.keys(config.chains).join(", ")}`);
  console.log(`  Contracts: ${config.contracts.length}`);

  // Create adapters for each chain
  const adapters = new Map<string, Web3Adapter>();

  for (const [chainName, chainConfig] of Object.entries(config.chains)) {
    if (chainConfig.rpcUrl) {
      adapters.set(chainName, runnerConfig.createAdapter(chainConfig));
    }
  }

  // Run verification for each contract
  const results: ContractVerificationResult[] = [];
  let passed = 0;
  let failed = 0;
  let warnings = 0;
  let skipped = 0;

  // Filter contracts if specified
  let contractsToVerify = config.contracts;
  if (options.contract) {
    contractsToVerify = contractsToVerify.filter((c) => c.name.toLowerCase() === options.contract!.toLowerCase());
  }
  if (options.chain) {
    contractsToVerify = contractsToVerify.filter((c) => c.chain.toLowerCase() === options.chain!.toLowerCase());
  }

  if (contractsToVerify.length === 0) {
    console.log("\nNo contracts match the specified filters.");
    process.exit(0);
  }

  console.log(`\nVerifying ${contractsToVerify.length} contract(s)...\n`);

  for (const contract of contractsToVerify) {
    const adapter = adapters.get(contract.chain);
    if (!adapter) {
      console.log(`Skipping ${contract.name}: no adapter for chain '${contract.chain}'`);
      skipped++;
      continue;
    }

    const chain = config.chains[contract.chain];
    const verifier = new Verifier(adapter);

    console.log(`Verifying ${contract.name} on ${contract.chain}...`);
    console.log(`  Address: ${contract.address}`);

    const result = await verifier.verifyContract(
      contract,
      chain,
      {
        verbose: options.verbose,
        skipBytecode: options.skipBytecode,
        skipAbi: options.skipAbi,
        skipState: options.skipState,
      },
      dirname(configPath),
    );

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

        // Show immutable differences in verbose mode
        if (options.verbose && br.immutableDifferences && br.immutableDifferences.length > 0) {
          for (const imm of br.immutableDifferences) {
            const typeInfo = imm.possibleType ? ` (${imm.possibleType})` : "";
            console.log(`    - Position ${imm.position}: ${imm.remoteValue}${typeInfo}`);
          }
        }
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

        // Show individual state checks in verbose mode
        if (options.verbose) {
          printVerboseStateResults(sr);
        }
      }

      // Print immutable values result if present (shows which named immutables matched/failed)
      if (result.immutableValuesResult) {
        const ivr = result.immutableValuesResult;
        const icon = ivr.status === "pass" ? "✓" : ivr.status === "fail" ? "✗" : ivr.status === "warn" ? "!" : "-";
        console.log(`  Immutables: ${icon} ${ivr.message}`);

        // Show individual immutable results in verbose mode, or all failed ones in non-verbose mode
        if (ivr.results && ivr.results.length > 0) {
          const failedImmutables = ivr.results.filter((r) => r.status === "fail");
          if (failedImmutables.length > 0) {
            for (const immResult of failedImmutables) {
              console.log(`    ✗ ${immResult.name}: ${immResult.message}`);
            }
          }
          // Show all immutable results in verbose mode
          if (options.verbose) {
            const passedImmutables = ivr.results.filter((r) => r.status === "pass");
            for (const immResult of passedImmutables) {
              console.log(`    ✓ ${immResult.message}`);
            }
          }
        }
      }

      // Count results (include immutableValuesResult and definitiveResult)
      const bytecodeStatus = result.bytecodeResult?.status;
      const abiStatus = result.abiResult?.status;
      const stateStatus = result.stateResult?.status;
      const immutableValuesStatus = result.immutableValuesResult?.status;
      const definitiveStatus = result.definitiveResult?.status;

      if (
        bytecodeStatus === "fail" ||
        abiStatus === "fail" ||
        stateStatus === "fail" ||
        immutableValuesStatus === "fail" ||
        definitiveStatus === "fail"
      ) {
        failed++;
      } else if (
        bytecodeStatus === "warn" ||
        abiStatus === "warn" ||
        stateStatus === "warn" ||
        immutableValuesStatus === "warn" ||
        definitiveStatus === "warn"
      ) {
        warnings++;
      } else {
        passed++;
      }
    }

    console.log("");
  }

  // Print summary
  printSummary({
    total: contractsToVerify.length,
    passed,
    failed,
    warnings,
    skipped,
    results,
  });

  // Exit with appropriate code
  process.exit(failed > 0 ? 1 : 0);
}

/**
 * Print verbose state verification results.
 */
function printVerboseStateResults(sr: import("./types").StateVerificationResult): void {
  // View call results
  if (sr.viewCallResults) {
    for (const vc of sr.viewCallResults) {
      const vcIcon = vc.status === "pass" ? "✓" : vc.status === "fail" ? "✗" : "!";
      const paramsStr = vc.params?.length ? `(${vc.params.map((p) => truncateValue(String(p))).join(", ")})` : "()";
      if (vc.status === "fail") {
        console.log(
          `    ${vcIcon} ${vc.function}${paramsStr}: expected ${truncateValue(String(vc.expected))}, got ${truncateValue(String(vc.actual))}`,
        );
      } else {
        console.log(`    ${vcIcon} ${vc.function}${paramsStr} = ${truncateValue(String(vc.actual))}`);
      }
    }
  }

  // Slot results
  if (sr.slotResults) {
    for (const slot of sr.slotResults) {
      const slotIcon = slot.status === "pass" ? "✓" : slot.status === "fail" ? "✗" : "!";
      if (slot.status === "fail") {
        console.log(
          `    ${slotIcon} ${slot.name} (${slot.slot}): expected ${truncateValue(String(slot.expected))}, got ${truncateValue(String(slot.actual))}`,
        );
      } else {
        console.log(`    ${slotIcon} ${slot.name} (${slot.slot}) = ${truncateValue(String(slot.actual))}`);
      }
    }
  }

  // Namespace results
  if (sr.namespaceResults) {
    for (const ns of sr.namespaceResults) {
      for (const v of ns.variables) {
        const vIcon = v.status === "pass" ? "✓" : v.status === "fail" ? "✗" : "!";
        if (v.status === "fail") {
          console.log(
            `    ${vIcon} ${ns.namespaceId}:${v.name}: expected ${truncateValue(String(v.expected))}, got ${truncateValue(String(v.actual))}`,
          );
        } else {
          console.log(`    ${vIcon} ${ns.namespaceId}:${v.name} = ${truncateValue(String(v.actual))}`);
        }
      }
    }
  }

  // Storage path results
  if (sr.storagePathResults) {
    for (const sp of sr.storagePathResults) {
      const spIcon = sp.status === "pass" ? "✓" : sp.status === "fail" ? "✗" : "!";
      if (sp.status === "fail") {
        console.log(
          `    ${spIcon} ${sp.path}: expected ${truncateValue(String(sp.expected))}, got ${truncateValue(String(sp.actual))}`,
        );
      } else {
        console.log(`    ${spIcon} ${sp.path} = ${truncateValue(String(sp.actual))}`);
      }
    }
  }
}
