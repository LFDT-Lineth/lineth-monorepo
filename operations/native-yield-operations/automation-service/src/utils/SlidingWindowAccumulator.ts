import { ISlidingWindowAccumulator } from "../core/utils/ISlidingWindowAccumulator.js";

/**
 * Implementation of a sliding window accumulator using a circular buffer.
 * Maintains a running total of values added within a fixed-size rolling window.
 *
 * When values are added via `push()`, they are stored in a buffer. Once the buffer
 * reaches the window size, adding a new value automatically removes the oldest
 * value (FIFO behavior), maintaining a rolling window of the most recent values.
 *
 * @remarks
 * - Setting window size to 0 disables the accumulator (all operations become no-ops)
 * - Useful for tracking cumulative values over time windows (e.g., quotas, rate limits)
 * - Efficient O(1) operations for push, getTotal, and getLength
 */
export class SlidingWindowAccumulator implements ISlidingWindowAccumulator {
  private readonly size: number;
  private readonly arr: bigint[] = [];
  private total: bigint = 0n;

  /**
   * Creates a new SlidingWindowAccumulator instance.
   *
   * @param size - The maximum number of values to keep in the sliding window.
   *               Set to 0 to disable the accumulator (all operations become no-ops).
   */
  constructor(size: number) {
    this.size = size;
  }

  /**
   * Adds a value to the accumulator and updates the running total.
   * If the window is full, removes the oldest value before adding the new one.
   * If window size is 0, this is a no-op.
   *
   * @param value - The value to add to the accumulator.
   */
  push(value: bigint): void {
    // If size is 0, accumulator is disabled - no-op
    if (this.size === 0) {
      return;
    }

    this.arr.push(value);
    this.total += value;

    if (this.arr.length > this.size) {
      const old = this.arr.shift();
      if (old !== undefined) {
        this.total -= old;
      }
    }
  }

  /**
   * Gets the current total of all values in the sliding window.
   *
   * @returns The sum of all values currently in the window.
   */
  getTotal(): bigint {
    return this.total;
  }

  /**
   * Gets the current number of values in the accumulator.
   * This will be at most equal to the window size.
   *
   * @returns The number of values currently stored.
   */
  getLength(): number {
    return this.arr.length;
  }
}
