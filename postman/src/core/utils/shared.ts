/**
 * Subtracts a specified number of seconds from a given date.
 *
 * @param {Date} date - The original date.
 * @param {number} seconds - The number of seconds to subtract from the date.
 * @returns {Date} A new date object representing the time after subtracting the specified seconds.
 */
export const subtractSeconds = (date: Date, seconds: number): Date => {
  const dateCopy = new Date(date);
  dateCopy.setSeconds(date.getSeconds() - seconds);
  return dateCopy;
};

/**
 * Stringifier that handles bigint values.
 *
 * @param {unknown} value - Value to stringify.
 * @returns {string} The stringified output.
 */
export function serialize(value: unknown): string {
  return JSON.stringify(value, (_, v: unknown) => (typeof v === "bigint" ? v.toString() : v));
}

/**
 * Checks if a given bytes string is empty or represents an empty value.
 *
 * @param {string} bytes - The bytes string to check.
 * @returns {boolean} `true` if the bytes string is empty or "0x".
 */
export function isEmptyBytes(bytes: string): boolean {
  return bytes === "0x" || bytes === "";
}

/**
 * Returns a promise that resolves after the given number of milliseconds.
 *
 * @param {number} ms - The number of milliseconds to wait.
 * @returns {Promise<void>}
 */
export function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
