/**
 * Test data factories for creating mock objects in unit tests.
 *
 * All factories accept Partial<T> overrides to allow customization
 * while providing sensible defaults for testing.
 */

import { IRetryService } from "../../core/services/IRetryService";
import { ILogger } from "../../logging/ILogger";

/**
 * Creates a mock logger for testing.
 *
 * @param overrides - Optional partial overrides for logger properties
 * @returns Mocked ILogger with all methods stubbed
 *
 * @example
 * const logger = createLoggerMock();
 * const logger = createLoggerMock({ name: "custom-logger" });
 */
export const createLoggerMock = (overrides: Partial<jest.Mocked<ILogger>> = {}): jest.Mocked<ILogger> => {
  const mock: jest.Mocked<ILogger> = {
    name: "test-logger",
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  };
  return { ...mock, ...overrides };
};

/**
 * Creates a mock retry service for testing.
 *
 * By default, the retry method immediately executes the provided function
 * without actual retry logic (passthrough behavior).
 *
 * @param overrides - Optional partial overrides for retry service methods
 * @returns Mocked IRetryService
 *
 * @example
 * const retryService = createRetryServiceMock();
 * const retryService = createRetryServiceMock({
 *   retry: jest.fn().mockRejectedValue(new Error("Retry failed"))
 * });
 */
export const createRetryServiceMock = (
  overrides: Partial<jest.Mocked<IRetryService>> = {},
): jest.Mocked<IRetryService> => {
  const mock: jest.Mocked<IRetryService> = {
    retry: jest.fn(async (fn) => fn()) as jest.MockedFunction<IRetryService["retry"]>,
  };
  return { ...mock, ...overrides };
};
