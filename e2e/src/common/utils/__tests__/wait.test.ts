jest.mock("@lfdt-lineth/shared-utils", () => ({
  wait: jest.fn(() => Promise.resolve()),
}));

jest.mock("../../../config/logger/logger", () => ({
  createTestLogger: () => ({ debug: jest.fn(), error: jest.fn(), warn: jest.fn() }),
}));

import { wait } from "@lfdt-lineth/shared-utils";

import { awaitUntil, AwaitUntilTimeoutError } from "../wait";

const waitMock = wait as jest.MockedFunction<typeof wait>;

describe("awaitUntil", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    waitMock.mockResolvedValue(undefined);
  });

  describe("basic behaviour (no shouldRetry)", () => {
    it("returns the result immediately when the callback succeeds on the first try", async () => {
      const callback = jest.fn().mockResolvedValue("ok");

      const result = await awaitUntil(callback, () => true, { timeoutMs: 1000 });

      expect(result).toBe("ok");
      expect(callback).toHaveBeenCalledTimes(1);
    });

    it("retries until stopRetry returns true", async () => {
      const callback = jest
        .fn()
        .mockResolvedValueOnce("not-yet")
        .mockResolvedValueOnce("not-yet")
        .mockResolvedValue("done");

      const result = await awaitUntil(callback, (v) => v === "done", { timeoutMs: 5000, pollingIntervalMs: 0 });

      expect(result).toBe("done");
      expect(callback).toHaveBeenCalledTimes(3);
    });

    it("retries on any error when shouldRetry is not provided", async () => {
      const error = new Error("transient");
      const callback = jest.fn().mockRejectedValueOnce(error).mockResolvedValue("ok");

      const result = await awaitUntil(callback, () => true, { timeoutMs: 5000, pollingIntervalMs: 0 });

      expect(result).toBe("ok");
      expect(callback).toHaveBeenCalledTimes(2);
    });

    it("throws AwaitUntilTimeoutError after the deadline", async () => {
      const callback = jest.fn().mockRejectedValue(new Error("always fails"));

      await expect(awaitUntil(callback, () => true, { timeoutMs: 0, pollingIntervalMs: 0 })).rejects.toBeInstanceOf(
        AwaitUntilTimeoutError,
      );
    });
  });

  describe("shouldRetry predicate", () => {
    it("retries when shouldRetry returns true", async () => {
      const transientError = new Error("transient");
      const callback = jest.fn().mockRejectedValueOnce(transientError).mockResolvedValue("ok");
      const shouldRetry = jest.fn().mockReturnValue(true);

      const result = await awaitUntil(callback, () => true, {
        timeoutMs: 5000,
        pollingIntervalMs: 0,
        shouldRetry,
      });

      expect(result).toBe("ok");
      expect(shouldRetry).toHaveBeenCalledWith(transientError);
      expect(callback).toHaveBeenCalledTimes(2);
    });

    it("rethrows immediately when shouldRetry returns false", async () => {
      const fatalError = new Error("non-transient");
      const callback = jest.fn().mockRejectedValue(fatalError);
      const shouldRetry = jest.fn().mockReturnValue(false);

      await expect(
        awaitUntil(callback, () => true, { timeoutMs: 5000, pollingIntervalMs: 0, shouldRetry }),
      ).rejects.toThrow(fatalError);

      expect(callback).toHaveBeenCalledTimes(1);
      expect(shouldRetry).toHaveBeenCalledWith(fatalError);
    });

    it("rethrows a non-retryable error before the deadline is reached", async () => {
      const transientError = new Error("transient");
      const fatalError = new Error("fatal");
      const callback = jest
        .fn()
        .mockRejectedValueOnce(transientError)
        .mockRejectedValueOnce(fatalError)
        .mockResolvedValue("should not reach");
      const shouldRetry = (error: unknown) => (error as Error).message === "transient";

      await expect(
        awaitUntil(callback, () => true, { timeoutMs: 5000, pollingIntervalMs: 0, shouldRetry }),
      ).rejects.toThrow(fatalError);

      expect(callback).toHaveBeenCalledTimes(2);
    });

    it("throws AwaitUntilTimeoutError when retryable errors persist past the deadline", async () => {
      const transientError = new Error("transient");
      const callback = jest.fn().mockRejectedValue(transientError);
      const shouldRetry = jest.fn().mockReturnValue(true);

      await expect(
        awaitUntil(callback, () => true, { timeoutMs: 0, pollingIntervalMs: 0, shouldRetry }),
      ).rejects.toBeInstanceOf(AwaitUntilTimeoutError);
    });
  });
});
