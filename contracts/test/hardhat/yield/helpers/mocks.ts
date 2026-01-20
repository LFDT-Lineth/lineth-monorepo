import { ethers } from "hardhat";
import { YieldProviderRegistration } from "./types";
import { deployMockWithdrawTarget, deployMockYieldProvider } from "./deploy";
import { TestYieldManager } from "contracts/typechain-types";
import { getAccountsFixture } from "../../common/helpers";
import { EMPTY_CALLDATA } from "../../common/constants";
import { YieldProviderVendor } from "../../common/constants";

export const buildMockYieldProviderRegistration = (
  overrides: Partial<{
    yieldProviderVendor: number;
    primaryEntrypoint: string;
    ossifiedEntrypoint: string;
    usersFundsIncrement: bigint;
  }> = {},
): YieldProviderRegistration => ({
  yieldProviderVendor:
    overrides.yieldProviderVendor ?? Math.random() < 0.5
      ? YieldProviderVendor.UNUSED_YIELD_PROVIDER_VENDOR
      : YieldProviderVendor.LIDO_ST_VAULT_YIELD_PROVIDER_VENDOR,
  primaryEntrypoint: overrides.primaryEntrypoint ?? ethers.Wallet.createRandom().address,
  ossifiedEntrypoint: overrides.ossifiedEntrypoint ?? ethers.Wallet.createRandom().address,
  usersFundsIncrement: overrides.usersFundsIncrement ?? 0n,
});

export const buildVendorExitData = (
  overrides: Partial<{
    newVaultOwner: string;
  }> = {},
) => {
  const params = {
    newVaultOwner: overrides.newVaultOwner ?? ethers.Wallet.createRandom().address,
  };

  return ethers.AbiCoder.defaultAbiCoder().encode(["address"], [params.newVaultOwner]);
};

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
    defaultAdmin: overrides.defaultAdmin ?? ethers.Wallet.createRandom().address,
    nodeOperator: overrides.nodeOperator ?? ethers.Wallet.createRandom().address,
    nodeOperatorManager: overrides.nodeOperatorManager ?? ethers.Wallet.createRandom().address,
    nodeOperatorFeeBP: overrides.nodeOperatorFeeBP ?? 0n,
    confirmExpiry: overrides.confirmExpiry ?? 0n,
    roleAssignments: overrides.roleAssignments ?? [],
  };

  return ethers.AbiCoder.defaultAbiCoder().encode(
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

export const addMockYieldProvider = async (yieldManager: TestYieldManager) => {
  const { securityCouncil } = await getAccountsFixture();
  const mockYieldProvider = await deployMockYieldProvider();
  const mockYieldProviderAddress = await mockYieldProvider.getAddress();
  const mockRegistration = buildMockYieldProviderRegistration();
  await yieldManager.connect(securityCouncil).addYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA);
  const mockWithdrawTarget = await deployMockWithdrawTarget();
  const mockWithdrawTargetAddress = await mockWithdrawTarget.getAddress();
  await yieldManager
    .connect(securityCouncil)
    .setMockWithdrawTarget(mockYieldProviderAddress, mockWithdrawTargetAddress);
  return { mockWithdrawTarget, mockYieldProvider, mockYieldProviderAddress, mockRegistration };
};
