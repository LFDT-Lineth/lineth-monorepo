import { randomBytes } from "node:crypto";
import { type Address, type Hex, bytesToHex, getAddress, isHex } from "viem";

export const range = (start: number, end: number) => Array.from(Array(end - start + 1).keys()).map((x) => x + start);

export const generateRandomBytes = (length: number): Hex => bytesToHex(randomBytes(length));

export function buildAccessErrorMessage(account: { address: string }, role: string): string {
  return `AccessControl: account ${account.address.toLowerCase()} is missing role ${role}`;
}

/** Validates a `0x`-prefixed hex string coming from untyped data (JSON fixtures, env) and types it as `Hex`. */
export function hex(value: string): Hex {
  if (!isHex(value)) {
    throw new Error(`Expected a 0x-prefixed hex string, got: ${value}`);
  }
  return value;
}

/** Validates an address coming from untyped data and returns it checksummed. */
export function address(value: string): Address {
  return getAddress(value);
}
