#!/usr/bin/env node
/**
 * Contract Integrity Verifier CLI (Ethers)
 *
 * Command-line interface using ethers.js adapter.
 *
 * Usage:
 *   npx @consensys/linea-contract-integrity-verifier-ethers -c config.json -v
 */

import { resolve, dirname } from "path";
import {
  loadConfig,
  Verifier,
  printSummary,
  type ContractVerificationResult,
} from "@consensys/linea-contract-integrity-verifier";
import { EthersAdapter } from "./index";

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Truncate long values for display (e.g., addresses, hashes)
 */
function truncateValue(value: string, maxLength: number = 12): string {
  if (value.length <= maxLength) return value;
  if (value.startsWith("0x") && value.length > maxLength + 4) {
    const halfLen = Math.floor((maxLength - 2) / 2);
    return `${value.slice(0, 2 + halfLen)}...${value.slice(-halfLen)}`;
  }
  return value.slice(0, maxLength) + "...";
}

// ============================================================================
// CLI Argument Parsing
// ============================================================================

interface CliOptions {
  config: string;
  verbose: boolean;
  contract?: string;
  chain?: string;
  skipBytecode: boolean;
  skipAbi: boolean;
  skipState: boolean;
}

function parseArgs(): CliOptions {
  const args = process.argv.slice(2);
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
        printUsage();
        process.exit(0);
    }
  }

  if (!options.config) {
    console.error("Error: --config is required\n");
    printUsage();
    process.exit(1);
  }

  return options as CliOptions;
}

function printUsage(): void {
  console.log(`
Contract Integrity Verifier (Ethers)

Verify smart contract bytecode, ABI, and state against local artifacts.
Uses ethers.js for blockchain interactions.

Usage:
  npx @consensys/linea-contract-integrity-verifier-ethers -c <config> [options]

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
  npx @consensys/linea-contract-integrity-verifier-ethers -c ./config.json -v
  npx @consensys/linea-contract-integrity-verifier-ethers -c ./config.md --contract MyProxy
`);
}

// ============================================================================
// Main
// ============================================================================

async function main(): Promise<void> {
  const options = parseArgs();

  console.log("=".repeat(60));
  console.log("Contract Integrity Verifier (Ethers)");
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
  const adapters = new Map<string, EthersAdapter>();

  for (const [chainName, chainConfig] of Object.entries(config.chains)) {
    if (chainConfig.rpcUrl) {
      adapters.set(chainName, new EthersAdapter({ rpcUrl: chainConfig.rpcUrl, chainId: chainConfig.chainId }));
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
          // View call results
          if (sr.viewCallResults) {
            for (const vc of sr.viewCallResults) {
              const vcIcon = vc.status === "pass" ? "✓" : vc.status === "fail" ? "✗" : "!";
              const paramsStr = vc.params?.length
                ? `(${vc.params.map((p) => truncateValue(String(p))).join(", ")})`
                : "()";
              console.log(`    ${vcIcon} ${vc.function}${paramsStr}: ${vc.function}() = ${truncateValue(String(vc.actual))}`);
            }
          }

          // Slot results
          if (sr.slotResults) {
            for (const slot of sr.slotResults) {
              const slotIcon = slot.status === "pass" ? "✓" : slot.status === "fail" ? "✗" : "!";
              console.log(`    ${slotIcon} ${slot.name} (${slot.slot}): ${slot.name} = ${truncateValue(String(slot.actual))}`);
            }
          }

          // Namespace results
          if (sr.namespaceResults) {
            for (const ns of sr.namespaceResults) {
              for (const v of ns.variables) {
                const vIcon = v.status === "pass" ? "✓" : v.status === "fail" ? "✗" : "!";
                console.log(`    ${vIcon} ${ns.namespaceId}:${v.name}: ${v.name} = ${truncateValue(String(v.actual))}`);
              }
            }
          }

          // Storage path results
          if (sr.storagePathResults) {
            for (const sp of sr.storagePathResults) {
              const spIcon = sp.status === "pass" ? "✓" : sp.status === "fail" ? "✗" : "!";
              console.log(`    ${spIcon} ${sp.path}: ${sp.path} = ${truncateValue(String(sp.actual))}`);
            }
          }
        }
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

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
