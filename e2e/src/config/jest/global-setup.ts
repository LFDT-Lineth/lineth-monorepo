import { createTestLogger } from "../logger";
import { ensureOnceOffPrerequisites } from "./prerequisites";
import { startL2TrafficGeneration } from "./traffic";
import { setStopL2TrafficGeneration } from "./traffic-state";
import { createTestContext } from "../setup";

const logger = createTestLogger();

export default async (): Promise<void> => {
  const context = createTestContext();

  await ensureOnceOffPrerequisites(context, logger);

  logger.info("Generating L2 traffic...");
  const stopPolling = await startL2TrafficGeneration(context, { pollingIntervalMs: 5_000 });
  logger.info("L2 traffic generation started.");

  setStopL2TrafficGeneration(stopPolling);
};
