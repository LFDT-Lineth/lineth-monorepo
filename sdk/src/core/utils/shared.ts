/**
 * Creates a promise that resolves after a specified timeout period.
 *
 * @param {number} timeout - The duration in milliseconds to wait before resolving the promise.
 * @returns {Promise<void>} A promise that resolves after the specified timeout period.
 */
export const wait = (timeout: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, timeout));

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
 * Checks if a given bytes string is empty or represents a zero value.
 *
 * @param {string} bytes - The bytes string to check.
 * @returns {boolean} `true` if the bytes string is empty, represents a zero value, or is otherwise considered "empty", `false` otherwise.
 */
export function isEmptyBytes(bytes: string): boolean {
  if (!bytes || bytes === "0x" || bytes === "" || bytes === "0") {
    return true;
  }

  const hexString = bytes.replace(/^0x/, "");
  return /^00*$/.test(hexString);
}

/**
 * Type guard function to check if a given value is a string.
 *
 * @param {unknown} value - The value to check.
 * @returns {boolean} `true` if the value is a string, `false` otherwise.
 */
export function isString(value: unknown): value is string {
  return typeof value === "string";
}

/**
 * Type guard function to check if a given object is `undefined`.
 *
 * @param {unknown} obj - The object to check.
 * @returns {boolean} `true` if the object is undefined, `false` otherwise.
 */
export const isUndefined = (obj: unknown): obj is undefined => typeof obj === "undefined";

/**
 * Type guard function to check if a given value is `null`.
 *
 * @param {unknown} val - The value to check.
 * @returns {boolean} `true` if the value is `null`, `false` otherwise.
 */
export const isNull = (val: unknown): val is null => val === null;
