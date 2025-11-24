import { WEI_PER_GWEI } from "../core/constants/blockchain";

/**
 * Converts wei to gwei (rounded down).
 * @param wei - Value in wei.
 * @returns Value in gwei as bigint.
 */
export function weiToGwei(wei: bigint): bigint {
  return wei / WEI_PER_GWEI;
}

/**
 * Converts wei to gwei (rounded down).
 * @param wei - Value in wei.
 * @returns Value in gwei as number - safely store up to ~9M ETH.
 */
export function weiToGweiNumber(wei: bigint): number {
  return Number(weiToGwei(wei));
}

/**
 * Converts gwei to wei.
 * @param gwei - Value in gwei.
 * @returns Value in wei as bigint.
 */
export function gweiToWei(gwei: bigint): bigint {
  return gwei * WEI_PER_GWEI;
}
