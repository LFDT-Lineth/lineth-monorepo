/**
 * Performs safe subtraction of two bigint values, preventing negative results.
 * If the result would be negative, returns 0 instead.
 *
 * @param {bigint} a - The minuend (value to subtract from).
 * @param {bigint} b - The subtrahend (value to subtract).
 * @returns {bigint} The result of a - b if a > b, otherwise 0n.
 */
export function safeSub(a: bigint, b: bigint): bigint {
  return a > b ? a - b : 0n;
}

/**
 * Returns the minimum of two bigint values.
 *
 * @param {bigint} a - The first value to compare.
 * @param {bigint} b - The second value to compare.
 * @returns {bigint} The smaller of the two values.
 */
export function min(a: bigint, b: bigint): bigint {
  return a < b ? a : b;
}

/**
 * Returns the absolute difference between two bigint values.
 *
 * @param {bigint} a - The first value.
 * @param {bigint} b - The second value.
 * @returns {bigint} The absolute difference between a and b (always non-negative).
 */
export function absDiff(a: bigint, b: bigint): bigint {
  return a > b ? a - b : b - a;
}
