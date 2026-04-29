import { TokenBridgeInitializationData } from "contracts/scripts/tokenBridge/test/deployTokenBridges";
import { ethers, AbiCoder } from "ethers";

export const encodeData = (types: string[], values: unknown[], packed?: boolean) => {
  if (packed) {
    return ethers.solidityPacked(types, values);
  }
  return AbiCoder.defaultAbiCoder().encode(types, values);
};

/**
 * Serializes TokenBridgeInitializationData to the positional array format
 * that ethers v6 `toArray()` produces from parsed event structs.
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
