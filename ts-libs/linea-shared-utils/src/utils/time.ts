import { MS_PER_SECOND } from "../core/constants/time";

/**
 * Gets the current Unix timestamp in seconds.
 * @returns The current Unix timestamp as a number of seconds since the Unix epoch (January 1, 1970 UTC), floored.
 */
export function getCurrentUnixTimestampSeconds(): number {
  return Math.floor(Date.now() / 1000);
}

/**
 * Converts milliseconds to whole seconds (rounded down).
 * @param ms - Milliseconds value
 * @returns Number of seconds, floored
 */
export function msToSeconds(ms: number): number {
  return Math.floor(ms / MS_PER_SECOND);
}

/**
 * Creates a promise that resolves after a specified timeout period.
 *
 * @param {number} timeout - The duration in milliseconds to wait before resolving the promise.
 * @returns {Promise<void>} A promise that resolves after the specified timeout period.
 */
export const wait = (timeout: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, timeout));
