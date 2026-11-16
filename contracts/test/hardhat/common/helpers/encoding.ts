import { getAddress } from "viem";

import type { TokenBridgeInitializationData } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";

/**
 * Serializes TokenBridgeInitializationData to the positional array format used by the event assertions
 * (structs compare positionally, in ABI order, as viem decodes them: checksummed addresses, `uint8` as numbers).
 *
 * Struct field order in Solidity:
 * 1. defaultAdmin, 2. messageService, 3. tokenBeacon, 4. sourceChainId,
 * 5. targetChainId, 6. remoteSender, 7. reservedTokens, 8. roleAddresses,
 * 9. pauseTypeRoles, 10. unpauseTypeRoles
 */
export function serializeTokenBridgeInitData(data: TokenBridgeInitializationData): unknown[] {
  return [
    getAddress(data.defaultAdmin),
    getAddress(data.messageService),
    getAddress(data.tokenBeacon),
    BigInt(data.sourceChainId),
    BigInt(data.targetChainId),
    getAddress(data.remoteSender),
    data.reservedTokens.map((token) => getAddress(token)),
    data.roleAddresses.map((r) => [getAddress(r.addressWithRole), r.role]),
    data.pauseTypeRoles.map((r) => [Number(r.pauseType), r.role]),
    data.unpauseTypeRoles.map((r) => [Number(r.pauseType), r.role]),
  ];
}
