/**
 * Replacer function for JSON.stringify that converts bigint values to strings.
 *
 * @param {string} key - The property key (unused).
 * @param {unknown} value - The value to check and potentially convert.
 * @returns {unknown} The value as a string if it's a bigint, otherwise the original value.
 */
export function bigintReplacer(key: string, value: unknown) {
  return typeof value === "bigint" ? value.toString() : value;
}

/**
 * Serializes a value to a JSON string, converting bigint values to strings.
 *
 * @param {unknown} value - The value to serialize.
 * @returns {string} A JSON string representation of the value with bigints converted to strings.
 */
export function serialize(value: unknown): string {
  return JSON.stringify(value, (_, value: unknown) => (typeof value === "bigint" ? value.toString() : value));
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
