import { constants } from "node:os";

import { createTestLogger } from "../logger";
import { globalTeardown } from "./global-teardown";
import { ensureOnceOffPrerequisites } from "./prerequisites";
import { startL2TrafficGeneration } from "./traffic";
import { setStopL2TrafficGeneration } from "./traffic-state";
import { createTestContext } from "../setup";

const logger = createTestLogger();

let shutdownAlreadyTried = false;
async function handleForcedShutdown(signal: NodeJS.Signals) {
  // In case of a forced shutdown, prevent another cleanup being run
  if (shutdownAlreadyTried) {
    process.exit(constants.signals[signal] ?? 1);
  }

  logger.info("Forcefully shutting down, stopping background processes...");
  shutdownAlreadyTried = true;

  try {
    await globalTeardown();
  } catch (error) {
    logger.error(`Failed to run teardown successfully, ${error}`);
  } finally {
    process.exit(constants.signals[signal] ?? 1);
  }
}

export default async (): Promise<void> => {
  const context = createTestContext();

  await ensureOnceOffPrerequisites(context, logger);

  logger.info("Generating L2 traffic...");
  const stopPolling = await startL2TrafficGeneration(context, { pollingIntervalMs: 5_000 });
  logger.info("L2 traffic generation started.");

  setStopL2TrafficGeneration(stopPolling);

  process.on("SIGINT", (signal) => handleForcedShutdown(signal));
  process.on("SIGTERM", (signal) => handleForcedShutdown(signal));
};
