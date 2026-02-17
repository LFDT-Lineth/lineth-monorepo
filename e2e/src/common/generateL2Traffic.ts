import { etherToWei } from "@consensys/linea-shared-utils";
import * as dotenv from "dotenv";

import { sendTransactionsToGenerateTrafficWithInterval } from "./utils";
import { createTestLogger } from "../config/logger";
import { createTestContext } from "../config/setup";

dotenv.config();

const logger = createTestLogger();

async function main() {
  logger.info("Generating L2 traffic...");

  const context = createTestContext();
  const pollingAccount = await context.getL2AccountManager().generateAccount(etherToWei("200"));
  const walletClient = context.l2WalletClient({ account: pollingAccount });
  const publicClient = context.l2PublicClient();

  const stopPolling = await sendTransactionsToGenerateTrafficWithInterval(walletClient, publicClient, {
    pollingInterval: 2_500,
  });

  process.on("SIGINT", () => {
    logger.info("Caught interrupt signal.");
    cleanup();
  });

  process.on("SIGTERM", () => {
    logger.info("Caught termination signal.");
    cleanup();
  });

  async function cleanup() {
    await stopPolling();
    logger.info("Terminated L2 traffic...");
    process.exit(0);
  }
}

main().catch((error) => {
  logger.error("Error generating L2 traffic", error);
  process.exit(1);
});
