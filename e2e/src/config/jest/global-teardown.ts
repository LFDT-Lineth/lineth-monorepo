import { stopL2TrafficGeneration } from "./traffic-state";
import { createTestLogger } from "../logger";

const logger = createTestLogger();

export default async (): Promise<void> => {
  try {
    await stopL2TrafficGeneration();
    logger.debug("Stopped L2 traffic generation");
  } catch (error) {
    logger.error(`Error stopping L2 traffic generation: ${error}`);
    // Don't throw - teardown failures shouldn't mask test failures
  }
};
