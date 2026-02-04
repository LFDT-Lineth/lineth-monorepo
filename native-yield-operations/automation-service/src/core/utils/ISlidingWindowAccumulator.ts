/**
 * Interface for a sliding window accumulator that maintains a running total
 * of values over a fixed-size rolling window.
 *
 * Values are added via `push()`, and when the window is full, the oldest value
 * is automatically removed (FIFO behavior). The accumulator maintains the
 * sum of all values currently within the window.
 *
 * @remarks
 * - Setting window size to 0 disables the accumulator (all operations become no-ops)
 * - The accumulator uses a circular buffer internally for efficient operations
 * - Useful for tracking cumulative values over time windows (e.g., quotas, rate limits)
 */
export interface ISlidingWindowAccumulator {
  /**
   * Adds a value to the accumulator and updates the running total.
   * If the window is full, removes the oldest value before adding the new one.
   * If window size is 0, this is a no-op.
   *
   * @param value - The value to add to the accumulator.
   */
  push(value: bigint): void;

  /**
   * Gets the current total of all values in the sliding window.
   *
   * @returns The sum of all values currently in the window.
   */
  getTotal(): bigint;

  /**
   * Gets the current number of values in the accumulator.
   * This will be at most equal to the window size.
   *
   * @returns The number of values currently stored.
   */
  getLength(): number;
}
