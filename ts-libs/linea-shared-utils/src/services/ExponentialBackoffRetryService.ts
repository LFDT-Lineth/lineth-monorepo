import { IRetryService } from "../core/services/IRetryService";
import { ILogger } from "../logging/ILogger";
import { wait } from "../utils/time";

/**
 * A retry service that implements exponential backoff with jitter for retrying failed operations.
 * Retries are performed with increasing delays between attempts, with randomization to prevent thundering herd problems.
 *
 * @example
 * ```ts
 * const retryService = new ExponentialBackoffRetryService(logger, 3, 1000, 10000);
 * const result = await retryService.retry(() => someAsyncOperation());
 * ```
 */
export class ExponentialBackoffRetryService implements IRetryService {
  /**
   * Creates a new ExponentialBackoffRetryService instance.
   *
   * @param {ILogger} logger - The logger instance for logging retry attempts and errors.
   * @param {number} [maxRetryAttempts=3] - Maximum number of retry attempts (must be at least 1).
   * @param {number} [baseDelayMs=1000] - Base delay in milliseconds for exponential backoff (must be non-negative).
   * @param {number} [defaultTimeoutMs=10000] - Default timeout in milliseconds for each operation attempt.
   * @throws {Error} If maxRetryAttempts is less than 1 or baseDelayMs is negative.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly maxRetryAttempts: number = 3,
    private readonly baseDelayMs: number = 1000,
    private readonly defaultTimeoutMs: number = 10000,
  ) {
    if (maxRetryAttempts < 1) {
      throw new Error("maxRetryAttempts must be at least 1");
    }
    if (baseDelayMs < 0) {
      throw new Error("baseDelay must be non-negative");
    }
  }

  /**
   * Retries an asynchronous operation with exponential backoff until success or max attempts are reached.
   * Each retry attempt uses a timeout, and failed attempts are retried with increasing delays.
   *
   * @param {() => Promise<TReturn>} fn - An async callback returning a promise of type TReturn.
   * @param {number} [timeoutMs] - Optional timeout in milliseconds for each attempt. Defaults to defaultTimeoutMs.
   * @returns {Promise<TReturn>} The result of the successful operation.
   * @throws {Error} If timeoutMs is provided and is less than or equal to 0.
   * @throws {Error} If all retry attempts are exhausted, throws the last encountered error.
   */
  public async retry<TReturn>(fn: () => Promise<TReturn>, timeoutMs?: number): Promise<TReturn> {
    const effectiveTimeoutMs = timeoutMs ?? this.defaultTimeoutMs;

    if (effectiveTimeoutMs <= 0) {
      throw new Error("timeoutMs must be greater than 0");
    }

    let lastError: unknown;

    for (let attempt = 1; attempt <= this.maxRetryAttempts; attempt += 1) {
      try {
        return await this.executeWithTimeout(fn, effectiveTimeoutMs);
      } catch (error) {
        if (attempt >= this.maxRetryAttempts) {
          this.logger.error(`Retry attempts exhausted maxRetryAttempts=${this.maxRetryAttempts}`, { error });
          throw error;
        }

        lastError = error;
        this.logger.warn(`Retry attempt failed attempt=${attempt} maxRetryAttempts=${this.maxRetryAttempts}`, {
          error,
        });

        const delayMs = this.getDelayMs(attempt);
        this.logger.debug(`Retrying after delay=${delayMs}ms`);
        await wait(delayMs);
      }
    }

    // Unreachable, but required to simplify TS return type.
    throw lastError;
  }

  /**
   * Executes an async function with a timeout, rejecting if the operation takes longer than the specified timeout.
   *
   * @param {() => Promise<TReturn>} fn - The async function to execute.
   * @param {number} timeoutMs - Timeout in milliseconds (must be greater than 0).
   * @returns {Promise<TReturn>} The result of the function if it completes within the timeout.
   * @throws {Error} If timeoutMs is less than or equal to 0.
   * @throws {Error} If the operation times out after timeoutMs milliseconds.
   */
  private executeWithTimeout<TReturn>(fn: () => Promise<TReturn>, timeoutMs: number): Promise<TReturn> {
    if (timeoutMs <= 0) {
      throw new Error("timeoutMs must be greater than 0");
    }

    return new Promise<TReturn>((resolve, reject) => {
      const timeoutHandle = setTimeout(() => {
        reject(new Error(`${fn.name} timed out after ${timeoutMs}ms`));
      }, timeoutMs);

      fn()
        .then(resolve)
        .catch(reject)
        .finally(() => clearTimeout(timeoutHandle));
    });
  }

  /**
   * Calculates the delay in milliseconds for a given retry attempt using exponential backoff with jitter.
   * The delay doubles with each attempt (2^(attempt-1) * baseDelayMs) and includes random jitter
   * to prevent synchronized retries across multiple instances.
   *
   * @param {number} attempt - The current attempt number (1-indexed).
   * @returns {number} The delay in milliseconds before the next retry attempt.
   */
  private getDelayMs(attempt: number): number {
    const exponentialDelay = this.baseDelayMs * 2 ** (attempt - 1);
    const jitter = Math.random() * exponentialDelay;
    return exponentialDelay + jitter;
  }
}
