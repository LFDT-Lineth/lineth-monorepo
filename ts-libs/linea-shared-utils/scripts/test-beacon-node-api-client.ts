// pnpm --filter @consensys/linea-shared-utils exec tsx scripts/test-beacon-node-api-client.ts

import { ExponentialBackoffRetryService } from "../src";
import { BeaconNodeApiClient } from "../src/clients/BeaconNodeApiClient";
import { WinstonLogger } from "../src/logging/WinstonLogger";

async function main() {
  const rpcUrl = process.env.BEACON_NODE_RPC_URL;

  if (!rpcUrl) {
    console.error("Missing required env var: BEACON_NODE_RPC_URL");
    process.exitCode = 1;
    return;
  }

  const retryService = new ExponentialBackoffRetryService(new WinstonLogger(ExponentialBackoffRetryService.name));
  const client = new BeaconNodeApiClient(new WinstonLogger("BeaconNodeApiClient.integration"), retryService, rpcUrl);

  console.log(`Fetching pending partial withdrawals from ${rpcUrl}...`);
  try {
    const withdrawals = await client.getPendingPartialWithdrawals();
    if (!withdrawals) {
      throw "undefined withdrawals";
    }
    console.log(`Received ${withdrawals.length} withdrawals.`);
    withdrawals.forEach((withdrawal, index) => {
      console.log(`Withdrawal ${index + 1}:`, withdrawal);
    });
  } catch (err) {
    console.error("BeaconNodeApiClient integration script failed:", err);
    process.exitCode = 1;
  }

  console.log(`Fetching pending deposits from ${rpcUrl}...`);
  try {
    const deposits = await client.getPendingDeposits();
    if (!deposits) {
      throw "undefined deposits";
    }
    console.log(`Received ${deposits.length} deposits.`);
    deposits.forEach((deposit, index) => {
      console.log(`Deposit ${index + 1}:`, deposit);
    });
  } catch (err) {
    console.error("BeaconNodeApiClient integration script failed:", err);
    process.exitCode = 1;
  }

  console.log(`Fetching current epoch from ${rpcUrl}...`);
  try {
    const epoch = await client.getCurrentEpoch();
    if (epoch === undefined) {
      throw "undefined epoch";
    }
    console.log(`Current epoch: ${epoch}`);
  } catch (err) {
    console.error("BeaconNodeApiClient integration script failed:", err);
    process.exitCode = 1;
  }
}

main();
