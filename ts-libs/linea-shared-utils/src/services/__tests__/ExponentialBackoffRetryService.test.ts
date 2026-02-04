jest.mock("../../utils/time", () => ({
  wait: jest.fn(() => Promise.resolve()),
}));

import { createLoggerMock } from "../../__tests__/helpers/factories";
import { wait } from "../../utils/time";
import { ExponentialBackoffRetryService } from "../ExponentialBackoffRetryService";

const waitMock = wait as jest.MockedFunction<typeof wait>;

describe("ExponentialBackoffRetryService", () => {
  // Constants
  const MIN_RETRY_ATTEMPTS = 1;
  const DEFAULT_MAX_RETRIES = 3;
  const BASE_DELAY_MS = 100;
  const SHORT_DELAY_MS = 50;
  const TIMEOUT_MS = 50;
  const JITTER_ZERO = 0;

  beforeEach(() => {
    jest.clearAllMocks();
    waitMock.mockResolvedValue(undefined);
  });

  describe("constructor validation", () => {
    it("throw error when maxRetryAttempts is less than 1", () => {
      // Arrange
      const logger = createLoggerMock();
      const invalidMaxRetries = 0;

      // Act & Assert
      expect(() => new ExponentialBackoffRetryService(logger, invalidMaxRetries)).toThrow(
        "maxRetryAttempts must be at least 1",
      );
    });

    it("throw error when baseDelay is negative", () => {
      // Arrange
      const logger = createLoggerMock();
      const negativeDelay = -1;

      // Act & Assert
      expect(() => new ExponentialBackoffRetryService(logger, DEFAULT_MAX_RETRIES, negativeDelay)).toThrow(
        "baseDelay must be non-negative",
      );
    });
  });

  describe("retry - successful operations", () => {
    it("resolve immediately when operation succeeds on first attempt", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger);
      const expectedResult = "ok";
      const operation = jest.fn().mockResolvedValue(expectedResult);

      // Act
      const result = await service.retry(operation);

      // Assert
      expect(result).toBe(expectedResult);
      expect(operation).toHaveBeenCalledTimes(1);
      expect(waitMock).not.toHaveBeenCalled();
    });

    it("retry after failure and succeed on subsequent attempt", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger, DEFAULT_MAX_RETRIES, BASE_DELAY_MS);
      const error = new Error("boom");
      const expectedResult = "ok";
      const operation = jest.fn().mockRejectedValueOnce(error).mockResolvedValueOnce(expectedResult);
      const randomSpy = jest.spyOn(Math, "random").mockReturnValue(JITTER_ZERO);

      // Act
      const result = await service.retry(operation);

      // Assert
      expect(result).toBe(expectedResult);
      expect(operation).toHaveBeenCalledTimes(2);
      expect(logger.warn).toHaveBeenCalledWith(
        `Retry attempt failed attempt=1 maxRetryAttempts=${DEFAULT_MAX_RETRIES}`,
        { error },
      );
      expect(waitMock).toHaveBeenCalledWith(BASE_DELAY_MS);

      randomSpy.mockRestore();
    });
  });

  describe("retry - exhausted attempts", () => {
    it("propagate final error when all retry attempts are exhausted", async () => {
      // Arrange
      const logger = createLoggerMock();
      const maxRetries = 2;
      const service = new ExponentialBackoffRetryService(logger, maxRetries, SHORT_DELAY_MS);
      const error = new Error("still failing");
      const operation = jest.fn().mockRejectedValue(error);
      const randomSpy = jest.spyOn(Math, "random").mockReturnValue(JITTER_ZERO);

      // Act & Assert
      await expect(service.retry(operation)).rejects.toBe(error);
      expect(operation).toHaveBeenCalledTimes(maxRetries);
      expect(logger.error).toHaveBeenCalledWith(`Retry attempts exhausted maxRetryAttempts=${maxRetries}`, { error });
      expect(waitMock).toHaveBeenCalledWith(SHORT_DELAY_MS);

      randomSpy.mockRestore();
    });
  });

  describe("retry - timeout validation", () => {
    it("throw error when timeout is zero", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger);
      const operation = jest.fn().mockResolvedValue(undefined);
      const invalidTimeout = 0;

      // Act & Assert
      await expect(service.retry(operation, invalidTimeout)).rejects.toThrow("timeoutMs must be greater than 0");
    });

    it("not execute operation when timeout validation fails", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger, 2, 25);
      const operation = jest.fn<Promise<void>, []>(() => Promise.resolve());
      const invalidTimeout = -5;

      // Act & Assert
      await expect(service.retry(operation, invalidTimeout)).rejects.toThrow("timeoutMs must be greater than 0");
      expect(operation).not.toHaveBeenCalled();
      expect(waitMock).not.toHaveBeenCalled();
    });
  });

  describe("retry - timeout behavior", () => {
    beforeEach(() => {
      jest.useFakeTimers();
    });

    afterEach(() => {
      jest.useRealTimers();
    });

    it("timeout when operation does not resolve within specified time", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger, MIN_RETRY_ATTEMPTS, 10);
      const operation = jest.fn(
        () =>
          new Promise<void>(() => {
            /* never resolves */
          }),
      );

      // Act
      const promise = service.retry(operation, TIMEOUT_MS);
      jest.advanceTimersByTime(TIMEOUT_MS);

      // Assert
      await expect(promise).rejects.toThrow(`mockConstructor timed out after ${TIMEOUT_MS}ms`);
      expect(logger.error).toHaveBeenCalledWith(
        `Retry attempts exhausted maxRetryAttempts=${MIN_RETRY_ATTEMPTS}`,
        expect.objectContaining({
          error: expect.any(Error),
        }),
      );
    });
  });

  describe("executeWithTimeout - private method validation", () => {
    it("throw error when timeout is not positive", async () => {
      // Arrange
      const logger = createLoggerMock();
      const service = new ExponentialBackoffRetryService(logger);
      const operation = jest.fn().mockResolvedValue("ok");
      const executeWithTimeout = (service as any).executeWithTimeout.bind(service);
      const invalidTimeout = 0;

      // Act & Assert
      expect(() => executeWithTimeout(operation, invalidTimeout)).toThrow("timeoutMs must be greater than 0");
      expect(operation).not.toHaveBeenCalled();
    });
  });
});
