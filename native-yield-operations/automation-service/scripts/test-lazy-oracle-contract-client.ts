/**
 * Manual integration runner for LidoAccountingReportClient.
 *
 * Example usage:
 * RPC_URL=https://mainnet.infura.io/v3/YOUR_KEY \
 * PRIVATE_KEY=0xabc123... \
 * LAZY_ORACLE_ADDRESS=0x... \
 * LIDO_VAULT_ADDRESS=0x... \
 * pnpm --filter @consensys/linea-native-yield-automation-service exec tsx scripts/test-lazy-oracle-contract-client.ts
 *
 * Optional flags:
 * POLL_INTERVAL_MS=60000 \
 * WAIT_TIMEOUT_MS=300000 \
 */

import {
  ViemBlockchainClientAdapter,
  ViemWalletSignerClientAdapter,
  WinstonLogger,
} from "@consensys/linea-shared-utils";
import { LazyOracleContractClient } from "../src/clients/contracts/LazyOracleContractClient.js";
import { Address, Hex } from "viem";
import { hoodi } from "viem/chains";

async function main() {
  const requiredEnvVars = ["RPC_URL", "PRIVATE_KEY", "LAZY_ORACLE_ADDRESS"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const rpcUrl = process.env.RPC_URL as string;
  const privateKey = process.env.PRIVATE_KEY as Hex;
  const lazyOracleAddress = process.env.LAZY_ORACLE_ADDRESS as Address;
  const pollIntervalMs = Number.parseInt(process.env.POLL_INTERVAL_MS ?? "60000", 10);
  const waitTimeoutMs = Number.parseInt(process.env.WAIT_TIMEOUT_MS ?? "300000", 10);

  const signer = new ViemWalletSignerClientAdapter(
    new WinstonLogger("ViemWalletSignerClientAdapter.integration"),
    rpcUrl,
    privateKey,
    hoodi,
  );
  const contractClientLibrary = new ViemBlockchainClientAdapter(
    new WinstonLogger("ViemBlockchainClientAdapter.integration"),
    rpcUrl,
    hoodi,
    signer,
  );

  const lazyOracleClient = new LazyOracleContractClient(
    new WinstonLogger("LazyOracleContractClient.integration"),
    contractClientLibrary,
    lazyOracleAddress,
    pollIntervalMs,
    waitTimeoutMs,
  );

  try {
    await lazyOracleClient.waitForVaultsReportDataUpdatedEvent();
  } catch (err) {
    console.error("LazyOracleContractClient integration script failed:", err);
    process.exitCode = 1;
  }
}

void main();
