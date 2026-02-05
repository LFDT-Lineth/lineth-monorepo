/**
 * Manual integration runner for YieldManagerContractClient.
 *
 * Example usage:
 * RPC_URL=https://0xrpc.io/hoodi \
 * PRIVATE_KEY=0xabc123... \
 * YIELD_MANAGER_ADDRESS=0x... \
 * pnpm --filter @consensys/linea-native-yield-automation-service exec tsx scripts/test-yield-manager-contract-client.ts
 *
 * Optional flags:
 * REBALANCE_TOLERANCE_AMOUNT_WEI=1000000000000000000 \
 * MIN_WITHDRAWAL_THRESHOLD_ETH=0 \
 */

import {
  ViemBlockchainClientAdapter,
  ViemWalletSignerClientAdapter,
  WinstonLogger,
} from "@consensys/linea-shared-utils";
import { YieldManagerContractClient } from "../src/clients/contracts/YieldManagerContractClient.js";
import { Address, Hex } from "viem";
import { hoodi } from "viem/chains";
import { IRebalanceQuotaService } from "../src/core/services/IRebalanceQuotaService.js";
import { RebalanceDirection } from "../src/core/entities/RebalanceRequirement.js";

async function main() {
  const requiredEnvVars = ["RPC_URL", "PRIVATE_KEY", "YIELD_MANAGER_ADDRESS"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const rpcUrl = process.env.RPC_URL as string;
  const privateKey = process.env.PRIVATE_KEY as Hex;
  const yieldManagerAddress = process.env.YIELD_MANAGER_ADDRESS as Address;
  const rebalanceToleranceAmountWei = BigInt(process.env.REBALANCE_TOLERANCE_AMOUNT_WEI ?? "1000000000000000000");
  const minWithdrawalThresholdEth = BigInt(process.env.MIN_WITHDRAWAL_THRESHOLD_ETH ?? "0");

  // Create a simple stub implementation of IRebalanceQuotaService that passes through amounts without quota enforcement
  const rebalanceQuotaService: IRebalanceQuotaService = {
    getRebalanceAmountAfterQuota: (_vaultAddress: Address, _totalSystemBalance: bigint, reBalanceAmountWei: bigint) =>
      reBalanceAmountWei,
    getStakingDirection: () => RebalanceDirection.STAKE,
  };

  const signer = new ViemWalletSignerClientAdapter(
    new WinstonLogger("ViemWalletSignerClientAdapter.integration"),
    rpcUrl,
    privateKey,
    hoodi,
  );
  const contractClientLibrary = new ViemBlockchainClientAdapter(
    new WinstonLogger("ViemBlockchainClientAdapter.integration"),
    signer,
    hoodi,
    rpcUrl,
  );

  const yieldManagerClient = new YieldManagerContractClient(
    new WinstonLogger("YieldManagerContractClient.integration"),
    contractClientLibrary,
    yieldManagerAddress,
    rebalanceToleranceAmountWei,
    minWithdrawalThresholdEth,
    rebalanceQuotaService,
    undefined, // metricsUpdater is optional
  );

  try {
    await yieldManagerClient.peekYieldReport(
      "0x000000000000000000000000000000000000dEaD",
      "0x000000000000000000000000000000000000dEaD",
    );
  } catch (err) {
    console.error("YieldManagerContractClient integration script failed:", err);
    process.exitCode = 1;
  }
}

void main();
