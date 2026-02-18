#!/usr/bin/env node
/**
 * Contract Integrity Verifier CLI (Ethers)
 *
 * Command-line interface using ethers.js adapter.
 *
 * Usage:
 *   npx @consensys/linea-contract-integrity-verifier-ethers -c config.json -v
 */

import { runCli, type CliRunnerConfig } from "@consensys/linea-contract-integrity-verifier";

import { EthersAdapter } from "./index";

const config: CliRunnerConfig = {
  adapterName: "Ethers",
  packageName: "@consensys/linea-contract-integrity-verifier-ethers",
  createAdapter: (chainConfig) => new EthersAdapter({ rpcUrl: chainConfig.rpcUrl, chainId: chainConfig.chainId }),
};

runCli(config).catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
