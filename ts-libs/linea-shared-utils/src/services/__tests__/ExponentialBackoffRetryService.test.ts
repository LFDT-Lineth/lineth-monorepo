jest.mock("../../utils/time", () => ({
  wait: jest.fn(() => Promise.resolve()),
}));

import { wait } from "../../utils/time";
import { ILogger } from "../../logging/ILogger";
import { ExponentialBackoffRetryService } from "../ExponentialBackoffRetryService";

const waitMock = wait as jest.MockedFunction<typeof wait>;

const createLogger = (): jest.Mocked<ILogger> =>
  ({
    name: "test",
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  }) as unknown as jest.Mocked<ILogger>;

describe("ExponentialBackoffRetryService", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    waitMock.mockResolvedValue(undefined);
  });

  it("throws when instantiated with maxRetryAttempts less than 1", () => {
    const logger = createLogger();
    expect(() => new ExponentialBackoffRetryService(logger, 0)).toThrow("maxRetryAttempts must be at least 1");
  });

  it("throws when instantiated with a negative base delay", () => {
    const logger = createLogger();
    expect(() => new ExponentialBackoffRetryService(logger, 3, -1)).toThrow("baseDelay must be non-negative");
  });

  it("resolves immediately when the operation succeeds on the first attempt", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger);
    const fn = jest.fn().mockResolvedValue("ok");

    await expect(service.retry(fn)).resolves.toBe("ok");

    expect(fn).toHaveBeenCalledTimes(1);
    expect(logger.warn).not.toHaveBeenCalled();
    expect(logger.debug).not.toHaveBeenCalled();
    expect(logger.error).not.toHaveBeenCalled();
    expect(waitMock).not.toHaveBeenCalled();
  });

  it("retries after failures and succeeds when a later attempt resolves", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger, 3, 100);
    const error = new Error("boom");
    const fn = jest.fn().mockRejectedValueOnce(error).mockResolvedValueOnce("ok");
    const randomSpy = jest.spyOn(Math, "random").mockReturnValue(0);

    await expect(service.retry(fn)).resolves.toBe("ok");

    expect(fn).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith("Retry attempt failed attempt=1 maxRetryAttempts=3", { error });
    expect(logger.debug).toHaveBeenCalledWith("Retrying after delay=100ms");
    expect(waitMock).toHaveBeenCalledWith(100);

    randomSpy.mockRestore();
  });

  it("propagates the last error when retry attempts are exhausted", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger, 2, 50);
    const error = new Error("still failing");
    const fn = jest.fn().mockRejectedValue(error);
    const randomSpy = jest.spyOn(Math, "random").mockReturnValue(0);

    await expect(service.retry(fn)).rejects.toBe(error);

    expect(fn).toHaveBeenCalledTimes(2);
    expect(logger.warn).toHaveBeenCalledWith("Retry attempt failed attempt=1 maxRetryAttempts=2", { error });
    expect(logger.error).toHaveBeenCalledWith("Retry attempts exhausted maxRetryAttempts=2", { error });
    expect(logger.debug).toHaveBeenCalledWith("Retrying after delay=50ms");
    expect(waitMock).toHaveBeenCalledWith(50);

    randomSpy.mockRestore();
  });

  it("throws when provided timeout is not greater than zero", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger);
    const fn = jest.fn().mockResolvedValue(undefined);

    await expect(service.retry(fn, 0)).rejects.toThrow("timeoutMs must be greater than 0");
  });

  it("does not perform retry side effects when timeout validation fails", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger, 2, 25);
    const fn = jest.fn<Promise<void>, []>(() => Promise.resolve());

    await expect(service.retry(fn, -5)).rejects.toThrow("timeoutMs must be greater than 0");

    expect(fn).not.toHaveBeenCalled();
    expect(logger.warn).not.toHaveBeenCalled();
    expect(logger.debug).not.toHaveBeenCalled();
    expect(logger.error).not.toHaveBeenCalled();
    expect(waitMock).not.toHaveBeenCalled();
  });

  it("uses the provided timeout value and times out when the operation does not resolve in time", async () => {
    jest.useFakeTimers();
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger, 1, 10);
    const fn = jest.fn(
      () =>
        new Promise<void>(() => {
          /* never resolves */
        }),
    );

    const promise = service.retry(fn, 50);

    jest.advanceTimersByTime(50);

    await expect(promise).rejects.toThrow("mockConstructor timed out after 50ms");
    expect(logger.error).toHaveBeenCalledTimes(1);
    expect(logger.error.mock.calls[0][0]).toBe("Retry attempts exhausted maxRetryAttempts=1");
    expect(logger.error.mock.calls[0][1].error).toBeInstanceOf(Error);
    expect(logger.warn).not.toHaveBeenCalled();

    jest.useRealTimers();
  });

  it("throws when executeWithTimeout receives a non-positive timeout", async () => {
    const logger = createLogger();
    const service = new ExponentialBackoffRetryService(logger);
    const fn = jest.fn().mockResolvedValue("ok");

    const executeWithTimeout = (service as any).executeWithTimeout.bind(service);

    expect(() => executeWithTimeout(fn, 0)).toThrow("timeoutMs must be greater than 0");
    expect(fn).not.toHaveBeenCalled();
  });
});
