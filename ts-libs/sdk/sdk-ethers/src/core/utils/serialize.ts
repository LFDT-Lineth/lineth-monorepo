/**
 * Stringifier that handles bigint value.
 * @param {any} value Value to stringify.
 * @returns {string} the stringified output.
 */
export function serialize(value: unknown): string {
  return JSON.stringify(value, (_, value: unknown) => (typeof value === "bigint" ? value.toString() : value));
}
