import { wait } from "@consensys/linea-shared-utils";

import { createTestLogger } from "../../config/logger/logger";

const logger = createTestLogger();

export class AwaitUntilTimeoutError extends Error {
  constructor(
    public readonly timeoutMs: number,
    public readonly lastError?: Error,
  ) {
    const message = [`awaitUntil timed out after ${timeoutMs}ms`, lastError && `error=${lastError.message}`]
      .filter(Boolean)
      .join(". ");
    super(message);
    this.name = "AwaitUntilTimeoutError";
  }
}

export type AwaitUntilOptions = {
  pollingIntervalMs?: number;
  timeoutMs?: number;
};

export async function awaitUntil<T>(
  callback: () => Promise<T>,
  stopRetry: (value: T) => boolean,
  options: AwaitUntilOptions = {},
): Promise<T> {
  const { pollingIntervalMs = 500, timeoutMs = 2 * 60 * 1000 } = options;
  const deadline = Date.now() + timeoutMs;
  let lastError: Error | undefined;
  let attemptCount = 0;

  while (Date.now() < deadline) {
    try {
      const result = await callback();
      lastError = undefined;

      if (stopRetry(result)) {
        return result;
      }
    } catch (error) {
      lastError = error as Error;
      attemptCount++;

      logger.debug(
        `awaitUntil callback error (attempt ${attemptCount}). Will retry until timeout. error=${lastError.message}`,
      );
    }

    await wait(pollingIntervalMs);
  }

  throw new AwaitUntilTimeoutError(timeoutMs, lastError);
}
