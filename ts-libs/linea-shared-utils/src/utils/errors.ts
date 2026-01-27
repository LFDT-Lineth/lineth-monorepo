import { ResultAsync } from "neverthrow";

import { ILogger } from "../logging/ILogger";

/**
 * @notice Wraps a potentially-throwing function (sync or async) into a `ResultAsync`.
 * @dev Useful for making code resilient to thrown errors, enabling Go-style error handling.
 *
 * Example:
 * const result = await tryResult(() => mightThrowAsync());
 * if (result.isErr()) return handle(result.error);
 * doSomething(result.value);
 *
 * @param fn A function that may throw or reject.
 * @returns A `ResultAsync<T, Error>` that resolves to `ok(value)` or `err(error)`.
 */
export const tryResult = <T>(fn: () => Promise<T> | T): ResultAsync<T, Error> => {
  return ResultAsync.fromPromise(Promise.resolve().then(fn), (e) => (e instanceof Error ? e : new Error(String(e))));
};

/**
 * @notice Attempts to execute a fn but does not throw on failure.
 * @dev Intended for operations where errors should be logged and tolerated rather than propagated.
 * @dev Wraps the call in a `ResultAsync` via {@link tryResult}, automatically logging a warning if an error occurs.
 *      This is useful for fault-tolerant operations where failures should be logged but not thrown.
 *
 * Example:
 * ```ts
 * const result = await attempt(logger, () => mightThrowAsync(), "Failed to execute task");
 * if (result.isErr()) return handle(result.error);
 * doSomething(result.value);
 * ```
 *
 * @param logger The logger instance used to record warnings when the operation fails.
 * @param fn A function that may throw or reject.
 * @param msg A message to include in the warning log if an error occurs.
 * @returns A `ResultAsync<T, Error>` that resolves to `ok(value)` or `err(error)`.
 */
export const attempt = <T>(logger: ILogger, fn: () => Promise<T> | T, msg: string): ResultAsync<T, Error> =>
  tryResult(fn).mapErr((error) => {
    logger.warn(msg, { error });
    return error;
  });
