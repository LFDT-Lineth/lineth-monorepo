/**
 * Manual integration runner for LidoAccountingReportClient.
 *
 * Example usage:
 * RPC_URL=https://mainnet.infura.io/v3/YOUR_KEY \
 * PRIVATE_KEY=0xabc123... \
 * LAZY_ORACLE_ADDRESS=0x... \
 * LIDO_VAULT_ADDRESS=0x... \
 * IPFS_BASE_URL=https://gateway.ipfs.io/ipfs \
 * pnpm --filter @consensys/linea-native-yield-automation-service exec tsx scripts/test-lido-accounting-report-client.ts
 *
 * Optional flags:
 * POLL_INTERVAL_MS=60000 \
 * SKIP_SIMULATION=true \
 * SUBMIT_LATEST_REPORT=true \
 */

import {
  ExponentialBackoffRetryService,
  ViemBlockchainClientAdapter,
  ViemWalletSignerClientAdapter,
  WinstonLogger,
} from "@consensys/linea-shared-utils";
import { LidoAccountingReportClient } from "../src/clients/LidoAccountingReportClient.js";
import { LazyOracleContractClient } from "../src/clients/contracts/LazyOracleContractClient.js";
import { Address, Hex } from "viem";
import { hoodi } from "viem/chains";

async function main() {
  const requiredEnvVars = ["RPC_URL", "PRIVATE_KEY", "LAZY_ORACLE_ADDRESS", "LIDO_VAULT_ADDRESS", "IPFS_BASE_URL"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const rpcUrl = process.env.RPC_URL as string;
  const privateKey = process.env.PRIVATE_KEY as Hex;
  const lazyOracleAddress = process.env.LAZY_ORACLE_ADDRESS as Address;
  const lidoVaultAddress = process.env.LIDO_VAULT_ADDRESS as Address;
  const ipfsGatewayUrl = process.env.IPFS_BASE_URL as string;
  const pollIntervalMs = Number.parseInt(process.env.POLL_INTERVAL_MS ?? "60000", 10);

  const signer = new ViemWalletSignerClientAdapter(
    new WinstonLogger("ViemWalletSignerClientAdapter.integration", { level: "debug" }),
    rpcUrl,
    privateKey,
    hoodi,
  );
  const contractClientLibrary = new ViemBlockchainClientAdapter(
    new WinstonLogger("ViemBlockchainClientAdapter.integration", { level: "debug" }),
    rpcUrl,
    hoodi,
    signer,
  );

  const lazyOracleClient = new LazyOracleContractClient(
    new WinstonLogger("LazyOracleContractClient.integration", { level: "debug" }),
    contractClientLibrary,
    lazyOracleAddress,
    pollIntervalMs,
    300_000,
  );

  const retryService = new ExponentialBackoffRetryService(
    new WinstonLogger(ExponentialBackoffRetryService.name, { level: "debug" }),
  );
  const lidoAccountingClient = new LidoAccountingReportClient(
    new WinstonLogger("LidoAccountingReportClient.integration", { level: "debug" }),
    retryService,
    lazyOracleClient,
    ipfsGatewayUrl,
  );

  try {
    console.log("Fetching latest vault report parameters...");
    const params = await lidoAccountingClient.getLatestSubmitVaultReportParams(lidoVaultAddress);
    console.log("Latest updateVaultData params:");
    console.log({
      ...params,
      totalValue: params.totalValue.toString(),
      cumulativeLidoFees: params.cumulativeLidoFees.toString(),
      liabilityShares: params.liabilityShares.toString(),
      maxLiabilityShares: params.maxLiabilityShares.toString(),
      slashingReserve: params.slashingReserve.toString(),
    });

    if (process.env.SUBMIT_LATEST_REPORT === "true") {
      console.log("Submitting latest vault report...");
      await lidoAccountingClient.submitLatestVaultReport(lidoVaultAddress);
      console.log("Submission transaction sent ✔️");
    } else {
      console.log("Submission skipped. Set SUBMIT_LATEST_REPORT=true to send the transaction.");
    }
  } catch (err) {
    console.error("LidoAccountingReportClient integration script failed:", err);
    process.exitCode = 1;
  }
}

void main();
