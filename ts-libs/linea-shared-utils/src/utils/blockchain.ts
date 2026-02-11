import {
  isAddress,
  parseEther,
  encodeAbiParameters,
  encodeFunctionData,
  encodePacked,
  keccak256,
  EncodeFunctionDataParameters,
  getAddress,
} from "viem";

import { WEI_PER_GWEI, SLOTS_PER_EPOCH } from "../core/constants/blockchain";

/**
 * Converts ether to wei.
 * @param amount - Value in ether as a string (e.g., "1.5").
 * @returns Value in wei as bigint.
 */
export function etherToWei(amount: string): bigint {
  return parseEther(amount);
}

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

/**
 * Encodes an Ethereum function call to its ABI representation.
 * @notice Returns the ABI encoded payload for a function call.
 * @param params ABI encoding parameters including function signature and arguments.
 * @return The ABI encoded call data as a hex string.
 */
export function encodeFunctionCall(params: EncodeFunctionDataParameters) {
  return encodeFunctionData(params);
}

/**
 * Computes the keccak256 hash of provided ABI values.
 * @notice Returns the keccak256 of the ABI-encoded or packed-encoded parameters.
 * @param types The ABI types (as strings) for the values.
 * @param values The ABI values corresponding to the provided types.
 * @param packed If true, performs packed encoding; otherwise, standard ABI encoding.
 * @return The keccak256 hash as a hex string.
 */
export function generateKeccak256(types: string[], values: unknown[], packed?: boolean) {
  return keccak256(encodeData(types, values, packed));
}

/**
 * ABI-encodes or packed-encodes input values according to specified types.
 * @notice Returns the ABI-encoded or packed-encoded bytes for the input fields.
 * @param types ABI types (as strings) to encode.
 * @param values Values to encode per type specification.
 * @param packed If true, use packed encoding.
 * @return The encoded bytes as a hex string.
 */
export function encodeData(types: string[], values: unknown[], packed?: boolean) {
  if (packed) {
    return encodePacked(types, values);
  }
  const params = types.map((type) => ({ type }));
  return encodeAbiParameters(params, values);
}

/**
 * Normalizes a given Ethereum address to EIP-55 checksum format.
 * @notice Returns the checksummed (mixed-case) address string.
 * @param address The Ethereum address to normalize.
 * @return The checksummed Ethereum address.
 */
export function normalizeAddress(address: string) {
  return getAddress(address);
}
