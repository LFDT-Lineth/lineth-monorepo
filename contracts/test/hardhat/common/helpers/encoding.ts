import { type Hex, encodeAbiParameters, encodePacked, parseAbiParameters } from "viem";

import type { TokenBridgeInitializationData } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";

/**
 * ABI-encodes `values` for the given Solidity `types` (e.g. `["uint256", "bytes32"]`), or tightly packs them when
 * `packed` is set. Replaces ethers' `AbiCoder.encode` / `solidityPacked`. Integers must be `bigint` or `number`.
 */
export const encodeData = (types: string[], values: unknown[], packed?: boolean): Hex => {
  if (packed) {
    return encodePacked(types, values);
  }
  return encodeAbiParameters(parseAbiParameters(types.join(", ")), values);
};

/**
 * Serializes TokenBridgeInitializationData to the positional array format used by the event assertions
 * (structs compare positionally, in ABI order).
 *
 * Struct field order in Solidity:
 * 1. defaultAdmin, 2. messageService, 3. tokenBeacon, 4. sourceChainId,
 * 5. targetChainId, 6. remoteSender, 7. reservedTokens, 8. roleAddresses,
 * 9. pauseTypeRoles, 10. unpauseTypeRoles
 */
export function serializeTokenBridgeInitData(data: TokenBridgeInitializationData): unknown[] {
  return [
    data.defaultAdmin,
    data.messageService,
    data.tokenBeacon,
    BigInt(data.sourceChainId),
    BigInt(data.targetChainId),
    data.remoteSender,
    data.reservedTokens,
    data.roleAddresses.map((r) => [r.addressWithRole, r.role]),
    data.pauseTypeRoles.map((r) => [BigInt(r.pauseType), r.role]),
    data.unpauseTypeRoles.map((r) => [BigInt(r.pauseType), r.role]),
  ];
}
