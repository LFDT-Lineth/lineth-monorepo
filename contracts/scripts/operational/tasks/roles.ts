import { type Hex, isHex, size } from "viem";

/** Splits a comma-separated `CONTRACT_ROLES` value into bytes32 role ids, rejecting malformed entries. */
export function parseRoles(contractRoles: string): Hex[] {
  return contractRoles.split(",").map((role) => {
    const trimmed = role.trim();
    if (!isHex(trimmed, { strict: true }) || size(trimmed) !== 32) {
      throw new Error(`Invalid role "${role}": expected a 0x-prefixed bytes32 value.`);
    }
    return trimmed;
  });
}
