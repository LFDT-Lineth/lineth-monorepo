import { type Address, type Hex, encodeAbiParameters, parseAbiParameters } from "viem";
import { generatePrivateKey, privateKeyToAddress } from "viem/accounts";

const randomAddress = () => privateKeyToAddress(generatePrivateKey());

export const buildVendorInitializationData = (
  overrides: Partial<{
    defaultAdmin: Address;
    nodeOperator: Address;
    nodeOperatorManager: Address;
    nodeOperatorFeeBP: bigint;
    confirmExpiry: bigint;
    roleAssignments: { addressWithRole: Address; role: Hex }[];
  }> = {},
): Hex => {
  const params = {
    defaultAdmin: overrides.defaultAdmin ?? randomAddress(),
    nodeOperator: overrides.nodeOperator ?? randomAddress(),
    nodeOperatorManager: overrides.nodeOperatorManager ?? randomAddress(),
    nodeOperatorFeeBP: overrides.nodeOperatorFeeBP ?? 0n,
    confirmExpiry: overrides.confirmExpiry ?? 0n,
    roleAssignments: overrides.roleAssignments ?? [],
  };

  return encodeAbiParameters(
    parseAbiParameters("address, address, address, uint256, uint256, (address addressWithRole, bytes32 role)[]"),
    [
      params.defaultAdmin,
      params.nodeOperator,
      params.nodeOperatorManager,
      params.nodeOperatorFeeBP,
      params.confirmExpiry,
      params.roleAssignments,
    ],
  );
};
