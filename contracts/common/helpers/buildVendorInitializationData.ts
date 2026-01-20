import { AbiCoder, Wallet } from "ethers";

export const buildVendorInitializationData = (
  overrides: Partial<{
    defaultAdmin: string;
    nodeOperator: string;
    nodeOperatorManager: string;
    nodeOperatorFeeBP: bigint;
    confirmExpiry: bigint;
    roleAssignments: { addressWithRole: string; role: string }[];
  }> = {},
) => {
  const params = {
    defaultAdmin: overrides.defaultAdmin ?? Wallet.createRandom().address,
    nodeOperator: overrides.nodeOperator ?? Wallet.createRandom().address,
    nodeOperatorManager: overrides.nodeOperatorManager ?? Wallet.createRandom().address,
    nodeOperatorFeeBP: overrides.nodeOperatorFeeBP ?? 0n,
    confirmExpiry: overrides.confirmExpiry ?? 0n,
    roleAssignments: overrides.roleAssignments ?? [],
  };

  return AbiCoder.defaultAbiCoder().encode(
    ["address", "address", "address", "uint256", "uint256", "tuple(address addressWithRole, bytes32 role)[]"],
    [
      params.defaultAdmin,
      params.nodeOperator,
      params.nodeOperatorManager,
      params.nodeOperatorFeeBP,
      params.confirmExpiry,
      params.roleAssignments.map(({ addressWithRole, role }) => [addressWithRole, role]),
    ],
  );
};
