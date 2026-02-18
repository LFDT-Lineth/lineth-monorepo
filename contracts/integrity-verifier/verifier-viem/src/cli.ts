#!/usr/bin/env node
/**
 * Contract Integrity Verifier CLI (Viem)
 *
 * Command-line interface using viem adapter.
 *
 * Usage:
 *   npx @consensys/linea-contract-integrity-verifier-viem -c config.json -v
 */

import { runCli, type CliRunnerConfig } from "@consensys/linea-contract-integrity-verifier";

import { ViemAdapter } from "./index";

const config: CliRunnerConfig = {
  adapterName: "Viem",
  packageName: "@consensys/linea-contract-integrity-verifier-viem",
  createAdapter: (chainConfig) => new ViemAdapter({ rpcUrl: chainConfig.rpcUrl }),
};

runCli(config).catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
