import { config } from "./../config/tests-config";
import { etherToWei, sendTransactionsToGenerateTrafficWithInterval } from "./utils";
import * as dotenv from "dotenv";

dotenv.config();

async function main() {
  console.log("Generating L2 traffic...");

  const pollingAccount = await config.getL2AccountManager().generateAccount(etherToWei("200"));
  const stopPolling = await sendTransactionsToGenerateTrafficWithInterval(pollingAccount, 2_500);

  process.on("SIGINT", () => {
    console.log("Caught interrupt signal.");
    cleanup();
  });

  process.on("SIGTERM", () => {
    console.log("Caught termination signal.");
    cleanup();
  });

  function cleanup() {
    stopPolling();
    console.log("Terminated L2 traffic...");
    process.exit(0);
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
