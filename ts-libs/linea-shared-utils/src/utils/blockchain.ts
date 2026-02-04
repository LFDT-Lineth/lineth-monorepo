import { WEI_PER_GWEI, SLOTS_PER_EPOCH } from "../core/constants/blockchain";
import { isAddress } from "viem";

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

/**
 * Converts an Ethereum address to 0x02 withdrawal credentials format.
 * @param address - Ethereum address with "0x" prefix (e.g., "0x2101af8b812b529fc303c976b6dd747618cfdadb").
 * @returns Withdrawal credentials in format "0x020000000000000000000000{address}".
 * @throws Error if the input is not a valid Ethereum address.
 */
export function get0x02WithdrawalCredentials(address: string): string {
  // Normalize to lowercase for validation (isAddress is case-sensitive)
  const normalizedAddress = address.toLowerCase();
  if (!isAddress(normalizedAddress)) {
    throw new Error(`Invalid Ethereum address: ${address}`);
  }

  const addressWithoutPrefix = normalizedAddress.slice(2); // Remove "0x" prefix
  return `0x020000000000000000000000${addressWithoutPrefix}`;
}

/**
 * Converts a slot number to an epoch number (rounded down).
 * @param slot - Slot number.
 * @returns Epoch number.
 */
export function slotToEpoch(slot: number): number {
  return Math.floor(slot / SLOTS_PER_EPOCH);
}
