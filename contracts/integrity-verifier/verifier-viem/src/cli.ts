#!/usr/bin/env node
/**
 * Contract Integrity Verifier CLI (Viem)
 *
 * Command-line interface using viem adapter.
 *
 * Usage:
 *   pnpx @lfdt-lineth/contract-integrity-verifier-viem -c config.json -v
 */

import { runCli, type CliRunnerConfig } from "@lfdt-lineth/contract-integrity-verifier";

import { ViemAdapter } from "./index";

const config: CliRunnerConfig = {
  adapterName: "Viem",
  packageName: "@lfdt-lineth/contract-integrity-verifier-viem",
  createAdapter: (chainConfig) => new ViemAdapter({ rpcUrl: chainConfig.rpcUrl }),
};

runCli(config).catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
